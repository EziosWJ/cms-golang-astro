package media

import (
	"errors"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/authz"
	platform "github.com/EziosWJ/cms-golang-astro/cms-api/internal/platform/http"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"strconv"
)

type Handler struct{ db *gorm.DB }

func NewHandler(db *gorm.DB) *Handler { return &Handler{db} }
func (h *Handler) Register(r gin.IRouter) {
	g := r.Group("/media")
	g.Use(authz.RequireAny(h.db, "content:media:edit", "content:article:edit", "content:config:edit"))
	g.GET("", h.list)
	g.GET("/:id", h.detail)
	g.GET("/:id/references", h.references)
}
func (h *Handler) list(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 || size < 1 || size > 100 || page > 1000000 {
		platform.WriteError(c, 400, 400, "分页参数无效", nil)
		return
	}
	query := h.db.WithContext(c.Request.Context()).Model(&File{}).Where("deleted=0")
	if name := c.Query("name"); name != "" {
		query = query.Where("original_name LIKE ?", "%"+name+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		reply(c, err)
		return
	}
	files := []File{}
	if err := query.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&files).Error; err != nil {
		reply(c, err)
		return
	}
	records := []any{}
	for _, f := range files {
		records = append(records, struct {
			File
			StablePath string `json:"stablePath"`
			Image      bool   `json:"image"`
		}{f, Path(f), Image(f)})
	}
	platform.OK(c, map[string]any{"records": records, "total": total, "page": page, "pageSize": size})
}
func (h *Handler) detail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		platform.WriteError(c, 400, 400, ErrInvalid.Error(), nil)
		return
	}
	var f File
	if err := h.db.WithContext(c.Request.Context()).Where("id=? AND deleted=0", id).Take(&f).Error; err != nil {
		reply(c, err)
		return
	}
	platform.OK(c, struct {
		File
		StablePath string `json:"stablePath"`
		Image      bool   `json:"image"`
	}{f, Path(f), Image(f)})
}
func reply(c *gin.Context, err error) {
	if platform.IsTemporaryUnavailable(err) {
		platform.TemporaryUnavailable(c)
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		platform.WriteError(c, 404, 404, "媒体不存在", nil)
	} else {
		platform.WriteError(c, 500, 500, "媒体读取失败", nil)
	}
}

func (h *Handler) UploadGuard() gin.HandlerFunc { return authz.Require(h.db, "content:media:edit") }

func (h *Handler) references(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		platform.WriteError(c, 400, 400, ErrInvalid.Error(), nil)
		return
	}
	var file File
	if err := h.db.WithContext(c.Request.Context()).Where("id=? AND deleted=0", id).Take(&file).Error; err != nil {
		reply(c, err)
		return
	}
	rows := []struct {
		OwnerType string `json:"ownerType"`
		OwnerID   int64  `json:"ownerId"`
		ArticleID *int64 `json:"articleId"`
		Title     string `json:"title"`
	}{}
	if err := h.db.WithContext(c.Request.Context()).Table("cms_media_ref ref").Select("ref.owner_type, ref.owner_id, COALESCE(r.article_id,d.article_id) AS article_id, COALESCE(r.title,d.title,'') AS title").Joins("LEFT JOIN cms_article_revision r ON ref.owner_type='revision' AND r.id=ref.owner_id").Joins("LEFT JOIN cms_working_draft d ON ref.owner_type='draft' AND d.article_id=ref.owner_id").Where("ref.file_id=?", id).Order("ref.owner_type,ref.owner_id").Find(&rows).Error; err != nil {
		reply(c, err)
		return
	}
	platform.OK(c, rows)
}
