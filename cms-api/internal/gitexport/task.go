package gitexport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/audit"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/builder"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/deployment"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/publishing"
	"gorm.io/gorm"
)

type Task struct {
	ID               int64     `gorm:"primaryKey" json:"id"`
	ReleaseID        int64     `json:"releaseId"`
	ReleaseKey       string    `json:"releaseKey"`
	Manifest         string    `json:"-"`
	ManifestHash     string    `json:"-"`
	Repository       string    `json:"repository"`
	Branch           string    `json:"branch"`
	SourceRepository string    `json:"sourceRepository"`
	SourceSHA        string    `json:"sourceSha"`
	InputHash        string    `json:"inputHash"`
	Prepared         bool      `json:"-"`
	Status           string    `json:"status"`
	Error            string    `json:"error"`
	PlannedSHA       string    `json:"-"`
	CommitSHA        string    `json:"commitSha"`
	CommitURL        string    `gorm:"-" json:"commitUrl"`
	Unchanged        bool      `json:"unchanged"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
	CreatedBy        int64     `json:"createdBy"`
	Attempts         []Attempt `gorm:"-" json:"attempts,omitempty"`
}

func (Task) TableName() string { return "cms_git_push" }

type Attempt struct {
	ID         int64      `gorm:"primaryKey" json:"id"`
	TaskID     int64      `json:"taskId"`
	Status     string     `json:"status"`
	PlannedSHA string     `json:"plannedSha"`
	Error      string     `json:"error"`
	CreatedAt  time.Time  `json:"createdAt"`
	FinishedAt *time.Time `json:"finishedAt"`
}

func (Attempt) TableName() string { return "cms_git_push_attempt" }

type Request struct {
	Key       string `gorm:"primaryKey"`
	ActorID   int64
	TaskID    int64
	Operation string
}

func (Request) TableName() string { return "cms_git_push_request" }

var ErrBaseline = errors.New("当前成功发布版本无法核实，请先完成本地发布后重试")

func requestKeyValid(key string) bool {
	if key == "" || len(key) > 200 {
		return false
	}
	for _, c := range key {
		if unicode.IsControl(c) || unicode.IsSpace(c) {
			return false
		}
	}
	return true
}
func (s *Service) lockConnection(tx *gorm.DB) error {
	result := tx.Model(&Connection{}).Where("id=1").UpdateColumn("login", gorm.Expr("login"))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrCredential
	}
	return nil
}
func (s *Service) active(tx *gorm.DB) error {
	var n int64
	if err := tx.Model(&Task{}).Where("status IN ('queued','running')").Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return ErrConflict
	}
	return nil
}
func (s *Service) Submit(ctx context.Context, meta audit.Metadata, key string) (Task, error) {
	if !requestKeyValid(key) {
		return Task{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// Fast replay avoids requiring remote access after a successful submission.
	var prior Request
	if err := s.DB.WithContext(ctx).Where("key=?", key).Take(&prior).Error; err == nil {
		if prior.ActorID != meta.ActorID || prior.Operation != "create" {
			return Task{}, ErrInvalid
		}
		return s.Detail(ctx, prior.TaskID)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return Task{}, err
	}
	connection, err := s.Read(ctx)
	if err != nil || !connection.HasCredential {
		return Task{}, ErrCredential
	}
	token, err := s.token(ctx, "")
	if err != nil {
		return Task{}, err
	}
	sha, err := s.GitHub.Resolve(ctx, connection.SourceRepository, connection.SourceRef, token)
	if err != nil {
		return Task{}, err
	}
	var task Task
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.lockConnection(tx); err != nil {
			return err
		}
		var current Connection
		if err := tx.First(&current, 1).Error; err != nil {
			return err
		}
		if !current.UpdatedAt.Equal(connection.UpdatedAt) {
			return ErrConflict
		}
		// Serialize with local publication bookkeeping; never advance its baseline.
		if err := tx.Model(&publishing.State{}).Where("id=1").Update("version", gorm.Expr("version")).Error; err != nil {
			return err
		}
		var state publishing.State
		if err := tx.First(&state, 1).Error; err != nil {
			return err
		}
		if state.CurrentReleaseID == nil || state.BlockedReason != "" {
			return ErrBaseline
		}
		var release publishing.Release
		if err := tx.Where("id=? AND cleaned_at IS NULL", *state.CurrentReleaseID).Take(&release).Error; err != nil {
			return ErrBaseline
		}
		pointer, err := deployment.Read(filepath.Dir(s.Root))
		if err != nil || pointer == nil || pointer.AttemptID != release.AttemptID || pointer.ReleaseKey != release.ReleaseKey || pointer.ManifestHash != release.ManifestHash {
			return ErrBaseline
		}
		raw, err := os.ReadFile(filepath.Join(filepath.Dir(s.Root), "releases", release.ReleaseKey, ".release.json"))
		if err != nil {
			return ErrBaseline
		}
		var marker builder.Marker
		if json.Unmarshal(raw, &marker) != nil || marker.AttemptID != release.AttemptID || marker.ManifestHash != release.ManifestHash || marker.ReleaseKey != release.ReleaseKey {
			return ErrBaseline
		}
		now := time.Now().UTC()
		task = Task{ReleaseID: release.ID, ReleaseKey: release.ReleaseKey, Manifest: release.Manifest, ManifestHash: release.ManifestHash, Repository: connection.Repository, Branch: connection.Branch, SourceRepository: connection.SourceRepository, SourceSHA: sha, Status: "queued", CreatedAt: now, UpdatedAt: now, CreatedBy: meta.ActorID}
		if err := tx.Create(&task).Error; err != nil {
			return err
		}
		if err := tx.Create(&Request{Key: key, ActorID: meta.ActorID, TaskID: task.ID, Operation: "create"}).Error; err != nil {
			return err
		}
		return audit.RecordOn(ctx, tx, audit.Event{Action: "CREATE", Resource: "integration.git.push", ResourceID: task.ID, Metadata: meta})
	})
	return task, err
}
func (s *Service) Detail(ctx context.Context, id int64) (Task, error) {
	var out Task
	err := s.DB.WithContext(ctx).First(&out, id).Error
	if err != nil {
		return out, err
	}
	if out.CommitSHA != "" {
		out.CommitURL = "https://github.com/" + out.Repository + "/commit/" + out.CommitSHA
	}
	err = s.DB.WithContext(ctx).Where("task_id=?", id).Order("id ASC").Find(&out.Attempts).Error
	return out, err
}

type Page struct {
	Records  []Task `json:"records"`
	Total    int64  `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}

func (s *Service) List(ctx context.Context, page, size int) (Page, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	out := Page{Records: []Task{}, Page: page, PageSize: size}
	q := s.DB.WithContext(ctx).Model(&Task{})
	if err := q.Count(&out.Total).Error; err != nil {
		return out, err
	}
	err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&out.Records).Error
	for i := range out.Records {
		if out.Records[i].CommitSHA != "" {
			out.Records[i].CommitURL = "https://github.com/" + out.Records[i].Repository + "/commit/" + out.Records[i].CommitSHA
		}
	}
	return out, err
}
func (s *Service) Retry(ctx context.Context, meta audit.Metadata, id int64, key string) (Task, error) {
	if !requestKeyValid(key) {
		return Task{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var task Task
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.lockConnection(tx); err != nil {
			return err
		}
		var prior Request
		err := tx.Where("key=?", key).Take(&prior).Error
		if err == nil {
			if prior.TaskID != id || prior.ActorID != meta.ActorID || prior.Operation != "retry" {
				return ErrInvalid
			}
			return tx.First(&task, id).Error
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := tx.First(&task, id).Error; err != nil {
			return err
		}
		var connection Connection
		if err := tx.First(&connection, 1).Error; err != nil {
			return err
		}
		if connection.Repository != task.Repository || connection.Branch != task.Branch {
			return ErrInvalid
		}
		if task.Status != "failed" && task.Status != "interrupted" {
			return ErrInvalid
		}
		var newer int64
		if err := tx.Model(&Task{}).Where("repository=? AND branch=? AND id>? AND status='succeeded'", task.Repository, task.Branch, id).Count(&newer).Error; err != nil {
			return err
		}
		status := "queued"
		if newer > 0 {
			status = "superseded"
		}
		if err := tx.Model(&task).Updates(map[string]any{"status": status, "error": "", "updated_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		if err := tx.Create(&Request{Key: key, ActorID: meta.ActorID, TaskID: id, Operation: "retry"}).Error; err != nil {
			return err
		}
		return audit.RecordOn(ctx, tx, audit.Event{Action: "UPDATE", Resource: "integration.git.push", ResourceID: id, Metadata: meta})
	})
	if err != nil {
		return task, err
	}
	return s.Detail(ctx, id)
}
func (s *Service) inputDirectory(t Task) string {
	return filepath.Join(s.Root, "inputs", fmt.Sprint(t.ID))
}
func (s *Service) prepare(ctx context.Context, t *Task) error {
	directory := s.inputDirectory(*t)
	if t.Prepared {
		return verifyInput(directory, t.SourceRepository, t.SourceSHA, t.InputHash)
	}
	if err := os.MkdirAll(filepath.Dir(directory), 0700); err != nil {
		return err
	}
	if err := os.RemoveAll(directory); err != nil {
		return err
	}
	var manifest builder.Manifest
	if err := json.Unmarshal([]byte(t.Manifest), &manifest); err != nil {
		return err
	}
	metadata, err := builder.ExportInput(ctx, manifest, t.ManifestHash, filepath.Join(filepath.Dir(s.Root), "releases", t.ReleaseKey), directory, builder.SourceVersion{Repository: t.SourceRepository, SHA: t.SourceSHA})
	if err != nil {
		return err
	}
	if err := s.DB.WithContext(ctx).Model(t).Updates(map[string]any{"prepared": true, "input_hash": metadata.InputHash}).Error; err != nil {
		return err
	}
	t.Prepared = true
	t.InputHash = metadata.InputHash
	return nil
}
func verifyInput(directory, repo, sha, hash string) error {
	return builder.VerifyExport(directory, builder.SourceVersion{Repository: repo, SHA: sha}, hash)
}
func publicError(err error) string {
	var remote remoteError
	if errors.As(err, &remote) || errors.Is(err, ErrRemote) || errors.Is(err, ErrCredential) || errors.Is(err, ErrBaseline) {
		return err.Error()
	}
	if errors.Is(err, context.Canceled) {
		return "执行中断，请核实后重试"
	}
	return "输入准备或 Git 推送失败，请检查发布资源与连接后重试"
}

// CleanupInputs keeps retryable copies and all history/key material.
func (s *Service) CleanupInputs(ctx context.Context) error {
	var tasks []Task
	if err := s.DB.WithContext(ctx).Where("status IN ('succeeded','superseded') AND updated_at<?", time.Now().UTC().Add(-7*24*time.Hour)).Find(&tasks).Error; err != nil {
		return err
	}
	for _, task := range tasks {
		if err := os.RemoveAll(s.inputDirectory(task)); err != nil {
			return err
		}
	}
	return nil
}

// ownedInputPath prevents accidental publication outside the input namespace.
func ownedInputPath(path string) bool {
	return path == "cms-input" || strings.HasPrefix(path, "cms-input/")
}
