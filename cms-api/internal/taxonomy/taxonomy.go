// Package taxonomy owns flat categories/tags and immutable name snapshots.
package taxonomy

import (
	"errors"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/audit"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/auth"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/authz"
	platform "github.com/EziosWJ/cms-golang-astro/cms-api/internal/platform/http"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var ErrConflict = errors.New("分类标签已变更、被引用或身份已锁定")
var ErrInvalid = errors.New("分类标签名称、URL 或版本无效")

type Term struct {
	ID       int64      `gorm:"primaryKey" json:"id"`
	Kind     string     `json:"kind"`
	Name     string     `json:"name"`
	URL      string     `json:"url"`
	Version  int64      `json:"version"`
	LockedAt *time.Time `json:"lockedAt"`
}

func (Term) TableName() string { return "cms_taxonomy" }

type Snapshot struct {
	ID   int64  `json:"id"`
	Kind string `json:"kind"`
	Name string `json:"name"`
	URL  string `json:"url"`
}
type Reference struct {
	OwnerType string `gorm:"primaryKey"`
	OwnerID   int64  `gorm:"primaryKey"`
	TermID    int64  `gorm:"primaryKey"`
}

func (Reference) TableName() string { return "cms_taxonomy_ref" }

// Resolve locks selected identities before snapshots or references are written.
func Resolve(tx *gorm.DB, categories, tags []int64) ([]Snapshot, error) {
	out := []Snapshot{}
	seen := map[int64]bool{}
	for _, selection := range []struct {
		kind string
		ids  []int64
	}{{"category", categories}, {"tag", tags}} {
		if len(selection.ids) > 100 {
			return nil, ErrInvalid
		}
		for _, id := range selection.ids {
			if id < 1 || seen[id] {
				return nil, ErrInvalid
			}
			seen[id] = true
			var term Term
			q := tx
			if tx.Dialector.Name() == "postgres" {
				q = q.Clauses(clause.Locking{Strength: "UPDATE"})
			}
			if err := q.Where("id=? AND kind=?", id, selection.kind).Take(&term).Error; err != nil {
				return nil, ErrInvalid
			}
			out = append(out, Snapshot{term.ID, term.Kind, term.Name, term.URL})
		}
	}
	return out, nil
}
func ValidateSnapshots(tx *gorm.DB, terms []Snapshot) error {
	for _, snap := range terms {
		var term Term
		q := tx
		if tx.Dialector.Name() == "postgres" {
			q = q.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := q.Where("id=? AND kind=?", snap.ID, snap.Kind).Take(&term).Error; err != nil {
			return ErrConflict
		}
		if term.LockedAt != nil && (term.Name != snap.Name || term.URL != snap.URL) {
			return ErrConflict
		}
	}
	return nil
}
func SetReferences(tx *gorm.DB, owner string, id int64, terms []Snapshot) error {
	if err := tx.Where("owner_type=? AND owner_id=?", owner, id).Delete(&Reference{}).Error; err != nil {
		return err
	}
	for _, term := range terms {
		if err := tx.Create(&Reference{owner, id, term.ID}).Error; err != nil {
			return err
		}
	}
	return nil
}

type Handler struct{ db *gorm.DB }

func NewHandler(db *gorm.DB) *Handler { return &Handler{db} }
func (h *Handler) Register(r gin.IRouter) {
	for _, kind := range []string{"category", "tag"} {
		path := "/categories"
		if kind == "tag" {
			path = "/tags"
		}
		g := r.Group(path)
		g.GET("", authz.RequireAny(h.db, "content:taxonomy:edit", "content:article:edit"), h.list(kind))
		g.POST("", authz.Require(h.db, "content:taxonomy:edit"), h.write(kind, false))
		g.PUT("/:id", authz.Require(h.db, "content:taxonomy:edit"), h.write(kind, true))
		g.DELETE("/:id", authz.Require(h.db, "content:taxonomy:edit"), h.remove(kind))
	}
}
func (h *Handler) list(kind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		out := []Term{}
		err := h.db.WithContext(c.Request.Context()).Where("kind=?", kind).Order("name ASC, id ASC").Find(&out).Error
		reply(c, out, err)
	}
}
func valid(name, path string) bool {
	if strings.TrimSpace(name) == "" || utf8.RuneCountInString(name) > 200 || path == "" || utf8.RuneCountInString(path) > 200 || path == "." || path == ".." || strings.ContainsAny(path, "/\\%?#") {
		return false
	}
	if _, err := url.PathUnescape(path); err != nil {
		return false
	}
	for _, r := range name + path {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return false
		}
	}
	return true
}
func (h *Handler) write(kind string, edit bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in struct {
			Name            string `json:"name"`
			URL             string `json:"url"`
			ExpectedVersion int64  `json:"expectedVersion"`
		}
		if c.ShouldBindJSON(&in) != nil || !valid(in.Name, in.URL) {
			reply(c, nil, ErrInvalid)
			return
		}
		id, parseErr := strconv.ParseInt(c.Param("id"), 10, 64)
		if edit && (parseErr != nil || id < 1 || in.ExpectedVersion < 1) {
			reply(c, nil, ErrInvalid)
			return
		}
		term := Term{Kind: kind, Name: in.Name, URL: in.URL, Version: 1}
		err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			if !edit {
				if err := tx.Create(&term).Error; err != nil {
					return err
				}
			} else {
				result := tx.Model(&Term{}).Where("id=? AND kind=? AND version=? AND locked_at IS NULL", id, kind, in.ExpectedVersion).Updates(map[string]any{"name": in.Name, "url": in.URL, "version": gorm.Expr("version+1")})
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected != 1 {
					return ErrConflict
				}
				var pending int64
				if err := tx.Model(&Reference{}).Where("term_id=? AND owner_type='task'", id).Count(&pending).Error; err != nil {
					return err
				}
				if pending > 0 {
					return ErrConflict
				}
				term.ID = id
				term.Version = in.ExpectedVersion + 1
			}
			p, _ := auth.PrincipalFromContext(c.Request.Context())
			meta, _ := platform.RequestMetaFromContext(c.Request.Context())
			action := "CREATE"
			if edit {
				action = "UPDATE"
			}
			return audit.RecordOn(c.Request.Context(), tx, audit.Event{Action: action, Resource: "content.taxonomy", ResourceID: term.ID, Metadata: audit.Metadata{ActorID: p.UserID, RequestID: meta.RequestID, RequestMethod: c.Request.Method, RequestURL: c.Request.URL.RequestURI()}})
		})
		reply(c, term, err)
	}
}
func (h *Handler) remove(kind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id < 1 {
			reply(c, nil, ErrInvalid)
			return
		}
		err = h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			// Obtain the identity lock before checking references, including SQLite's writer lock.
			result := tx.Model(&Term{}).Where("id=? AND kind=? AND locked_at IS NULL", id, kind).Update("version", gorm.Expr("version+1"))
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrConflict
			}
			var count int64
			if err := tx.Model(&Reference{}).Where("term_id=?", id).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return ErrConflict
			}
			if err := tx.Delete(&Term{}, id).Error; err != nil {
				return err
			}
			p, _ := auth.PrincipalFromContext(c.Request.Context())
			return audit.RecordOn(c.Request.Context(), tx, audit.Event{Action: "DELETE", Resource: "content.taxonomy", ResourceID: id, Metadata: audit.Metadata{ActorID: p.UserID, RequestMethod: c.Request.Method, RequestURL: c.Request.URL.RequestURI()}})
		})
		reply(c, nil, err)
	}
}
func reply(c *gin.Context, data any, err error) {
	if err == nil {
		platform.OK(c, data)
		return
	}
	if platform.IsTemporaryUnavailable(err) {
		platform.TemporaryUnavailable(c)
		return
	}
	status := 500
	message := "系统错误"
	if errors.Is(err, ErrInvalid) {
		status = 400
		message = err.Error()
	}
	if errors.Is(err, ErrConflict) || errors.Is(err, gorm.ErrDuplicatedKey) {
		status = 409
		message = ErrConflict.Error()
	}
	platform.WriteError(c, status, status, message, nil)
}
