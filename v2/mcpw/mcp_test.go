package mcpw

import (
	"context"
	"net/http"
	"testing"
)

// TestServerInterfaceCompliance ensures mcpServer implements Server.
func TestServerInterfaceCompliance(t *testing.T) {
	var _ Server = (*mcpServer)(nil)
}

// TestClientInterfaceCompliance ensures mcpClient implements Client.
func TestClientInterfaceCompliance(t *testing.T) {
	var _ Client = (*mcpClient)(nil)
}

// TestClientSessionInterfaceCompliance ensures mcpClientSession implements ClientSession.
func TestClientSessionInterfaceCompliance(t *testing.T) {
	var _ ClientSession = (*mcpClientSession)(nil)
}

// TestContentTypeMarker ensures all content types implement the Content interface.
func TestContentTypeMarker(t *testing.T) {
	var _ Content = TextContent{}
	var _ Content = ImageContent{}
	var _ Content = AudioContent{}
	var _ Content = EmbeddedResource{}
}

// TestNewServer_NilConfig ensures NewServer handles nil config gracefully.
func TestNewServer_NilConfig(t *testing.T) {
	srv := NewServer(nil)
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}

// TestNewClient_NilConfig ensures NewClient handles nil config gracefully.
func TestNewClient_NilConfig(t *testing.T) {
	cli := NewClient(nil)
	if cli == nil {
		t.Fatal("expected non-nil client")
	}
}

// TestNewServer_ValidConfig tests server creation with valid config.
func TestNewServer_ValidConfig(t *testing.T) {
	srv := NewServer(&ServerConfig{
		Name:    "test-server",
		Version: "1.0.0",
	})
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}

// TestNewClient_ValidConfig tests client creation with valid config.
func TestNewClient_ValidConfig(t *testing.T) {
	cli := NewClient(&ClientConfig{
		Name:    "test-client",
		Version: "1.0.0",
	})
	if cli == nil {
		t.Fatal("expected non-nil client")
	}
}

// TestServer_AddTool tests that a tool can be registered without panicking.
func TestServer_AddTool(t *testing.T) {
	srv := NewServer(&ServerConfig{Name: "test", Version: "1.0"})
	srv.AddTool(&ToolInfo{
		Name:        "greet",
		Description: "Say hello",
	}, func(ctx context.Context, params *ToolCallParams) (*ToolCallResult, error) {
		return &ToolCallResult{
			Content: []Content{TextContent{Text: "Hello!"}},
		}, nil
	})
}

// TestServer_AddResource tests that a resource can be registered without panicking.
func TestServer_AddResource(t *testing.T) {
	srv := NewServer(&ServerConfig{Name: "test", Version: "1.0"})
	srv.AddResource(&ResourceInfo{
		URI:      "file:///data/config.json",
		Name:     "Config",
		MIMEType: "application/json",
	}, func(ctx context.Context, uri string) (*ReadResourceResult, error) {
		return &ReadResourceResult{
			Contents: []ResourceContent{{
				URI:      uri,
				MIMEType: "application/json",
				Text:     `{"key": "value"}`,
			}},
		}, nil
	})
}

// TestServer_AddResourceTemplate tests that a resource template can be registered.
func TestServer_AddResourceTemplate(t *testing.T) {
	srv := NewServer(&ServerConfig{Name: "test", Version: "1.0"})
	srv.AddResourceTemplate(&ResourceTemplateInfo{
		URITemplate: "file:///{path}",
		Name:        "File",
		MIMEType:    "application/octet-stream",
	}, func(ctx context.Context, uri string) (*ReadResourceResult, error) {
		return &ReadResourceResult{
			Contents: []ResourceContent{{
				URI:      uri,
				MIMEType: "application/octet-stream",
				Blob:     []byte("data"),
			}},
		}, nil
	})
}

// TestServer_AddPrompt tests that a prompt can be registered without panicking.
func TestServer_AddPrompt(t *testing.T) {
	srv := NewServer(&ServerConfig{Name: "test", Version: "1.0"})
	srv.AddPrompt(&PromptInfo{
		Name:        "greeting",
		Description: "A greeting prompt",
		Arguments: []PromptArg{
			{Name: "name", Description: "The person to greet", Required: true},
		},
	}, func(ctx context.Context, name string, args map[string]string) (*PromptResult, error) {
		personName := args["name"]
		return &PromptResult{
			Description: "Greeting prompt",
			Messages: []PromptMessage{
				{Role: RoleUser, Content: TextContent{Text: "Hello " + personName}},
			},
		}, nil
	})
}

// TestContentTypes verifies that the content type values are correct.
func TestContentTypes(t *testing.T) {
	text := TextContent{Text: "hello"}
	img := ImageContent{Data: []byte{0x89, 0x50}, MIMEType: "image/png"}
	audio := AudioContent{Data: []byte{0xFF}, MIMEType: "audio/wav"}

	if text.Text != "hello" {
		t.Errorf("expected text 'hello', got %q", text.Text)
	}
	if img.MIMEType != "image/png" {
		t.Errorf("expected MIME type 'image/png', got %q", img.MIMEType)
	}
	if audio.MIMEType != "audio/wav" {
		t.Errorf("expected MIME type 'audio/wav', got %q", audio.MIMEType)
	}
}

// TestTransportCreation tests that transport factory functions don't panic.
func TestTransportCreation(t *testing.T) {
	_ = StdioTransport()
	_ = CommandTransport("echo", "hello")
	_ = StreamableClientTransport("http://localhost:8080/mcp")
	_ = SSEClientTransport("http://localhost:8080/sse")
}

// TestInMemoryTransport tests that InMemoryTransport creates connected transports.
func TestInMemoryTransport(t *testing.T) {
	client, server := InMemoryTransport()
	if client.inner == nil {
		t.Fatal("expected non-nil client transport")
	}
	if server.inner == nil {
		t.Fatal("expected non-nil server transport")
	}
}

// TestStreamableHTTPHandler tests handler creation without panicking.
func TestStreamableHTTPHandler(t *testing.T) {
	srv := NewServer(&ServerConfig{Name: "test", Version: "1.0"})
	handler := NewStreamableHTTPHandler(func(r *http.Request) Server {
		return srv
	}, nil)
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}
