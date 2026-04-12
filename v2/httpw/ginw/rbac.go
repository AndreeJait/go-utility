package ginw

import (
	"github.com/AndreeJait/go-utility/v2/authw"
	"github.com/AndreeJait/go-utility/v2/logw"
	"github.com/AndreeJait/go-utility/v2/statusw"
	"github.com/gin-gonic/gin"
)

// RequireRole returns a Gin middleware that checks if the authenticated user
// has the specified role. Must be used after AuthMiddleware so the auth Result
// is available in context. Returns 401 if not authenticated, 403 if unauthorized.
func RequireRole(rbac *authw.RBAC, role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.Request
		result := authw.FromContext(req.Context())
		if result == nil {
			_ = c.Error(statusw.InvalidCredential.WithCustomMessage("Authentication required"))
			c.Abort()
			return
		}

		ok, err := rbac.CheckRole(req.Context(), result.GetUserID(), role)
		if err != nil {
			logw.CtxErrorf(req.Context(), "[GIN-RBAC] role check error: %v", err)
			_ = c.Error(err)
			c.Abort()
			return
		}
		if !ok {
			logw.CtxInfof(req.Context(), "[GIN-RBAC] access denied: role %q required", role)
			_ = c.Error(statusw.InvalidAccess.WithCustomMessage("Insufficient role: " + role))
			c.Abort()
			return
		}

		logw.CtxInfof(req.Context(), "[GIN-RBAC] role check passed: %s", role)
		c.Next()
	}
}

// RequirePermission returns a Gin middleware that checks if the authenticated user
// has the specified permission. Must be used after AuthMiddleware so the auth Result
// is available in context. Returns 401 if not authenticated, 403 if unauthorized.
func RequirePermission(rbac *authw.RBAC, permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.Request
		result := authw.FromContext(req.Context())
		if result == nil {
			_ = c.Error(statusw.InvalidCredential.WithCustomMessage("Authentication required"))
			c.Abort()
			return
		}

		ok, err := rbac.CheckPermission(req.Context(), result.GetUserID(), permission)
		if err != nil {
			logw.CtxErrorf(req.Context(), "[GIN-RBAC] permission check error: %v", err)
			_ = c.Error(err)
			c.Abort()
			return
		}
		if !ok {
			logw.CtxInfof(req.Context(), "[GIN-RBAC] access denied: permission %q required", permission)
			_ = c.Error(statusw.InvalidAccess.WithCustomMessage("Insufficient permission: " + permission))
			c.Abort()
			return
		}

		logw.CtxInfof(req.Context(), "[GIN-RBAC] permission check passed: %s", permission)
		c.Next()
	}
}