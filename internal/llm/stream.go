package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/packages/ssestream"
	"github.com/openai/openai-go/shared"
)

// StreamHandler receives text deltas during streaming.
type StreamHandler func(delta string)

// ChatCompletionStream sends a streaming chat completion request.
// The handler is called with each text delta as it arrives.
// Returns the full response once streaming is complete.
func (c *OpenAICompatClient) ChatCompletionStream(ctx context.Context, messages []Message, tools []ToolDef, handler StreamHandler) (*Response, error) {
	params := openai.ChatCompletionNewParams{
		Model:    c.model,
		Messages: convertMessages(messages),
	}

	if len(tools) > 0 {
		params.Tools = convertStreamTools(tools)
	}

	var stream *ssestream.Stream[openai.ChatCompletionChunk]
	var err error
	for attempt := range 3 {
		stream = c.client.Chat.Completions.NewStreaming(ctx, params)
		err = stream.Err()
		if err == nil {
			break
		}
		if !strings.Contains(err.Error(), "429") || attempt == 2 {
			return nil, fmt.Errorf("chat completion stream: %w", err)
		}
		stream.Close()
		wait := time.Duration(2<<attempt) * time.Second
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, fmt.Errorf("chat completion stream: %w", ctx.Err())
		}
	}
	defer stream.Close()

	acc := openai.ChatCompletionAccumulator{}

	for stream.Next() {
		chunk := stream.Current()
		acc.AddChunk(chunk)

		// Send text deltas to handler
		if len(chunk.Choices) > 0 && handler != nil {
			delta := chunk.Choices[0].Delta.Content
			if delta != "" {
				handler(delta)
			}
		}
	}

	if err := stream.Err(); err != nil {
		return nil, fmt.Errorf("streaming: %w", err)
	}

	if len(acc.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned")
	}

	choice := acc.Choices[0]
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

func convertStreamTools(tools []ToolDef) []openai.ChatCompletionToolParam {
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
