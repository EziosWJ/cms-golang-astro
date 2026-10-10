package gitexport

import (
	"encoding/json"
	"errors"
	"io"
	"strconv"

	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/audit"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/auth"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/authz"
	platform "github.com/EziosWJ/cms-golang-astro/cms-api/internal/platform/http"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct{ Service *Service }

func NewHandler(s *Service) *Handler { return &Handler{s} }
func (h *Handler) Register(r gin.IRouter) {

	pushes := r.Group("/git-pushes")
	pushes.GET("", authz.Require(h.Service.DB, "integration:git:view"), func(c *gin.Context) {
		page, _ := strconv.Atoi(c.Query("page"))
		size, _ := strconv.Atoi(c.Query("pageSize"))
		v, e := h.Service.List(c.Request.Context(), page, size)
		reply(c, v, e)
	})
	pushes.GET("/:id", authz.Require(h.Service.DB, "integration:git:view"), func(c *gin.Context) {
		id, e := strconv.ParseInt(c.Param("id"), 10, 64)
		if e != nil || id < 1 {
			reply(c, nil, ErrInvalid)
			return
		}
		v, e := h.Service.Detail(c.Request.Context(), id)
		reply(c, v, e)
	})
	pushes.POST("", authz.Require(h.Service.DB, "integration:git:push"), func(c *gin.Context) {
		v, e := h.Service.Submit(c.Request.Context(), metadata(c), c.GetHeader("Idempotency-Key"))
		reply(c, v, e)
	})
	pushes.POST("/:id/retry", authz.Require(h.Service.DB, "integration:git:push"), func(c *gin.Context) {
		id, e := strconv.ParseInt(c.Param("id"), 10, 64)
		if e != nil || id < 1 {
			reply(c, nil, ErrInvalid)
			return
		}
		v, e := h.Service.Retry(c.Request.Context(), metadata(c), id, c.GetHeader("Idempotency-Key"))
		reply(c, v, e)
	})
	g := r.Group("/git-connection")
	g.Use(authz.Require(h.Service.DB, "integration:git:manage"))
	g.GET("", func(c *gin.Context) { v, e := h.Service.Read(c.Request.Context()); reply(c, v, e) })
	g.PUT("", func(c *gin.Context) {
		var in ConnectionInput
		if !decode(c, &in) {
			return
		}
		v, e := h.Service.Save(c.Request.Context(), in, metadata(c))
		reply(c, v, e)
	})
	g.DELETE("", func(c *gin.Context) { reply(c, nil, h.Service.Disconnect(c.Request.Context(), metadata(c))) })
	g.POST("/detect", func(c *gin.Context) {
		var in struct {
			Token string `json:"token"`
		}
		if !decode(c, &in) {
			return
		}
		token, e := h.Service.token(c.Request.Context(), in.Token)
		if e != nil {
			reply(c, nil, e)
			return
		}
		v, e := h.Service.GitHub.Detect(c.Request.Context(), token)
		reply(c, v, e)
	})
	g.POST("/refs", func(c *gin.Context) {
		var in struct {
			Token      string `json:"token"`
			Repository string `json:"repository"`
			Kind       string `json:"kind"`
			Source     bool   `json:"source"`
		}
		if !decode(c, &in) {
			return
		}
		token := ""
		var e error
		token, e = h.Service.token(c.Request.Context(), in.Token)
		if in.Source && errors.Is(e, ErrCredential) {
			token = ""
			e = nil
		}
		if e != nil {
			reply(c, nil, e)
			return
		}
		v, e := h.Service.GitHub.Options(c.Request.Context(), token, in.Repository, in.Kind)
		reply(c, v, e)
	})
}
func decode(c *gin.Context, out any) bool {
	d := json.NewDecoder(io.LimitReader(c.Request.Body, 16*1024))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil || d.Decode(new(any)) != io.EOF {
		platform.WriteError(c, 400, 400, "Git 请求参数无效", nil)
		return false
	}
	return true
}
func metadata(c *gin.Context) audit.Metadata {
	p, _ := auth.PrincipalFromContext(c.Request.Context())
	m, _ := platform.RequestMetaFromContext(c.Request.Context())
	return audit.Metadata{ActorID: p.UserID, RequestID: m.RequestID, RequestMethod: c.Request.Method, RequestURL: c.Request.URL.Path}
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
	status := 400
	if errors.Is(err, gorm.ErrRecordNotFound) {
		platform.WriteError(c, 404, 404, "Git 推送任务不存在", nil)
		return
	}
	message := "Git 操作失败，请检查配置后重试"
	var remote remoteError
	if errors.Is(err, ErrPublic) || errors.Is(err, ErrPushPermission) || errors.Is(err, ErrBranch) || errors.Is(err, ErrBaseline) || errors.Is(err, ErrCredential) || errors.Is(err, ErrInvalid) || errors.As(err, &remote) || errors.Is(err, ErrRemote) {
		message = err.Error()
	} else if errors.Is(err, ErrConflict) {
		status = 409
		message = err.Error()
	}
	platform.WriteError(c, status, status, message, nil)
}
