package mcpw

import (
	"context"
	"fmt"

	"github.com/AndreeJait/go-utility/v2/logw"
	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpClient struct {
	cfg    *ClientConfig
	client *mcp.Client
}

// NewClient creates a new MCP Client using the provided configuration.
func NewClient(cfg *ClientConfig) Client {
	if cfg == nil {
		cfg = &ClientConfig{}
	}

	sdkClient := mcp.NewClient(&mcp.Implementation{
		Name:    cfg.Name,
		Version: cfg.Version,
	}, nil)

	return &mcpClient{
		cfg:    cfg,
		client: sdkClient,
	}
}

// Connect establishes a connection to an MCP server using the given transport.
func (c *mcpClient) Connect(ctx context.Context, transport Transport) (ClientSession, error) {
	sdkTransport, ok := transport.inner.toSDK().(mcp.Transport)
	if !ok {
		return nil, fmt.Errorf("mcpw: invalid transport type for client Connect")
	}

	logw.CtxInfof(ctx, "mcpw: client %q connecting...", c.cfg.Name)

	session, err := c.client.Connect(ctx, sdkTransport, nil)
	if err != nil {
		logw.CtxErrorf(ctx, "mcpw: client %q connect failed: %v", c.cfg.Name, err)
		return nil, fmt.Errorf("mcpw: connect failed: %w", err)
	}

	logw.CtxInfof(ctx, "mcpw: client %q connected successfully", c.cfg.Name)
	return &mcpClientSession{session: session}, nil
}

// Close gracefully closes the MCP client.
func (c *mcpClient) Close() error {
	logw.Infof("mcpw: closing client %q", c.cfg.Name)
	return nil
}

// Compile-time interface compliance check.
var _ Client = (*mcpClient)(nil)

// --- ClientSession Implementation ---

type mcpClientSession struct {
	session *mcp.ClientSession
}

func (cs *mcpClientSession) CallTool(ctx context.Context, params *ToolCallParams) (*ToolCallResult, error) {
	result, err := cs.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      params.Name,
		Arguments: params.Arguments,
	})
	if err != nil {
		logw.CtxErrorf(ctx, "mcpw: CallTool %q failed: %v", params.Name, err)
		return nil, fmt.Errorf("mcpw: call tool %q failed: %w", params.Name, err)
	}
	return fromSDKCallToolResult(result), nil
}

func (cs *mcpClientSession) ListTools(ctx context.Context) ([]ToolInfo, error) {
	var tools []ToolInfo
	for tool, err := range cs.session.Tools(ctx, nil) {
		if err != nil {
			return nil, fmt.Errorf("mcpw: list tools failed: %w", err)
		}
		tools = append(tools, fromSDKToolInfo(tool))
	}
	return tools, nil
}

func (cs *mcpClientSession) ReadResource(ctx context.Context, uri string) (*ReadResourceResult, error) {
	result, err := cs.session.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
	if err != nil {
		logw.CtxErrorf(ctx, "mcpw: ReadResource %q failed: %v", uri, err)
		return nil, fmt.Errorf("mcpw: read resource %q failed: %w", uri, err)
	}
	return fromSDKReadResourceResult(result), nil
}

func (cs *mcpClientSession) ListResources(ctx context.Context) ([]ResourceInfo, error) {
	var resources []ResourceInfo
	for resource, err := range cs.session.Resources(ctx, nil) {
		if err != nil {
			return nil, fmt.Errorf("mcpw: list resources failed: %w", err)
		}
		resources = append(resources, fromSDKResourceInfo(resource))
	}
	return resources, nil
}

func (cs *mcpClientSession) ListResourceTemplates(ctx context.Context) ([]ResourceTemplateInfo, error) {
	var templates []ResourceTemplateInfo
	for rt, err := range cs.session.ResourceTemplates(ctx, nil) {
		if err != nil {
			return nil, fmt.Errorf("mcpw: list resource templates failed: %w", err)
		}
		templates = append(templates, fromSDKResourceTemplateInfo(rt))
	}
	return templates, nil
}

func (cs *mcpClientSession) GetPrompt(ctx context.Context, name string, args map[string]string) (*PromptResult, error) {
	params := &mcp.GetPromptParams{Name: name}
	if args != nil {
		params.Arguments = args
	}

	result, err := cs.session.GetPrompt(ctx, params)
	if err != nil {
		logw.CtxErrorf(ctx, "mcpw: GetPrompt %q failed: %v", name, err)
		return nil, fmt.Errorf("mcpw: get prompt %q failed: %w", name, err)
	}
	return fromSDKGetPromptResult(result), nil
}

func (cs *mcpClientSession) ListPrompts(ctx context.Context) ([]PromptInfo, error) {
	var prompts []PromptInfo
	for prompt, err := range cs.session.Prompts(ctx, nil) {
		if err != nil {
			return nil, fmt.Errorf("mcpw: list prompts failed: %w", err)
		}
		prompts = append(prompts, fromSDKPromptInfo(prompt))
	}
	return prompts, nil
}

func (cs *mcpClientSession) Ping(ctx context.Context) error {
	return cs.session.Ping(ctx, nil)
}

func (cs *mcpClientSession) Close() error {
	return cs.session.Close()
}

// Compile-time interface compliance check.
var _ ClientSession = (*mcpClientSession)(nil)