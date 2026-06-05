package openaiw

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/AndreeJait/go-utility/v2/llmw"
	"github.com/AndreeJait/go-utility/v2/logw"
	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type openaiLLM struct {
	client *openai.Client
	cfg    *Config
}

// Config holds the configuration for the OpenAI LLM provider.
type Config struct {
	// APIKey is the OpenAI API key. Defaults to OPENAI_API_KEY environment variable.
	APIKey string
	// Model is the default model to use for chat completions. Defaults to "gpt-4o".
	Model string
	// BaseURL is the API base URL. Set this for OpenAI-compatible APIs
	// (Ollama, DeepSeek, etc.). Defaults to "https://api.openai.com/v1".
	BaseURL string
	// OrgID is the optional organization ID.
	OrgID string
	// HTTPClient is an optional custom HTTP client. Use this to inject authenticated
	// clients (e.g., gcpw.AuthenticatedHTTPClient for GCP Cloud Run). If nil, the
	// SDK's default client is used.
	HTTPClient *http.Client
}

// New creates a new OpenAI LLM provider.
// If cfg is nil, defaults are applied (API key from OPENAI_API_KEY env, model "gpt-4o").
func New(cfg *Config) (llmw.LLM, error) {
	if cfg == nil {
		cfg = &Config{}
	}

	var opts []option.RequestOption
	if cfg.APIKey != "" {
		opts = append(opts, option.WithAPIKey(cfg.APIKey))
	}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}
	if cfg.OrgID != "" {
		opts = append(opts, option.WithOrganization(cfg.OrgID))
	}
	if cfg.HTTPClient != nil {
		opts = append(opts, option.WithHTTPClient(cfg.HTTPClient))
	}

	client := openai.NewClient(opts...)

	if cfg.Model == "" {
		cfg.Model = "gpt-4o"
	}

	logw.Infof("openaiw: initialized LLM provider with model %q", cfg.Model)

	return &openaiLLM{
		client: &client,
		cfg:    cfg,
	}, nil
}

// Chat sends messages to the OpenAI API and returns a complete response.
func (l *openaiLLM) Chat(ctx context.Context, messages []llmw.Message, opts ...llmw.Option) (*llmw.Response, error) {
	rc := llmw.ResolveOptions(opts...)

	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(l.resolveModel(rc)),
		Messages: mustToSDKMessages(messages),
	}

	// Apply optional parameters
	if rc.Temperature != nil {
		params.Temperature = openai.Float(*rc.Temperature)
	}
	if rc.MaxTokens != nil {
		params.MaxTokens = openai.Int(int64(*rc.MaxTokens))
	}
	if rc.TopP != nil {
		params.TopP = openai.Float(*rc.TopP)
	}
	if len(rc.StopWords) > 0 {
		params.Stop = openai.ChatCompletionNewParamsStopUnion{
			OfStringArray: rc.StopWords,
		}
	}

	// Convert tools
	if len(rc.Tools) > 0 {
		params.Tools = mustToSDKTools(rc.Tools)
	}

	logw.CtxInfof(ctx, "openaiw: sending chat request (model=%s, messages=%d, tools=%d)",
		l.resolveModel(rc), len(messages), len(rc.Tools))

	completion, err := l.client.Chat.Completions.New(ctx, params)
	if err != nil {
		logw.CtxErrorf(ctx, "openaiw: chat request failed: %v", err)
		return nil, fmt.Errorf("openaiw: chat request failed: %w", err)
	}

	if len(completion.Choices) == 0 {
		return nil, errors.New("openaiw: no choices in response")
	}

	resp := fromSDKCompletion(completion)
	logw.CtxInfof(ctx, "openaiw: chat response received (id=%s, tokens=%d/%d)",
		resp.ID, resp.Usage.InputTokens, resp.Usage.OutputTokens)

	return &resp, nil
}

// Stream sends messages to the OpenAI API and returns a channel of streaming chunks.
func (l *openaiLLM) Stream(ctx context.Context, messages []llmw.Message, opts ...llmw.Option) (<-chan llmw.StreamChunk, error) {
	rc := llmw.ResolveOptions(opts...)

	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(l.resolveModel(rc)),
		Messages: mustToSDKMessages(messages),
	}

	// Apply optional parameters
	if rc.Temperature != nil {
		params.Temperature = openai.Float(*rc.Temperature)
	}
	if rc.MaxTokens != nil {
		params.MaxTokens = openai.Int(int64(*rc.MaxTokens))
	}
	if rc.TopP != nil {
		params.TopP = openai.Float(*rc.TopP)
	}
	if len(rc.StopWords) > 0 {
		params.Stop = openai.ChatCompletionNewParamsStopUnion{
			OfStringArray: rc.StopWords,
		}
	}
	if len(rc.Tools) > 0 {
		params.Tools = mustToSDKTools(rc.Tools)
	}

	logw.CtxInfof(ctx, "openaiw: starting stream request (model=%s, messages=%d)",
		l.resolveModel(rc), len(messages))

	stream := l.client.Chat.Completions.NewStreaming(ctx, params)

	ch := make(chan llmw.StreamChunk, 64)

	go func() {
		defer close(ch)

		for stream.Next() {
			chunk := stream.Current()

			// Skip empty choices (keep-alive)
			if len(chunk.Choices) == 0 {
				continue
			}

			choice := chunk.Choices[0]
			delta := choice.Delta

			sdkChunk := llmw.StreamChunk{}

			// Content delta
			if delta.Content != "" {
				sdkChunk.Content = delta.Content
			}

			// Tool call deltas
			for _, tc := range delta.ToolCalls {
				deltaTC := llmw.ToolCallDelta{
					Index: int(tc.Index),
				}
				if tc.ID != "" {
					deltaTC.ID = tc.ID
				}
				if tc.Function.Name != "" {
					deltaTC.Name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					deltaTC.Arguments = tc.Function.Arguments
				}
				sdkChunk.ToolCalls = append(sdkChunk.ToolCalls, deltaTC)
			}

			// Check for finish
			if choice.FinishReason != "" {
				sdkChunk.Done = true

				// Final usage - check via JSON field presence
				if chunk.JSON.Usage.Valid() {
					sdkChunk.Usage = &llmw.Usage{
						InputTokens:  int(chunk.Usage.PromptTokens),
						OutputTokens: int(chunk.Usage.CompletionTokens),
						TotalTokens:  int(chunk.Usage.TotalTokens),
					}
				}
			}

			select {
			case ch <- sdkChunk:
			case <-ctx.Done():
				return
			}
		}

		if err := stream.Err(); err != nil && !errors.Is(err, context.Canceled) {
			ch <- llmw.StreamChunk{Done: true}
		} else if stream.Err() == nil {
			// Stream ended normally — ensure we send a done signal
			// only if we haven't already sent one via finish_reason
		}
	}()

	return ch, nil
}

// Close is a no-op for the OpenAI provider (the SDK manages connections).
func (l *openaiLLM) Close() error {
	logw.Infof("openaiw: closing LLM provider")
	return nil
}

// resolveModel returns the model to use, preferring the per-request model over the default.
func (l *openaiLLM) resolveModel(rc *llmw.RequestConfig) string {
	if rc.Model != "" {
		return rc.Model
	}
	return l.cfg.Model
}

// mustToSDKMessages converts messages and panics on error.
func mustToSDKMessages(messages []llmw.Message) []openai.ChatCompletionMessageParamUnion {
	sdkMessages, err := toSDKMessages(messages)
	if err != nil {
		panic(fmt.Sprintf("openaiw: failed to convert messages: %v", err))
	}
	return sdkMessages
}

// mustToSDKTools converts tools and panics on error.
func mustToSDKTools(tools []llmw.ToolInfo) []openai.ChatCompletionToolUnionParam {
	sdkTools, err := toSDKTools(tools)
	if err != nil {
		panic(fmt.Sprintf("openaiw: failed to convert tools: %v", err))
	}
	return sdkTools
}

// Compile-time interface compliance check.
var _ llmw.LLM = (*openaiLLM)(nil)