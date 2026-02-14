package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/michaelbrown/forge/internal/llm"
)

// Registry manages multiple MCP tool server connections.
type Registry struct {
	connections map[string]*MCPConnection // server name → connection
	toolIndex   map[string]string         // tool name → server name
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{
		connections: make(map[string]*MCPConnection),
		toolIndex:   make(map[string]string),
	}
}

// Register launches an MCP tool server and adds its tools to the registry.
func (r *Registry) Register(name string, cfg ToolServerConfig) error {
	if !cfg.Enabled {
		return nil
	}

	// Build environment variables
	var env []string
	env = append(env, os.Environ()...)
	for k, v := range cfg.Env {
		// Expand environment variable references like ${VAR}
		if strings.HasPrefix(v, "${") && strings.HasSuffix(v, "}") {
			envVar := v[2 : len(v)-1]
			v = os.Getenv(envVar)
		}
		env = append(env, k+"="+v)
	}

	conn, err := NewMCPConnection(name, cfg.Binary, env)
	if err != nil {
		return err
	}

	r.connections[name] = conn
	for _, toolName := range conn.ToolNames() {
		r.toolIndex[toolName] = name
	}

	return nil
}

// AllTools returns tool definitions from all registered servers.
func (r *Registry) AllTools() []llm.ToolDef {
	var all []llm.ToolDef
	for _, conn := range r.connections {
		all = append(all, conn.ToolDefs()...)
	}
	return all
}

// CallTool routes a tool call to the appropriate MCP server.
func (r *Registry) CallTool(ctx context.Context, name string, args map[string]any) (string, error) {
	serverName, ok := r.toolIndex[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	conn := r.connections[serverName]
	return conn.CallTool(ctx, name, args)
}

// HasTools returns true if any tools are registered.
func (r *Registry) HasTools() bool {
	return len(r.toolIndex) > 0
}

// Close shuts down all MCP server connections.
func (r *Registry) Close() {
	for _, conn := range r.connections {
		conn.Close()
	}
}
