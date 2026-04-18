package ginw

import (
	"github.com/AndreeJait/go-utility/v2/websocketw"
	"github.com/gin-gonic/gin"
)

// WSUpgradeHandler returns a Gin handler that upgrades HTTP connections
// to WebSocket using the provided websocketw.Server.
//
// The server handles authentication internally if configured via gorillaw.Config.Authenticator.
// If the Gin router already has AuthMiddleware applied, the server will detect the
// existing auth result in context and skip redundant authentication.
//
// Usage:
//
//	ws := gorillaw.New(&gorillaw.Config{Authenticator: auth})
//	r.GET("/ws", ginw.WSUpgradeHandler(ws))
func WSUpgradeHandler(srv websocketw.Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		srv.ServeHTTP(c.Writer, c.Request)
	}
}