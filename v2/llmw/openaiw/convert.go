package openaiw

import (
	"fmt"

	"github.com/AndreeJait/go-utility/v2/llmw"
	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/param"
)

// openaiString is a convenience wrapper for creating optional string parameters.
func openaiString(s string) param.Opt[string] { return param.NewOpt(s) }

// --- Domain → SDK conversions ---

// toSDKMessages converts llmw messages to OpenAI SDK message params.
func toSDKMessages(messages []llmw.Message) ([]openai.ChatCompletionMessageParamUnion, error) {
	sdkMessages := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))

	for _, msg := range messages {
		switch msg.Role {
		case llmw.RoleSystem:
			sdkMsg := openai.SystemMessage(msg.Content)
			if msg.Name != "" {
				sdkMsg.OfSystem = &openai.ChatCompletionSystemMessageParam{
					Content: openai.ChatCompletionSystemMessageParamContentUnion{
						OfString: openaiString(msg.Content),
					},
					Name: openaiString(msg.Name),
				}
			}
			sdkMessages = append(sdkMessages, sdkMsg)

		case llmw.RoleUser:
			sdkMsg := openai.UserMessage(msg.Content)
			if msg.Name != "" {
				sdkMsg.OfUser = &openai.ChatCompletionUserMessageParam{
					Content: openai.ChatCompletionUserMessageParamContentUnion{
						OfString: openaiString(msg.Content),
					},
					Name: openaiString(msg.Name),
				}
			}
			sdkMessages = append(sdkMessages, sdkMsg)

		case llmw.RoleAssistant:
			if len(msg.ToolCalls) > 0 {
				// Assistant message with tool calls requires manual construction
				assistantMsg := &openai.ChatCompletionAssistantMessageParam{
					ToolCalls: make([]openai.ChatCompletionMessageToolCallUnionParam, 0, len(msg.ToolCalls)),
				}
				if msg.Content != "" {
					assistantMsg.Content = openai.ChatCompletionAssistantMessageParamContentUnion{
						OfString: openaiString(msg.Content),
					}
				}
				if msg.Name != "" {
					assistantMsg.Name = openaiString(msg.Name)
				}
				for _, tc := range msg.ToolCalls {
					assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, openai.ChatCompletionMessageToolCallUnionParam{
						OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
							ID:   tc.ID,
							Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
								Name:      tc.Name,
								Arguments: tc.Arguments,
							},
						},
					})
				}
				sdkMessages = append(sdkMessages, openai.ChatCompletionMessageParamUnion{
					OfAssistant: assistantMsg,
				})
			} else {
				sdkMsg := openai.AssistantMessage(msg.Content)
				if msg.Name != "" {
					sdkMsg.OfAssistant = &openai.ChatCompletionAssistantMessageParam{
						Content: openai.ChatCompletionAssistantMessageParamContentUnion{
							OfString: openaiString(msg.Content),
						},
						Name: openaiString(msg.Name),
					}
				}
				sdkMessages = append(sdkMessages, sdkMsg)
			}

		case llmw.RoleTool:
			sdkMsg := openai.ToolMessage(msg.Content, msg.ToolCallID)
			sdkMessages = append(sdkMessages, sdkMsg)

		default:
			return nil, fmt.Errorf("openaiw: unsupported message role: %s", msg.Role)
		}
	}

	return sdkMessages, nil
}

// toSDKTools converts llmw tool definitions to OpenAI SDK tool params.
func toSDKTools(tools []llmw.ToolInfo) ([]openai.ChatCompletionToolUnionParam, error) {
	sdkTools := make([]openai.ChatCompletionToolUnionParam, 0, len(tools))

	for _, tool := range tools {
		params := openai.FunctionParameters{}
		for k, v := range tool.Parameters {
			params[k] = v
		}

		sdkTools = append(sdkTools, openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        tool.Name,
			Description: openaiString(tool.Description),
			Parameters:  params,
		}))
	}

	return sdkTools, nil
}

// --- SDK → Domain conversions ---

// fromSDKCompletion converts an OpenAI ChatCompletion response to llmw.Response.
func fromSDKCompletion(completion *openai.ChatCompletion) llmw.Response {
	choice := completion.Choices[0]
	msg := choice.Message

	resp := llmw.Response{
		ID: completion.ID,
		Usage: llmw.Usage{
			InputTokens:  int(completion.Usage.PromptTokens),
			OutputTokens: int(completion.Usage.CompletionTokens),
			TotalTokens:  int(completion.Usage.TotalTokens),
		},
		Message: llmw.Message{
			Role:    llmw.RoleAssistant,
			Content: msg.Content,
		},
	}

	// Convert tool calls from the response
	// The response type uses ChatCompletionMessageToolCallUnion which has
	// Function, ID, Type, and Custom fields directly
	for _, tc := range msg.ToolCalls {
		resp.Message.ToolCalls = append(resp.Message.ToolCalls, llmw.ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}

	return resp
}