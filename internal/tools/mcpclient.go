package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/michaelbrown/forge/internal/llm"
)

// MCPConnection wraps an mcp-go stdio client for a single tool server.
type MCPConnection struct {
	name   string
	client *client.Client
	tools  []mcp.Tool
}

// NewMCPConnection launches an MCP server subprocess and initializes the connection.
func NewMCPConnection(name, binary string, env []string) (*MCPConnection, error) {
	c, err := client.NewStdioMCPClient(binary, env)
	if err != nil {
		return nil, fmt.Errorf("starting MCP server %s (%s): %w", name, binary, err)
	}

	ctx := context.Background()

	// Initialize the MCP protocol
	_, err = c.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ClientInfo: mcp.Implementation{
				Name:    "forge",
				Version: "0.1.0",
			},
		},
	})
	if err != nil {
		c.Close()
		return nil, fmt.Errorf("initializing MCP server %s: %w", name, err)
	}

	// Discover tools
	result, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		c.Close()
		return nil, fmt.Errorf("listing tools from %s: %w", name, err)
	}

	return &MCPConnection{
		name:   name,
		client: c,
		tools:  result.Tools,
	}, nil
}

// ToolDefs converts MCP tool schemas to llm.ToolDef for the LLM API.
func (mc *MCPConnection) ToolDefs() []llm.ToolDef {
	var defs []llm.ToolDef
	for _, t := range mc.tools {
		params := map[string]any{
			"type": t.InputSchema.Type,
		}
		if t.InputSchema.Properties != nil {
			params["properties"] = t.InputSchema.Properties
		}
		if len(t.InputSchema.Required) > 0 {
			params["required"] = t.InputSchema.Required
		}
		defs = append(defs, llm.ToolDef{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  params,
		})
	}
	return defs
}

// CallTool invokes a tool on this MCP server and returns the text result.
func (mc *MCPConnection) CallTool(ctx context.Context, name string, args map[string]any) (string, error) {
	result, err := mc.client.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      name,
			Arguments: args,
		},
	})
	if err != nil {
		return "", fmt.Errorf("calling tool %s on %s: %w", name, mc.name, err)
	}

	// Extract text content from the result
	var parts []string
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}

	text := strings.Join(parts, "\n")
	if result.IsError {
		return "error: " + text, nil
	}
	return text, nil
}

// ToolNames returns the names of all tools on this server.
func (mc *MCPConnection) ToolNames() []string {
	names := make([]string, len(mc.tools))
	for i, t := range mc.tools {
		names[i] = t.Name
	}
	return names
}

// Close shuts down the MCP server subprocess.
func (mc *MCPConnection) Close() {
	mc.client.Close()
}
