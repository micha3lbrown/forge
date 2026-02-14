package main

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	s := server.NewMCPServer("forge-shell-exec", "0.1.0")

	s.AddTool(mcp.Tool{
		Name:        "shell_exec",
		Description: "Execute a shell command and return the combined stdout and stderr output. Use this to run system commands, check files, install packages, etc.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "The shell command to execute",
				},
				"workdir": map[string]any{
					"type":        "string",
					"description": "Working directory for the command (optional)",
				},
			},
			Required: []string{"command"},
		},
	}, handleShellExec)

	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("server error: %v\n", err)
	}
}

func handleShellExec(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := request.Params.Arguments.(map[string]any)
	if args == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "error: invalid arguments"}},
			IsError: true,
		}, nil
	}

	command, ok := args["command"].(string)
	if !ok {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "error: 'command' argument must be a string"}},
			IsError: true,
		}, nil
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", command)

	if workdir, ok := args["workdir"].(string); ok && workdir != "" {
		cmd.Dir = workdir
	}

	output, err := cmd.CombinedOutput()
	result := string(output)
	if err != nil {
		result += "\nexit error: " + err.Error()
	}

	const maxLen = 4000
	if len(result) > maxLen {
		result = result[:maxLen] + "\n... (output truncated)"
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.TextContent{Type: "text", Text: result}},
	}, nil
}
