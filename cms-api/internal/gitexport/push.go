package gitexport

import (
	"context"
	"encoding/base64"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/deployment"
	"gorm.io/gorm"
)

type gitObject struct {
	SHA string `json:"sha"`
}
type gitRef struct {
	Object gitObject `json:"object"`
}
type gitCommit struct {
	SHA  string    `json:"sha"`
	Tree gitObject `json:"tree"`
}
type treeEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
}
type gitTree struct {
	SHA  string      `json:"sha"`
	Tree []treeEntry `json:"tree"`
}

func (g GitHub) head(ctx context.Context, token string, t Task) (string, error) {
	var out gitRef
	err := g.request(ctx, token, "GET", "/repos/"+t.Repository+"/git/ref/heads/"+url.PathEscape(t.Branch), nil, &out)
	var remote remoteError
	if errors.As(err, &remote) && (remote.Status == 404 || remote.Status == 409) {
		return "", nil
	}
	return out.Object.SHA, err
}
func (g GitHub) contains(ctx context.Context, token string, t Task, sha string) (bool, error) {
	head, err := g.head(ctx, token, t)
	if err != nil || head == "" {
		return false, err
	}
	if head == sha {
		return true, nil
	}
	var compare struct {
		Status string `json:"status"`
	}
	err = g.request(ctx, token, "GET", "/repos/"+t.Repository+"/compare/"+sha+"..."+head, nil, &compare)
	return compare.Status == "ahead" || compare.Status == "identical", err
}
func (s *Service) finish(ctx context.Context, t Task, a Attempt, status, message, sha string, unchanged bool) error {
	now := time.Now().UTC()
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Task{}).Where("id=?", t.ID).Updates(map[string]any{"status": status, "error": message, "commit_sha": sha, "unchanged": unchanged, "updated_at": now}).Error; err != nil {
			return err
		}
		if a.ID != 0 {
			return tx.Model(&Attempt{}).Where("id=?", a.ID).Updates(map[string]any{"status": status, "error": message, "finished_at": now}).Error
		}
		return nil
	})
}
func (s *Service) planned(ctx context.Context, t *Task, a *Attempt, sha string) error {
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(t).Update("planned_sha", sha).Error; err != nil {
			return err
		}
		return tx.Model(a).Update("planned_sha", sha).Error
	})
	if err == nil {
		t.PlannedSHA = sha
		a.PlannedSHA = sha
	}
	return err
}

// Recover runs under the independent kernel worker lock. It never blindly resubmits refs.
func (s *Service) Recover(ctx context.Context) error {
	var tasks []Task
	if err := s.DB.WithContext(ctx).Where("status='running'").Find(&tasks).Error; err != nil {
		return err
	}
	for _, t := range tasks {
		var a Attempt
		if err := s.DB.WithContext(ctx).Where("task_id=?", t.ID).Order("id DESC").Take(&a).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		status, message, sha := "interrupted", "执行中断，请核实后明确重试", ""
		if t.PlannedSHA != "" {
			token, err := s.token(ctx, "")
			if err == nil {
				found, err := s.GitHub.contains(ctx, token, t, t.PlannedSHA)
				if err == nil && found {
					status, message, sha = "succeeded", "", t.PlannedSHA
				}
			}
		}
		if err := s.finish(ctx, t, a, status, message, sha, false); err != nil {
			return err
		}
	}
	return nil
}

// RunNext executes one claimed task; the caller must hold the independent worker lock.
func (s *Service) RunNext(ctx context.Context) (bool, error) {
	var t Task
	var a Attempt
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.lockConnection(tx); err != nil {
			return err
		}
		if err := tx.Where("status='queued'").Order("id ASC").Take(&t).Error; err != nil {
			return err
		}
		var newer int64
		if err := tx.Model(&Task{}).Where("repository=? AND branch=? AND id>? AND status='succeeded'", t.Repository, t.Branch, t.ID).Count(&newer).Error; err != nil {
			return err
		}
		if newer > 0 {
			return tx.Model(&t).Update("status", "superseded").Error
		}
		if err := tx.Model(&t).Updates(map[string]any{"status": "running", "updated_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		a = Attempt{TaskID: t.ID, Status: "running", CreatedAt: time.Now().UTC()}
		return tx.Create(&a).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, ErrCredential) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if a.ID == 0 {
		return true, nil
	}
	token, err := s.token(ctx, "")
	if err == nil {
		var connection Connection
		connection, err = s.Read(ctx)
		if err == nil && (connection.Repository != t.Repository || connection.Branch != t.Branch) {
			err = ErrInvalid
		}
	}
	if err == nil && t.PlannedSHA != "" {
		var found bool
		found, err = s.GitHub.contains(ctx, token, t, t.PlannedSHA)
		if err == nil && found {
			return true, s.finish(ctx, t, a, "succeeded", "", t.PlannedSHA, false)
		}
		// Unknown outcomes must remain interrupted until they can be verified.
		if err != nil {
			return true, s.finish(context.WithoutCancel(ctx), t, a, "interrupted", publicError(err), "", false)
		}
	}
	if err == nil {
		err = s.prepare(ctx, &t)
	}
	if err != nil {
		return true, s.finish(context.WithoutCancel(ctx), t, a, "failed", publicError(err), "", false)
	}
	sha, unchanged, uncertain, err := s.push(ctx, token, &t, &a)
	status, message := "succeeded", ""
	if err != nil {
		status = "failed"
		message = publicError(err)
		if uncertain || ctx.Err() != nil {
			status = "interrupted"
		}
	}
	return true, s.finish(context.WithoutCancel(ctx), t, a, status, message, sha, unchanged)
}
func (s *Service) push(ctx context.Context, token string, t *Task, a *Attempt) (sha string, unchanged, uncertain bool, err error) {
	g := s.GitHub
	prefix := "/repos/" + t.Repository
	if _, err = g.repository(ctx, token, t.Repository); err != nil {
		return
	}
	head, err := g.head(ctx, token, *t)
	if err != nil {
		return
	}
	directory := s.inputDirectory(*t)
	// Git Database API cannot initialize a completely empty repository. Bootstrap via
	// Contents API, only when no branch exists. A lost response becomes interrupted;
	// explicit retry reads the new head before any further write.
	if head == "" {
		var repo Repository
		repo, err = g.repository(ctx, token, t.Repository)
		if err != nil {
			return
		}
		if repo.DefaultBranch != t.Branch {
			err = ErrInvalid
			return
		}
		var raw []byte
		raw, err = os.ReadFile(filepath.Join(directory, "export.json"))
		if err != nil {
			return
		}
		var result struct {
			Commit gitObject `json:"commit"`
		}
		err = g.request(ctx, token, "PUT", prefix+"/contents/cms-input/export.json", map[string]any{"message": "Initialize CMS input repository", "content": base64.StdEncoding.EncodeToString(raw), "branch": t.Branch}, &result)
		if err != nil {
			uncertain = true
			return
		}
		head = result.Commit.SHA
	}
	entries := []treeEntry{}
	err = filepath.WalkDir(directory, func(path string, entry os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return ErrInvalid
		}
		raw, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		var blob gitObject
		if e = g.request(ctx, token, "POST", prefix+"/git/blobs", map[string]string{"encoding": "base64", "content": base64.StdEncoding.EncodeToString(raw)}, &blob); e != nil {
			return e
		}
		rel, e := filepath.Rel(directory, path)
		if e != nil {
			return e
		}
		entries = append(entries, treeEntry{filepath.ToSlash(rel), "100644", "blob", blob.SHA})
		return nil
	})
	if err != nil {
		return
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	var generated gitTree
	err = g.request(ctx, token, "POST", prefix+"/git/trees", map[string]any{"tree": entries}, &generated)
	if err != nil {
		return
	}
	for try := 0; try < 3; try++ {
		if try > 0 {
			head, err = g.head(ctx, token, *t)
			if err != nil {
				return
			}
			if head == "" {
				err = ErrRemote
				return
			}
		}
		var parent gitCommit
		err = g.request(ctx, token, "GET", prefix+"/git/commits/"+head, nil, &parent)
		if err != nil {
			return
		}
		var tree gitTree
		err = g.request(ctx, token, "GET", prefix+"/git/trees/"+parent.Tree.SHA, nil, &tree)
		if err != nil {
			return
		}
		for _, entry := range tree.Tree {
			if entry.Path == "cms-input" && entry.Type == "tree" && entry.SHA == generated.SHA {
				return head, true, false, nil
			}
		}
		var nextTree gitTree
		err = g.request(ctx, token, "POST", prefix+"/git/trees", map[string]any{"base_tree": parent.Tree.SHA, "tree": []treeEntry{{"cms-input", "040000", "tree", generated.SHA}}}, &nextTree)
		if err != nil {
			return
		}
		var commit gitObject
		err = g.request(ctx, token, "POST", prefix+"/git/commits", map[string]any{"message": "Publish CMS input " + t.InputHash, "tree": nextTree.SHA, "parents": []string{head}}, &commit)
		if err != nil {
			return
		}
		if err = s.planned(ctx, t, a, commit.SHA); err != nil {
			return
		}
		err = g.request(ctx, token, "PATCH", prefix+"/git/refs/heads/"+url.PathEscape(t.Branch), map[string]any{"sha": commit.SHA, "force": false}, nil)
		if err == nil {
			return commit.SHA, false, false, nil
		}
		// Verify before deciding whether a ref update failed; external responses are not authoritative.
		found, verifyErr := g.contains(ctx, token, *t, commit.SHA)
		if verifyErr != nil {
			return "", false, true, verifyErr
		}
		if found {
			return commit.SHA, false, false, nil
		}
		var remote remoteError
		if !errors.As(err, &remote) || (remote.Status != 409 && remote.Status != 422) {
			return "", false, true, err
		}
	}
	return "", false, false, ErrRemote
}

// Run owns a separate lock, so Git network I/O never holds the local publication lock.
func (s *Service) Run(ctx context.Context) error {
	lock, err := deployment.Acquire(s.Root)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err = s.Recover(ctx); err != nil {
		return err
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if _, err = s.RunNext(ctx); err != nil {
				return err
			}
			if err = s.CleanupInputs(ctx); err != nil {
				return err
			}
		}
	}
}
