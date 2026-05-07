package graphw

import (
	"context"
	"fmt"

	"github.com/AndreeJait/go-utility/v2/llmw"
	"github.com/AndreeJait/go-utility/v2/logw"
)

// ChatNode creates a NodeFunc that calls an LLM and processes the response.
// This is the most common node pattern: build messages from state, call the LLM,
// then process the response into a state update.
//
// Example:
//
//	thinkNode := graphw.ChatNode[MyState](
//	    llm,
//	    func(ctx context.Context, state MyState) ([]llmw.Message, error) {
//	        return []llmw.Message{
//	            {Role: llmw.RoleSystem, Content: "You are a helpful assistant."},
//	            {Role: llmw.RoleUser, Content: state.Input},
//	        }, nil
//	    },
//	    func(ctx context.Context, state MyState, resp *llmw.Response) (graphw.NodeResult[MyState], error) {
//	        state.Messages = append(state.Messages, resp.Message.Content)
//	        return graphw.NodeResult[MyState]{State: state}, nil
//	    },
//	)
func ChatNode[S any](
	llm llmw.LLM,
	buildMessages func(ctx context.Context, state S) ([]llmw.Message, error),
	processResponse func(ctx context.Context, state S, resp *llmw.Response) (NodeResult[S], error),
) NodeFunc[S] {
	return func(ctx context.Context, state S) (NodeResult[S], error) {
		messages, err := buildMessages(ctx, state)
		if err != nil {
			return NodeResult[S]{}, fmt.Errorf("graphw: chat node build messages: %w", err)
		}

		resp, err := llm.Chat(ctx, messages)
		if err != nil {
			return NodeResult[S]{}, fmt.Errorf("graphw: chat node LLM call: %w", err)
		}

		logw.CtxInfof(ctx, "graphw: chat node got response (id=%s, tokens=%d)",
			resp.ID, resp.Usage.TotalTokens)

		return processResponse(ctx, state, resp)
	}
}

// ToolResult represents the result of a tool execution.
type ToolResult struct {
	// ToolCallID is the ID of the tool call this result responds to.
	ToolCallID string
	// Name is the name of the tool that was executed.
	Name string
	// Content is the text result of the tool execution.
	Content string
}

// ToolNodeConfig holds the configuration for a tool execution node.
type ToolNodeConfig[S any] struct {
	// ExtractToolCalls extracts tool calls from the state.
	// This is typically called after an LLM response that contains tool calls.
	ExtractToolCalls func(state S) []llmw.ToolCall
	// SetToolResults stores tool execution results back into the state.
	SetToolResults func(state S, results []ToolResult) S
	// ToolExecutor executes a single tool call and returns the result.
	ToolExecutor func(ctx context.Context, name string, arguments string) (string, error)
}

// NewToolNode creates a NodeFunc that executes tool calls from the state.
// This is the companion to ChatNode in an agent loop pattern:
//
//	agent -> tool -> agent -> tool -> ... -> END
//
// Example:
//
//	toolNode := graphw.NewToolNode[MyState](graphw.ToolNodeConfig[MyState]{
//	    ExtractToolCalls: func(state MyState) []llmw.ToolCall {
//	        return state.PendingToolCalls
//	    },
//	    SetToolResults: func(state MyState, results []graphw.ToolResult) MyState {
//	        state.ToolResults = results
//	        state.PendingToolCalls = nil
//	        return state
//	    },
//	    ToolExecutor: func(ctx context.Context, name string, args string) (string, error) {
//	        // Execute the tool
//	        return executeTool(name, args)
//	    },
//	})
func NewToolNode[S any](cfg ToolNodeConfig[S]) NodeFunc[S] {
	return func(ctx context.Context, state S) (NodeResult[S], error) {
		toolCalls := cfg.ExtractToolCalls(state)
		if len(toolCalls) == 0 {
			// No tool calls to execute — return state unchanged
			return NodeResult[S]{State: state}, nil
		}

		results := make([]ToolResult, 0, len(toolCalls))
		for _, tc := range toolCalls {
			logw.CtxInfof(ctx, "graphw: executing tool %q (id=%s)", tc.Name, tc.ID)

			content, err := cfg.ToolExecutor(ctx, tc.Name, tc.Arguments)
			if err != nil {
				content = fmt.Sprintf("error: %v", err)
				logw.CtxErrorf(ctx, "graphw: tool %q execution failed: %v", tc.Name, err)
			}

			results = append(results, ToolResult{
				ToolCallID: tc.ID,
				Name:       tc.Name,
				Content:    content,
			})
		}

		updatedState := cfg.SetToolResults(state, results)
		return NodeResult[S]{State: updatedState}, nil
	}
}

// MCPToolExecutor creates a tool executor function from an MCP client session.
// This bridges graphw with mcpw, allowing MCP tools to be used as LLM tool calls.
//
// Note: Requires importing mcpw package. To avoid circular dependencies,
// this function returns a function signature matching ToolExecutor.
// You need to pass an mcpw.ClientSession and call its CallTool method.
//
// Example:
//
//	executor := graphw.MCPToolExecutor(mcpSession)
//	toolNode := graphw.NewToolNode[MyState](graphw.ToolNodeConfig[MyState]{
//	    ToolExecutor: executor,
//	    ...
//	})
func MCPToolExecutor(callTool func(ctx context.Context, name string, arguments string) (string, error)) func(ctx context.Context, name string, arguments string) (string, error) {
	return callTool
}