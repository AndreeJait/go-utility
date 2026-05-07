package mcpw

import (
	"encoding/json"
	"fmt"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- Server-side: Domain -> SDK conversions ---

func toSDKImplementation(cfg *ServerConfig) *mcp.Implementation {
	return &mcp.Implementation{
		Name:    cfg.Name,
		Version: cfg.Version,
	}
}

func toSDKTool(info *ToolInfo) *mcp.Tool {
	t := &mcp.Tool{
		Name:        info.Name,
		Description: info.Description,
	}
	if info.InputSchema != nil {
		t.InputSchema = info.InputSchema
	}
	if info.OutputSchema != nil {
		t.OutputSchema = info.OutputSchema
	}
	if info.Annotations != nil {
		t.Annotations = &mcp.ToolAnnotations{
			Title:          info.Annotations.Title,
			ReadOnlyHint:    info.Annotations.ReadOnlyHint,
			IdempotentHint: info.Annotations.IdempotentHint,
		}
		if info.Annotations.DestructiveHint {
			t.Annotations.DestructiveHint = boolPtr(true)
		}
		if info.Annotations.OpenWorldHint {
			t.Annotations.OpenWorldHint = boolPtr(true)
		}
	}
	return t
}

func toSDKResource(info *ResourceInfo) *mcp.Resource {
	return &mcp.Resource{
		URI:         info.URI,
		Name:        info.Name,
		Description: info.Description,
		MIMEType:    info.MIMEType,
	}
}

func toSDKResourceTemplate(info *ResourceTemplateInfo) *mcp.ResourceTemplate {
	return &mcp.ResourceTemplate{
		URITemplate: info.URITemplate,
		Name:        info.Name,
		Description: info.Description,
		MIMEType:    info.MIMEType,
	}
}

func toSDKPrompt(info *PromptInfo) *mcp.Prompt {
	p := &mcp.Prompt{
		Name:        info.Name,
		Description: info.Description,
	}
	for _, arg := range info.Arguments {
		p.Arguments = append(p.Arguments, &mcp.PromptArgument{
			Name:        arg.Name,
			Description: arg.Description,
			Required:    arg.Required,
		})
	}
	return p
}

func toSDKCallToolResult(result *ToolCallResult) *mcp.CallToolResult {
	r := &mcp.CallToolResult{
		IsError: result.IsError,
	}
	for _, content := range result.Content {
		r.Content = append(r.Content, toSDKContent(content))
	}
	return r
}

func toSDKContent(content Content) mcp.Content {
	switch c := content.(type) {
	case TextContent:
		return &mcp.TextContent{Text: c.Text}
	case ImageContent:
		return &mcp.ImageContent{Data: c.Data, MIMEType: c.MIMEType}
	case AudioContent:
		return &mcp.AudioContent{Data: c.Data, MIMEType: c.MIMEType}
	case EmbeddedResource:
		rc := toSDKResourceContentsFromEmbedded(c)
		return &mcp.EmbeddedResource{Resource: &rc}
	default:
		return &mcp.TextContent{Text: ""}
	}
}

func toSDKResourceContentsFromEmbedded(c EmbeddedResource) mcp.ResourceContents {
	return mcp.ResourceContents{
		URI:      c.URI,
		MIMEType: c.MIMEType,
		Text:     c.Text,
		Blob:     c.Blob,
	}
}

func toSDKResourceContentsFromContent(rc ResourceContent) mcp.ResourceContents {
	return mcp.ResourceContents{
		URI:      rc.URI,
		MIMEType: rc.MIMEType,
		Text:     rc.Text,
		Blob:     rc.Blob,
	}
}

func toSDKReadResourceResult(result *ReadResourceResult) *mcp.ReadResourceResult {
	r := &mcp.ReadResourceResult{}
	for _, rc := range result.Contents {
		sdkRC := toSDKResourceContentsFromContent(rc)
		r.Contents = append(r.Contents, &sdkRC)
	}
	return r
}

func toSDKGetPromptResult(result *PromptResult) *mcp.GetPromptResult {
	r := &mcp.GetPromptResult{
		Description: result.Description,
	}
	for _, msg := range result.Messages {
		r.Messages = append(r.Messages, &mcp.PromptMessage{
			Role:    mcp.Role(msg.Role),
			Content: toSDKContent(msg.Content),
		})
	}
	return r
}

// --- Client-side: SDK -> Domain conversions ---

func fromSDKCallToolResult(result *mcp.CallToolResult) *ToolCallResult {
	r := &ToolCallResult{
		IsError: result.IsError,
	}
	for _, content := range result.Content {
		r.Content = append(r.Content, fromSDKContent(content))
	}
	return r
}

func fromSDKContent(content mcp.Content) Content {
	switch c := content.(type) {
	case *mcp.TextContent:
		return TextContent{Text: c.Text}
	case *mcp.ImageContent:
		return ImageContent{Data: c.Data, MIMEType: c.MIMEType}
	case *mcp.AudioContent:
		return AudioContent{Data: c.Data, MIMEType: c.MIMEType}
	case *mcp.EmbeddedResource:
		if c.Resource != nil {
			return EmbeddedResource{
				URI:      c.Resource.URI,
				MIMEType: c.Resource.MIMEType,
				Text:     c.Resource.Text,
				Blob:     c.Resource.Blob,
			}
		}
		return EmbeddedResource{}
	default:
		return TextContent{Text: ""}
	}
}

func fromSDKToolInfo(tool *mcp.Tool) ToolInfo {
	info := ToolInfo{
		Name:        tool.Name,
		Description: tool.Description,
	}
	if tool.InputSchema != nil {
		info.InputSchema = tool.InputSchema
	}
	if tool.OutputSchema != nil {
		info.OutputSchema = tool.OutputSchema
	}
	if tool.Annotations != nil {
		info.Annotations = &ToolAnnotations{
			Title:          tool.Annotations.Title,
			ReadOnlyHint:    tool.Annotations.ReadOnlyHint,
			IdempotentHint: tool.Annotations.IdempotentHint,
		}
		if tool.Annotations.DestructiveHint != nil {
			info.Annotations.DestructiveHint = *tool.Annotations.DestructiveHint
		}
		if tool.Annotations.OpenWorldHint != nil {
			info.Annotations.OpenWorldHint = *tool.Annotations.OpenWorldHint
		}
	}
	return info
}

func fromSDKResourceInfo(resource *mcp.Resource) ResourceInfo {
	return ResourceInfo{
		URI:         resource.URI,
		Name:        resource.Name,
		Description: resource.Description,
		MIMEType:    resource.MIMEType,
	}
}

func fromSDKResourceTemplateInfo(rt *mcp.ResourceTemplate) ResourceTemplateInfo {
	return ResourceTemplateInfo{
		URITemplate: rt.URITemplate,
		Name:        rt.Name,
		Description: rt.Description,
		MIMEType:    rt.MIMEType,
	}
}

func fromSDKPromptInfo(prompt *mcp.Prompt) PromptInfo {
	info := PromptInfo{
		Name:        prompt.Name,
		Description: prompt.Description,
	}
	for _, arg := range prompt.Arguments {
		info.Arguments = append(info.Arguments, PromptArg{
			Name:        arg.Name,
			Description: arg.Description,
			Required:    arg.Required,
		})
	}
	return info
}

func fromSDKReadResourceResult(result *mcp.ReadResourceResult) *ReadResourceResult {
	r := &ReadResourceResult{}
	for _, rc := range result.Contents {
		if rc == nil {
			continue
		}
		r.Contents = append(r.Contents, ResourceContent{
			URI:      rc.URI,
			MIMEType: rc.MIMEType,
			Text:     rc.Text,
			Blob:     rc.Blob,
		})
	}
	return r
}

func fromSDKGetPromptResult(result *mcp.GetPromptResult) *PromptResult {
	r := &PromptResult{
		Description: result.Description,
	}
	for _, msg := range result.Messages {
		r.Messages = append(r.Messages, PromptMessage{
			Role:    Role(msg.Role),
			Content: fromSDKContent(msg.Content),
		})
	}
	return r
}

// extractArgsFromCallToolRequest extracts the arguments from a CallToolRequest as map[string]any.
// The SDK passes Arguments as json.RawMessage when using Server.AddTool (raw handler).
func extractArgsFromCallToolRequest(req *mcp.CallToolRequest) map[string]any {
	args := make(map[string]any)
	if req.Params == nil || req.Params.Arguments == nil {
		return args
	}
	// CallToolParamsRaw.Arguments is json.RawMessage
	_ = json.Unmarshal(req.Params.Arguments, &args)
	return args
}

// extractArgsFromGetPromptRequest extracts the arguments from a GetPromptRequest as map[string]string.
func extractArgsFromGetPromptRequest(req *mcp.GetPromptRequest) map[string]string {
	args := make(map[string]string)
	if req.Params != nil && req.Params.Arguments != nil {
		for k, v := range req.Params.Arguments {
			args[k] = v
		}
	}
	return args
}

// boolPtr returns a pointer to the given bool value.
func boolPtr(b bool) *bool {
	return &b
}

// Format helper for errors
func formatError(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}