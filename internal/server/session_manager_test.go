package server

import (
	"context"
	"testing"

	"github.com/michaelbrown/forge/internal/config"
	"github.com/michaelbrown/forge/internal/storage"
	"github.com/michaelbrown/forge/internal/storage/sqlite"
	"github.com/michaelbrown/forge/internal/tools"
)

func TestSessionManager_GetOrCreate(t *testing.T) {
	sm := NewSessionManager()
	defer sm.CloseAll()

	store, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"test": {
				BaseURL: "http://localhost:11434/v1/",
				APIKey:  "test",
				Models:  map[string]string{"default": "test-model"},
			},
		},
		DefaultProvider: "test",
		Agent: config.AgentConfig{
			MaxIterations:    5,
			ContextMaxTokens: 4000,
		},
	}

	sess := &storage.Session{
		ID:       "test-session-1",
		Status:   storage.StatusActive,
		Provider: "test",
		Model:    "test-model",
	}
	if err := store.CreateSession(context.Background(), sess); err != nil {
		t.Fatal(err)
	}

	registry := tools.NewRegistry()
	defer registry.Close()

	// First call should create
	as1, err := sm.GetOrCreate(context.Background(), sess, cfg, store, registry)
	if err != nil {
		t.Fatal(err)
	}
	if as1 == nil {
		t.Fatal("expected non-nil ActiveSession")
	}
	if as1.Agent == nil {
		t.Fatal("expected non-nil Agent")
	}

	// Second call should return same instance
	as2, err := sm.GetOrCreate(context.Background(), sess, cfg, store, registry)
	if err != nil {
		t.Fatal(err)
	}
	if as1 != as2 {
		t.Error("expected same ActiveSession instance on second call")
	}
}

func TestSessionManager_Remove(t *testing.T) {
	sm := NewSessionManager()

	store, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"test": {
				BaseURL: "http://localhost:11434/v1/",
				APIKey:  "test",
				Models:  map[string]string{"default": "test-model"},
			},
		},
		DefaultProvider: "test",
		Agent: config.AgentConfig{
			MaxIterations:    5,
			ContextMaxTokens: 4000,
		},
	}

	sess := &storage.Session{
		ID:       "test-session-2",
		Status:   storage.StatusActive,
		Provider: "test",
		Model:    "test-model",
	}
	if err := store.CreateSession(context.Background(), sess); err != nil {
		t.Fatal(err)
	}

	registry := tools.NewRegistry()
	defer registry.Close()

	_, err = sm.GetOrCreate(context.Background(), sess, cfg, store, registry)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := sm.Get("test-session-2"); !ok {
		t.Error("expected session to exist")
	}

	sm.Remove("test-session-2")

	if _, ok := sm.Get("test-session-2"); ok {
		t.Error("expected session to be removed")
	}
}

func TestSessionManager_CloseAll(t *testing.T) {
	sm := NewSessionManager()

	store, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"test": {
				BaseURL: "http://localhost:11434/v1/",
				APIKey:  "test",
				Models:  map[string]string{"default": "test-model"},
			},
		},
		DefaultProvider: "test",
		Agent: config.AgentConfig{
			MaxIterations:    5,
			ContextMaxTokens: 4000,
		},
	}

	registry := tools.NewRegistry()
	defer registry.Close()

	for i := 0; i < 3; i++ {
		id := "session-" + string(rune('a'+i))
		sess := &storage.Session{
			ID:       id,
			Status:   storage.StatusActive,
			Provider: "test",
			Model:    "test-model",
		}
		store.CreateSession(context.Background(), sess)
		sm.GetOrCreate(context.Background(), sess, cfg, store, registry)
	}

	sm.CloseAll()

	if _, ok := sm.Get("session-a"); ok {
		t.Error("expected all sessions to be cleared")
	}
}
