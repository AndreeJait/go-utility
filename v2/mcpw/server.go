package mcpw

import (
	"context"
	"fmt"

	"github.com/AndreeJait/go-utility/v2/logw"
	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpServer struct {
	cfg    *ServerConfig
	server *mcp.Server
}

// NewServer creates a new MCP Server using the provided configuration.
func NewServer(cfg *ServerConfig) Server {
	if cfg == nil {
		cfg = &ServerConfig{}
	}

	sdkServer := mcp.NewServer(toSDKImplementation(cfg), nil)

	return &mcpServer{
		cfg:    cfg,
		server: sdkServer,
	}
}

// AddTool registers a tool with a handler on the MCP server.
// The handler receives the tool call parameters and returns the result.
func (s *mcpServer) AddTool(info *ToolInfo, handler ToolHandler) {
	tool := toSDKTool(info)

	// If no input schema is provided, default to empty object schema
	// (required by the SDK for Server.AddTool)
	if tool.InputSchema == nil {
		tool.InputSchema = map[string]any{"type": "object"}
	}

	sdkHandler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := extractArgsFromCallToolRequest(req)

		result, err := handler(ctx, &ToolCallParams{
			Name:      info.Name,
			Arguments: args,
		})
		if err != nil {
			// Return error as a tool error (not protocol error) per MCP spec
			errResult := &mcp.CallToolResult{IsError: true}
			errResult.SetError(err)
			return errResult, nil
		}

		return toSDKCallToolResult(result), nil
	}

	s.server.AddTool(tool, sdkHandler)
	logw.Infof("mcpw: registered tool %q", info.Name)
}

// AddResource registers a static resource with a handler on the MCP server.
func (s *mcpServer) AddResource(info *ResourceInfo, handler ResourceHandler) {
	resource := toSDKResource(info)

	sdkHandler := func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		result, err := handler(ctx, req.Params.URI)
		if err != nil {
			return nil, fmt.Errorf("mcpw: resource %q handler error: %w", req.Params.URI, err)
		}

		return toSDKReadResourceResult(result), nil
	}

	s.server.AddResource(resource, sdkHandler)
	logw.Infof("mcpw: registered resource %q", info.URI)
}

// AddResourceTemplate registers a resource template with a handler on the MCP server.
func (s *mcpServer) AddResourceTemplate(info *ResourceTemplateInfo, handler ResourceHandler) {
	template := toSDKResourceTemplate(info)

	sdkHandler := func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		result, err := handler(ctx, req.Params.URI)
		if err != nil {
			return nil, fmt.Errorf("mcpw: resource template %q handler error: %w", info.URITemplate, err)
		}

		return toSDKReadResourceResult(result), nil
	}

	s.server.AddResourceTemplate(template, sdkHandler)
	logw.Infof("mcpw: registered resource template %q", info.URITemplate)
}

// AddPrompt registers a prompt with a handler on the MCP server.
func (s *mcpServer) AddPrompt(info *PromptInfo, handler PromptHandler) {
	prompt := toSDKPrompt(info)

	sdkHandler := func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		args := extractArgsFromGetPromptRequest(req)

		result, err := handler(ctx, info.Name, args)
		if err != nil {
			return nil, fmt.Errorf("mcpw: prompt %q handler error: %w", info.Name, err)
		}

		return toSDKGetPromptResult(result), nil
	}

	s.server.AddPrompt(prompt, sdkHandler)
	logw.Infof("mcpw: registered prompt %q", info.Name)
}

// Run starts the server using the given transport, blocking until the context is canceled.
func (s *mcpServer) Run(ctx context.Context, transport Transport) error {
	sdkTransport, ok := transport.inner.toSDK().(mcp.Transport)
	if !ok {
		return fmt.Errorf("mcpw: invalid transport type for server Run")
	}

	logw.Infof("mcpw: starting server %q v%s", s.cfg.Name, s.cfg.Version)
	return s.server.Run(ctx, sdkTransport)
}

// Close gracefully shuts down the MCP server.
// The MCP SDK handles shutdown via context cancellation in Run().
func (s *mcpServer) Close() error {
	logw.Infof("mcpw: closing server %q", s.cfg.Name)
	return nil
}

// SDKServer returns the underlying *mcp.Server for advanced usage
// (e.g., custom middleware, StreamableHTTPHandler integration).
func (s *mcpServer) SDKServer() *mcp.Server {
	return s.server
}

// Compile-time interface compliance check.
var _ Server = (*mcpServer)(nil)
