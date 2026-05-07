// Package mcpw provides a production-ready wrapper around the official MCP Go SDK
// (github.com/modelcontextprotocol/go-sdk/mcp). It exposes Server and Client interfaces
// for the Model Context Protocol, supporting Streamable HTTP, Stdio, and SSE transports.
//
// MCP (Model Context Protocol) standardizes how LLM applications connect to external
// tools, resources, and prompts via JSON-RPC 2.0.
//
// Server usage:
//
//	srv := mcpw.NewServer(&mcpw.ServerConfig{Name: "my-server", Version: "1.0"})
//	srv.AddTool(&mcpw.ToolInfo{Name: "greet", Description: "Say hello"}, handler)
//	srv.Run(ctx, mcpw.StdioTransport())
//
// Client usage:
//
//	cli := mcpw.NewClient(&mcpw.ClientConfig{Name: "my-client", Version: "1.0"})
//	session, _ := cli.Connect(ctx, mcpw.StreamableClientTransport("http://localhost:8080/mcp"))
//	result, _ := session.CallTool(ctx, &mcpw.ToolCallParams{Name: "greet", Arguments: map[string]any{"name": "you"}})
package mcpw

import (
	"context"
	"net/http"
)

// --- Domain Types ---

// ToolInfo describes an MCP tool that a server exposes or a client discovers.
type ToolInfo struct {
	Name         string         // Unique tool name (required)
	Description  string         // Human-readable description
	InputSchema  any            // JSON Schema for input (must be type "object"; nil = auto-inferred via AddTypedTool)
	OutputSchema any            // JSON Schema for output (nil = no output schema)
	Annotations  *ToolAnnotations // Optional hints for the client
}

// ToolAnnotations provides optional hints about how a tool should be used.
type ToolAnnotations struct {
	Title           string // Human-readable title for the tool
	ReadOnlyHint    bool   // Tool only reads data, no side effects
	DestructiveHint bool   // Tool may destroy data
	IdempotentHint  bool   // Repeated calls produce the same result
	OpenWorldHint   bool   // Tool interacts with external entities
}

// PromptInfo describes an MCP prompt that a server exposes.
type PromptInfo struct {
	Name        string      // Unique prompt name (required)
	Description string      // Human-readable description
	Arguments   []PromptArg // Prompt argument definitions
}

// PromptArg defines a single argument for a prompt.
type PromptArg struct {
	Name        string // Argument name (required)
	Description string // Human-readable description
	Required    bool   // Whether the argument is required
}

// ResourceInfo describes an MCP resource that a server exposes.
type ResourceInfo struct {
	URI         string // Unique resource URI (required, must be absolute)
	Name        string // Human-readable name
	Description string // Human-readable description
	MIMEType    string // MIME type of the resource content
}

// ResourceTemplateInfo describes an MCP resource template with URI pattern.
type ResourceTemplateInfo struct {
	URITemplate string // URI template pattern (required, e.g., "file:///{path}")
	Name        string // Human-readable name
	Description string // Human-readable description
	MIMEType    string // MIME type of the resource content
}

// ToolCallParams holds the parameters for calling a tool.
type ToolCallParams struct {
	Name      string         // Tool name to call (required)
	Arguments map[string]any // Input arguments for the tool
}

// ToolCallResult holds the result of a tool call.
type ToolCallResult struct {
	Content []Content // Ordered list of content items
	IsError bool     // True if the tool execution resulted in an error
}

// Content is the interface for all MCP content types.
// It is satisfied by TextContent, ImageContent, AudioContent, and EmbeddedResource.
type Content interface {
	contentMarker()
}

// TextContent represents a text content item in a tool result or prompt message.
type TextContent struct {
	Text string
}

func (TextContent) contentMarker() {}

// ImageContent represents a base64-encoded image content item.
type ImageContent struct {
	Data     []byte
	MIMEType string
}

func (ImageContent) contentMarker() {}

// AudioContent represents a base64-encoded audio content item.
type AudioContent struct {
	Data     []byte
	MIMEType string
}

func (AudioContent) contentMarker() {}

// EmbeddedResource represents a resource embedded in content.
type EmbeddedResource struct {
	URI      string
	MIMEType string
	Text     string // Text content (if text-based)
	Blob     []byte // Binary content (if binary)
}

func (EmbeddedResource) contentMarker() {}

// PromptResult holds the result of getting a prompt.
type PromptResult struct {
	Description string         // Optional description of the prompt result
	Messages    []PromptMessage // Ordered list of messages
}

// PromptMessage represents a single message in a prompt result.
type PromptMessage struct {
	Role    Role    // "user" or "assistant"
	Content Content // The message content
}

// Role represents the role of a message sender.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// ReadResourceResult holds the result of reading a resource.
type ReadResourceResult struct {
	Contents []ResourceContent // The resource contents
}

// ResourceContent holds the content of a resource.
type ResourceContent struct {
	URI      string
	MIMEType string
	Text     string // Text content (if text-based)
	Blob     []byte // Binary content (if binary)
}

// --- Handler Types ---

// ToolHandler is the function signature for processing a tool call.
type ToolHandler func(ctx context.Context, params *ToolCallParams) (*ToolCallResult, error)

// ResourceHandler is the function signature for reading a resource.
type ResourceHandler func(ctx context.Context, uri string) (*ReadResourceResult, error)

// PromptHandler is the function signature for processing a prompt request.
type PromptHandler func(ctx context.Context, name string, args map[string]string) (*PromptResult, error)

// --- Transport ---

// Transport is an opaque transport configuration.
// Use the transport factory functions (StdioTransport, StreamableClientTransport, etc.)
// to create instances.
type Transport struct {
	inner transportImpl
}

// transportImpl is the unexported interface that all transport wrappers must satisfy.
type transportImpl interface {
	toSDK() any
}

// --- Interfaces ---

// Server defines the contract for an MCP server that exposes tools, resources, and prompts.
type Server interface {
	// AddTool registers a tool with a handler.
	AddTool(info *ToolInfo, handler ToolHandler)

	// AddResource registers a static resource with a handler.
	AddResource(info *ResourceInfo, handler ResourceHandler)

	// AddResourceTemplate registers a resource template with a handler.
	// The handler receives the resolved URI and returns the resource content.
	AddResourceTemplate(info *ResourceTemplateInfo, handler ResourceHandler)

	// AddPrompt registers a prompt with a handler.
	AddPrompt(info *PromptInfo, handler PromptHandler)

	// Run starts the server using the given transport, blocking until the context
	// is canceled or a fatal error occurs. This is the primary way to run a Stdio server.
	Run(ctx context.Context, transport Transport) error

	// Close gracefully shuts down the server.
	// This should be registered with gracefulw for clean shutdown.
	Close() error
}

// Client defines the contract for an MCP client that connects to MCP servers.
type Client interface {
	// Connect establishes a connection to an MCP server using the given transport.
	// Returns a ClientSession that can be used for all server interactions.
	Connect(ctx context.Context, transport Transport) (ClientSession, error)

	// Close gracefully closes the client.
	Close() error
}

// ClientSession represents an active connection to an MCP server.
// It is obtained from Client.Connect() and used for all server interactions.
type ClientSession interface {
	// CallTool invokes a tool on the connected server.
	CallTool(ctx context.Context, params *ToolCallParams) (*ToolCallResult, error)

	// ListTools retrieves the list of tools available on the connected server.
	ListTools(ctx context.Context) ([]ToolInfo, error)

	// ReadResource reads a specific resource from the connected server.
	ReadResource(ctx context.Context, uri string) (*ReadResourceResult, error)

	// ListResources retrieves the list of resources available on the connected server.
	ListResources(ctx context.Context) ([]ResourceInfo, error)

	// ListResourceTemplates retrieves the list of resource templates available on the connected server.
	ListResourceTemplates(ctx context.Context) ([]ResourceTemplateInfo, error)

	// GetPrompt retrieves a prompt from the connected server.
	GetPrompt(ctx context.Context, name string, args map[string]string) (*PromptResult, error)

	// ListPrompts retrieves the list of prompts available on the connected server.
	ListPrompts(ctx context.Context) ([]PromptInfo, error)

	// Ping checks if the server is still responsive.
	Ping(ctx context.Context) error

	// Close closes the client session.
	Close() error
}

// --- HTTP Handler Wrappers ---

// StreamableHTTPHandler wraps the MCP SDK's StreamableHTTPHandler and implements http.Handler.
// Create one using StreamableHTTPHandler(), then mount it on any httpw router.
type StreamableHTTPHandler struct {
	handler http.Handler
}

// ServeHTTP implements http.Handler, allowing the handler to be mounted
// directly on any HTTP router (Echo, Gin, Mux, net/http).
func (h *StreamableHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.handler.ServeHTTP(w, r)
}

// SSEHandler wraps the MCP SDK's SSEHandler and implements http.Handler.
// Deprecated: use StreamableHTTPHandler instead.
type SSEHandler struct {
	handler http.Handler
}

// ServeHTTP implements http.Handler.
func (h *SSEHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.handler.ServeHTTP(w, r)
}

// --- Config ---

// ServerConfig holds the configuration for creating an MCP server.
type ServerConfig struct {
	Name    string // Server implementation name (required)
	Version string // Server implementation version (required)
}

// ClientConfig holds the configuration for creating an MCP client.
type ClientConfig struct {
	Name    string // Client implementation name (required)
	Version string // Client implementation version (required)
}