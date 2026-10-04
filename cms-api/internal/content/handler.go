package content

import (
	"encoding/json"
	"errors"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/media"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/taxonomy"
	"io"
	"net/http"
	"strconv"

	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/audit"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/auth"
	platform "github.com/EziosWJ/cms-golang-astro/cms-api/internal/platform/http"
	"github.com/gin-gonic/gin"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }
func (h *Handler) Register(r gin.IRouter) {
	g := r.Group("/articles")
	g.GET("", h.page)
	g.GET("/:id", h.detail)
	g.POST("", h.create)
	g.POST("/:id/lifecycle", h.lifecycle)
	g.PUT("/:id/draft", h.save)
	g.GET("/:id/revisions", h.revisions)
	g.GET("/:id/revisions/:revisionID", h.revision)
	g.POST("/:id/revisions/:revisionID/restore", h.restore)
}
func (h *Handler) page(c *gin.Context) {
	q := Query{Page: intQuery(c, "page", 1), PageSize: intQuery(c, "pageSize", 10), Title: c.Query("title"), Lifecycle: c.Query("lifecycle")}
	v, err := h.service.Page(c.Request.Context(), metadata(c).ActorID, q)
	respond(c, v, err)
}
func (h *Handler) detail(c *gin.Context) {
	id, ok := articleID(c)
	if !ok {
		return
	}
	v, err := h.service.Detail(c.Request.Context(), metadata(c).ActorID, id)
	respond(c, v, err)
}
func (h *Handler) create(c *gin.Context) {
	var in DraftInput
	if !decode(c, &in) {
		return
	}
	v, err := h.service.Create(c.Request.Context(), metadata(c), in)
	respond(c, v, err)
}
func (h *Handler) save(c *gin.Context) {
	id, ok := articleID(c)
	if !ok {
		return
	}
	var in SaveInput
	if !decode(c, &in) {
		return
	}
	v, err := h.service.Save(c.Request.Context(), metadata(c), id, in)
	respond(c, v, err)
}
func decode(c *gin.Context, target any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 6*1024*1024)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		respond(c, nil, ErrInvalid)
		return false
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		respond(c, nil, ErrInvalid)
		return false
	}
	return true
}
func articleID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		respond(c, nil, ErrInvalid)
		return 0, false
	}
	return id, true
}
func intQuery(c *gin.Context, key string, fallback int) int {
	if c.Query(key) == "" {
		return fallback
	}
	n, err := strconv.Atoi(c.Query(key))
	if err != nil {
		return 0
	}
	return n
}
func metadata(c *gin.Context) audit.Metadata {
	p, _ := auth.PrincipalFromContext(c.Request.Context())
	m, _ := platform.RequestMetaFromContext(c.Request.Context())
	return audit.Metadata{ActorID: p.UserID, RequestID: m.RequestID, ClientIP: m.ClientIP, UserAgent: m.UserAgent, RequestMethod: c.Request.Method, RequestURL: c.Request.URL.RequestURI()}
}
func respond(c *gin.Context, data any, err error) {
	if err == nil {
		platform.OK(c, data)
		return
	}
	status := http.StatusInternalServerError
	message := "系统错误"
	var errorData any
	switch {
	case platform.IsTemporaryUnavailable(err):
		platform.TemporaryUnavailable(c)
		return
	case errors.Is(err, ErrForbidden):
		status, message = http.StatusForbidden, err.Error()
	case errors.Is(err, ErrNotFound):
		status, message = http.StatusNotFound, err.Error()
	case errors.Is(err, ErrConflict):
		status, message = http.StatusConflict, err.Error()
		errorData = map[string]string{"reason": "version_conflict"}
	case errors.Is(err, ErrSlugTaken), errors.Is(err, ErrReadOnly), errors.Is(err, taxonomy.ErrConflict):
		status, message = http.StatusConflict, err.Error()
	case errors.Is(err, ErrInvalid), errors.Is(err, ErrSlugInvalid), errors.Is(err, taxonomy.ErrInvalid), errors.Is(err, media.ErrInvalid):
		status, message = http.StatusBadRequest, err.Error()
	}
	platform.WriteError(c, status, status, message, errorData)
}

func (h *Handler) revisions(c *gin.Context) {
	id, ok := articleID(c)
	if !ok {
		return
	}
	v, err := h.service.Revisions(c.Request.Context(), metadata(c).ActorID, id, Query{Page: intQuery(c, "page", 1), PageSize: intQuery(c, "pageSize", 20)})
	respond(c, v, err)
}
func revisionID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("revisionID"), 10, 64)
	if err != nil || id < 1 {
		respond(c, nil, ErrInvalid)
		return 0, false
	}
	return id, true
}
func (h *Handler) revision(c *gin.Context) {
	id, ok := articleID(c)
	if !ok {
		return
	}
	rid, ok := revisionID(c)
	if !ok {
		return
	}
	v, err := h.service.Revision(c.Request.Context(), metadata(c).ActorID, id, rid)
	respond(c, v, err)
}
func (h *Handler) restore(c *gin.Context) {
	id, ok := articleID(c)
	if !ok {
		return
	}
	rid, ok := revisionID(c)
	if !ok {
		return
	}
	var in struct {
		ExpectedVersion *int64 `json:"expectedVersion"`
	}
	if !decode(c, &in) {
		return
	}
	v, err := h.service.Restore(c.Request.Context(), metadata(c), id, rid, in.ExpectedVersion)
	respond(c, v, err)
}

func (h *Handler) lifecycle(c *gin.Context) {
	id, ok := articleID(c)
	if !ok {
		return
	}
	var in struct {
		Lifecycle       string `json:"lifecycle"`
		ExpectedVersion *int64 `json:"expectedVersion"`
	}
	if !decode(c, &in) {
		return
	}
	v, e := h.service.Lifecycle(c.Request.Context(), metadata(c), id, in.Lifecycle, in.ExpectedVersion)
	respond(c, v, e)
}
