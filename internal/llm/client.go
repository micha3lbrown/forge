package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/shared"
)

// Client is the interface for LLM interactions.
type Client interface {
	ChatCompletion(ctx context.Context, messages []Message, tools []ToolDef) (*Response, error)
	ChatCompletionStream(ctx context.Context, messages []Message, tools []ToolDef, handler StreamHandler) (*Response, error)
}

// OpenAICompatClient works with any OpenAI-compatible API (Ollama, Claude, Gemini).
type OpenAICompatClient struct {
	client  *openai.Client
	model   string
	baseURL string
}

// NewClient creates an LLM client for the given provider.
func NewClient(baseURL, apiKey, model string) *OpenAICompatClient {
	client := openai.NewClient(
		option.WithBaseURL(baseURL),
		option.WithAPIKey(apiKey),
	)
	return &OpenAICompatClient{
		client:  &client,
		model:   model,
		baseURL: baseURL,
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

	var completion *openai.ChatCompletion
	var err error
	for attempt := range 3 {
		completion, err = c.client.Chat.Completions.New(ctx, params)
		if err == nil {
			break
		}
		if !strings.Contains(err.Error(), "429") || attempt == 2 {
			return nil, fmt.Errorf("chat completion: %w", err)
		}
		wait := time.Duration(2<<attempt) * time.Second // 2s, 4s
		fmt.Printf("\n  (rate limited, retrying in %s...)\n", wait)
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, fmt.Errorf("chat completion: %w", ctx.Err())
		}
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
			out = append(out, openai.ToolMessage(m.Content, m.ToolCallID))
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

// ListModels queries Ollama's native /api/tags endpoint for available models.
// The baseURL is expected to end with /v1/ (OpenAI-compat); we strip that to
// reach the native Ollama API.
func (c *OpenAICompatClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	// Strip /v1/ suffix to get Ollama's base, e.g. "http://host:11434/v1/" -> "http://host:11434"
	base := strings.TrimRight(c.baseURL, "/")
	base = strings.TrimSuffix(base, "/v1")
	url := base + "/api/tags"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama API returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Models []struct {
			Name       string `json:"name"`
			Size       int64  `json:"size"`
			ModifiedAt string `json:"modified_at"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	models := make([]ModelInfo, len(result.Models))
	for i, m := range result.Models {
		models[i] = ModelInfo{
			Name:       m.Name,
			Size:       m.Size,
			ModifiedAt: m.ModifiedAt,
		}
	}
	return models, nil
}
