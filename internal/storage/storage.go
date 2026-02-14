package storage

import (
	"context"
	"time"

	"github.com/michaelbrown/forge/internal/llm"
)

// SessionStatus represents the lifecycle state of a session.
type SessionStatus string

const (
	StatusActive    SessionStatus = "active"
	StatusRunning   SessionStatus = "running"
	StatusCompleted SessionStatus = "completed"
	StatusFailed    SessionStatus = "failed"
)

// Session is the metadata for a saved conversation.
type Session struct {
	ID        string        `json:"id"`
	Title     string        `json:"title"`
	Status    SessionStatus `json:"status"`
	Provider  string        `json:"provider"`
	Model     string        `json:"model"`
	Profile   string        `json:"profile"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// SessionListOptions controls filtering and pagination for ListSessions.
type SessionListOptions struct {
	Status SessionStatus
	Limit  int
	Offset int
}

// Store is the persistence interface for sessions and messages.
type Store interface {
	// CreateSession inserts a new session. The ID field must be set by the caller.
	CreateSession(ctx context.Context, s *Session) error

	// GetSession returns a session by ID or ID prefix.
	GetSession(ctx context.Context, id string) (*Session, error)

	// ListSessions returns sessions ordered by updated_at descending.
	ListSessions(ctx context.Context, opts SessionListOptions) ([]Session, error)

	// UpdateSession updates mutable fields (title, status, updated_at).
	UpdateSession(ctx context.Context, s *Session) error

	// DeleteSession removes a session and its messages.
	DeleteSession(ctx context.Context, id string) error

	// SaveMessages overwrites the full message history for a session.
	SaveMessages(ctx context.Context, sessionID string, messages []llm.Message) error

	// LoadMessages returns the message history for a session.
	LoadMessages(ctx context.Context, sessionID string) ([]llm.Message, error)

	// Close releases resources.
	Close() error
}
