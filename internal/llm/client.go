package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/shared"
)

// Client is the interface for LLM interactions.
type Client interface {
	ChatCompletion(ctx context.Context, messages []Message, tools []ToolDef) (*Response, error)
}

// OpenAICompatClient works with any OpenAI-compatible API (Ollama, Claude, Gemini).
type OpenAICompatClient struct {
	client *openai.Client
	model  string
}

// NewClient creates an LLM client for the given provider.
func NewClient(baseURL, apiKey, model string) *OpenAICompatClient {
	client := openai.NewClient(
		option.WithBaseURL(baseURL),
		option.WithAPIKey(apiKey),
	)
	return &OpenAICompatClient{
		client: &client,
		model:  model,
	}
}

func (c *OpenAICompatClient) ChatCompletion(ctx context.Context, messages []Message, tools []ToolDef) (*Response, error) {
	params := openai.ChatCompletionNewParams{
		Model:    c.model,
		Messages: convertMessages(messages),
	}

	if len(tools) > 0 {
		params.Tools = convertTools(tools)
	}

	completion, err := c.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("chat completion: %w", err)
	}

	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned")
	}

	choice := completion.Choices[0]
	resp := &Response{
		Message: Message{
			Role:    RoleAssistant,
			Content: choice.Message.Content,
		},
	}

	for _, tc := range choice.Message.ToolCalls {
		var args map[string]any
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			args = map[string]any{"_raw": tc.Function.Arguments}
		}
		resp.Message.ToolCalls = append(resp.Message.ToolCalls, ToolCall{
			ID:   tc.ID,
			Name: tc.Function.Name,
			Args: args,
		})
	}

	return resp, nil
}

func convertMessages(msgs []Message) []openai.ChatCompletionMessageParamUnion {
	var out []openai.ChatCompletionMessageParamUnion
	for _, m := range msgs {
		switch m.Role {
		case RoleSystem:
			out = append(out, openai.SystemMessage(m.Content))
		case RoleUser:
			out = append(out, openai.UserMessage(m.Content))
		case RoleAssistant:
			if len(m.ToolCalls) > 0 {
				toolCalls := make([]openai.ChatCompletionMessageToolCallParam, len(m.ToolCalls))
				for i, tc := range m.ToolCalls {
					argsJSON, _ := json.Marshal(tc.Args)
					toolCalls[i] = openai.ChatCompletionMessageToolCallParam{
						ID: tc.ID,
						Function: openai.ChatCompletionMessageToolCallFunctionParam{
							Name:      tc.Name,
							Arguments: string(argsJSON),
						},
					}
				}
				assistant := openai.ChatCompletionAssistantMessageParam{
					ToolCalls: toolCalls,
				}
				if m.Content != "" {
					assistant.Content.OfString = param.NewOpt(m.Content)
				}
				out = append(out, openai.ChatCompletionMessageParamUnion{
					OfAssistant: &assistant,
				})
			} else {
				out = append(out, openai.AssistantMessage(m.Content))
			}
		case RoleTool:
			out = append(out, openai.ToolMessage(m.ToolCallID, m.Content))
		}
	}
	return out
}

func convertTools(tools []ToolDef) []openai.ChatCompletionToolParam {
	var out []openai.ChatCompletionToolParam
	for _, t := range tools {
		out = append(out, openai.ChatCompletionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        t.Name,
				Description: param.NewOpt(t.Description),
				Parameters:  shared.FunctionParameters(t.Parameters),
			},
		})
	}
	return out
}
