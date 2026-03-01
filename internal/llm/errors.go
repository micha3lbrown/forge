package llm

import (
	"errors"
	"strings"
)

// ErrorKind categorizes LLM errors for retry and fallback decisions.
type ErrorKind string

const (
	ErrKindRateLimit    ErrorKind = "rate_limit"
	ErrKindConnRefused  ErrorKind = "conn_refused"
	ErrKindTimeout      ErrorKind = "timeout"
	ErrKindModelNotFound ErrorKind = "model_not_found"
	ErrKindAuth         ErrorKind = "auth"
	ErrKindServerError  ErrorKind = "server_error"
	ErrKindUnknown      ErrorKind = "unknown"
)

// LLMError wraps an error with classification metadata.
type LLMError struct {
	Kind ErrorKind
	Err  error
}

func (e *LLMError) Error() string {
	return e.Err.Error()
}

func (e *LLMError) Unwrap() error {
	return e.Err
}

// IsRetryable returns true if the error is worth retrying.
func (e *LLMError) IsRetryable() bool {
	switch e.Kind {
	case ErrKindRateLimit, ErrKindConnRefused, ErrKindTimeout, ErrKindServerError:
		return true
	default:
		return false
	}
}

// IsFallbackEligible returns true if the user should be offered a provider switch.
func (e *LLMError) IsFallbackEligible() bool {
	switch e.Kind {
	case ErrKindConnRefused, ErrKindTimeout, ErrKindModelNotFound, ErrKindServerError:
		return true
	default:
		return false
	}
}

// classifyError inspects an error string to determine its kind.
func classifyError(err error) ErrorKind {
	msg := strings.ToLower(err.Error())

	// Rate limit
	if strings.Contains(msg, "429") || strings.Contains(msg, "rate limit") {
		return ErrKindRateLimit
	}

	// Auth
	if strings.Contains(msg, "401") || strings.Contains(msg, "403") || strings.Contains(msg, "unauthorized") || strings.Contains(msg, "forbidden") {
		return ErrKindAuth
	}

	// Model not found
	if strings.Contains(msg, "404") || strings.Contains(msg, "not found") {
		return ErrKindModelNotFound
	}

	// Connection refused
	if strings.Contains(msg, "connection refused") || strings.Contains(msg, "econnrefused") {
		return ErrKindConnRefused
	}

	// Timeout
	if strings.Contains(msg, "deadline exceeded") || strings.Contains(msg, "timed out") || strings.Contains(msg, "timeout") {
		return ErrKindTimeout
	}

	// Server error (5xx)
	if strings.Contains(msg, "500") || strings.Contains(msg, "502") || strings.Contains(msg, "503") || strings.Contains(msg, "504") || strings.Contains(msg, "internal server error") {
		return ErrKindServerError
	}

	return ErrKindUnknown
}

// NewLLMError creates a classified LLMError from a raw error.
func NewLLMError(err error) *LLMError {
	return &LLMError{
		Kind: classifyError(err),
		Err:  err,
	}
}

// IsFallbackEligible checks if an error (possibly wrapped) is fallback-eligible.
func IsFallbackEligible(err error) bool {
	var llmErr *LLMError
	if errors.As(err, &llmErr) {
		return llmErr.IsFallbackEligible()
	}
	return false
}
