package sandbox

import "time"

// Policy defines resource limits for sandbox execution.
type Policy struct {
	MaxMemory  string        // Docker memory limit (e.g. "256m")
	MaxTimeout time.Duration // Maximum execution time
	Network    bool          // Whether network access is allowed
	Images     []string      // Allowed Docker images
}

// DefaultPolicy returns safe defaults for code execution.
func DefaultPolicy() Policy {
	return Policy{
		MaxMemory:  "256m",
		MaxTimeout: 30 * time.Second,
		Network:    false,
		Images: []string{
			"python:3.12-slim",
			"node:22-slim",
			"golang:1.23-alpine",
			"ruby:3.3-slim",
		},
	}
}

// IsImageAllowed checks if an image is on the allowlist.
func (p Policy) IsImageAllowed(image string) bool {
	for _, allowed := range p.Images {
		if allowed == image {
			return true
		}
	}
	return false
}
