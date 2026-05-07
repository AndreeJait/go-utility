package llmw

import (
	"context"
	"errors"
)

// Role represents the role of a message sender in a conversation.
type Role string

const (
	// RoleSystem indicates a system instruction message.
	RoleSystem Role = "system"
	// RoleUser indicates a human user message.
	RoleUser Role = "user"
	// RoleAssistant indicates an AI assistant message.
	RoleAssistant Role = "assistant"
	// RoleTool indicates a tool response message.
	RoleTool Role = "tool"
)

// Message represents a single message in a conversation.
type Message struct {
	// Role identifies who sent the message.
	Role Role
	// Content is the text content of the message.
	Content string
	// ToolCalls contains tool call requests from the assistant.
	// Only set when Role is RoleAssistant.
	ToolCalls []ToolCall
	// ToolCallID identifies which tool call this message responds to.
	// Only set when Role is RoleTool.
	ToolCallID string
	// Name is an optional participant name for the message.
	Name string
}

// ToolCall represents a tool call request from the assistant.
type ToolCall struct {
	// ID is the unique identifier for this tool call.
	ID string
	// Name is the name of the tool to call.
	Name string
	// Arguments is the JSON-encoded arguments for the tool call.
	Arguments string
}

// ToolCallDelta represents a partial tool call update during streaming.
type ToolCallDelta struct {
	// Index is the position of this tool call in the response.
	Index int
	// ID is the tool call identifier (set in the first chunk).
	ID string
	// Name is the tool name (set in the first chunk).
	Name string
	// Arguments is the partial JSON arguments accumulated so far.
	Arguments string
}

// ToolInfo describes a tool that can be called by the LLM.
type ToolInfo struct {
	// Name is the function name.
	Name string
	// Description explains what the tool does.
	Description string
	// Parameters is the JSON Schema describing the tool's input parameters.
	Parameters map[string]any
}

// Response contains the LLM's response to a chat completion request.
type Response struct {
	// ID is the unique identifier for this response.
	ID string
	// Message is the assistant's response message.
	Message Message
	// Usage contains token usage statistics.
	Usage Usage
}

// Usage contains token usage statistics for an LLM response.
type Usage struct {
	// InputTokens is the number of tokens in the prompt.
	InputTokens int
	// OutputTokens is the number of tokens in the response.
	OutputTokens int
	// TotalTokens is the total number of tokens used.
	TotalTokens int
}

// StreamChunk represents a single chunk of a streaming LLM response.
type StreamChunk struct {
	// Content is the text content accumulated in this chunk.
	Content string
	// ToolCalls contains partial tool call updates in this chunk.
	ToolCalls []ToolCallDelta
	// Usage contains token usage statistics. Only non-nil on the final chunk.
	Usage *Usage
	// Done indicates whether this is the final chunk.
	Done bool
}

// LLM is the interface for LLM chat completion providers.
type LLM interface {
	// Chat sends messages to the LLM and returns a complete response.
	Chat(ctx context.Context, messages []Message, opts ...Option) (*Response, error)
	// Stream sends messages to the LLM and returns a channel of streaming chunks.
	Stream(ctx context.Context, messages []Message, opts ...Option) (<-chan StreamChunk, error)
	// Close releases any resources held by the provider.
	Close() error
}

// Embedder is the interface for LLM embedding providers.
type Embedder interface {
	// Embed generates vector embeddings for the given texts.
	Embed(ctx context.Context, texts []string, opts ...Option) ([][]float64, error)
	// Close releases any resources held by the provider.
	Close() error
}

var (
	// ErrNotSupported indicates that the operation is not supported by this provider.
	ErrNotSupported = errors.New("llmw: operation not supported by this provider")
	// ErrModelNotAvailable indicates that the requested model is not available.
	ErrModelNotAvailable = errors.New("llmw: model not available")
)

// --- Options ---

// Option is a functional option for configuring LLM requests.
type Option func(*RequestConfig)

// RequestConfig holds the resolved option values for an LLM request.
// Sub-packages use this to read per-request configuration.
type RequestConfig struct {
	Model       string
	Tools       []ToolInfo
	Temperature *float64
	MaxTokens   *int
	TopP        *float64
	StopWords   []string
}

// ResolveOptions applies functional options and returns the resolved config.
// This is intended for use by sub-packages (providers) to read per-request configuration.
func ResolveOptions(opts ...Option) *RequestConfig {
	cfg := &RequestConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

// WithModel sets the model to use for this request.
func WithModel(model string) Option {
	return func(c *RequestConfig) {
		c.Model = model
	}
}

// WithTools registers tools that the LLM can call during this request.
func WithTools(tools ...ToolInfo) Option {
	return func(c *RequestConfig) {
		c.Tools = append(c.Tools, tools...)
	}
}

// WithTemperature sets the sampling temperature for this request.
func WithTemperature(temp float64) Option {
	return func(c *RequestConfig) {
		c.Temperature = &temp
	}
}

// WithMaxTokens sets the maximum number of tokens to generate.
func WithMaxTokens(tokens int) Option {
	return func(c *RequestConfig) {
		c.MaxTokens = &tokens
	}
}

// WithTopP sets the top-p sampling parameter for this request.
func WithTopP(p float64) Option {
	return func(c *RequestConfig) {
		c.TopP = &p
	}
}

// WithStopWords sets the stop sequences for this request.
func WithStopWords(words ...string) Option {
	return func(c *RequestConfig) {
		c.StopWords = words
	}
}