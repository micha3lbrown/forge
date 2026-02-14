package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DockerSandbox runs code in Docker containers.
type DockerSandbox struct {
	Policy Policy
}

// NewDockerSandbox creates a sandbox with the given policy.
func NewDockerSandbox(policy Policy) *DockerSandbox {
	return &DockerSandbox{Policy: policy}
}

func (d *DockerSandbox) Exec(ctx context.Context, opts ExecOpts) (*ExecResult, error) {
	if !d.Policy.IsImageAllowed(opts.Image) {
		return nil, fmt.Errorf("image %q not in allowlist", opts.Image)
	}

	// Create a temp dir for the code file
	tmpDir, err := os.MkdirTemp("", "forge-sandbox-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write code to a file
	codePath := filepath.Join(tmpDir, "code")
	if err := os.WriteFile(codePath, []byte(opts.Code), 0o644); err != nil {
		return nil, fmt.Errorf("writing code file: %w", err)
	}

	// Write stdin if provided
	var stdinPath string
	if opts.Stdin != "" {
		stdinPath = filepath.Join(tmpDir, "stdin")
		if err := os.WriteFile(stdinPath, []byte(opts.Stdin), 0o644); err != nil {
			return nil, fmt.Errorf("writing stdin file: %w", err)
		}
	}

	// Build docker command
	timeout := d.Policy.MaxTimeout

	args := []string{
		"run", "--rm",
		"--memory", d.Policy.MaxMemory,
		"--stop-timeout", fmt.Sprintf("%d", int(timeout.Seconds())),
		"-v", tmpDir + ":/workspace:ro",
		"-w", "/workspace",
	}

	if !d.Policy.Network {
		args = append(args, "--network=none")
	}

	args = append(args, opts.Image)
	args = append(args, opts.Command...)

	cmd := exec.CommandContext(ctx, "docker", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if opts.Stdin != "" {
		cmd.Stdin = strings.NewReader(opts.Stdin)
	}

	err = cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("running docker: %w", err)
		}
	}

	return &ExecResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}, nil
}
