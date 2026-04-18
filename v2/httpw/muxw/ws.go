package muxw

import (
	"net/http"

	"github.com/AndreeJait/go-utility/v2/websocketw"
)

// WSUpgradeHandler returns an http.HandlerFunc that upgrades HTTP connections
// to WebSocket using the provided websocketw.Server.
//
// Since websocketw.Server already implements http.Handler via ServeHTTP,
// you can also mount it directly without this adapter:
//
//	r.Handle("/ws", ws)
//
// The server handles authentication internally if configured via gorillaw.Config.Authenticator.
// If the Mux router already has AuthMiddleware applied, the server will detect the
// existing auth result in context and skip redundant authentication.
//
// Usage with adapter:
//
//	ws := gorillaw.New(&gorillaw.Config{Authenticator: auth})
//	r.HandleFunc("/ws", muxw.WSUpgradeHandler(ws))
func WSUpgradeHandler(srv websocketw.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		srv.ServeHTTP(w, r)
	}
}