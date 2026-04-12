package echow

import (
	"github.com/AndreeJait/go-utility/v2/authw"
	"github.com/AndreeJait/go-utility/v2/logw"
	"github.com/labstack/echo/v5"
)

// AuthMiddleware returns an Echo middleware that authenticates every request
// using the provided Authenticator. On success, the auth Result is injected
// into the request context (accessible via authw.FromContext). On failure,
// the error is returned and caught by the error pipeline (HTTP 401).
func AuthMiddleware(authenticator authw.Authenticator) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			req := c.Request()
			result, err := authenticator.Authenticate(req)
			if err != nil {
				logw.CtxInfof(req.Context(), "[ECHO-AUTH] authentication failed: %v", err)
				return err
			}

			ctx := authw.WithResult(req.Context(), result)
			c.SetRequest(req.WithContext(ctx))

			logw.CtxInfof(ctx, "[ECHO-AUTH] authenticated user: %s", result.GetUserID())
			return next(c)
		}
	}
}