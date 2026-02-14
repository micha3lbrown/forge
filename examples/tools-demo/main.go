// Demo: Using Forge MCP tool servers from Go.
//
// Build the tool binaries first:
//   make build-tools
//
// Then run this demo:
//   go run ./examples/tools-demo
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/michaelbrown/forge/internal/tools"
)

func main() {
	ctx := context.Background()

	// Find binaries relative to the project root
	root := findProjectRoot()

	fmt.Println("=== Forge MCP Tools Demo ===")
	fmt.Println()

	// -------------------------------------------------------
	// 1. Create a registry and register tool servers
	// -------------------------------------------------------
	registry := tools.NewRegistry()
	defer registry.Close()

	servers := map[string]string{
		"shell-exec": filepath.Join(root, "bin", "forge-tool-shell-exec"),
		"file-ops":   filepath.Join(root, "bin", "forge-tool-file-ops"),
	}

	for name, bin := range servers {
		fmt.Printf("Starting MCP server: %s\n", name)
		err := registry.Register(name, tools.ToolServerConfig{
			Binary:  bin,
			Enabled: true,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ERROR: %v\n", err)
			fmt.Fprintf(os.Stderr, "  (run 'make build-tools' first)\n")
			os.Exit(1)
		}
	}

	// -------------------------------------------------------
	// 2. Discover all available tools
	// -------------------------------------------------------
	fmt.Println()
	fmt.Println("--- Discovered Tools ---")
	for _, td := range registry.AllTools() {
		fmt.Printf("  %-15s %s\n", td.Name, td.Description[:min(60, len(td.Description))])
	}

	// -------------------------------------------------------
	// 3. shell_exec: Run a command
	// -------------------------------------------------------
	fmt.Println()
	fmt.Println("--- shell_exec: Run 'uname -a' ---")
	result, err := registry.CallTool(ctx, "shell_exec", map[string]any{
		"command": "uname -a",
	})
	printResult(result, err)

	// -------------------------------------------------------
	// 4. file_write: Create a file
	// -------------------------------------------------------
	demoFile := filepath.Join(os.TempDir(), "forge-demo-example.py")
	content := "#!/usr/bin/env python3\nimport sys\n\ndef greet(name):\n    return f\"Hello, {name}!\"\n\nprint(greet(\"Forge\"))\nprint(f\"Python {sys.version}\")\n"

	fmt.Println("--- file_write: Create a Python script ---")
	result, err = registry.CallTool(ctx, "file_write", map[string]any{
		"path":    demoFile,
		"content": content,
	})
	printResult(result, err)

	// -------------------------------------------------------
	// 5. file_read: Read it back
	// -------------------------------------------------------
	fmt.Println("--- file_read: Read the full file ---")
	result, err = registry.CallTool(ctx, "file_read", map[string]any{
		"path": demoFile,
	})
	printResult(result, err)

	// -------------------------------------------------------
	// 6. file_read: Read just lines 5-7
	// -------------------------------------------------------
	fmt.Println("--- file_read: Lines 5-7 only ---")
	result, err = registry.CallTool(ctx, "file_read", map[string]any{
		"path":       demoFile,
		"start_line": float64(5),
		"end_line":   float64(7),
	})
	printResult(result, err)

	// -------------------------------------------------------
	// 7. file_patch: Change the greeting
	// -------------------------------------------------------
	fmt.Println("--- file_patch: Change the greeting ---")
	result, err = registry.CallTool(ctx, "file_patch", map[string]any{
		"path":    demoFile,
		"search":  `return f"Hello, {name}!"`,
		"replace": `return f"Patched by MCP, {name}!"`,
	})
	printResult(result, err)

	// Verify the patch
	fmt.Println("--- file_read: Verify the patch ---")
	result, err = registry.CallTool(ctx, "file_read", map[string]any{
		"path": demoFile,
	})
	printResult(result, err)

	// -------------------------------------------------------
	// 8. file_list: List temp directory with glob
	// -------------------------------------------------------
	fmt.Println("--- file_list: Glob for forge-demo* ---")
	result, err = registry.CallTool(ctx, "file_list", map[string]any{
		"path":    os.TempDir(),
		"pattern": "forge-demo*",
	})
	printResult(result, err)

	// -------------------------------------------------------
	// 9. shell_exec: Run the patched Python script
	// -------------------------------------------------------
	fmt.Println("--- shell_exec: Run the patched script ---")
	result, err = registry.CallTool(ctx, "shell_exec", map[string]any{
		"command": "python3 " + demoFile,
	})
	printResult(result, err)

	// Cleanup
	os.Remove(demoFile)
	fmt.Println("=== Demo complete ===")
}

func printResult(result string, err error) {
	if err != nil {
		fmt.Printf("  ERROR: %v\n\n", err)
		return
	}
	for _, line := range strings.Split(result, "\n") {
		fmt.Printf("  > %s\n", line)
	}
	fmt.Println()
}

func findProjectRoot() string {
	dir, _ := os.Getwd()
	for d := dir; d != "/"; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
	}
	return dir
}
