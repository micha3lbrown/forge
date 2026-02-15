package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/michaelbrown/forge/internal/llm"
	"github.com/michaelbrown/forge/internal/tools"
)

const defaultSystemPrompt = `You are Forge, a helpful AI assistant with access to tools.
When you need information from the system (files, commands, etc.), use the available tools.
Always explain what you're doing and why. After using a tool, interpret the results for the user.`

// Agent manages a conversation and executes the ReAct loop.
type Agent struct {
	llm          llm.Client
	utilityLLM   llm.Client // optional, for summarization/titles
	registry     *tools.Registry
	history      []llm.Message
	tools        []llm.ToolDef
	maxIter      int
	maxTokens    int
	OnToolCall   func(name string, args map[string]any)
	OnToolResult func(name string, result string)
	OnTextDelta  func(delta string)
}

const defaultMaxTokens = 6000

// New creates an Agent with the given LLM client, tool registry, and iteration limit.
func New(client llm.Client, registry *tools.Registry, maxIterations int) *Agent {
	a := &Agent{
		llm:       client,
		registry:  registry,
		maxIter:   maxIterations,
		maxTokens: defaultMaxTokens,
		history: []llm.Message{
			llm.SystemMessage(defaultSystemPrompt),
		},
	}

	// Use registry tools if available, otherwise fall back to builtins
	if registry != nil && registry.HasTools() {
		a.tools = registry.AllTools()
	} else {
		a.tools = a.builtinTools()
	}
	return a
}

// SetSystemPrompt overrides the default system prompt.
func (a *Agent) SetSystemPrompt(prompt string) {
	if prompt != "" {
		a.history[0] = llm.SystemMessage(prompt)
	}
}

// FilterTools restricts available tools to the given names.
func (a *Agent) FilterTools(names []string) {
	if len(names) == 0 {
		return
	}
	allowed := make(map[string]bool, len(names))
	for _, n := range names {
		allowed[n] = true
	}
	var filtered []llm.ToolDef
	for _, t := range a.tools {
		if allowed[t.Name] {
			filtered = append(filtered, t)
		}
	}
	a.tools = filtered
}

// SetMaxTokens sets the context window token budget for history compaction.
func (a *Agent) SetMaxTokens(maxTokens int) {
	if maxTokens > 0 {
		a.maxTokens = maxTokens
	}
}

// SetUtilityLLM sets an optional lightweight LLM client for housekeeping tasks
// like summarization and title generation.
func (a *Agent) SetUtilityLLM(client llm.Client) {
	a.utilityLLM = client
}

// SetClient swaps the main conversation LLM client (for mid-session model switching).
func (a *Agent) SetClient(client llm.Client) {
	a.llm = client
}

// compactHistory summarizes older messages when history exceeds the token budget.
func (a *Agent) compactHistory(ctx context.Context) error {
	total := estimateHistoryTokens(a.history)
	if total <= a.maxTokens {
		return nil
	}

	// Keep recent messages within 60% of budget
	recentBudget := a.maxTokens * 60 / 100
	splitIdx := findSplitPoint(a.history, recentBudget)
	if splitIdx >= len(a.history) {
		return nil // nothing to compact
	}

	// Old messages are indices 1 through splitIdx-1 (skip system prompt at 0)
	oldMessages := a.history[1:splitIdx]
	if len(oldMessages) == 0 {
		return nil
	}

	summarizer := a.llm
	if a.utilityLLM != nil {
		summarizer = a.utilityLLM
	}
	summary, err := summarizeMessages(ctx, summarizer, oldMessages)
	if err != nil {
		// Fallback: simple trim, keep last few messages
		a.trimHistory(10)
		return nil
	}

	// Rebuild history: system prompt + summary + recent messages
	summaryMsg := llm.SystemMessage("[Prior conversation summary]\n" + summary)
	newHistory := make([]llm.Message, 0, 2+len(a.history)-splitIdx)
	newHistory = append(newHistory, a.history[0]) // system prompt
	newHistory = append(newHistory, summaryMsg)
	newHistory = append(newHistory, a.history[splitIdx:]...)
	a.history = newHistory

	return nil
}

// Run sends a user message and executes the full ReAct loop.
// Returns the final assistant text response.
func (a *Agent) Run(ctx context.Context, userMessage string) (string, error) {
	a.compactHistory(ctx)
	a.history = append(a.history, llm.UserMessage(userMessage))

	for i := 0; i < a.maxIter; i++ {
		resp, err := a.llm.ChatCompletion(ctx, a.history, a.tools)
		if err != nil {
			return "", fmt.Errorf("llm call (iteration %d): %w", i+1, err)
		}

		a.history = append(a.history, resp.Message)

		// If no tool calls, the LLM is done — return the text response
		if len(resp.Message.ToolCalls) == 0 {
			return resp.Message.Content, nil
		}

		// Execute each tool call and append results
		for _, tc := range resp.Message.ToolCalls {
			if a.OnToolCall != nil {
				a.OnToolCall(tc.Name, tc.Args)
			}

			result := a.executeTool(ctx, tc)

			if a.OnToolResult != nil {
				a.OnToolResult(tc.Name, result)
			}

			a.history = append(a.history, llm.ToolResultMessage(tc.ID, result))
		}
		// Loop back — LLM will see the tool results and decide next action
	}

	return "", fmt.Errorf("agent reached max iterations (%d) without a final response", a.maxIter)
}

// RunStreaming is like Run but streams text output token-by-token via OnTextDelta.
func (a *Agent) RunStreaming(ctx context.Context, userMessage string) (string, error) {
	a.compactHistory(ctx)
	a.history = append(a.history, llm.UserMessage(userMessage))

	for i := 0; i < a.maxIter; i++ {
		resp, err := a.llm.ChatCompletionStream(ctx, a.history, a.tools, a.OnTextDelta)
		if err != nil {
			return "", fmt.Errorf("llm call (iteration %d): %w", i+1, err)
		}

		a.history = append(a.history, resp.Message)

		if len(resp.Message.ToolCalls) == 0 {
			return resp.Message.Content, nil
		}

		for _, tc := range resp.Message.ToolCalls {
			if a.OnToolCall != nil {
				a.OnToolCall(tc.Name, tc.Args)
			}

			result := a.executeTool(ctx, tc)

			if a.OnToolResult != nil {
				a.OnToolResult(tc.Name, result)
			}

			a.history = append(a.history, llm.ToolResultMessage(tc.ID, result))
		}
	}

	return "", fmt.Errorf("agent reached max iterations (%d) without a final response", a.maxIter)
}

// executeTool dispatches a tool call to the registry or builtin handler.
func (a *Agent) executeTool(ctx context.Context, tc llm.ToolCall) string {
	// Try registry first
	if a.registry != nil && a.registry.HasTools() {
		result, err := a.registry.CallTool(ctx, tc.Name, tc.Args)
		if err != nil {
			return fmt.Sprintf("error: %s", err)
		}
		return result
	}

	// Builtin fallback
	switch tc.Name {
	case "shell_exec":
		return a.toolShellExec(ctx, tc.Args)
	default:
		return fmt.Sprintf("error: unknown tool %q", tc.Name)
	}
}

// toolShellExec runs a shell command and returns stdout+stderr.
func (a *Agent) toolShellExec(ctx context.Context, args map[string]any) string {
	command, ok := args["command"].(string)
	if !ok {
		return "error: 'command' argument must be a string"
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", command)

	// Set working directory if provided
	if workdir, ok := args["workdir"].(string); ok && workdir != "" {
		cmd.Dir = workdir
	}

	output, err := cmd.CombinedOutput()
	result := string(output)
	if err != nil {
		result += "\nexit error: " + err.Error()
	}

	// Truncate very long outputs to keep context window manageable
	const maxLen = 4000
	if len(result) > maxLen {
		result = result[:maxLen] + "\n... (output truncated)"
	}

	return result
}

// builtinTools returns the tool definitions for Phase 1 hardcoded tools.
func (a *Agent) builtinTools() []llm.ToolDef {
	return []llm.ToolDef{
		{
			Name:        "shell_exec",
			Description: "Execute a shell command and return the combined stdout and stderr output. Use this to run system commands, check files, install packages, etc.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{
						"type":        "string",
						"description": "The shell command to execute",
					},
					"workdir": map[string]any{
						"type":        "string",
						"description": "Working directory for the command (optional)",
					},
				},
				"required": []string{"command"},
			},
		},
	}
}

// History returns the current conversation history (for debugging/display).
func (a *Agent) History() []llm.Message {
	return a.history
}

// HistoryJSON returns the conversation as formatted JSON (for debugging).
func (a *Agent) HistoryJSON() string {
	data, _ := json.MarshalIndent(a.history, "", "  ")
	return string(data)
}

// trimHistory keeps the conversation within reasonable bounds.
// Preserves the system message and last N messages.
func (a *Agent) trimHistory(keepLast int) {
	if len(a.history) <= keepLast+1 {
		return
	}
	system := a.history[0]
	recent := a.history[len(a.history)-keepLast:]
	a.history = append([]llm.Message{system}, recent...)
}

// SetHistory replaces the conversation history (used when resuming a session).
func (a *Agent) SetHistory(messages []llm.Message) {
	a.history = messages
}

// Reset clears conversation history (keeps system prompt).
func (a *Agent) Reset() {
	a.history = a.history[:1]
}

// String returns a summary of the agent state.
func (a *Agent) String() string {
	return fmt.Sprintf("Agent(tools=%d, history=%d messages, maxIter=%d)",
		len(a.tools), len(a.history), a.maxIter)
}

// FormatToolCall returns a human-readable string for a tool call.
func FormatToolCall(name string, args map[string]any) string {
	var parts []string
	for k, v := range args {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	return fmt.Sprintf("%s(%s)", name, strings.Join(parts, ", "))
}
