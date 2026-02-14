package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/michaelbrown/forge/internal/llm"
	"github.com/michaelbrown/forge/internal/storage"

	_ "modernc.org/sqlite"
)

// SQLiteStore implements storage.Store backed by a SQLite database.
type SQLiteStore struct {
	db *sql.DB
}

// Open creates or opens a SQLite database at the given path and runs migrations.
// Use ":memory:" for an in-memory database (useful for testing).
func Open(dbPath string) (*SQLiteStore, error) {
	if dbPath != ":memory:" {
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("creating db directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := runMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) CreateSession(ctx context.Context, sess *storage.Session) error {
	now := time.Now().UTC()
	sess.CreatedAt = now
	sess.UpdatedAt = now

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions (id, title, status, provider, model, profile, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sess.ID, sess.Title, sess.Status, sess.Provider, sess.Model, sess.Profile,
		sess.CreatedAt.Format(time.RFC3339), sess.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("inserting session: %w", err)
	}

	// Initialize empty messages row
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO session_messages (session_id, messages) VALUES (?, '[]')`,
		sess.ID,
	)
	return err
}

func (s *SQLiteStore) GetSession(ctx context.Context, id string) (*storage.Session, error) {
	// Try exact match first, then prefix match
	sess, err := s.getSessionExact(ctx, id)
	if err == nil {
		return sess, nil
	}

	// Prefix match
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, status, provider, model, profile, created_at, updated_at
		FROM sessions WHERE id LIKE ? || '%'`, id)
	if err != nil {
		return nil, fmt.Errorf("querying session: %w", err)
	}
	defer rows.Close()

	var matches []*storage.Session
	for rows.Next() {
		sess, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		matches = append(matches, sess)
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("session not found: %s", id)
	case 1:
		return matches[0], nil
	default:
		return nil, fmt.Errorf("ambiguous session prefix %q matches %d sessions", id, len(matches))
	}
}

func (s *SQLiteStore) getSessionExact(ctx context.Context, id string) (*storage.Session, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, title, status, provider, model, profile, created_at, updated_at
		FROM sessions WHERE id = ?`, id)
	return scanSessionRow(row)
}

func (s *SQLiteStore) ListSessions(ctx context.Context, opts storage.SessionListOptions) ([]storage.Session, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}

	query := `SELECT id, title, status, provider, model, profile, created_at, updated_at FROM sessions`
	var args []any

	if opts.Status != "" {
		query += ` WHERE status = ?`
		args = append(args, string(opts.Status))
	}

	query += ` ORDER BY updated_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, opts.Offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing sessions: %w", err)
	}
	defer rows.Close()

	var sessions []storage.Session
	for rows.Next() {
		sess, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, *sess)
	}
	return sessions, rows.Err()
}

func (s *SQLiteStore) UpdateSession(ctx context.Context, sess *storage.Session) error {
	sess.UpdatedAt = time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		UPDATE sessions SET title = ?, status = ?, updated_at = ? WHERE id = ?`,
		sess.Title, sess.Status, sess.UpdatedAt.Format(time.RFC3339), sess.ID,
	)
	return err
}

func (s *SQLiteStore) DeleteSession(ctx context.Context, id string) error {
	// Resolve prefix first
	sess, err := s.GetSession(ctx, id)
	if err != nil {
		return err
	}

	// Delete messages first (foreign key), then session
	_, err = s.db.ExecContext(ctx, `DELETE FROM session_messages WHERE session_id = ?`, sess.ID)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, sess.ID)
	return err
}

func (s *SQLiteStore) SaveMessages(ctx context.Context, sessionID string, messages []llm.Message) error {
	data, err := json.Marshal(messages)
	if err != nil {
		return fmt.Errorf("marshaling messages: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO session_messages (session_id, messages, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET messages = excluded.messages, updated_at = excluded.updated_at`,
		sessionID, string(data), now,
	)
	return err
}

func (s *SQLiteStore) LoadMessages(ctx context.Context, sessionID string) ([]llm.Message, error) {
	var data string
	err := s.db.QueryRowContext(ctx, `
		SELECT messages FROM session_messages WHERE session_id = ?`, sessionID).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("loading messages: %w", err)
	}

	var messages []llm.Message
	if err := json.Unmarshal([]byte(data), &messages); err != nil {
		return nil, fmt.Errorf("unmarshaling messages: %w", err)
	}
	return messages, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// Scanner interface to work with both *sql.Row and *sql.Rows
type scanner interface {
	Scan(dest ...any) error
}

func scanSessionFromScanner(s scanner) (*storage.Session, error) {
	var sess storage.Session
	var createdAt, updatedAt string
	err := s.Scan(&sess.ID, &sess.Title, &sess.Status, &sess.Provider,
		&sess.Model, &sess.Profile, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	sess.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	sess.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &sess, nil
}

func scanSession(rows *sql.Rows) (*storage.Session, error) {
	return scanSessionFromScanner(rows)
}

func scanSessionRow(row *sql.Row) (*storage.Session, error) {
	return scanSessionFromScanner(row)
}
