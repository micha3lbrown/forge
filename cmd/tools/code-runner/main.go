package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/michaelbrown/forge/internal/sandbox"
)

var languageConfig = map[string]struct {
	image   string
	command func(string) []string
}{
	"python": {
		image:   "python:3.12-slim",
		command: func(_ string) []string { return []string{"python", "/workspace/code"} },
	},
	"javascript": {
		image:   "node:22-slim",
		command: func(_ string) []string { return []string{"node", "/workspace/code"} },
	},
	"go": {
		image:   "golang:1.23-alpine",
		command: func(_ string) []string { return []string{"go", "run", "/workspace/code"} },
	},
	"ruby": {
		image:   "ruby:3.3-slim",
		command: func(_ string) []string { return []string{"ruby", "/workspace/code"} },
	},
}

func main() {
	s := server.NewMCPServer("forge-code-runner", "0.1.0")

	// Build language list for description
	var langs []string
	for lang := range languageConfig {
		langs = append(langs, lang)
	}

	s.AddTool(mcp.Tool{
		Name:        "code_run",
		Description: fmt.Sprintf("Execute code in a Docker sandbox. Supported languages: %s.", strings.Join(langs, ", ")),
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"language": map[string]any{
					"type":        "string",
					"description": "Programming language (python, javascript, go, ruby)",
				},
				"code": map[string]any{
					"type":        "string",
					"description": "Source code to execute",
				},
				"stdin": map[string]any{
					"type":        "string",
					"description": "Standard input to provide to the program (optional)",
				},
			},
			Required: []string{"language", "code"},
		},
	}, handleCodeRun)

	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("server error: %v\n", err)
	}
}

func handleCodeRun(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := request.Params.Arguments.(map[string]any)
	if args == nil {
		return errResult("error: invalid arguments"), nil
	}

	language, _ := args["language"].(string)
	code, _ := args["code"].(string)
	stdin, _ := args["stdin"].(string)

	if language == "" || code == "" {
		return errResult("error: 'language' and 'code' are required"), nil
	}

	langCfg, ok := languageConfig[language]
	if !ok {
		return errResult(fmt.Sprintf("error: unsupported language %q", language)), nil
	}

	policy := sandbox.DefaultPolicy()
	sb := sandbox.NewDockerSandbox(policy)

	result, err := sb.Exec(ctx, sandbox.ExecOpts{
		Image:   langCfg.image,
		Command: langCfg.command(language),
		Code:    code,
		Stdin:   stdin,
	})
	if err != nil {
		return errResult(fmt.Sprintf("error: %v", err)), nil
	}

	var output strings.Builder
	if result.Stdout != "" {
		output.WriteString(result.Stdout)
	}
	if result.Stderr != "" {
		if output.Len() > 0 {
			output.WriteString("\n")
		}
		output.WriteString("STDERR:\n" + result.Stderr)
	}
	if result.ExitCode != 0 {
		output.WriteString(fmt.Sprintf("\nexit code: %d", result.ExitCode))
	}

	text := output.String()
	if len(text) > 4000 {
		text = text[:4000] + "\n... (output truncated)"
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.TextContent{Type: "text", Text: text}},
		IsError: result.ExitCode != 0,
	}, nil
}

func errResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.TextContent{Type: "text", Text: text}},
		IsError: true,
	}
}
