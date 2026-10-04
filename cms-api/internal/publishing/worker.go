package publishing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/builder"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/content"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/deployment"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/media"
	platformerrors "github.com/EziosWJ/cms-golang-astro/cms-api/internal/platform/errors"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/siteconfig"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/taxonomy"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"path/filepath"
	"sort"
	"time"
)

type BuildExecutor interface {
	Build(context.Context, builder.Manifest, string) (string, error)
}
type Worker struct {
	DB      *gorm.DB
	Root    string
	Builder BuildExecutor
	Fault   func(string) error
	Claim   func(func() error) error
	OnTask  func(int64)
}

func (w *Worker) checkpoint(point string) error {
	if w.Fault != nil {
		return w.Fault(point)
	}
	return nil
}
func (w *Worker) Run(ctx context.Context) error {
	lock, err := deployment.Acquire(w.Root)
	if err != nil {
		return fmt.Errorf("another worker/maintenance process holds the site lock: %w", err)
	}
	defer lock.Close()
	if err := w.Recover(ctx); err != nil {
		return err
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		worked, err := w.RunNext(ctx)
		if err != nil {
			return err
		}
		if !worked {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second):
			}
		}
	}
}
func (w *Worker) RunNext(ctx context.Context) (bool, error) {
	if err := w.verifyCurrent(ctx); err != nil {
		return false, w.block(ctx, err)
	}
	var task Task
	var attempt Attempt
	var manifest builder.Manifest
	claim := func() error {
		return w.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			state, err := lockState(tx)
			if err != nil {
				return err
			}
			query := tx.Where("status='queued'").Order("id ASC").Limit(1).Find(&task)
			if query.Error != nil {
				return query.Error
			}
			if query.RowsAffected == 0 {
				return gorm.ErrRecordNotFound
			}
			if err := validateTarget(tx, task); err != nil {
				return err
			}
			bytes := make([]byte, 16)
			if _, err := rand.Read(bytes); err != nil {
				return err
			}
			attempt = Attempt{TaskID: task.ID, Status: "running", BaselineReleaseID: state.CurrentReleaseID, ReleaseKey: hex.EncodeToString(bytes), CreatedAt: time.Now().UTC()}
			if err := tx.Create(&attempt).Error; err != nil {
				return err
			}
			manifest, err = makeManifest(tx, state, task, attempt)
			if err != nil {
				return err
			}
			encoded, hash, err := builder.Encode(manifest)
			if err != nil {
				return err
			}
			attempt.Manifest, attempt.ManifestHash = encoded, hash
			if err := tx.Model(&attempt).Updates(map[string]any{"manifest": encoded, "manifest_hash": hash}).Error; err != nil {
				return err
			}
			if err := manifestReferences(tx, "attempt", attempt.ID, manifest); err != nil {
				return err
			}
			return tx.Model(&task).Updates(map[string]any{"status": "running", "updated_at": time.Now().UTC()}).Error
		})
	}
	var err error
	if w.Claim != nil {
		err = w.Claim(claim)
	} else {
		err = claim()
	}
	if errors.Is(err, errClaimPaused) {
		return false, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) && task.ID == 0 {
		return false, nil
	}
	if err != nil {
		if task.ID != 0 {
			if failErr := w.fail(ctx, task, Attempt{}, "failed", err); failErr != nil {
				return true, failErr
			}
			return true, nil
		}
		return false, err
	}
	if w.OnTask != nil {
		w.OnTask(task.ID)
	}
	if err := w.checkpoint("before_build"); err != nil {
		return true, err
	}
	output, err := w.Builder.Build(ctx, manifest, attempt.ManifestHash)
	if err != nil {
		if ctx.Err() != nil {
			return true, w.fail(context.WithoutCancel(ctx), task, attempt, "interrupted", err)
		}
		return true, w.fail(ctx, task, attempt, "failed", err)
	}
	if err := w.checkpoint("after_build"); err != nil {
		return true, err
	}
	if task.Kind == "preview" {
		now := time.Now().UTC()
		err = w.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&attempt).Updates(map[string]any{"status": "succeeded", "finished_at": now}).Error; err != nil {
				return err
			}
			if err := clearTarget(tx, task.ID); err != nil {
				return err
			}
			return tx.Model(&task).Updates(map[string]any{"status": "succeeded", "updated_at": now}).Error
		})
		return true, err
	}
	if err := builder.Validate(output, builder.Marker{AttemptID: attempt.ID, ReleaseKey: attempt.ReleaseKey, ManifestHash: attempt.ManifestHash}); err != nil {
		return true, w.fail(ctx, task, attempt, "failed", err)
	}
	err = w.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		state, err := lockState(tx)
		if err != nil {
			return err
		}
		if !sameID(state.CurrentReleaseID, attempt.BaselineReleaseID) {
			return ErrConflict
		}
		if err := validateTarget(tx, task); err != nil {
			return err
		}
		return tx.Model(&attempt).Update("switch_intent", 1).Error
	})
	if err != nil {
		return true, w.fail(ctx, task, attempt, "failed", err)
	}
	if err := w.verifyCurrent(ctx); err != nil {
		return true, w.block(ctx, err)
	}
	attempt.SwitchIntent = 1
	if err := w.checkpoint("before_switch"); err != nil {
		return true, err
	}
	if err := deployment.Switch(w.Root, deployment.Pointer{AttemptID: attempt.ID, ReleaseKey: attempt.ReleaseKey, ManifestHash: attempt.ManifestHash}); err != nil {
		if recoverErr := w.Recover(context.WithoutCancel(ctx)); recoverErr != nil {
			return true, recoverErr
		}
		return true, nil
	}
	if err := w.checkpoint("after_switch"); err != nil {
		return true, err
	}
	if err := w.finish(ctx, task, attempt); err != nil {
		return true, err
	}
	if err := w.checkpoint("after_register"); err != nil {
		return true, err
	}
	return true, nil
}
func sameID(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
func makeManifest(tx *gorm.DB, state State, task Task, attempt Attempt) (builder.Manifest, error) {
	manifest := builder.Manifest{AttemptID: attempt.ID, ReleaseKey: attempt.ReleaseKey, Articles: []builder.Article{}, Media: []builder.Media{}}
	if state.CurrentReleaseID != nil {
		var base Release
		if err := tx.Where("id=? AND cleaned_at IS NULL", *state.CurrentReleaseID).Take(&base).Error; err != nil {
			return manifest, err
		}
		if err := json.Unmarshal([]byte(base.Manifest), &manifest); err != nil {
			return manifest, err
		}
		manifest.AttemptID = attempt.ID
		manifest.ReleaseKey = attempt.ReleaseKey
	}
	switch task.Kind {
	case "config":
		manifest.Config = siteconfig.Revision{}
		if err := tx.Where("id=?", task.ConfigRevisionID).Take(&manifest.Config).Error; err != nil {
			return manifest, err
		}
	case "article", "preview", "unpublish":
		kept := []builder.Article{}
		first := attempt.CreatedAt
		var identity content.Article
		if err := tx.Where("id=?", task.ArticleID).Take(&identity).Error; err != nil {
			return manifest, err
		}
		if identity.SlugLockedAt != nil {
			first = *identity.SlugLockedAt
		}
		for _, article := range manifest.Articles {
			if article.Revision.ArticleID == *task.ArticleID {
				first = article.FirstPublishedAt
			} else {
				kept = append(kept, article)
			}
		}
		if task.Kind != "unpublish" {
			var revision content.Revision
			if task.Kind == "preview" {
				if err := json.Unmarshal([]byte(task.TargetSnapshot), &revision); err != nil {
					return manifest, err
				}
			} else if err := tx.Where("id=?", task.RevisionID).Take(&revision).Error; err != nil {
				return manifest, err
			}
			updated := attempt.CreatedAt
			if identity.SlugLockedAt == nil && identity.ImportedUpdatedAt != nil {
				updated = *identity.ImportedUpdatedAt
			}
			kept = append(kept, builder.Article{Revision: revision, FirstPublishedAt: first, UpdatedAt: updated})
		}
		manifest.Articles = kept
	}
	if task.Kind == "preview" {
		manifest.PreviewTaskID = task.ID
	} else {
		manifest.PreviewTaskID = 0
	}
	sort.Slice(manifest.Articles, func(i, j int) bool {
		return manifest.Articles[i].Revision.ArticleID < manifest.Articles[j].Revision.ArticleID
	})
	files := map[int64]media.File{}
	avatar, err := media.Resolve(tx, "", manifest.Config.Data.AvatarMediaID)
	if err != nil {
		return manifest, err
	}
	for _, f := range avatar {
		files[f.ID] = f
	}
	for _, article := range manifest.Articles {
		if err := taxonomy.ValidateSnapshots(tx, article.Revision.Taxonomy); err != nil {
			return manifest, err
		}
		refs, err := media.Resolve(tx, article.Revision.Markdown, article.Revision.CoverMediaID)
		if err != nil {
			return manifest, err
		}
		for _, f := range refs {
			files[f.ID] = f
		}
	}
	ids := []int64{}
	for id := range files {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	manifest.Media = []builder.Media{}
	for _, id := range ids {
		manifest.Media = append(manifest.Media, builder.Media{File: files[id], Path: media.Path(files[id])})
		var aliases []media.Alias
		if err := tx.Where("file_id=?", id).Order("path ASC").Find(&aliases).Error; err != nil {
			return manifest, err
		}
		for _, alias := range aliases {
			manifest.Media = append(manifest.Media, builder.Media{File: files[id], Path: alias.Path})
		}
	}
	return manifest, nil
}
func manifestReferences(tx *gorm.DB, owner string, id int64, manifest builder.Manifest) error {
	files := []media.File{}
	for _, resource := range manifest.Media {
		seenFiles := false
		for _, f := range files {
			if f.ID == resource.File.ID {
				seenFiles = true
				break
			}
		}
		if !seenFiles {
			files = append(files, resource.File)
		}
	}
	if err := media.SetReferences(tx, owner, id, files); err != nil {
		return err
	}
	terms := []taxonomy.Snapshot{}
	seen := map[int64]bool{}
	for _, article := range manifest.Articles {
		for _, term := range article.Revision.Taxonomy {
			if !seen[term.ID] {
				seen[term.ID] = true
				terms = append(terms, term)
			}
		}
	}
	return taxonomy.SetReferences(tx, owner, id, terms)
}
func (w *Worker) finish(ctx context.Context, task Task, attempt Attempt) error {
	return w.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		state, err := lockState(tx)
		if err != nil {
			return err
		}
		var existing Release
		err = tx.Where("attempt_id=?", attempt.ID).Take(&existing).Error
		if err == nil {
			if state.CurrentReleaseID == nil || *state.CurrentReleaseID != existing.ID {
				return ErrBlocked
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if !sameID(state.CurrentReleaseID, attempt.BaselineReleaseID) {
			return ErrBlocked
		}
		pointer, err := deployment.Read(w.Root)
		if err != nil || pointer == nil || pointer.AttemptID != attempt.ID || pointer.ManifestHash != attempt.ManifestHash || pointer.ReleaseKey != attempt.ReleaseKey {
			return ErrBlocked
		}
		if err := builder.Validate(filepath.Join(w.Root, "releases", attempt.ReleaseKey), builder.Marker{AttemptID: attempt.ID, ReleaseKey: attempt.ReleaseKey, ManifestHash: attempt.ManifestHash}); err != nil {
			return err
		}
		var manifest builder.Manifest
		if err := json.Unmarshal([]byte(attempt.Manifest), &manifest); err != nil {
			return err
		}
		now := time.Now().UTC()
		release := Release{AttemptID: attempt.ID, ReleaseKey: attempt.ReleaseKey, Manifest: attempt.Manifest, ManifestHash: attempt.ManifestHash, CreatedAt: now}
		if err := tx.Create(&release).Error; err != nil {
			return err
		}
		if err := manifestReferences(tx, "release", release.ID, manifest); err != nil {
			return err
		}
		if err := tx.Model(&State{}).Where("id=1").Update("current_release_id", release.ID).Error; err != nil {
			return err
		}
		if task.Kind == "article" {
			if err := tx.Model(&content.Article{}).Where("id=? AND slug_locked_at IS NULL", task.ArticleID).Update("slug_locked_at", now).Error; err != nil {
				return err
			}
			first := now
			var identity content.Article
			if err := tx.Where("id=?", task.ArticleID).Take(&identity).Error; err != nil {
				return err
			}
			if identity.SlugLockedAt != nil {
				first = *identity.SlugLockedAt
			}
			var old PublishedArticle
			if err := tx.Where("article_id=?", task.ArticleID).Take(&old).Error; err == nil {
				first = old.FirstPublishedAt
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			published := PublishedArticle{ArticleID: *task.ArticleID, RevisionID: *task.RevisionID, FirstPublishedAt: first, UpdatedAt: now}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "article_id"}}, DoUpdates: clause.AssignmentColumns([]string{"revision_id", "updated_at"})}).Create(&published).Error; err != nil {
				return err
			}
			for _, article := range manifest.Articles {
				if article.Revision.ArticleID == *task.ArticleID {
					for _, term := range article.Revision.Taxonomy {
						if err := tx.Model(&taxonomy.Term{}).Where("id=? AND locked_at IS NULL", term.ID).Update("locked_at", now).Error; err != nil {
							return err
						}
					}
				}
			}
		} else if task.Kind == "unpublish" {
			if err := tx.Where("article_id=?", task.ArticleID).Delete(&PublishedArticle{}).Error; err != nil {
				return err
			}
		}
		if err := clearTarget(tx, task.ID); err != nil {
			return err
		}
		if err := tx.Model(&attempt).Updates(map[string]any{"status": "succeeded", "finished_at": now, "error": ""}).Error; err != nil {
			return err
		}
		return tx.Model(&task).Updates(map[string]any{"status": "succeeded", "updated_at": now, "error": ""}).Error
	})
}
func (w *Worker) fail(ctx context.Context, task Task, attempt Attempt, status string, cause error) error {
	return w.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		if attempt.ID != 0 {
			if err := tx.Model(&attempt).Updates(map[string]any{"status": status, "error": cause.Error(), "finished_at": now}).Error; err != nil {
				return err
			}
		}
		if err := clearTarget(tx, task.ID); err != nil {
			return err
		}
		return tx.Model(&task).Updates(map[string]any{"status": status, "error": cause.Error(), "updated_at": now}).Error
	})
}
func (w *Worker) block(ctx context.Context, cause error) error {
	if errors.Is(cause, platformerrors.ErrTemporarilyUnavailable) {
		return cause
	}
	err := w.DB.WithContext(ctx).Model(&State{}).Where("id=1").Update("blocked_reason", cause.Error()).Error
	if err != nil {
		return err
	}
	return fmt.Errorf("%w: %v", ErrBlocked, cause)
}
func (w *Worker) Recover(ctx context.Context) error {
	pointer, err := deployment.Read(w.Root)
	if err != nil {
		return w.block(ctx, err)
	}
	var state State
	if err := w.DB.WithContext(ctx).Where("id=1").Take(&state).Error; err != nil {
		return err
	}
	running := []Task{}
	if err := w.DB.WithContext(ctx).Where("status='running'").Find(&running).Error; err != nil {
		return err
	}
	matched := false
	for _, task := range running {
		var attempt Attempt
		if err := w.DB.WithContext(ctx).Where("task_id=? AND status='running'", task.ID).Order("id DESC").Take(&attempt).Error; err != nil {
			return w.block(ctx, err)
		}
		if pointer != nil && pointer.AttemptID == attempt.ID {
			if task.Kind == "preview" || attempt.SwitchIntent != 1 || pointer.ReleaseKey != attempt.ReleaseKey || pointer.ManifestHash != attempt.ManifestHash {
				return w.block(ctx, errors.New("pointer does not match persisted switch intent"))
			}
			matched = true
			if state.BlockedReason != "" {
				if err := w.DB.WithContext(ctx).Model(&State{}).Where("id=1").Update("blocked_reason", "").Error; err != nil {
					return err
				}
			}
			if err := w.finish(ctx, task, attempt); err != nil {
				return w.block(ctx, err)
			}
		} else {
			if err := w.fail(ctx, task, attempt, "interrupted", errors.New("worker interrupted before release switch; explicit retry required")); err != nil {
				return err
			}
		}
	}
	if !matched {
		if state.CurrentReleaseID == nil {
			if pointer != nil {
				return w.block(ctx, errors.New("pointer exists without a registered or switching release"))
			}
		} else {
			var release Release
			if err := w.DB.WithContext(ctx).Where("id=? AND cleaned_at IS NULL", *state.CurrentReleaseID).Take(&release).Error; err != nil {
				return w.block(ctx, err)
			}
			if pointer == nil || pointer.AttemptID != release.AttemptID || pointer.ReleaseKey != release.ReleaseKey || pointer.ManifestHash != release.ManifestHash {
				return w.block(ctx, errors.New("pointer and current release disagree"))
			}
			if err := builder.Validate(filepath.Join(w.Root, "releases", release.ReleaseKey), builder.Marker{AttemptID: release.AttemptID, ReleaseKey: release.ReleaseKey, ManifestHash: release.ManifestHash}); err != nil {
				return w.block(ctx, err)
			}
		}
	}
	return w.DB.WithContext(ctx).Model(&State{}).Where("id=1").Update("blocked_reason", "").Error
}

func (w *Worker) verifyCurrent(ctx context.Context) error {
	pointer, err := deployment.Read(w.Root)
	if err != nil {
		return err
	}
	var state State
	if err := w.DB.WithContext(ctx).Where("id=1").Take(&state).Error; err != nil {
		return err
	}
	if state.BlockedReason != "" {
		return ErrBlocked
	}
	if state.CurrentReleaseID == nil {
		if pointer != nil {
			return ErrBlocked
		}
		return nil
	}
	var release Release
	if err := w.DB.WithContext(ctx).Where("id=? AND cleaned_at IS NULL", state.CurrentReleaseID).Take(&release).Error; err != nil {
		return err
	}
	if pointer == nil || pointer.AttemptID != release.AttemptID || pointer.ReleaseKey != release.ReleaseKey || pointer.ManifestHash != release.ManifestHash {
		return ErrBlocked
	}
	return builder.Validate(filepath.Join(w.Root, "releases", release.ReleaseKey), builder.Marker{AttemptID: release.AttemptID, ReleaseKey: release.ReleaseKey, ManifestHash: release.ManifestHash})
}
