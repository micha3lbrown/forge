package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/michaelbrown/forge/internal/llm"
)

// estimateTokens returns an approximate token count for a message.
// Uses chars/4 heuristic — accurate enough for context management.
func estimateTokens(m llm.Message) int {
	tokens := len(m.Content) / 4
	for _, tc := range m.ToolCalls {
		tokens += len(tc.Name) / 4
		if argsJSON, err := json.Marshal(tc.Args); err == nil {
			tokens += len(argsJSON) / 4
		}
	}
	// Minimum 1 token per message for role overhead
	if tokens == 0 {
		tokens = 1
	}
	return tokens
}

// estimateHistoryTokens returns approximate total tokens for a message slice.
func estimateHistoryTokens(messages []llm.Message) int {
	total := 0
	for _, m := range messages {
		total += estimateTokens(m)
	}
	return total
}

// findSplitPoint finds a clean boundary to split history into old and recent sections.
// It works backward from the end to find the point where recent messages fit within
// the given token budget. The split point will always be at the start of a user message
// to avoid breaking tool call/result pairs.
// Returns the index where the "recent" section begins. Index 0 (system prompt) is never included.
func findSplitPoint(messages []llm.Message, recentTokenBudget int) int {
	if len(messages) <= 2 {
		return len(messages) // nothing to split
	}

	// Walk backward from end, accumulating tokens until we hit the budget.
	// splitIdx will be the first index of the "recent" section to keep.
	tokens := 0
	budgetExceeded := false
	splitIdx := len(messages)
	for i := len(messages) - 1; i >= 1; i-- {
		msgTokens := estimateTokens(messages[i])
		if tokens+msgTokens > recentTokenBudget {
			splitIdx = i + 1
			budgetExceeded = true
			break
		}
		tokens += msgTokens
	}

	// If everything fits within budget, nothing to compact
	if !budgetExceeded {
		return len(messages)
	}

	// Clamp: keep at least the last message
	if splitIdx >= len(messages) {
		splitIdx = len(messages) - 1
	}

	// Ensure we don't split in the middle of a tool call/result sequence.
	// Scan backward from splitIdx to find the nearest user message boundary.
	for splitIdx > 1 {
		if messages[splitIdx].Role == llm.RoleUser {
			break
		}
		splitIdx--
	}

	// Must leave at least the system prompt + 1 message to summarize
	if splitIdx <= 1 || messages[splitIdx].Role != llm.RoleUser {
		return len(messages)
	}

	return splitIdx
}

// summarizeMessages asks the LLM to produce a concise summary of the given messages.
func summarizeMessages(ctx context.Context, client llm.Client, messages []llm.Message) (string, error) {
	// Build a prompt that includes the messages to summarize
	var content string
	for _, m := range messages {
		prefix := string(m.Role)
		if m.ToolCallID != "" {
			prefix = fmt.Sprintf("tool_result(%s)", m.ToolCallID)
		}
		text := m.Content
		if len(m.ToolCalls) > 0 {
			for _, tc := range m.ToolCalls {
				argsJSON, _ := json.Marshal(tc.Args)
				text += fmt.Sprintf("\n[tool_call: %s(%s)]", tc.Name, string(argsJSON))
			}
		}
		content += fmt.Sprintf("[%s]: %s\n", prefix, text)
	}

	summarizationPrompt := []llm.Message{
		llm.SystemMessage("You are a summarization assistant. Produce a concise summary of the following conversation excerpt. " +
			"Preserve key facts, decisions, tool results, and context the user or assistant may need later. " +
			"Be concise but complete. Output only the summary, no preamble."),
		llm.UserMessage("Summarize this conversation:\n\n" + content),
	}

	resp, err := client.ChatCompletion(ctx, summarizationPrompt, nil)
	if err != nil {
		return "", fmt.Errorf("summarization LLM call: %w", err)
	}

	summary := resp.Message.Content

	// Truncate if summary itself is too large (~1000 tokens = ~4000 chars)
	const maxSummaryChars = 4000
	if len(summary) > maxSummaryChars {
		summary = summary[:maxSummaryChars] + "\n... (summary truncated)"
	}

	return summary, nil
}
