package sandbox

import "context"

// ExecOpts describes a code execution request.
type ExecOpts struct {
	Image   string // Docker image (e.g. "python:3.12-slim")
	Command []string
	Code    string // Source code to execute
	Stdin   string
	Workdir string
}

// ExecResult is the output of a sandboxed execution.
type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Sandbox runs code in an isolated environment.
type Sandbox interface {
	Exec(ctx context.Context, opts ExecOpts) (*ExecResult, error)
}
