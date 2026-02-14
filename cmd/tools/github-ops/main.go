package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	s := server.NewMCPServer("forge-github-ops", "0.1.0")

	s.AddTool(mcp.Tool{
		Name:        "github_list_prs",
		Description: "List pull requests for a GitHub repository.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"repo": map[string]any{
					"type":        "string",
					"description": "Repository in owner/repo format (optional, uses current repo if omitted)",
				},
				"state": map[string]any{
					"type":        "string",
					"description": "Filter by state: open, closed, merged, all (default: open)",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum number of PRs to return (default: 10)",
				},
			},
		},
	}, handleListPRs)

	s.AddTool(mcp.Tool{
		Name:        "github_list_issues",
		Description: "List issues for a GitHub repository.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"repo": map[string]any{
					"type":        "string",
					"description": "Repository in owner/repo format (optional, uses current repo if omitted)",
				},
				"state": map[string]any{
					"type":        "string",
					"description": "Filter by state: open, closed, all (default: open)",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum number of issues to return (default: 10)",
				},
			},
		},
	}, handleListIssues)

	s.AddTool(mcp.Tool{
		Name:        "github_view_pr",
		Description: "View details of a specific pull request.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"repo": map[string]any{
					"type":        "string",
					"description": "Repository in owner/repo format (optional)",
				},
				"number": map[string]any{
					"type":        "integer",
					"description": "PR number to view",
				},
			},
			Required: []string{"number"},
		},
	}, handleViewPR)

	s.AddTool(mcp.Tool{
		Name:        "github_repo_info",
		Description: "Get information about a GitHub repository.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"repo": map[string]any{
					"type":        "string",
					"description": "Repository in owner/repo format (optional, uses current repo if omitted)",
				},
			},
		},
	}, handleRepoInfo)

	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("server error: %v\n", err)
	}
}

func getArgs(request mcp.CallToolRequest) map[string]any {
	args, _ := request.Params.Arguments.(map[string]any)
	if args == nil {
		args = make(map[string]any)
	}
	return args
}

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.TextContent{Type: "text", Text: text}},
	}
}

func errResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.TextContent{Type: "text", Text: text}},
		IsError: true,
	}
}

func runGH(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s\n%s", err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

func repoFlag(args map[string]any) []string {
	repo, _ := args["repo"].(string)
	if repo != "" {
		return []string{"-R", repo}
	}
	return nil
}

func handleListPRs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	state, _ := args["state"].(string)
	if state == "" {
		state = "open"
	}
	limit := "10"
	if l, ok := args["limit"].(float64); ok {
		limit = fmt.Sprintf("%d", int(l))
	}

	ghArgs := []string{"pr", "list", "--state", state, "--limit", limit}
	ghArgs = append(ghArgs, repoFlag(args)...)

	out, err := runGH(ctx, ghArgs...)
	if err != nil {
		return errResult(fmt.Sprintf("error: %v", err)), nil
	}
	if out == "" {
		return textResult("No pull requests found."), nil
	}
	return textResult(out), nil
}

func handleListIssues(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	state, _ := args["state"].(string)
	if state == "" {
		state = "open"
	}
	limit := "10"
	if l, ok := args["limit"].(float64); ok {
		limit = fmt.Sprintf("%d", int(l))
	}

	ghArgs := []string{"issue", "list", "--state", state, "--limit", limit}
	ghArgs = append(ghArgs, repoFlag(args)...)

	out, err := runGH(ctx, ghArgs...)
	if err != nil {
		return errResult(fmt.Sprintf("error: %v", err)), nil
	}
	if out == "" {
		return textResult("No issues found."), nil
	}
	return textResult(out), nil
}

func handleViewPR(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	number, ok := args["number"].(float64)
	if !ok {
		return errResult("error: 'number' is required"), nil
	}

	ghArgs := []string{"pr", "view", fmt.Sprintf("%d", int(number))}
	ghArgs = append(ghArgs, repoFlag(args)...)

	out, err := runGH(ctx, ghArgs...)
	if err != nil {
		return errResult(fmt.Sprintf("error: %v", err)), nil
	}
	return textResult(out), nil
}

func handleRepoInfo(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	ghArgs := []string{"repo", "view"}
	ghArgs = append(ghArgs, repoFlag(args)...)

	out, err := runGH(ctx, ghArgs...)
	if err != nil {
		return errResult(fmt.Sprintf("error: %v", err)), nil
	}
	return textResult(out), nil
}
