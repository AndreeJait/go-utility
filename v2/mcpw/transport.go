package mcpw

import (
	"net/http"
	"os/exec"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- Transport Factory Functions ---

// StdioTransport creates a transport that communicates over stdin/stdout.
// This is the standard transport for MCP servers running as CLI tools or subprocesses.
func StdioTransport() Transport {
	return Transport{inner: &stdioTransport{}}
}

type stdioTransport struct{}

func (t *stdioTransport) toSDK() any {
	return &mcp.StdioTransport{}
}

// CommandTransport creates a transport that connects to an MCP server
// by spawning a subprocess. This is the standard client transport for
// connecting to CLI-based MCP servers.
func CommandTransport(name string, args ...string) Transport {
	return Transport{inner: &commandTransport{name: name, args: args}}
}

type commandTransport struct {
	name string
	args []string
}

func (t *commandTransport) toSDK() any {
	cmd := exec.Command(t.name, t.args...)
	return &mcp.CommandTransport{Command: cmd}
}

// StreamableClientTransport creates a transport for connecting to a
// Streamable HTTP MCP server at the given URL.
func StreamableClientTransport(url string) Transport {
	return Transport{inner: &streamableClientTransport{url: url}}
}

type streamableClientTransport struct {
	url string
}

func (t *streamableClientTransport) toSDK() any {
	return &mcp.StreamableClientTransport{Endpoint: t.url}
}

// SSEClientTransport creates a transport for connecting to an SSE MCP server.
// Deprecated: use StreamableClientTransport for new implementations.
func SSEClientTransport(url string) Transport {
	return Transport{inner: &sseClientTransport{url: url}}
}

type sseClientTransport struct {
	url string
}

func (t *sseClientTransport) toSDK() any {
	return &mcp.SSEClientTransport{Endpoint: t.url}
}

// --- Server-side HTTP Handlers ---

// NewStreamableHTTPHandler creates a Streamable HTTP transport handler for mounting
// an MCP server on an HTTP router. This is the recommended transport for web servers.
//
// The getServer function is called for each incoming request to obtain the MCP server instance.
// Typically, you create a single server and return it from getServer.
//
// Usage:
//
//	server := mcpw.NewServer(&mcpw.ServerConfig{Name: "my-server", Version: "1.0"})
//	handler := mcpw.NewStreamableHTTPHandler(func(r *http.Request) *mcpw.ServerConfig {
//	    return server
//	}, nil)
func NewStreamableHTTPHandler(getServer func(r *http.Request) Server, opts *StreamableHTTPOptions) *StreamableHTTPHandler {
	var sdkOpts *mcp.StreamableHTTPOptions
	if opts != nil {
		sdkOpts = &mcp.StreamableHTTPOptions{}
		// Map options as they become available
	}

	sdkHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		srv := getServer(r)
		if s, ok := srv.(*mcpServer); ok {
			return s.server
		}
		return nil
	}, sdkOpts)

	return &StreamableHTTPHandler{handler: sdkHandler}
}

// StreamableHTTPOptions holds configuration for the Streamable HTTP transport.
type StreamableHTTPOptions struct {
	// Logger, if set, overrides the default logger for the Streamable HTTP handler.
	// Most users should leave this nil and rely on the mcpw server's logging.
}

// NewSSEHandler creates a Server-Sent Events transport handler (deprecated: use NewStreamableHTTPHandler).
// This is provided for backward compatibility with older MCP clients.
func NewSSEHandler(getServer func(r *http.Request) Server, opts *SSEOptions) *SSEHandler {
	var sdkOpts *mcp.SSEOptions
	_ = opts // options mapping if needed later

	sdkHandler := mcp.NewSSEHandler(func(r *http.Request) *mcp.Server {
		srv := getServer(r)
		if s, ok := srv.(*mcpServer); ok {
			return s.server
		}
		return nil
	}, sdkOpts)

	return &SSEHandler{handler: sdkHandler}
}

// SSEOptions holds configuration for the SSE transport.
type SSEOptions struct{}

// --- InMemoryTransport for testing ---

// InMemoryTransport creates a pair of connected transports for in-process
// testing. Returns (client transport, server transport).
func InMemoryTransport() (client Transport, server Transport) {
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	return Transport{inner: &inMemoryTransport{t: clientTransport}},
		Transport{inner: &inMemoryTransport{t: serverTransport}}
}

type inMemoryTransport struct {
	t mcp.Transport
}

func (t *inMemoryTransport) toSDK() any {
	return t.t
}