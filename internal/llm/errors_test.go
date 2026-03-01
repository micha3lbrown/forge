package llm

import (
	"errors"
	"fmt"
	"testing"
)

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want ErrorKind
	}{
		{"429 status", fmt.Errorf("status code: 429"), ErrKindRateLimit},
		{"rate limit text", fmt.Errorf("rate limit exceeded"), ErrKindRateLimit},
		{"401 status", fmt.Errorf("status code: 401"), ErrKindAuth},
		{"403 status", fmt.Errorf("status code: 403"), ErrKindAuth},
		{"unauthorized text", fmt.Errorf("unauthorized"), ErrKindAuth},
		{"404 status", fmt.Errorf("status code: 404"), ErrKindModelNotFound},
		{"model not found", fmt.Errorf("model 'foo' not found"), ErrKindModelNotFound},
		{"connection refused", fmt.Errorf("dial tcp: connection refused"), ErrKindConnRefused},
		{"ECONNREFUSED", fmt.Errorf("connect ECONNREFUSED"), ErrKindConnRefused},
		{"timeout", fmt.Errorf("context deadline exceeded"), ErrKindTimeout},
		{"timeout text", fmt.Errorf("request timed out"), ErrKindTimeout},
		{"500 status", fmt.Errorf("status code: 500"), ErrKindServerError},
		{"502 status", fmt.Errorf("status code: 502"), ErrKindServerError},
		{"503 status", fmt.Errorf("status code: 503"), ErrKindServerError},
		{"internal server error", fmt.Errorf("internal server error"), ErrKindServerError},
		{"unknown error", fmt.Errorf("something unexpected"), ErrKindUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyError(tt.err)
			if got != tt.want {
				t.Errorf("classifyError(%q) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestLLMError_IsRetryable(t *testing.T) {
	tests := []struct {
		kind ErrorKind
		want bool
	}{
		{ErrKindRateLimit, true},
		{ErrKindConnRefused, true},
		{ErrKindTimeout, true},
		{ErrKindServerError, true},
		{ErrKindModelNotFound, false},
		{ErrKindAuth, false},
		{ErrKindUnknown, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			e := &LLMError{Kind: tt.kind}
			if got := e.IsRetryable(); got != tt.want {
				t.Errorf("LLMError{Kind: %v}.IsRetryable() = %v, want %v", tt.kind, got, tt.want)
			}
		})
	}
}

func TestLLMError_IsFallbackEligible(t *testing.T) {
	tests := []struct {
		kind ErrorKind
		want bool
	}{
		{ErrKindConnRefused, true},
		{ErrKindTimeout, true},
		{ErrKindModelNotFound, true},
		{ErrKindServerError, true},
		{ErrKindRateLimit, false},
		{ErrKindAuth, false},
		{ErrKindUnknown, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			e := &LLMError{Kind: tt.kind}
			if got := e.IsFallbackEligible(); got != tt.want {
				t.Errorf("LLMError{Kind: %v}.IsFallbackEligible() = %v, want %v", tt.kind, got, tt.want)
			}
		})
	}
}

func TestIsFallbackEligible_PublicHelper(t *testing.T) {
	// Wrapped LLMError should be detected
	inner := &LLMError{Kind: ErrKindConnRefused, Err: fmt.Errorf("connection refused")}
	wrapped := fmt.Errorf("chat completion: %w", inner)
	if !IsFallbackEligible(wrapped) {
		t.Error("IsFallbackEligible should return true for wrapped conn refused error")
	}

	// Plain error should return false
	plain := fmt.Errorf("something else")
	if IsFallbackEligible(plain) {
		t.Error("IsFallbackEligible should return false for plain error")
	}

	// Auth error should return false
	authErr := &LLMError{Kind: ErrKindAuth, Err: fmt.Errorf("unauthorized")}
	if IsFallbackEligible(authErr) {
		t.Error("IsFallbackEligible should return false for auth error")
	}
}

func TestLLMError_ErrorMessage(t *testing.T) {
	inner := fmt.Errorf("connection refused")
	e := &LLMError{Kind: ErrKindConnRefused, Err: inner}
	if e.Error() != "connection refused" {
		t.Errorf("Error() = %q, want %q", e.Error(), "connection refused")
	}
}

func TestLLMError_Unwrap(t *testing.T) {
	inner := fmt.Errorf("original error")
	e := &LLMError{Kind: ErrKindTimeout, Err: inner}
	if !errors.Is(e, inner) {
		t.Error("Unwrap should expose inner error")
	}
}
