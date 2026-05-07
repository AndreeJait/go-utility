package echow

import (
	"github.com/AndreeJait/go-utility/v2/mcpw"
	"github.com/labstack/echo/v5"
)

// MCPHandler returns an Echo handler that serves MCP requests
// using the Streamable HTTP transport.
//
// Usage:
//
//	srv := mcpw.NewServer(&mcpw.ServerConfig{Name: "my-mcp", Version: "1.0"})
//	handler := mcpw.NewStreamableHTTPHandler(func(r *http.Request) mcpw.Server { return srv }, nil)
//	e.Any("/mcp", echow.MCPHandler(handler))
func MCPHandler(handler *mcpw.StreamableHTTPHandler) echo.HandlerFunc {
	return func(c *echo.Context) error {
		handler.ServeHTTP(c.Response(), c.Request())
		return nil
	}
}