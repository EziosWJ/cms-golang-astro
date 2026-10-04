package publishing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/audit"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/authz"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/content"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/media"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/siteconfig"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/taxonomy"
	"gorm.io/gorm"
	"math"
	"strings"
	"time"
	"unicode"
)

type Service struct {
	DB            *gorm.DB
	RuntimeRoot   string
	SecurePreview bool
	Executor      *Executor
}

func NewService(db *gorm.DB, root string, secure ...bool) *Service {
	required := len(secure) > 0 && secure[0]
	return &Service{DB: db, RuntimeRoot: root, SecurePreview: required}
}
func permission(kind string) string {
	if kind == "config" {
		return "content:config:publish"
	}
	if kind == "preview" {
		return content.EditPermission
	}
	return "content:publish"
}
func (s *Service) authorize(ctx context.Context, actor int64, kind string) error {
	ok, err := authz.Allowed(ctx, s.DB, actor, permission(kind))
	if err != nil {
		return err
	}
	if !ok {
		return authz.ErrForbidden
	}
	return nil
}
func lockState(db *gorm.DB) (State, error) {
	var state State
	err := db.Model(&State{}).Where("id=1").Update("version", gorm.Expr("version+1")).Error
	if err != nil {
		return state, err
	}
	err = db.Where("id=1").Take(&state).Error
	if err == nil && state.BlockedReason != "" {
		err = ErrBlocked
	}
	return state, err
}
func requestHash(value any) string {
	bytes, _ := json.Marshal(value)
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:])
}
func validKey(key string) bool {
	if key == "" || len(key) > 200 {
		return false
	}
	for _, r := range key {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func existing(db *gorm.DB, key, hash string) (Task, bool, error) {
	var request Request
	err := db.Where("key=?", key).Take(&request).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Task{}, false, nil
	}
	if err != nil {
		return Task{}, false, err
	}
	if request.RequestHash != hash {
		return Task{}, true, ErrConflict
	}
	var task Task
	err = db.Where("id=?", request.TaskID).Take(&task).Error
	return task, true, err
}
func (s *Service) Submit(ctx context.Context, meta audit.Metadata, key string, in SubmitInput) (Task, error) {
	if !validKey(key) || !strings.Contains("|config|article|unpublish|preview|", "|"+in.Kind+"|") {
		return Task{}, ErrInvalid
	}
	if err := s.authorize(ctx, meta.ActorID, in.Kind); err != nil {
		return Task{}, err
	}
	hash := requestHash(in)
	var task Task
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		state, err := lockState(tx)
		if err != nil {
			return err
		}
		prior, found, err := existing(tx, key, hash)
		if err != nil {
			return err
		}
		if found {
			task = prior
			return nil
		}
		if s.Executor != nil {
			if err := s.Executor.Admission(); err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		task = Task{Kind: in.Kind, Status: "queued", CreatedAt: now, UpdatedAt: now, CreatedBy: meta.ActorID}
		if in.Kind == "config" {
			var revision siteconfig.Revision
			if in.ConfigRevisionID < 1 {
				return ErrInvalid
			}
			if err := tx.Where("id=?", in.ConfigRevisionID).Take(&revision).Error; err != nil {
				return ErrInvalid
			}
			if len(siteconfig.Validate(revision.Data)) > 0 {
				return ErrInvalid
			}
			task.ConfigRevisionID = &revision.ID
		} else {
			if state.CurrentReleaseID == nil {
				return ErrNoBaseline
			}
			if in.ArticleID < 1 {
				return ErrInvalid
			}
			task.ArticleID = &in.ArticleID
			if in.Kind == "unpublish" {
				var article content.Article
				if err := tx.Where("id=? AND lifecycle='active'", in.ArticleID).Take(&article).Error; err != nil {
					return ErrInvalid
				}
				var current PublishedArticle
				if err := tx.Where("article_id=?", in.ArticleID).Take(&current).Error; err != nil {
					return ErrInvalid
				}
			} else {
				if strings.TrimSpace(in.Save.Title) == "" || strings.TrimSpace(in.Save.Markdown) == "" || in.Save.Slug == "" {
					return ErrInvalid
				}
				in.Save.Mode = "manual"
				if in.Kind == "preview" {
					in.Save.Mode = "autosave"
				}
				if in.Kind == "article" && in.Save.DisplayDate == nil {
					date := now
					in.Save.DisplayDate = &date
				}
				saved, err := content.NewService(content.NewRepository(tx)).Save(ctx, meta, in.ArticleID, in.Save)
				if err != nil {
					return err
				}
				task.RevisionID = saved.RevisionID
				if in.Kind == "preview" {
					snapshot := content.Revision{ArticleID: saved.ID, Version: saved.Draft.Version, Title: saved.Draft.Title, Markdown: saved.Draft.Markdown, Summary: saved.Draft.Summary, Slug: saved.Slug, DisplayDate: saved.Draft.DisplayDate, CoverMediaID: saved.Draft.CoverMediaID, Taxonomy: saved.Draft.Taxonomy, CreatedAt: now, CreatedBy: meta.ActorID}
					bytes, err := json.Marshal(snapshot)
					if err != nil {
						return err
					}
					task.TargetSnapshot = string(bytes)
				}
			}
		}
		if err := tx.Create(&task).Error; err != nil {
			return err
		}
		if err := validateTarget(tx, task); err != nil {
			return err
		}
		if err := targetReferences(tx, task); err != nil {
			return err
		}
		if err := tx.Create(&Request{Key: key, RequestHash: hash, TaskID: task.ID, CreatedAt: now}).Error; err != nil {
			return err
		}
		return audit.RecordOn(ctx, tx, audit.Event{Action: "CREATE", Resource: "publishing.task", ResourceID: task.ID, Metadata: meta})
	})
	return task, err
}
func (s *Service) Retry(ctx context.Context, meta audit.Metadata, key string, id int64) (Task, error) {
	if !validKey(key) {
		return Task{}, ErrInvalid
	}
	var target Task
	if err := s.DB.WithContext(ctx).Where("id=?", id).Take(&target).Error; err != nil {
		return Task{}, err
	}
	if err := s.authorize(ctx, meta.ActorID, target.Kind); err != nil {
		return Task{}, err
	}
	hash := requestHash(struct{ Retry int64 }{id})
	var task Task
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockState(tx); err != nil {
			return err
		}
		prior, found, err := existing(tx, key, hash)
		if err != nil {
			return err
		}
		if found {
			task = prior
			return nil
		}
		if err := tx.Where("id=?", id).Take(&task).Error; err != nil {
			return err
		}
		if s.Executor != nil {
			if err := s.Executor.Admission(); err != nil {
				return err
			}
		}
		if task.Status != "failed" && task.Status != "interrupted" {
			return ErrConflict
		}
		if err := validateTarget(tx, task); err != nil {
			return err
		}
		if err := targetReferences(tx, task); err != nil {
			return err
		}
		now := time.Now().UTC()
		if err := tx.Model(&task).Updates(map[string]any{"status": "queued", "error": "", "updated_at": now}).Error; err != nil {
			return err
		}
		task.Status = "queued"
		task.Error = ""
		task.UpdatedAt = now
		if err := tx.Create(&Request{Key: key, RequestHash: hash, TaskID: id, CreatedAt: now}).Error; err != nil {
			return err
		}
		return audit.RecordOn(ctx, tx, audit.Event{Action: "UPDATE", Resource: "publishing.retry", ResourceID: id, Metadata: meta})
	})
	return task, err
}
func validateTarget(tx *gorm.DB, task Task) error {
	if task.Kind == "config" {
		if task.ConfigRevisionID == nil {
			return ErrInvalid
		}
		var config siteconfig.Revision
		if err := tx.Where("id=?", *task.ConfigRevisionID).Take(&config).Error; err != nil {
			return err
		}
		if len(siteconfig.Validate(config.Data)) > 0 {
			return ErrInvalid
		}
		_, err := media.Resolve(tx, "", config.Data.AvatarMediaID)
		return err
	}
	if task.ArticleID == nil {
		return ErrInvalid
	}
	var article content.Article
	if err := tx.Where("id=?", *task.ArticleID).Take(&article).Error; err != nil {
		return err
	}
	if article.Lifecycle != "active" {
		return ErrConflict
	}
	if task.Kind == "unpublish" {
		var current PublishedArticle
		if err := tx.Where("article_id=?", task.ArticleID).Take(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrConflict
			}
			return err
		}
		return nil
	}
	var rev content.Revision
	if task.Kind == "preview" {
		if err := json.Unmarshal([]byte(task.TargetSnapshot), &rev); err != nil {
			return ErrInvalid
		}
	} else {
		if task.RevisionID == nil {
			return ErrInvalid
		}
		if err := tx.Where("id=? AND article_id=?", *task.RevisionID, *task.ArticleID).Take(&rev).Error; err != nil {
			return err
		}
	}
	if (task.Kind != "preview" && (article.Slug == nil || *article.Slug != rev.Slug)) || rev.Slug == "" || strings.TrimSpace(rev.Title) == "" || strings.TrimSpace(rev.Markdown) == "" {
		return ErrConflict
	}
	if err := taxonomy.ValidateSnapshots(tx, rev.Taxonomy); err != nil {
		return err
	}
	for _, snap := range rev.Taxonomy {
		var term taxonomy.Term
		if err := tx.Where("id=?", snap.ID).Take(&term).Error; err != nil {
			return err
		}
		if task.Kind != "preview" && (term.Name != snap.Name || term.URL != snap.URL) {
			return taxonomy.ErrConflict
		}
	}
	_, err := media.Resolve(tx, rev.Markdown, rev.CoverMediaID)
	return err
}
func targetReferences(tx *gorm.DB, task Task) error {
	if task.Kind == "config" {
		var revision siteconfig.Revision
		if err := tx.Where("id=?", task.ConfigRevisionID).Take(&revision).Error; err != nil {
			return err
		}
		files, err := media.Resolve(tx, "", revision.Data.AvatarMediaID)
		if err != nil {
			return err
		}
		return media.SetReferences(tx, "task", task.ID, files)
	}
	if task.RevisionID == nil && task.Kind != "preview" {
		return nil
	}
	var revision content.Revision
	if task.Kind == "preview" {
		if err := json.Unmarshal([]byte(task.TargetSnapshot), &revision); err != nil {
			return err
		}
	} else if err := tx.Where("id=?", task.RevisionID).Take(&revision).Error; err != nil {
		return err
	}
	files, err := media.Resolve(tx, revision.Markdown, revision.CoverMediaID)
	if err != nil {
		return err
	}
	if err := media.SetReferences(tx, "task", task.ID, files); err != nil {
		return err
	}
	owner := "task"
	if task.Kind == "preview" {
		owner = "preview_task"
	}
	return taxonomy.SetReferences(tx, owner, task.ID, revision.Taxonomy)
}
func clearTarget(tx *gorm.DB, id int64) error {
	if err := media.SetReferences(tx, "task", id, nil); err != nil {
		return err
	}
	if err := taxonomy.SetReferences(tx, "preview_task", id, nil); err != nil {
		return err
	}
	return taxonomy.SetReferences(tx, "task", id, nil)
}
func (s *Service) Detail(ctx context.Context, actor, id int64) (Task, error) {
	var task Task
	if err := s.DB.WithContext(ctx).Where("id=?", id).Take(&task).Error; err != nil {
		return task, err
	}
	if err := s.authorize(ctx, actor, task.Kind); err != nil {
		return Task{}, err
	}
	task.Attempts = []Attempt{}
	err := s.DB.WithContext(ctx).Where("task_id=?", id).Order("id ASC").Find(&task.Attempts).Error
	return task, err
}

type Page struct {
	Records  []Task `json:"records"`
	Total    int64  `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}

func (s *Service) Page(ctx context.Context, actor int64, page, size int) (Page, error) {
	p := Page{Records: []Task{}, Page: page, PageSize: size}
	if page < 1 || size < 1 || size > 100 || page > math.MaxInt/size {
		return p, ErrInvalid
	}
	kinds := []string{}
	for _, kind := range []string{"article", "unpublish", "preview", "config"} {
		ok, err := authz.Allowed(ctx, s.DB, actor, permission(kind))
		if err != nil {
			return p, err
		}
		if ok {
			kinds = append(kinds, kind)
		}
	}
	if len(kinds) == 0 {
		return p, authz.ErrForbidden
	}
	q := s.DB.WithContext(ctx).Model(&Task{}).Where("kind IN ?", kinds)
	if err := q.Count(&p.Total).Error; err != nil {
		return p, err
	}
	err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&p.Records).Error
	return p, err
}
