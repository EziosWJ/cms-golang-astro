package publishing

import (
	"encoding/json"
	"errors"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/audit"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/auth"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/authz"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/content"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/media"
	platform "github.com/EziosWJ/cms-golang-astro/cms-api/internal/platform/http"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/taxonomy"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"io"
	"net/http"
	"strconv"
)

type Handler struct{ Service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service} }
func (h *Handler) Register(r gin.IRouter) {
	r.GET("/publication-worker", authz.RequireAny(h.Service.DB, content.EditPermission, "content:publish"), h.workerStatus)
	r.POST("/publication-worker/pause", authz.Require(h.Service.DB, "content:publish"), h.workerPause)
	r.POST("/publication-worker/resume", authz.Require(h.Service.DB, "content:publish"), h.workerResume)
	r.GET("/publications", h.page)
	r.POST("/publications", h.submit)
	r.GET("/publications/:id", h.detail)
	r.POST("/publications/:id/retry", h.retry)
	r.POST("/previews/:id/access", h.previewAccess)
}
func publishingMeta(c *gin.Context) audit.Metadata {
	p, _ := auth.PrincipalFromContext(c.Request.Context())
	m, _ := platform.RequestMetaFromContext(c.Request.Context())
	return audit.Metadata{ActorID: p.UserID, RequestID: m.RequestID, ClientIP: m.ClientIP, UserAgent: m.UserAgent, RequestMethod: c.Request.Method, RequestURL: c.Request.URL.RequestURI()}
}
func publicationID(c *gin.Context) (int64, bool) {
	id, e := strconv.ParseInt(c.Param("id"), 10, 64)
	if e != nil || id < 1 {
		reply(c, nil, ErrInvalid)
		return 0, false
	}
	return id, true
}
func reply(c *gin.Context, data any, err error) {
	if err == nil {
		platform.OK(c, data)
		return
	}
	status, message := 500, "发布操作失败"
	switch {
	case platform.IsTemporaryUnavailable(err):
		platform.TemporaryUnavailable(c)
		return
	case errors.Is(err, authz.ErrForbidden), errors.Is(err, content.ErrForbidden):
		status, message = 403, err.Error()
	case errors.Is(err, gorm.ErrRecordNotFound), errors.Is(err, content.ErrNotFound):
		status, message = 404, "记录不存在"
	case errors.Is(err, ErrConflict), errors.Is(err, content.ErrConflict), errors.Is(err, content.ErrReadOnly), errors.Is(err, content.ErrSlugTaken), errors.Is(err, taxonomy.ErrConflict):
		status, message = 409, err.Error()
	case errors.Is(err, ErrNoBaseline), errors.Is(err, ErrInvalid), errors.Is(err, content.ErrInvalid), errors.Is(err, content.ErrSlugInvalid), errors.Is(err, taxonomy.ErrInvalid), errors.Is(err, media.ErrInvalid):
		status, message = 400, err.Error()
	case errors.Is(err, ErrExecutorUnavailable), errors.Is(err, ErrBlocked):
		status, message = 503, err.Error()
	}
	platform.WriteError(c, status, status, message, nil)
}
func (h *Handler) submit(c *gin.Context) {
	var in SubmitInput
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 6*1024*1024)
	d := json.NewDecoder(c.Request.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(&in); err != nil {
		reply(c, nil, ErrInvalid)
		return
	}
	if err := d.Decode(new(any)); !errors.Is(err, io.EOF) {
		reply(c, nil, ErrInvalid)
		return
	}
	task, err := h.Service.Submit(c.Request.Context(), publishingMeta(c), c.GetHeader("Idempotency-Key"), in)
	reply(c, task, err)
}
func (h *Handler) detail(c *gin.Context) {
	id, ok := publicationID(c)
	if !ok {
		return
	}
	v, e := h.Service.Detail(c.Request.Context(), publishingMeta(c).ActorID, id)
	reply(c, v, e)
}
func (h *Handler) retry(c *gin.Context) {
	id, ok := publicationID(c)
	if !ok {
		return
	}
	v, e := h.Service.Retry(c.Request.Context(), publishingMeta(c), c.GetHeader("Idempotency-Key"), id)
	reply(c, v, e)
}
func (h *Handler) page(c *gin.Context) {
	page, size := 1, 20
	var e error
	if v := c.Query("page"); v != "" {
		page, e = strconv.Atoi(v)
		if e != nil {
			reply(c, nil, ErrInvalid)
			return
		}
	}
	if v := c.Query("pageSize"); v != "" {
		size, e = strconv.Atoi(v)
		if e != nil {
			reply(c, nil, ErrInvalid)
			return
		}
	}
	v, e := h.Service.Page(c.Request.Context(), publishingMeta(c).ActorID, page, size)
	reply(c, v, e)
}

func (h *Handler) workerStatus(c *gin.Context) {
	if h.Service.Executor == nil {
		reply(c, ExecutorStatus{State: "disabled"}, nil)
		return
	}
	reply(c, h.Service.Executor.Status(), nil)
}
func (h *Handler) workerPause(c *gin.Context) {
	if h.Service.Executor == nil {
		reply(c, nil, ErrConflict)
		return
	}
	err := h.Service.Executor.Pause()
	reply(c, h.Service.Executor.Status(), err)
}
func (h *Handler) workerResume(c *gin.Context) {
	if h.Service.Executor == nil {
		reply(c, nil, ErrConflict)
		return
	}
	err := h.Service.Executor.Resume()
	reply(c, h.Service.Executor.Status(), err)
}
