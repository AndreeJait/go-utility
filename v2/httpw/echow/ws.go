package echow

import (
	"github.com/AndreeJait/go-utility/v2/websocketw"
	"github.com/labstack/echo/v5"
)

// WSUpgradeHandler returns an Echo handler that upgrades HTTP connections
// to WebSocket using the provided websocketw.Server.
//
// The server handles authentication internally if configured via gorillaw.Config.Authenticator.
// If the Echo router already has AuthMiddleware applied, the server will detect the
// existing auth result in context and skip redundant authentication.
//
// Usage:
//
//	ws := gorillaw.New(&gorillaw.Config{Authenticator: auth})
//	e.GET("/ws", echow.WSUpgradeHandler(ws))
func WSUpgradeHandler(srv websocketw.Server) echo.HandlerFunc {
	return func(c *echo.Context) error {
		srv.ServeHTTP(c.Response(), c.Request())
		return nil
	}
}