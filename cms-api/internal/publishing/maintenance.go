package publishing

import (
	"context"
	"errors"
	"fmt"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/deployment"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/media"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/taxonomy"
	"gorm.io/gorm"
	"os"
	"path/filepath"
	"time"
)

type CleanupItem struct {
	Kind, Key string
	ID        int64
}
type CleanupReport struct {
	DryRun       bool          `json:"dryRun"`
	KeptReleases []int64       `json:"keptReleases"`
	Removed      []CleanupItem `json:"removed"`
	Errors       []string      `json:"errors"`
}

// Cleanup requires a stopped worker. Kernel locking excludes publication and recovery.
// Removing a directory precedes releasing its references; a crash retains protection.
func (w *Worker) Cleanup(ctx context.Context, previewTTL, failedTTL time.Duration, dryRun bool) (CleanupReport, error) {
	report := CleanupReport{DryRun: dryRun, KeptReleases: []int64{}, Removed: []CleanupItem{}, Errors: []string{}}
	if previewTTL <= 0 || failedTTL <= 0 {
		return report, ErrInvalid
	}
	lock, err := deployment.Acquire(w.Root)
	if err != nil {
		return report, fmt.Errorf("stop worker before maintenance: %w", err)
	}
	defer lock.Close()
	if err := w.Recover(ctx); err != nil {
		return report, err
	}
	var state State
	if err := w.DB.WithContext(ctx).Where("id=1").Take(&state).Error; err != nil {
		return report, err
	}
	keep := map[int64]bool{}
	var recent []Release
	if err := w.DB.WithContext(ctx).Where("cleaned_at IS NULL").Order("id DESC").Limit(5).Find(&recent).Error; err != nil {
		return report, err
	}
	for _, r := range recent {
		keep[r.ID] = true
	}
	if state.CurrentReleaseID != nil {
		keep[*state.CurrentReleaseID] = true
	}
	var releases []Release
	if err := w.DB.WithContext(ctx).Where("cleaned_at IS NULL").Order("id DESC").Find(&releases).Error; err != nil {
		return report, err
	}
	for _, r := range releases {
		if keep[r.ID] {
			report.KeptReleases = append(report.KeptReleases, r.ID)
			continue
		}
		item := CleanupItem{"release", r.ReleaseKey, r.ID}
		if !dryRun {
			if err := w.removeArtifacts(ctx, r.ReleaseKey, "releases", r.AttemptID, &r.ID); err != nil {
				report.Errors = append(report.Errors, err.Error())
				continue
			}
		}
		report.Removed = append(report.Removed, item)
	}
	now := time.Now().UTC()
	var attempts []Attempt
	if err := w.DB.WithContext(ctx).Table("cms_publish_attempt a").Select("a.*").Joins("JOIN cms_publish_task t ON t.id=a.task_id").Where("t.status NOT IN ('queued','running') AND a.status NOT IN ('running') AND ((t.kind='preview' AND a.created_at<?) OR (a.status IN ('failed','interrupted') AND a.created_at<?))", now.Add(-previewTTL), now.Add(-failedTTL)).Where("NOT EXISTS (SELECT 1 FROM cms_preview_access pa WHERE pa.task_id=t.id AND pa.expires_at>?)", now).Find(&attempts).Error; err != nil {
		return report, err
	}
	for _, a := range attempts {
		var retained int64
		if err := w.DB.WithContext(ctx).Model(&Release{}).Where("attempt_id=? AND cleaned_at IS NULL", a.ID).Count(&retained).Error; err != nil {
			return report, err
		}
		if retained > 0 {
			continue
		}
		var task Task
		if err := w.DB.WithContext(ctx).Where("id=?", a.TaskID).Take(&task).Error; err != nil {
			return report, err
		}
		folder := "releases"
		if task.Kind == "preview" {
			folder = "previews"
		}
		if !dryRun {
			if err := w.removeArtifacts(ctx, a.ReleaseKey, folder, a.ID, nil); err != nil {
				report.Errors = append(report.Errors, err.Error())
				continue
			}
			if task.Kind == "preview" {
				if err := w.DB.WithContext(ctx).Where("task_id=?", task.ID).Delete(&PreviewAccess{}).Error; err != nil {
					return report, err
				}
			}
		}
		report.Removed = append(report.Removed, CleanupItem{folder, a.ReleaseKey, a.ID})
	}
	if !dryRun {
		if err := w.DB.WithContext(ctx).Where("expires_at<=?", now).Delete(&PreviewAccess{}).Error; err != nil {
			return report, err
		}
	}
	return report, nil
}
func (w *Worker) removeArtifacts(ctx context.Context, key, folder string, attemptID int64, releaseID *int64) error {
	if !deployment.Valid(deployment.Pointer{AttemptID: attemptID, ReleaseKey: key, ManifestHash: string(make([]byte, 64))}) {
		return errors.New("unsafe cleanup identity")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(w.Root, folder, key)); err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(w.Root, "generated", key)); err != nil {
		return err
	}
	return w.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := media.SetReferences(tx, "attempt", attemptID, nil); err != nil {
			return err
		}
		if err := taxonomy.SetReferences(tx, "attempt", attemptID, nil); err != nil {
			return err
		}
		if releaseID != nil {
			if err := media.SetReferences(tx, "release", *releaseID, nil); err != nil {
				return err
			}
			if err := taxonomy.SetReferences(tx, "release", *releaseID, nil); err != nil {
				return err
			}
			return tx.Model(&Release{}).Where("id=?", releaseID).Update("cleaned_at", time.Now().UTC()).Error
		}
		return nil
	})
}
