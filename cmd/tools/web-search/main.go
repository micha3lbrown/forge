package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

func main() {
	s := server.NewMCPServer("forge-web-search", "0.1.0")

	s.AddTool(mcp.Tool{
		Name:        "web_search",
		Description: "Search the web using Tavily API. Returns relevant search results with snippets.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "The search query",
				},
			},
			Required: []string{"query"},
		},
	}, handleWebSearch)

	s.AddTool(mcp.Tool{
		Name:        "web_fetch",
		Description: "Fetch the text content of a URL via HTTP GET.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "The URL to fetch",
				},
			},
			Required: []string{"url"},
		},
	}, handleWebFetch)

	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("server error: %v\n", err)
	}
}

func getArgs(request mcp.CallToolRequest) map[string]any {
	args, _ := request.Params.Arguments.(map[string]any)
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

func handleWebSearch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	query, _ := args["query"].(string)
	if query == "" {
		return errResult("error: 'query' is required"), nil
	}

	apiKey := os.Getenv("TAVILY_API_KEY")
	if apiKey == "" {
		return errResult("error: TAVILY_API_KEY not set"), nil
	}

	body := map[string]any{
		"query":          query,
		"max_results":    5,
		"include_answer": true,
	}
	bodyJSON, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.tavily.com/search", strings.NewReader(string(bodyJSON)))
	if err != nil {
		return errResult(fmt.Sprintf("error: %v", err)), nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return errResult(fmt.Sprintf("error: %v", err)), nil
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return errResult(fmt.Sprintf("error reading response: %v", err)), nil
	}

	if resp.StatusCode != http.StatusOK {
		return errResult(fmt.Sprintf("error: Tavily API returned %d: %s", resp.StatusCode, string(respBody))), nil
	}

	var result struct {
		Answer  string `json:"answer"`
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return errResult(fmt.Sprintf("error parsing response: %v", err)), nil
	}

	var sb strings.Builder
	if result.Answer != "" {
		sb.WriteString("Answer: " + result.Answer + "\n\n")
	}
	for i, r := range result.Results {
		sb.WriteString(fmt.Sprintf("%d. %s\n   %s\n   %s\n\n", i+1, r.Title, r.URL, r.Content))
	}

	return textResult(sb.String()), nil
}

func handleWebFetch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	url, _ := args["url"].(string)
	if url == "" {
		return errResult("error: 'url' is required"), nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return errResult(fmt.Sprintf("error: %v", err)), nil
	}
	req.Header.Set("User-Agent", "Forge/0.1")

	resp, err := httpClient.Do(req)
	if err != nil {
		return errResult(fmt.Sprintf("error: %v", err)), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 50_000))
	if err != nil {
		return errResult(fmt.Sprintf("error reading body: %v", err)), nil
	}

	text := string(body)
	if len(text) > 4000 {
		text = text[:4000] + "\n... (truncated)"
	}

	return textResult(text), nil
}
