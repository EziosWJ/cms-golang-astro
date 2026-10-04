// Package authz checks domain permissions using the existing RBAC relationships.
package authz

import (
	"context"
	"errors"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/auth"
	platform "github.com/EziosWJ/cms-golang-astro/cms-api/internal/platform/http"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var ErrForbidden = errors.New("没有操作权限")

func Allowed(ctx context.Context, db *gorm.DB, actor int64, permission string) (bool, error) {
	var count int64
	err := db.WithContext(ctx).Table("sys_user u").Joins("JOIN sys_user_role ur ON ur.user_id=u.id").Joins("JOIN sys_role r ON r.id=ur.role_id").Joins("JOIN sys_role_menu rm ON rm.role_id=r.id").Joins("JOIN sys_menu m ON m.id=rm.menu_id").Where("u.id=? AND u.status=1 AND u.deleted=0 AND r.status=1 AND r.deleted=0 AND m.status=1 AND m.deleted=0 AND m.permission_code=?", actor, permission).Count(&count).Error
	return count > 0, err
}
func Require(db *gorm.DB, permission string) gin.HandlerFunc { return RequireAny(db, permission) }
func RequireAny(db *gorm.DB, permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := auth.PrincipalFromContext(c.Request.Context())
		if !ok {
			platform.AbortError(c, 401, 401, "请先登录", nil)
			return
		}
		var allowed bool
		var err error
		for _, permission := range permissions {
			allowed, err = Allowed(c.Request.Context(), db, p.UserID, permission)
			if err != nil || allowed {
				break
			}
		}
		if err != nil {
			c.Abort()
			if platform.IsTemporaryUnavailable(err) {
				platform.TemporaryUnavailable(c)
			} else {
				platform.WriteError(c, 500, 500, "权限检查失败", nil)
			}
			return
		}
		if !allowed {
			platform.AbortError(c, 403, 403, ErrForbidden.Error(), nil)
			return
		}
		c.Next()
	}
}
