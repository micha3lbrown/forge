package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	s := server.NewMCPServer("forge-file-ops", "0.1.0")

	s.AddTool(mcp.Tool{
		Name:        "file_read",
		Description: "Read the contents of a file. Optionally specify a line range.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file to read",
				},
				"start_line": map[string]any{
					"type":        "integer",
					"description": "First line to read (1-based, optional)",
				},
				"end_line": map[string]any{
					"type":        "integer",
					"description": "Last line to read (1-based, inclusive, optional)",
				},
			},
			Required: []string{"path"},
		},
	}, handleFileRead)

	s.AddTool(mcp.Tool{
		Name:        "file_write",
		Description: "Write content to a file, creating it if it doesn't exist. Overwrites existing content.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file to write",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Content to write to the file",
				},
			},
			Required: []string{"path", "content"},
		},
	}, handleFileWrite)

	s.AddTool(mcp.Tool{
		Name:        "file_patch",
		Description: "Replace the first occurrence of a search string with a replacement string in a file.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file to patch",
				},
				"search": map[string]any{
					"type":        "string",
					"description": "The exact text to search for",
				},
				"replace": map[string]any{
					"type":        "string",
					"description": "The text to replace it with",
				},
			},
			Required: []string{"path", "search", "replace"},
		},
	}, handleFilePatch)

	s.AddTool(mcp.Tool{
		Name:        "file_list",
		Description: "List files in a directory, optionally filtered by a glob pattern.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Directory path to list",
				},
				"pattern": map[string]any{
					"type":        "string",
					"description": "Glob pattern to filter files (e.g. '*.go', '**/*.js')",
				},
			},
			Required: []string{"path"},
		},
	}, handleFileList)

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

func handleFileRead(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	path, _ := args["path"].(string)
	if path == "" {
		return errResult("error: 'path' is required"), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return errResult(fmt.Sprintf("error reading file: %v", err)), nil
	}

	content := string(data)

	// Handle optional line range
	startLine, hasStart := toInt(args["start_line"])
	endLine, hasEnd := toInt(args["end_line"])

	if hasStart || hasEnd {
		lines := strings.Split(content, "\n")
		if !hasStart {
			startLine = 1
		}
		if !hasEnd {
			endLine = len(lines)
		}
		// Clamp to valid range
		if startLine < 1 {
			startLine = 1
		}
		if endLine > len(lines) {
			endLine = len(lines)
		}
		if startLine > endLine {
			return errResult("error: start_line > end_line"), nil
		}
		content = strings.Join(lines[startLine-1:endLine], "\n")
	}

	return textResult(content), nil
}

func handleFileWrite(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	path, _ := args["path"].(string)
	content, _ := args["content"].(string)
	if path == "" {
		return errResult("error: 'path' is required"), nil
	}

	// Create parent directories if needed
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return errResult(fmt.Sprintf("error creating directories: %v", err)), nil
		}
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return errResult(fmt.Sprintf("error writing file: %v", err)), nil
	}

	return textResult(fmt.Sprintf("wrote %d bytes to %s", len(content), path)), nil
}

func handleFilePatch(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	path, _ := args["path"].(string)
	search, _ := args["search"].(string)
	replace, _ := args["replace"].(string)
	if path == "" || search == "" {
		return errResult("error: 'path' and 'search' are required"), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return errResult(fmt.Sprintf("error reading file: %v", err)), nil
	}

	content := string(data)
	if !strings.Contains(content, search) {
		return errResult("error: search string not found in file"), nil
	}

	newContent := strings.Replace(content, search, replace, 1)
	if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {
		return errResult(fmt.Sprintf("error writing file: %v", err)), nil
	}

	return textResult(fmt.Sprintf("patched %s", path)), nil
}

func handleFileList(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := getArgs(request)
	path, _ := args["path"].(string)
	pattern, _ := args["pattern"].(string)
	if path == "" {
		path = "."
	}

	if pattern != "" {
		matches, err := filepath.Glob(filepath.Join(path, pattern))
		if err != nil {
			return errResult(fmt.Sprintf("error: %v", err)), nil
		}
		return textResult(strings.Join(matches, "\n")), nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return errResult(fmt.Sprintf("error listing directory: %v", err)), nil
	}

	var lines []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		lines = append(lines, name)
	}

	return textResult(strings.Join(lines, "\n")), nil
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case string:
		i, err := strconv.Atoi(n)
		return i, err == nil
	}
	return 0, false
}
