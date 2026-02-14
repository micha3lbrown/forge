package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/michaelbrown/forge/internal/tools"
)

// These integration tests require the tool server binaries to be built first.
// Run: make build-tools && go test ./internal/tools/ -v

func binPath(name string) string {
	// Walk up from the test's working directory to find the project root bin/
	wd, _ := os.Getwd()
	for d := wd; d != "/"; d = filepath.Dir(d) {
		candidate := filepath.Join(d, "bin", name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return filepath.Join("bin", name) // fallback
}

func skipIfNoBinary(t *testing.T, name string) string {
	t.Helper()
	path := binPath(name)
	if _, err := os.Stat(path); err != nil {
		t.Skipf("binary %s not found at %s (run make build-tools first)", name, path)
	}
	return path
}

// --- Registry tests ---

func TestRegistryEmpty(t *testing.T) {
	r := tools.NewRegistry()
	defer r.Close()

	if r.HasTools() {
		t.Fatal("empty registry should not have tools")
	}
	if got := r.AllTools(); len(got) != 0 {
		t.Fatalf("AllTools() = %d, want 0", len(got))
	}

	_, err := r.CallTool(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Fatal("CallTool on empty registry should return error")
	}
}

func TestRegistrySkipsDisabled(t *testing.T) {
	r := tools.NewRegistry()
	defer r.Close()

	err := r.Register("disabled-server", tools.ToolServerConfig{
		Binary:  "/nonexistent/binary",
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("Register disabled server should not error: %v", err)
	}
	if r.HasTools() {
		t.Fatal("disabled server should not register tools")
	}
}

func TestRegistryBadBinary(t *testing.T) {
	r := tools.NewRegistry()
	defer r.Close()

	err := r.Register("bad", tools.ToolServerConfig{
		Binary:  "/nonexistent/binary",
		Enabled: true,
	})
	if err == nil {
		t.Fatal("Register with bad binary should return error")
	}
}

// --- shell-exec integration tests ---

func TestShellExecMCP(t *testing.T) {
	bin := skipIfNoBinary(t, "forge-tool-shell-exec")

	r := tools.NewRegistry()
	defer r.Close()

	err := r.Register("shell-exec", tools.ToolServerConfig{
		Binary:  bin,
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("Register shell-exec: %v", err)
	}

	if !r.HasTools() {
		t.Fatal("registry should have tools after registering shell-exec")
	}

	// Verify tool discovery
	allTools := r.AllTools()
	found := false
	for _, td := range allTools {
		if td.Name == "shell_exec" {
			found = true
			if td.Description == "" {
				t.Error("shell_exec should have a description")
			}
		}
	}
	if !found {
		t.Fatalf("shell_exec not found in tools: %v", allTools)
	}

	// Call the tool
	ctx := context.Background()
	result, err := r.CallTool(ctx, "shell_exec", map[string]any{
		"command": "echo hello from mcp",
	})
	if err != nil {
		t.Fatalf("CallTool shell_exec: %v", err)
	}
	if !strings.Contains(result, "hello from mcp") {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestShellExecWorkdir(t *testing.T) {
	bin := skipIfNoBinary(t, "forge-tool-shell-exec")

	r := tools.NewRegistry()
	defer r.Close()

	if err := r.Register("shell-exec", tools.ToolServerConfig{Binary: bin, Enabled: true}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	result, err := r.CallTool(context.Background(), "shell_exec", map[string]any{
		"command": "pwd",
		"workdir": "/tmp",
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	// macOS resolves /tmp → /private/tmp
	if !strings.Contains(result, "tmp") {
		t.Errorf("expected /tmp in output, got: %q", result)
	}
}

// --- file-ops integration tests ---

func TestFileOpsMCP(t *testing.T) {
	bin := skipIfNoBinary(t, "forge-tool-file-ops")

	r := tools.NewRegistry()
	defer r.Close()

	if err := r.Register("file-ops", tools.ToolServerConfig{Binary: bin, Enabled: true}); err != nil {
		t.Fatalf("Register file-ops: %v", err)
	}

	// Verify all 4 tools are discovered
	allTools := r.AllTools()
	expected := map[string]bool{"file_read": false, "file_write": false, "file_patch": false, "file_list": false}
	for _, td := range allTools {
		if _, ok := expected[td.Name]; ok {
			expected[td.Name] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("tool %s not discovered", name)
		}
	}

	ctx := context.Background()
	tmpDir := t.TempDir()

	// file_write
	testFile := filepath.Join(tmpDir, "test.txt")
	result, err := r.CallTool(ctx, "file_write", map[string]any{
		"path":    testFile,
		"content": "line1\nline2\nline3\n",
	})
	if err != nil {
		t.Fatalf("file_write: %v", err)
	}
	if !strings.Contains(result, "wrote") {
		t.Errorf("file_write result: %q", result)
	}

	// file_read (full)
	result, err = r.CallTool(ctx, "file_read", map[string]any{
		"path": testFile,
	})
	if err != nil {
		t.Fatalf("file_read: %v", err)
	}
	if !strings.Contains(result, "line1") || !strings.Contains(result, "line3") {
		t.Errorf("file_read result: %q", result)
	}

	// file_read (line range)
	result, err = r.CallTool(ctx, "file_read", map[string]any{
		"path":       testFile,
		"start_line": float64(2), // JSON numbers come as float64
		"end_line":   float64(2),
	})
	if err != nil {
		t.Fatalf("file_read range: %v", err)
	}
	if result != "line2" {
		t.Errorf("file_read range = %q, want %q", result, "line2")
	}

	// file_patch
	result, err = r.CallTool(ctx, "file_patch", map[string]any{
		"path":    testFile,
		"search":  "line2",
		"replace": "REPLACED",
	})
	if err != nil {
		t.Fatalf("file_patch: %v", err)
	}
	if !strings.Contains(result, "patched") {
		t.Errorf("file_patch result: %q", result)
	}

	// Verify the patch
	data, _ := os.ReadFile(testFile)
	if !strings.Contains(string(data), "REPLACED") {
		t.Errorf("file not patched, contents: %q", string(data))
	}

	// file_list
	result, err = r.CallTool(ctx, "file_list", map[string]any{
		"path": tmpDir,
	})
	if err != nil {
		t.Fatalf("file_list: %v", err)
	}
	if !strings.Contains(result, "test.txt") {
		t.Errorf("file_list result: %q", result)
	}

	// file_list with glob
	result, err = r.CallTool(ctx, "file_list", map[string]any{
		"path":    tmpDir,
		"pattern": "*.txt",
	})
	if err != nil {
		t.Fatalf("file_list glob: %v", err)
	}
	if !strings.Contains(result, "test.txt") {
		t.Errorf("file_list glob result: %q", result)
	}
}

func TestFileOpsErrors(t *testing.T) {
	bin := skipIfNoBinary(t, "forge-tool-file-ops")

	r := tools.NewRegistry()
	defer r.Close()

	if err := r.Register("file-ops", tools.ToolServerConfig{Binary: bin, Enabled: true}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	ctx := context.Background()

	// Read nonexistent file
	result, err := r.CallTool(ctx, "file_read", map[string]any{
		"path": "/nonexistent/file.txt",
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !strings.Contains(result, "error") {
		t.Errorf("expected error for nonexistent file, got: %q", result)
	}

	// Patch with missing search string
	tmpFile := filepath.Join(t.TempDir(), "test.txt")
	os.WriteFile(tmpFile, []byte("hello world"), 0o644)
	result, err = r.CallTool(ctx, "file_patch", map[string]any{
		"path":    tmpFile,
		"search":  "not found",
		"replace": "x",
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !strings.Contains(result, "error") {
		t.Errorf("expected error for missing search string, got: %q", result)
	}
}

// --- Multi-server registry test ---

func TestRegistryMultipleServers(t *testing.T) {
	shellBin := skipIfNoBinary(t, "forge-tool-shell-exec")
	fileBin := skipIfNoBinary(t, "forge-tool-file-ops")

	r := tools.NewRegistry()
	defer r.Close()

	if err := r.Register("shell-exec", tools.ToolServerConfig{Binary: shellBin, Enabled: true}); err != nil {
		t.Fatalf("Register shell-exec: %v", err)
	}
	if err := r.Register("file-ops", tools.ToolServerConfig{Binary: fileBin, Enabled: true}); err != nil {
		t.Fatalf("Register file-ops: %v", err)
	}

	// Should have tools from both servers
	allTools := r.AllTools()
	if len(allTools) < 5 { // 1 from shell-exec + 4 from file-ops
		t.Fatalf("expected at least 5 tools, got %d", len(allTools))
	}

	ctx := context.Background()

	// Call a tool from each server to verify routing
	result, err := r.CallTool(ctx, "shell_exec", map[string]any{"command": "echo routed"})
	if err != nil {
		t.Fatalf("shell_exec: %v", err)
	}
	if !strings.Contains(result, "routed") {
		t.Errorf("shell_exec result: %q", result)
	}

	tmpFile := filepath.Join(t.TempDir(), "route-test.txt")
	result, err = r.CallTool(ctx, "file_write", map[string]any{
		"path":    tmpFile,
		"content": "routing works",
	})
	if err != nil {
		t.Fatalf("file_write: %v", err)
	}
	if !strings.Contains(result, "wrote") {
		t.Errorf("file_write result: %q", result)
	}
}
