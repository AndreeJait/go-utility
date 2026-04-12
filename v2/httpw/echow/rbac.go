package echow

import (
	"github.com/AndreeJait/go-utility/v2/authw"
	"github.com/AndreeJait/go-utility/v2/logw"
	"github.com/AndreeJait/go-utility/v2/statusw"
	"github.com/labstack/echo/v5"
)

// RequireRole returns an Echo middleware that checks if the authenticated user
// has the specified role. Must be used after AuthMiddleware so the auth Result
// is available in context. Returns 401 if not authenticated, 403 if unauthorized.
func RequireRole(rbac *authw.RBAC, role string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			req := c.Request()
			result := authw.FromContext(req.Context())
			if result == nil {
				return statusw.InvalidCredential.WithCustomMessage("Authentication required")
			}

			ok, err := rbac.CheckRole(req.Context(), result.GetUserID(), role)
			if err != nil {
				logw.CtxErrorf(req.Context(), "[ECHO-RBAC] role check error: %v", err)
				return err
			}
			if !ok {
				logw.CtxInfof(req.Context(), "[ECHO-RBAC] access denied: role %q required", role)
				return statusw.InvalidAccess.WithCustomMessage("Insufficient role: " + role)
			}

			logw.CtxInfof(req.Context(), "[ECHO-RBAC] role check passed: %s", role)
			return next(c)
		}
	}
}

// RequirePermission returns an Echo middleware that checks if the authenticated user
// has the specified permission. Must be used after AuthMiddleware so the auth Result
// is available in context. Returns 401 if not authenticated, 403 if unauthorized.
func RequirePermission(rbac *authw.RBAC, permission string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			req := c.Request()
			result := authw.FromContext(req.Context())
			if result == nil {
				return statusw.InvalidCredential.WithCustomMessage("Authentication required")
			}

			ok, err := rbac.CheckPermission(req.Context(), result.GetUserID(), permission)
			if err != nil {
				logw.CtxErrorf(req.Context(), "[ECHO-RBAC] permission check error: %v", err)
				return err
			}
			if !ok {
				logw.CtxInfof(req.Context(), "[ECHO-RBAC] access denied: permission %q required", permission)
				return statusw.InvalidAccess.WithCustomMessage("Insufficient permission: " + permission)
			}

			logw.CtxInfof(req.Context(), "[ECHO-RBAC] permission check passed: %s", permission)
			return next(c)
		}
	}
}