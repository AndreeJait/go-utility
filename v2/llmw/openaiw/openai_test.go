package openaiw

import (
	"testing"

	"github.com/AndreeJait/go-utility/v2/llmw"
)

// TestLLMInterfaceCompliance ensures openaiLLM implements llmw.LLM.
func TestLLMInterfaceCompliance(t *testing.T) {
	var _ llmw.LLM = (*openaiLLM)(nil)
}

// TestNew_NilConfig ensures New handles nil config gracefully.
func TestNew_NilConfig(t *testing.T) {
	// This will fail without an API key, but we test that it doesn't panic.
	// In a real test suite, you'd use a mock server.
	_, err := New(nil)
	// We don't assert on the error since it depends on the OPENAI_API_KEY env var.
	// The important thing is that it doesn't panic.
	_ = err
}

// TestNew_ValidConfig tests provider creation with valid config.
func TestNew_ValidConfig(t *testing.T) {
	_, err := New(&Config{
		APIKey: "test-key",
		Model:  "gpt-4o",
	})
	_ = err // doesn't panic
}

// TestNew_DefaultModel tests that the default model is set.
func TestNew_DefaultModel(t *testing.T) {
	cfg := &Config{APIKey: "test-key"}
	prov, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := prov.(*openaiLLM)
	if p.cfg.Model != "gpt-4o" {
		t.Errorf("expected default model 'gpt-4o', got %q", p.cfg.Model)
	}
}

// TestResolveModel tests that per-request model overrides the default.
func TestResolveModel(t *testing.T) {
	cfg := &Config{APIKey: "test-key", Model: "gpt-4o"}
	prov, _ := New(cfg)
	p := prov.(*openaiLLM)

	// Default model
	rc := &llmw.RequestConfig{}
	if model := p.resolveModel(rc); model != "gpt-4o" {
		t.Errorf("expected default model 'gpt-4o', got %q", model)
	}

	// Override model
	rc2 := &llmw.RequestConfig{Model: "gpt-4o-mini"}
	if model := p.resolveModel(rc2); model != "gpt-4o-mini" {
		t.Errorf("expected override model 'gpt-4o-mini', got %q", model)
	}
}

// TestToSDKMessages tests message conversion from domain to SDK types.
func TestToSDKMessages(t *testing.T) {
	messages := []llmw.Message{
		{Role: llmw.RoleSystem, Content: "You are a helpful assistant."},
		{Role: llmw.RoleUser, Content: "Hello!"},
		{Role: llmw.RoleAssistant, Content: "Hi there!"},
	}

	sdkMessages, err := toSDKMessages(messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sdkMessages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(sdkMessages))
	}
}

// TestToSDKMessages_ToolCalls tests message conversion with tool calls.
func TestToSDKMessages_ToolCalls(t *testing.T) {
	messages := []llmw.Message{
		{Role: llmw.RoleUser, Content: "What's the weather?"},
		{
			Role:    llmw.RoleAssistant,
			Content: "",
			ToolCalls: []llmw.ToolCall{
				{ID: "call_123", Name: "get_weather", Arguments: `{"location":"Boston"}`},
			},
		},
		{Role: llmw.RoleTool, Content: `{"temp": 72}`, ToolCallID: "call_123"},
	}

	sdkMessages, err := toSDKMessages(messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sdkMessages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(sdkMessages))
	}
}

// TestToSDKMessages_UnsupportedRole tests that unsupported roles return an error.
func TestToSDKMessages_UnsupportedRole(t *testing.T) {
	messages := []llmw.Message{
		{Role: "unknown", Content: "test"},
	}
	_, err := toSDKMessages(messages)
	if err == nil {
		t.Fatal("expected error for unsupported role, got nil")
	}
}

// TestToSDKTools tests tool definition conversion.
func TestToSDKTools(t *testing.T) {
	tools := []llmw.ToolInfo{
		{
			Name:        "get_weather",
			Description: "Get the current weather",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"location": map[string]any{"type": "string"},
				},
				"required": []any{"location"},
			},
		},
	}

	sdkTools, err := toSDKTools(tools)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sdkTools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(sdkTools))
	}
}

// TestRoleConstants verifies that role constants are correct.
func TestRoleConstants(t *testing.T) {
	tests := []struct {
		role     llmw.Role
		expected string
	}{
		{llmw.RoleSystem, "system"},
		{llmw.RoleUser, "user"},
		{llmw.RoleAssistant, "assistant"},
		{llmw.RoleTool, "tool"},
	}
	for _, tt := range tests {
		if string(tt.role) != tt.expected {
			t.Errorf("expected role %q, got %q", tt.expected, string(tt.role))
		}
	}
}

// TestClose tests that Close doesn't panic.
func TestClose(t *testing.T) {
	prov, _ := New(&Config{APIKey: "test-key"})
	if err := prov.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}