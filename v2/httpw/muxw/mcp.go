package muxw

import (
	"net/http"

	"github.com/AndreeJait/go-utility/v2/mcpw"
	"github.com/gorilla/mux"
)

// MCPHandler returns a Gorilla Mux handler that serves MCP requests
// using the Streamable HTTP transport.
//
// Since StreamableHTTPHandler already implements http.Handler via ServeHTTP,
// you can also mount it directly without this adapter:
//
//	r.Handle("/mcp", handler)
func MCPHandler(handler *mcpw.StreamableHTTPHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
	}
}

// MCPMount is a convenience function that mounts an MCP Streamable HTTP handler
// on the given Gorilla Mux router at the specified path.
func MCPMount(r *mux.Router, path string, handler *mcpw.StreamableHTTPHandler) {
	r.PathPrefix(path).Handler(handler)
}