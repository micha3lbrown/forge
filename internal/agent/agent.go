package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/michaelbrown/forge/internal/llm"
)

const defaultSystemPrompt = `You are Forge, a helpful AI assistant with access to tools.
When you need information from the system (files, commands, etc.), use the available tools.
Always explain what you're doing and why. After using a tool, interpret the results for the user.`

// Agent manages a conversation and executes the ReAct loop.
type Agent struct {
	llm       llm.Client
	history   []llm.Message
	tools     []llm.ToolDef
	maxIter   int
	OnToolCall func(name string, args map[string]any)
	OnToolResult func(name string, result string)
}

// New creates an Agent with the given LLM client and iteration limit.
func New(client llm.Client, maxIterations int) *Agent {
	a := &Agent{
		llm:     client,
		maxIter: maxIterations,
		history: []llm.Message{
			llm.SystemMessage(defaultSystemPrompt),
		},
	}
	a.tools = a.builtinTools()
	return a
}

// Run sends a user message and executes the full ReAct loop.
// Returns the final assistant text response.
func (a *Agent) Run(ctx context.Context, userMessage string) (string, error) {
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

// executeTool dispatches a tool call to the appropriate handler.
func (a *Agent) executeTool(ctx context.Context, tc llm.ToolCall) string {
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
