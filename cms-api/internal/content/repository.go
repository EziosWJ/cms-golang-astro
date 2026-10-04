package content

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/authz"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/media"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/taxonomy"
	"time"

	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/audit"
	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) CanEdit(ctx context.Context, actorID int64) (bool, error) {
	return authz.Allowed(ctx, r.db, actorID, EditPermission)
}
func (r *Repository) Page(ctx context.Context, q Query) (Page, error) {
	p := Page{Page: q.Page, PageSize: q.PageSize, Records: []ListItem{}}
	d := r.db.WithContext(ctx).Table("cms_article a").Joins("JOIN cms_working_draft d ON d.article_id = a.id").Where("a.lifecycle = ?", q.Lifecycle)
	if q.Title != "" {
		d = d.Where("d.title LIKE ?", "%"+q.Title+"%")
	}
	if q.Status == "draft" {
		d = d.Where("NOT EXISTS (SELECT 1 FROM cms_published_article p WHERE p.article_id=a.id)")
	}
	if q.Status == "published" || q.Status == "changed" {
		d = d.Where("EXISTS (SELECT 1 FROM cms_published_article p WHERE p.article_id=a.id)")
	}
	if q.Status == "changed" {
		// Compare content rather than version: manual snapshots and date normalization
		// must not create a false unpublished-change status.
		var ids []int64
		var candidates []int64
		if err := d.Pluck("a.id", &candidates).Error; err != nil {
			return p, err
		}
		for _, id := range candidates {
			v, err := readDetail(r.db.WithContext(ctx), id)
			if err != nil {
				return p, err
			}
			if v.UnpublishedChanges {
				ids = append(ids, id)
			}
		}
		d = d.Where("a.id IN ?", ids)
	}
	if err := d.Count(&p.Total).Error; err != nil {
		return p, err
	}
	err := d.Select("a.id, COALESCE(a.slug, '') AS slug, a.lifecycle, d.title, d.version, d.saved_at, d.display_date").Order("d.saved_at DESC, a.id DESC").Limit(q.PageSize).Offset((q.Page - 1) * q.PageSize).Scan(&p.Records).Error
	if err == nil {
		for i := range p.Records {
			detail, e := readDetail(r.db.WithContext(ctx), p.Records[i].ID)
			if e != nil {
				return p, e
			}
			p.Records[i].Taxonomy = detail.Draft.Taxonomy
			p.Records[i].Published = detail.Published
			p.Records[i].UnpublishedChanges = detail.UnpublishedChanges
		}
	}
	return p, err
}
func (r *Repository) Detail(ctx context.Context, id int64) (Detail, error) {
	return readDetail(r.db.WithContext(ctx), id)
}
func readDetail(db *gorm.DB, id int64) (Detail, error) {
	var v Detail
	// One joined read prevents a new identity/old draft pair during a save.
	var row struct {
		Article
		SlugValue                string
		CoverMediaID             *int64
		Taxonomy                 []taxonomy.Snapshot `gorm:"serializer:json"`
		DraftVersion             int64
		Title, Markdown, Summary string
		DisplayDate              *time.Time
		SavedAt                  time.Time
		SavedBy                  int64
	}
	err := db.Table("cms_article a").Select("a.*, COALESCE(a.slug, '') AS slug_value, d.version AS draft_version, d.title, d.markdown, d.summary, d.display_date, d.saved_at, d.saved_by, d.taxonomy, d.cover_media_id").Joins("JOIN cms_working_draft d ON d.article_id = a.id").Where("a.id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return v, ErrNotFound
	}
	if err != nil {
		return v, err
	}
	v.Article, v.Slug = row.Article, row.SlugValue
	v.Draft = Draft{ArticleID: id, Version: row.DraftVersion, Title: row.Title, Markdown: row.Markdown, Summary: row.Summary, DisplayDate: row.DisplayDate, SavedAt: row.SavedAt, SavedBy: row.SavedBy, Taxonomy: row.Taxonomy, CoverMediaID: row.CoverMediaID}
	var published struct{ RevisionID int64 }
	err = db.Table("cms_published_article").Where("article_id=?", id).Take(&published).Error
	if err == nil {
		v.Published = true
		v.PublishedRevisionID = &published.RevisionID
		var revision Revision
		if err := db.Where("id=?", published.RevisionID).Take(&revision).Error; err != nil {
			return v, err
		}
		v.UnpublishedChanges = !sameContent(v, revision)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return v, err
	} else {
		v.UnpublishedChanges = true
	}
	return v, nil
}
func (r *Repository) Create(ctx context.Context, in DraftInput, meta audit.Metadata) (Detail, error) {
	var result Detail
	encoded, err := json.Marshal(in)
	if err != nil {
		return result, err
	}
	fingerprint := fmt.Sprintf("%x", sha256.Sum256(encoded))
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("cms_publish_state").Where("id=1").Update("version", gorm.Expr("version+1")).Error; err != nil {
			return err
		}
		if in.RequestKey != "" {
			var previous CreateRequest
			err := tx.Where("actor_id=? AND request_key=?", meta.ActorID, in.RequestKey).Take(&previous).Error
			if err == nil {
				if previous.Fingerprint != fingerprint {
					return ErrConflict
				}
				result, err = readDetail(tx, previous.ArticleID)
				return err
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		now := time.Now().UTC()
		a := Article{Slug: nullableSlug(in.Slug), Lifecycle: "active", CreatedAt: now}
		if err := tx.Create(&a).Error; err != nil {
			return slugError(err)
		}
		d := draftFrom(in, a.ID, 1, meta.ActorID, now)
		terms, err := taxonomy.Resolve(tx, in.CategoryIDs, in.TagIDs)
		if err != nil {
			return err
		}
		d.Taxonomy = terms
		files, err := media.Resolve(tx, in.Markdown, in.CoverMediaID)
		if err != nil {
			return err
		}
		if err := tx.Create(&d).Error; err != nil {
			return err
		}
		if err := media.SetReferences(tx, "draft", a.ID, files); err != nil {
			return err
		}
		if err := taxonomy.SetReferences(tx, "draft", a.ID, d.Taxonomy); err != nil {
			return err
		}
		var revisionID *int64
		if in.CreateMode != "autosave" {
			rev := revisionFrom(d, in.Slug)
			if err := tx.Create(&rev).Error; err != nil {
				return err
			}
			if err := taxonomy.SetReferences(tx, "revision", rev.ID, rev.Taxonomy); err != nil {
				return err
			}
			if err := media.SetReferences(tx, "revision", rev.ID, files); err != nil {
				return err
			}
			revisionID = &rev.ID
		}
		if in.RequestKey != "" {
			if err := tx.Create(&CreateRequest{ActorID: meta.ActorID, RequestKey: in.RequestKey, Fingerprint: fingerprint, ArticleID: a.ID}).Error; err != nil {
				return err
			}
		}
		if err := audit.RecordOn(ctx, tx, audit.Event{Action: "CREATE", Resource: "content.article", ResourceID: a.ID, Metadata: meta}); err != nil {
			return err
		}
		result, err = readDetail(tx, a.ID)
		if err != nil {
			return err
		}
		result.RevisionID = revisionID
		return nil
	})
	return result, err
}
func (r *Repository) Save(ctx context.Context, id int64, in SaveInput, meta audit.Metadata) (Detail, error) {
	var result Detail
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("cms_publish_state").Where("id=1").Update("version", gorm.Expr("version+1")).Error; err != nil {
			return err
		}
		// The version compare-and-swap is the first write. In PostgreSQL it
		// locks this draft; in SQLite it obtains the writer lock before reading.
		now := time.Now().UTC()
		newVersion := *in.ExpectedVersion + 1
		d := draftFrom(in.DraftInput, id, newVersion, meta.ActorID, now)
		updated := tx.Model(&Draft{}).Where("article_id = ? AND version = ?", id, *in.ExpectedVersion).Updates(map[string]any{
			"version": newVersion, "title": d.Title, "markdown": d.Markdown, "summary": d.Summary,
			"display_date": d.DisplayDate, "saved_at": now, "saved_by": meta.ActorID, "cover_media_id": d.CoverMediaID,
		})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected == 0 {
			var count int64
			if err := tx.Model(&Article{}).Where("id = ?", id).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return ErrNotFound
			}
			return ErrConflict
		}
		var terms []taxonomy.Snapshot
		if in.Mode == "restore" {
			terms = in.RestoredTaxonomy
			if err := taxonomy.ValidateSnapshots(tx, terms); err != nil {
				return err
			}
		} else {
			var err error
			terms, err = taxonomy.Resolve(tx, in.CategoryIDs, in.TagIDs)
			if err != nil {
				return err
			}
		}
		d.Taxonomy = terms
		files, err := media.Resolve(tx, d.Markdown, d.CoverMediaID)
		if err != nil {
			return err
		}
		if err := media.SetReferences(tx, "draft", id, files); err != nil {
			return err
		}
		encoded, err := json.Marshal(terms)
		if err != nil {
			return err
		}
		if err := tx.Model(&Draft{}).Where("article_id=?", id).Update("taxonomy", string(encoded)).Error; err != nil {
			return err
		}
		if err := taxonomy.SetReferences(tx, "draft", id, terms); err != nil {
			return err
		}
		var a Article
		if err := tx.Where("id = ?", id).Take(&a).Error; err != nil {
			return err
		}
		var pending int64
		if stringSlug(a.Slug) != in.Slug {
			if err := tx.Table("cms_publish_task").Where("article_id=? AND kind <> 'preview' AND status IN ('queued','running')", id).Count(&pending).Error; err != nil {
				return err
			}
		}
		if a.Lifecycle != "active" || pending > 0 || (a.SlugLockedAt != nil && stringSlug(a.Slug) != in.Slug) {
			return ErrReadOnly
		}
		if err := tx.Model(&a).Update("slug", nullableSlug(in.Slug)).Error; err != nil {
			return slugError(err)
		}
		a.Slug = nullableSlug(in.Slug)
		var revisionID *int64
		if in.Mode == "manual" {
			rev := revisionFrom(d, in.Slug)
			if in.RevisionSource == "publish" {
				rev.Source = "publish"
			}
			if err := tx.Create(&rev).Error; err != nil {
				return err
			}
			if err := taxonomy.SetReferences(tx, "revision", rev.ID, rev.Taxonomy); err != nil {
				return err
			}
			if err := media.SetReferences(tx, "revision", rev.ID, files); err != nil {
				return err
			}
			revisionID = &rev.ID
		}
		if err := audit.RecordOn(ctx, tx, audit.Event{Action: "UPDATE", Resource: "content.article", ResourceID: id, Metadata: meta}); err != nil {
			return err
		}
		result, err = readDetail(tx, a.ID)
		if err != nil {
			return err
		}
		result.RevisionID = revisionID
		return nil
	})
	return result, err
}
func nullableSlug(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
func stringSlug(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
func slugError(err error) error {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrSlugTaken
	}
	return err
}
func draftFrom(in DraftInput, id, version, actor int64, now time.Time) Draft {
	date := in.DisplayDate
	if date != nil {
		utc := date.UTC()
		date = &utc
	}
	return Draft{ArticleID: id, Version: version, Title: in.Title, Markdown: in.Markdown, Summary: in.Summary, DisplayDate: date, SavedAt: now, SavedBy: actor, CoverMediaID: in.CoverMediaID}
}
func revisionFrom(d Draft, slug string) Revision {
	return Revision{Source: "manual", ArticleID: d.ArticleID, Version: d.Version, Slug: slug, Title: d.Title, Markdown: d.Markdown, Summary: d.Summary, DisplayDate: d.DisplayDate, CreatedAt: d.SavedAt, CreatedBy: d.SavedBy, Taxonomy: d.Taxonomy, CoverMediaID: d.CoverMediaID}
}

func (r *Repository) Revisions(ctx context.Context, id int64, q Query) (RevisionPage, error) {
	p := RevisionPage{Records: []RevisionSummary{}, Page: q.Page, PageSize: q.PageSize}
	if _, err := r.Detail(ctx, id); err != nil {
		return p, err
	}
	query := r.db.WithContext(ctx).Model(&Revision{}).Where("article_id = ?", id)
	if err := query.Count(&p.Total).Error; err != nil {
		return p, err
	}
	err := query.Select("id, article_id, version, title, source, created_at, created_by").Order("id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Scan(&p.Records).Error
	return p, err
}
func (r *Repository) Revision(ctx context.Context, id, revisionID int64) (Revision, error) {
	var rev Revision
	err := r.db.WithContext(ctx).Where("article_id = ? AND id = ?", id, revisionID).Take(&rev).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return rev, ErrNotFound
	}
	return rev, err
}

func sameContent(detail Detail, rev Revision) bool {
	d := detail.Draft
	if detail.Slug != rev.Slug || d.Title != rev.Title || d.Markdown != rev.Markdown || d.Summary != rev.Summary {
		return false
	}
	if (d.CoverMediaID == nil) != (rev.CoverMediaID == nil) || d.CoverMediaID != nil && *d.CoverMediaID != *rev.CoverMediaID {
		return false
	}
	if (d.DisplayDate == nil) != (rev.DisplayDate == nil) || d.DisplayDate != nil && !d.DisplayDate.Equal(*rev.DisplayDate) {
		return false
	}
	if len(d.Taxonomy) != len(rev.Taxonomy) {
		return false
	}
	m := map[int64]taxonomy.Snapshot{}
	for _, t := range rev.Taxonomy {
		m[t.ID] = t
	}
	for _, t := range d.Taxonomy {
		if m[t.ID] != t {
			return false
		}
	}
	return true
}
func (r *Repository) Lifecycle(ctx context.Context, id int64, lifecycle string, version int64, meta audit.Metadata) (Detail, error) {
	var result Detail
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("cms_publish_state").Where("id=1").Update("version", gorm.Expr("version+1")).Error; err != nil {
			return err
		}
		var pending int64
		if err := tx.Table("cms_publish_task").Where("article_id=? AND status IN ('queued','running')", id).Count(&pending).Error; err != nil {
			return err
		}
		if pending > 0 {
			return ErrReadOnly
		}
		if lifecycle == "archived" {
			var published int64
			if err := tx.Table("cms_published_article").Where("article_id=?", id).Count(&published).Error; err != nil {
				return err
			}
			if published > 0 {
				return ErrReadOnly
			}
		}
		expected := "active"
		if lifecycle == "active" {
			expected = "archived"
		}
		changed := tx.Model(&Draft{}).Where("article_id=? AND version=?", id, version).Update("version", version+1)
		if changed.Error != nil {
			return changed.Error
		}
		if changed.RowsAffected != 1 {
			return ErrConflict
		}
		changed = tx.Model(&Article{}).Where("id=? AND lifecycle=?", id, expected).Update("lifecycle", lifecycle)
		if changed.Error != nil {
			return changed.Error
		}
		if changed.RowsAffected != 1 {
			return ErrReadOnly
		}
		if err := audit.RecordOn(ctx, tx, audit.Event{Action: "UPDATE", Resource: "content.article.lifecycle", ResourceID: id, Metadata: meta}); err != nil {
			return err
		}
		var err error
		result, err = readDetail(tx, id)
		return err
	})
	return result, err
}
