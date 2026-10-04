// Package siteconfig owns working site/author data and immutable config revisions.
package siteconfig

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/audit"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/auth"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/authz"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/media"
	platform "github.com/EziosWJ/cms-golang-astro/cms-api/internal/platform/http"
	"github.com/gin-gonic/gin"
	"golang.org/x/text/language"
	"gorm.io/gorm"
	"math"
	"net/url"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode/utf8"
)

type Data struct {
	SiteName      string `json:"siteName"`
	Description   string `json:"description"`
	PublicURL     string `json:"publicUrl"`
	Language      string `json:"language"`
	Timezone      string `json:"timezone"`
	AuthorName    string `json:"authorName"`
	AuthorBio     string `json:"authorBio"`
	AvatarMediaID *int64 `json:"avatarMediaId"`
	Theme         string `json:"theme"`
	ThemeVersion  string `json:"themeVersion"`
}
type Working struct {
	Published  *Revision `gorm:"-" json:"published"`
	ID         int64     `gorm:"primaryKey" json:"-"`
	Version    int64     `json:"version"`
	Data       Data      `gorm:"serializer:json" json:"data"`
	SavedAt    time.Time `json:"savedAt"`
	SavedBy    int64     `json:"savedBy"`
	RevisionID *int64    `gorm:"-" json:"revisionId"`
}

func (Working) TableName() string { return "cms_site_config" }

type Revision struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	Version   int64     `json:"version"`
	Data      Data      `gorm:"serializer:json" json:"data"`
	CreatedAt time.Time `json:"createdAt"`
	CreatedBy int64     `json:"createdBy"`
}

func (Revision) TableName() string { return "cms_config_revision" }

var ErrConflict = errors.New("站点配置已被修改，请保留本地修改并重新读取")

func Validate(in Data) map[string]string {
	fields := map[string]string{}
	if strings.TrimSpace(in.SiteName) == "" || utf8.RuneCountInString(in.SiteName) > 200 {
		fields["siteName"] = "站点名称须为 1–200 字符"
	}
	if strings.TrimSpace(in.AuthorName) == "" || utf8.RuneCountInString(in.AuthorName) > 200 {
		fields["authorName"] = "作者名称须为 1–200 字符"
	}
	if utf8.RuneCountInString(in.Description) > 5000 {
		fields["description"] = "简介最多 5000 字符"
	}
	if utf8.RuneCountInString(in.AuthorBio) > 5000 {
		fields["authorBio"] = "作者简介最多 5000 字符"
	}
	u, err := url.Parse(in.PublicURL)
	if err != nil || u == nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" && u.Path != "/" {
		fields["publicUrl"] = "公开地址须为 HTTP(S) 站点根地址，不含账号、查询或片段"
	}
	if _, err := language.Parse(in.Language); err != nil || in.Language == "" {
		fields["language"] = "语言标记无效"
	}
	if _, err := time.LoadLocation(in.Timezone); err != nil {
		fields["timezone"] = "时区无效"
	}
	themeID := strings.TrimSpace(in.Theme)
	if themeID == "" {
		themeID = DefaultThemeID
	}
	theme, ok := LookupTheme(themeID)
	if !ok {
		fields["theme"] = "请选择有效主题"
	} else if in.ThemeVersion != "" && in.ThemeVersion != theme.Version {
		fields["theme"] = "主题版本已变化，请重新保存站点配置"
	}
	return fields
}
func Read(ctx context.Context, db *gorm.DB) (Working, error) {
	var work Working
	err := db.WithContext(ctx).Where("id=1").Take(&work).Error
	if err != nil {
		return work, err
	}
	legacyTheme := strings.TrimSpace(work.Data.Theme) == "" || strings.TrimSpace(work.Data.ThemeVersion) == ""
	work.Data = NormalizeLegacyTheme(work.Data)
	var revision Revision
	if err := db.WithContext(ctx).Where("version=?", work.Version).Take(&revision).Error; err == nil {
		if !legacyTheme {
			work.RevisionID = &revision.ID
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return work, err
	}
	var release struct{ Manifest string }
	err = db.WithContext(ctx).Table("cms_release r").Select("r.manifest").Joins("JOIN cms_publish_state s ON s.current_release_id=r.id").Take(&release).Error
	if err == nil {
		var manifest struct {
			Config Revision `json:"config"`
		}
		if err := json.Unmarshal([]byte(release.Manifest), &manifest); err != nil {
			return work, err
		}
		work.Published = &manifest.Config
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return work, err
	}
	return work, nil
}
func Save(ctx context.Context, tx *gorm.DB, version int64, in Data, meta audit.Metadata) (Working, error) {
	var work Working
	resolved, err := ResolveTheme(in)
	if err != nil {
		return work, err
	}
	in = resolved
	err = tx.WithContext(ctx).Transaction(func(db *gorm.DB) error {
		now := time.Now().UTC()
		result := db.Model(&Working{}).Where("id=1 AND version=?", version).Updates(map[string]any{"version": version + 1, "saved_at": now, "saved_by": meta.ActorID})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrConflict
		}
		files, err := media.Resolve(db, "", in.AvatarMediaID)
		if err != nil {
			return err
		}
		work = Working{ID: 1, Version: version + 1, Data: in, SavedAt: now, SavedBy: meta.ActorID}
		if err := db.Model(&Working{}).Where("id=1").Select("Data").Updates(&work).Error; err != nil {
			return err
		}
		revision := Revision{Version: work.Version, Data: in, CreatedAt: now, CreatedBy: meta.ActorID}
		if err := db.Create(&revision).Error; err != nil {
			return err
		}
		work.RevisionID = &revision.ID
		if err := media.SetReferences(db, "config", 1, files); err != nil {
			return err
		}
		if err := media.SetReferences(db, "config_revision", revision.ID, files); err != nil {
			return err
		}
		return audit.RecordOn(ctx, db, audit.Event{Action: "UPDATE", Resource: "content.siteconfig", ResourceID: 1, Metadata: meta})
	})
	if err != nil {
		return work, err
	}
	return Read(ctx, tx)
}

type Handler struct{ db *gorm.DB }

func NewHandler(db *gorm.DB) *Handler { return &Handler{db} }
func (h *Handler) Register(r gin.IRouter) {
	r.GET("/site-config/editor-context", authz.RequireAny(h.db, "content:article:edit", "content:config:edit"), func(c *gin.Context) {
		work, err := Read(c.Request.Context(), h.db)
		reply(c, map[string]any{"siteName": work.Data.SiteName, "timezone": work.Data.Timezone, "published": work.Published}, err)
	})
	r.GET("/site-config/editor-timezone", authz.RequireAny(h.db, "content:article:edit", "content:config:edit"), func(c *gin.Context) {
		working, err := Read(c.Request.Context(), h.db)
		reply(c, map[string]string{"timezone": working.Data.Timezone}, err)
	})
	g := r.Group("/site-config")
	g.Use(authz.Require(h.db, "content:config:edit"))
	g.GET("", h.get)
	g.GET("/themes", h.themes)
	g.PUT("", h.save)
}
func (h *Handler) get(c *gin.Context) {
	work, err := Read(c.Request.Context(), h.db)
	reply(c, work, err)
}
func (h *Handler) themes(c *gin.Context) { platform.OK(c, Themes()) }
func (h *Handler) save(c *gin.Context) {
	var input struct {
		ExpectedVersion *int64 `json:"expectedVersion"`
		Data            Data   `json:"data"`
	}
	if c.ShouldBindJSON(&input) != nil || input.ExpectedVersion == nil || *input.ExpectedVersion < 1 || *input.ExpectedVersion == math.MaxInt64 {
		platform.WriteError(c, 400, 400, "配置参数无效", nil)
		return
	}
	resolved, err := ResolveTheme(input.Data)
	if err != nil {
		platform.WriteError(c, 400, 400, "配置校验失败", map[string]string{"theme": "请选择有效主题"})
		return
	}
	input.Data = resolved
	fields := Validate(input.Data)
	if len(fields) > 0 {
		platform.WriteError(c, 400, 400, "配置校验失败", fields)
		return
	}
	p, _ := auth.PrincipalFromContext(c.Request.Context())
	m, _ := platform.RequestMetaFromContext(c.Request.Context())
	work, err := Save(c.Request.Context(), h.db, *input.ExpectedVersion, input.Data, audit.Metadata{ActorID: p.UserID, RequestID: m.RequestID, RequestMethod: c.Request.Method, RequestURL: c.Request.URL.RequestURI()})
	reply(c, work, err)
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
	message := "配置操作失败"
	if errors.Is(err, ErrConflict) {
		status = 409
		message = err.Error()
	}
	if errors.Is(err, media.ErrInvalid) || errors.Is(err, ErrThemeInvalid) {
		status = 400
		message = err.Error()
	}
	platform.WriteError(c, status, status, message, nil)
}
