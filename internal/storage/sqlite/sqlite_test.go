package sqlite

import (
	"context"
	"testing"

	"github.com/michaelbrown/forge/internal/llm"
	"github.com/michaelbrown/forge/internal/storage"
)

func testStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("opening memory db: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCreateAndGetSession(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	sess := &storage.Session{
		ID:       "abc12345-0000-0000-0000-000000000000",
		Title:    "test session",
		Status:   storage.StatusActive,
		Provider: "ollama",
		Model:    "qwen3:14b",
		Profile:  "default",
	}

	if err := s.CreateSession(ctx, sess); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	got, err := s.GetSession(ctx, sess.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}

	if got.Title != "test session" {
		t.Errorf("title = %q, want %q", got.Title, "test session")
	}
	if got.Status != storage.StatusActive {
		t.Errorf("status = %q, want %q", got.Status, storage.StatusActive)
	}
	if got.Provider != "ollama" {
		t.Errorf("provider = %q, want %q", got.Provider, "ollama")
	}
	if got.CreatedAt.IsZero() {
		t.Error("created_at should not be zero")
	}
}

func TestGetSessionByPrefix(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	sess := &storage.Session{
		ID:     "abc12345-0000-0000-0000-000000000000",
		Status: storage.StatusActive,
	}
	if err := s.CreateSession(ctx, sess); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	got, err := s.GetSession(ctx, "abc12345")
	if err != nil {
		t.Fatalf("GetSession by prefix: %v", err)
	}
	if got.ID != sess.ID {
		t.Errorf("got ID %q, want %q", got.ID, sess.ID)
	}
}

func TestGetSessionAmbiguousPrefix(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	for _, id := range []string{
		"abc00000-0000-0000-0000-000000000000",
		"abc11111-0000-0000-0000-000000000000",
	} {
		sess := &storage.Session{ID: id, Status: storage.StatusActive}
		if err := s.CreateSession(ctx, sess); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
	}

	_, err := s.GetSession(ctx, "abc")
	if err == nil {
		t.Fatal("expected error for ambiguous prefix")
	}
}

func TestListSessions(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	for _, id := range []string{"aaa", "bbb", "ccc"} {
		sess := &storage.Session{ID: id, Status: storage.StatusActive}
		if err := s.CreateSession(ctx, sess); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
	}

	sessions, err := s.ListSessions(ctx, storage.SessionListOptions{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 3 {
		t.Errorf("got %d sessions, want 3", len(sessions))
	}
}

func TestListSessionsFilterByStatus(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	s.CreateSession(ctx, &storage.Session{ID: "a1", Status: storage.StatusActive})
	s.CreateSession(ctx, &storage.Session{ID: "a2", Status: storage.StatusCompleted})
	s.CreateSession(ctx, &storage.Session{ID: "a3", Status: storage.StatusActive})

	sessions, err := s.ListSessions(ctx, storage.SessionListOptions{Status: storage.StatusActive})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("got %d active sessions, want 2", len(sessions))
	}
}

func TestListSessionsLimit(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		s.CreateSession(ctx, &storage.Session{ID: string(rune('a' + i)), Status: storage.StatusActive})
	}

	sessions, err := s.ListSessions(ctx, storage.SessionListOptions{Limit: 2})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("got %d sessions, want 2", len(sessions))
	}
}

func TestUpdateSession(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	sess := &storage.Session{ID: "upd1", Status: storage.StatusActive}
	s.CreateSession(ctx, sess)

	sess.Title = "updated title"
	sess.Status = storage.StatusCompleted
	if err := s.UpdateSession(ctx, sess); err != nil {
		t.Fatalf("UpdateSession: %v", err)
	}

	got, err := s.GetSession(ctx, "upd1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.Title != "updated title" {
		t.Errorf("title = %q, want %q", got.Title, "updated title")
	}
	if got.Status != storage.StatusCompleted {
		t.Errorf("status = %q, want %q", got.Status, storage.StatusCompleted)
	}
}

func TestDeleteSession(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	sess := &storage.Session{ID: "del1", Status: storage.StatusActive}
	s.CreateSession(ctx, sess)
	s.SaveMessages(ctx, "del1", []llm.Message{{Role: llm.RoleUser, Content: "hello"}})

	if err := s.DeleteSession(ctx, "del1"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	_, err := s.GetSession(ctx, "del1")
	if err == nil {
		t.Fatal("expected error after delete")
	}

	msgs, err := s.LoadMessages(ctx, "del1")
	if err != nil {
		t.Fatalf("LoadMessages after delete: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected no messages after delete, got %d", len(msgs))
	}
}

func TestSaveAndLoadMessages(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	sess := &storage.Session{ID: "msg1", Status: storage.StatusActive}
	s.CreateSession(ctx, sess)

	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: "You are helpful."},
		{Role: llm.RoleUser, Content: "Hello"},
		{
			Role:    llm.RoleAssistant,
			Content: "I'll check that for you.",
			ToolCalls: []llm.ToolCall{
				{ID: "tc1", Name: "shell_exec", Args: map[string]any{"command": "ls"}},
			},
		},
		{Role: llm.RoleTool, Content: "file1.txt\nfile2.txt", ToolCallID: "tc1"},
		{Role: llm.RoleAssistant, Content: "Here are the files."},
	}

	if err := s.SaveMessages(ctx, "msg1", messages); err != nil {
		t.Fatalf("SaveMessages: %v", err)
	}

	loaded, err := s.LoadMessages(ctx, "msg1")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}

	if len(loaded) != 5 {
		t.Fatalf("got %d messages, want 5", len(loaded))
	}

	if loaded[0].Role != llm.RoleSystem {
		t.Errorf("msg[0] role = %q, want system", loaded[0].Role)
	}
	if loaded[2].ToolCalls[0].Name != "shell_exec" {
		t.Errorf("msg[2] tool call name = %q, want shell_exec", loaded[2].ToolCalls[0].Name)
	}
	if loaded[3].ToolCallID != "tc1" {
		t.Errorf("msg[3] tool_call_id = %q, want tc1", loaded[3].ToolCallID)
	}
}

func TestSaveMessagesOverwrites(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	sess := &storage.Session{ID: "ow1", Status: storage.StatusActive}
	s.CreateSession(ctx, sess)

	// Save initial
	s.SaveMessages(ctx, "ow1", []llm.Message{{Role: llm.RoleUser, Content: "first"}})

	// Overwrite
	s.SaveMessages(ctx, "ow1", []llm.Message{
		{Role: llm.RoleUser, Content: "first"},
		{Role: llm.RoleAssistant, Content: "second"},
	})

	loaded, err := s.LoadMessages(ctx, "ow1")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}
	if len(loaded) != 2 {
		t.Errorf("got %d messages, want 2", len(loaded))
	}
}

func TestLoadMessagesEmpty(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	msgs, err := s.LoadMessages(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}
	if msgs != nil {
		t.Errorf("expected nil for nonexistent session, got %v", msgs)
	}
}
