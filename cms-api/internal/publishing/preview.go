package publishing

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/auth"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/authz"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/builder"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/content"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/staticweb"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/url"
	"path/filepath"
	"time"
)

type PreviewAccess struct {
	TokenHash      string `gorm:"primaryKey"`
	TaskID, UserID int64
	JTI            string
	ExpiresAt      time.Time
}

func (PreviewAccess) TableName() string { return "cms_preview_access" }
func (h *Handler) previewAccess(c *gin.Context) {
	id, ok := publicationID(c)
	if !ok {
		return
	}
	p, _ := auth.PrincipalFromContext(c.Request.Context())
	task, err := h.Service.Detail(c.Request.Context(), p.UserID, id)
	if err != nil {
		reply(c, nil, err)
		return
	}
	allowed, err := authz.Allowed(c.Request.Context(), h.Service.DB, p.UserID, content.EditPermission)
	if err != nil {
		reply(c, nil, err)
		return
	}
	if !allowed {
		reply(c, nil, authz.ErrForbidden)
		return
	}
	if task.Kind != "preview" || task.Status != "succeeded" {
		reply(c, nil, ErrConflict)
		return
	}
	var attempt Attempt
	if err := h.Service.DB.Where("task_id=? AND status='succeeded'", id).Order("id DESC").Take(&attempt).Error; err != nil {
		reply(c, nil, err)
		return
	}
	if err := builder.Validate(filepath.Join(h.Service.RuntimeRoot, "previews", attempt.ReleaseKey), builder.Marker{AttemptID: attempt.ID, ReleaseKey: attempt.ReleaseKey, ManifestHash: attempt.ManifestHash}); err != nil {
		reply(c, nil, ErrInvalid)
		return
	}
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		reply(c, nil, err)
		return
	}
	token := hex.EncodeToString(bytes)
	hash := sha256.Sum256([]byte(token))
	expires := time.Now().UTC().Add(10 * time.Minute)
	if p.ExpiresAt.Before(expires) {
		expires = p.ExpiresAt
	}
	if err := h.Service.DB.Create(&PreviewAccess{TokenHash: hex.EncodeToString(hash[:]), TaskID: id, UserID: p.UserID, JTI: p.JTI, ExpiresAt: expires}).Error; err != nil {
		reply(c, nil, err)
		return
	}
	base := fmt.Sprintf("/api/v1/previews/%d/", id)
	http.SetCookie(c.Writer, &http.Cookie{Name: "cms_preview", Value: token, Path: base, HttpOnly: true, Secure: h.Service.SecurePreview || c.Request.TLS != nil, SameSite: http.SameSiteStrictMode, Expires: expires, MaxAge: int(time.Until(expires).Seconds())})
	var snapshot content.Revision
	if err := json.Unmarshal([]byte(task.TargetSnapshot), &snapshot); err != nil {
		reply(c, nil, err)
		return
	}
	reply(c, map[string]any{"url": base + "archives/" + url.PathEscape(snapshot.Slug) + "/", "expiresAt": expires}, nil)
}
func (h *Handler) RegisterPreview(r gin.IRouter) {
	r.GET("/api/v1/previews/:id/*resource", h.servePreview)
	r.HEAD("/api/v1/previews/:id/*resource", h.servePreview)
}
func (h *Handler) servePreview(c *gin.Context) {
	id, ok := publicationID(c)
	if !ok {
		return
	}
	token, err := c.Cookie("cms_preview")
	if err != nil || len(token) != 64 {
		c.AbortWithStatus(401)
		return
	}
	hash := sha256.Sum256([]byte(token))
	now := time.Now().UTC()
	var access PreviewAccess
	if err := h.Service.DB.Where("token_hash=? AND task_id=? AND expires_at>?", hex.EncodeToString(hash[:]), id, now).Take(&access).Error; err != nil {
		c.AbortWithStatus(401)
		return
	}
	var sessions int64
	if err := h.Service.DB.Model(&auth.AuthSession{}).Where("user_id=? AND jti=? AND revoked_at IS NULL AND expires_at>?", access.UserID, access.JTI, now).Count(&sessions).Error; err != nil || sessions == 0 {
		c.AbortWithStatus(401)
		return
	}
	allowed, err := authz.Allowed(c.Request.Context(), h.Service.DB, access.UserID, content.EditPermission)
	if err != nil || !allowed {
		c.AbortWithStatus(403)
		return
	}
	var task Task
	if err := h.Service.DB.Where("id=? AND kind='preview' AND status='succeeded'", id).Take(&task).Error; err != nil {
		c.AbortWithStatus(404)
		return
	}
	var attempt Attempt
	if err := h.Service.DB.Where("task_id=? AND status='succeeded'", id).Order("id DESC").Take(&attempt).Error; err != nil {
		c.AbortWithStatus(404)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	staticweb.Serve(c.Writer, c.Request, filepath.Join(h.Service.RuntimeRoot, "previews", attempt.ReleaseKey), builder.Marker{AttemptID: attempt.ID, ReleaseKey: attempt.ReleaseKey, ManifestHash: attempt.ManifestHash}, c.Param("resource"))
}
