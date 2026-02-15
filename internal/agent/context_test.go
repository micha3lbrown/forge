package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/michaelbrown/forge/internal/llm"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name    string
		message llm.Message
		wantMin int
		wantMax int
	}{
		{
			name:    "empty message",
			message: llm.Message{Role: llm.RoleUser},
			wantMin: 1,
			wantMax: 1,
		},
		{
			name:    "short user message",
			message: llm.UserMessage("hello world"),
			wantMin: 2,
			wantMax: 4,
		},
		{
			name:    "long message",
			message: llm.UserMessage(strings.Repeat("a", 400)),
			wantMin: 99,
			wantMax: 101,
		},
		{
			name: "message with tool calls",
			message: llm.Message{
				Role: llm.RoleAssistant,
				ToolCalls: []llm.ToolCall{
					{ID: "1", Name: "shell_exec", Args: map[string]any{"command": "ls -la"}},
				},
			},
			wantMin: 5,
			wantMax: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateTokens(tt.message)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("estimateTokens() = %d, want between %d and %d", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestEstimateHistoryTokens(t *testing.T) {
	messages := []llm.Message{
		llm.SystemMessage("You are a helpful assistant."),
		llm.UserMessage("Hello"),
		llm.AssistantMessage("Hi there! How can I help?"),
	}
	total := estimateHistoryTokens(messages)
	if total < 10 {
		t.Errorf("estimateHistoryTokens() = %d, want at least 10", total)
	}
}

func TestFindSplitPoint(t *testing.T) {
	tests := []struct {
		name         string
		messages     []llm.Message
		recentBudget int
		wantIdx      int
	}{
		{
			name: "small history, no split needed",
			messages: []llm.Message{
				llm.SystemMessage("system"),
				llm.UserMessage("hi"),
			},
			recentBudget: 1000,
			wantIdx:      2, // len(messages), no split
		},
		{
			name: "history exceeds budget, splits at user boundary",
			messages: []llm.Message{
				llm.SystemMessage("system"),
				llm.UserMessage(strings.Repeat("first question ", 20)),
				llm.AssistantMessage(strings.Repeat("first answer ", 20)),
				llm.UserMessage(strings.Repeat("second question ", 20)),
				llm.AssistantMessage(strings.Repeat("second answer ", 20)),
				llm.UserMessage(strings.Repeat("third question ", 20)),
				llm.AssistantMessage(strings.Repeat("third answer ", 20)),
			},
			recentBudget: 120, // fits ~2 messages → split at index 5
			wantIdx:      5,   // should land on "third question" (a user message)
		},
		{
			name: "does not split tool call from result",
			messages: []llm.Message{
				llm.SystemMessage("system"),
				llm.UserMessage(strings.Repeat("do something ", 20)),
				{Role: llm.RoleAssistant, Content: "", ToolCalls: []llm.ToolCall{
					{ID: "tc1", Name: "shell_exec", Args: map[string]any{"command": strings.Repeat("ls ", 50)}},
				}},
				llm.ToolResultMessage("tc1", strings.Repeat("file1\nfile2\n", 20)),
				llm.AssistantMessage(strings.Repeat("I found files. ", 20)),
				llm.UserMessage(strings.Repeat("thanks ", 10)),
				llm.AssistantMessage(strings.Repeat("welcome ", 10)),
			},
			recentBudget: 50,
			wantIdx:      5, // should split at "thanks" (user msg), not in tool call/result
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findSplitPoint(tt.messages, tt.recentBudget)
			if got != tt.wantIdx {
				t.Errorf("findSplitPoint() = %d, want %d", got, tt.wantIdx)
			}
			// Verify split point is at a user message or at the end
			if got < len(tt.messages) && got > 0 {
				if tt.messages[got].Role != llm.RoleUser {
					t.Errorf("split point message role = %s, want user", tt.messages[got].Role)
				}
			}
		})
	}
}

// mockClient implements llm.Client for testing.
type mockClient struct {
	responses []llm.Response
	callCount int
}

func (m *mockClient) ChatCompletion(ctx context.Context, messages []llm.Message, tools []llm.ToolDef) (*llm.Response, error) {
	if m.callCount >= len(m.responses) {
		return nil, fmt.Errorf("no more mock responses")
	}
	resp := m.responses[m.callCount]
	m.callCount++
	return &resp, nil
}

func (m *mockClient) ChatCompletionStream(ctx context.Context, messages []llm.Message, tools []llm.ToolDef, handler llm.StreamHandler) (*llm.Response, error) {
	return m.ChatCompletion(ctx, messages, tools)
}

func (m *mockClient) ListModels(ctx context.Context) ([]llm.ModelInfo, error) {
	return nil, nil
}

func TestCompactHistory(t *testing.T) {
	mock := &mockClient{
		responses: []llm.Response{
			// Summarization response
			{Message: llm.AssistantMessage("User asked about files. Assistant listed them.")},
			// Final response for Run
			{Message: llm.AssistantMessage("Here you go!")},
		},
	}

	a := &Agent{
		llm:       mock,
		maxTokens: 50, // very small budget to force compaction
		maxIter:   5,
		history: []llm.Message{
			llm.SystemMessage("You are helpful."),
			llm.UserMessage("list files"),
			llm.AssistantMessage(strings.Repeat("file info ", 50)), // large response
			llm.UserMessage("tell me more"),
			llm.AssistantMessage(strings.Repeat("more info ", 50)), // large response
			llm.UserMessage("and more"),
			llm.AssistantMessage(strings.Repeat("even more ", 50)), // large response
		},
		tools: []llm.ToolDef{},
	}

	// Run compactHistory
	err := a.compactHistory(context.Background())
	if err != nil {
		t.Fatalf("compactHistory() error = %v", err)
	}

	// History should be shorter now
	if len(a.history) >= 7 {
		t.Errorf("expected compacted history to be shorter than 7, got %d", len(a.history))
	}

	// First message should still be system prompt
	if a.history[0].Role != llm.RoleSystem {
		t.Errorf("first message role = %s, want system", a.history[0].Role)
	}

	// Second message should be the summary
	if !strings.Contains(a.history[1].Content, "[Prior conversation summary]") {
		t.Errorf("second message should contain summary marker, got: %s", a.history[1].Content)
	}
}

func TestCompactHistoryUnderBudget(t *testing.T) {
	a := &Agent{
		maxTokens: 10000, // large budget
		history: []llm.Message{
			llm.SystemMessage("system"),
			llm.UserMessage("hi"),
			llm.AssistantMessage("hello"),
		},
	}

	err := a.compactHistory(context.Background())
	if err != nil {
		t.Fatalf("compactHistory() error = %v", err)
	}

	// Should not have changed
	if len(a.history) != 3 {
		t.Errorf("history length = %d, want 3 (no compaction)", len(a.history))
	}
}

func TestCompactHistoryFallbackOnError(t *testing.T) {
	mock := &mockClient{
		responses: []llm.Response{}, // no responses → will error
	}

	a := &Agent{
		llm:       mock,
		maxTokens: 10, // tiny budget to force compaction
		maxIter:   5,
		history: []llm.Message{
			llm.SystemMessage("system"),
			llm.UserMessage("q1"),
			llm.AssistantMessage(strings.Repeat("a", 200)),
			llm.UserMessage("q2"),
			llm.AssistantMessage(strings.Repeat("b", 200)),
			llm.UserMessage("q3"),
			llm.AssistantMessage(strings.Repeat("c", 200)),
			llm.UserMessage("q4"),
			llm.AssistantMessage(strings.Repeat("d", 200)),
			llm.UserMessage("q5"),
			llm.AssistantMessage(strings.Repeat("e", 200)),
			llm.UserMessage("q6"),
			llm.AssistantMessage(strings.Repeat("f", 200)),
		},
		tools: []llm.ToolDef{},
	}

	originalLen := len(a.history)
	err := a.compactHistory(context.Background())
	if err != nil {
		t.Fatalf("compactHistory() should not return error on fallback, got: %v", err)
	}

	// Should have trimmed via fallback
	if len(a.history) >= originalLen {
		t.Errorf("expected trimmed history, got same length %d", len(a.history))
	}
}
