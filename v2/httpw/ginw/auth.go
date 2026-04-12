package ginw

import (
	"github.com/AndreeJait/go-utility/v2/authw"
	"github.com/AndreeJait/go-utility/v2/logw"
	"github.com/gin-gonic/gin"
)

// AuthMiddleware returns a Gin middleware that authenticates every request
// using the provided Authenticator. On success, the auth Result is injected
// into the request context (accessible via authw.FromContext). On failure,
// the middleware aborts with an error (HTTP 401 via the error pipeline).
func AuthMiddleware(authenticator authw.Authenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.Request
		result, err := authenticator.Authenticate(req)
		if err != nil {
			logw.CtxInfof(req.Context(), "[GIN-AUTH] authentication failed: %v", err)
			_ = c.Error(err)
			c.Abort()
			return
		}

		ctx := authw.WithResult(req.Context(), result)
		c.Request = req.WithContext(ctx)

		logw.CtxInfof(ctx, "[GIN-AUTH] authenticated user: %s", result.GetUserID())
		c.Next()
	}
}