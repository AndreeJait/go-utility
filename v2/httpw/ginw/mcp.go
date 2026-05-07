package ginw

import (
	"github.com/AndreeJait/go-utility/v2/mcpw"
	"github.com/gin-gonic/gin"
)

// MCPHandler returns a Gin handler that serves MCP requests
// using the Streamable HTTP transport.
//
// Usage:
//
//	srv := mcpw.NewServer(&mcpw.ServerConfig{Name: "my-mcp", Version: "1.0"})
//	handler := mcpw.NewStreamableHTTPHandler(func(r *http.Request) mcpw.Server { return srv }, nil)
//	r.Any("/mcp", ginw.MCPHandler(handler))
func MCPHandler(handler *mcpw.StreamableHTTPHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		handler.ServeHTTP(c.Writer, c.Request)
	}
}