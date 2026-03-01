package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/michaelbrown/forge/internal/config"
	"github.com/michaelbrown/forge/internal/storage"
	"github.com/michaelbrown/forge/internal/storage/sqlite"
	"github.com/michaelbrown/forge/internal/tools"
)

// newTestServer creates a Server with an in-memory store and multiple providers.
func newTestServer(t *testing.T) *Server {
	t.Helper()

	store, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"ollama": {
				BaseURL: "http://localhost:11434/v1/",
				APIKey:  "ollama",
				Models:  map[string]string{"default": "qwen3:14b"},
			},
			"claude": {
				BaseURL: "https://api.anthropic.com/v1/",
				APIKey:  "test-key",
				Models:  map[string]string{"default": "claude-sonnet-4-5-20250929"},
			},
			"gemini": {
				BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai/",
				APIKey:  "test-key",
				Models:  map[string]string{"default": "gemini-2.0-flash"},
			},
		},
		DefaultProvider: "ollama",
		Agent: config.AgentConfig{
			MaxIterations:    5,
			ContextMaxTokens: 4000,
		},
	}

	registry := tools.NewRegistry()
	t.Cleanup(func() { registry.Close() })

	return New(cfg, store, registry)
}

func TestListProviders_ReturnsAllConfigured(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/providers", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var providers []providerInfo
	if err := json.Unmarshal(w.Body.Bytes(), &providers); err != nil {
		t.Fatal(err)
	}

	if len(providers) != 3 {
		t.Errorf("expected 3 providers, got %d", len(providers))
	}

	// Check all three are present
	names := make(map[string]bool)
	for _, p := range providers {
		names[p.Name] = true
	}
	for _, expected := range []string{"ollama", "claude", "gemini"} {
		if !names[expected] {
			t.Errorf("expected provider %q in response", expected)
		}
	}
}

func TestCreateSession_DefaultProvider(t *testing.T) {
	srv := newTestServer(t)

	body := `{}`
	req := httptest.NewRequest("POST", "/api/sessions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var sess storage.Session
	if err := json.Unmarshal(w.Body.Bytes(), &sess); err != nil {
		t.Fatal(err)
	}

	if sess.Provider != "ollama" {
		t.Errorf("expected provider 'ollama', got %q", sess.Provider)
	}
	if sess.Model != "qwen3:14b" {
		t.Errorf("expected model 'qwen3:14b', got %q", sess.Model)
	}
}

func TestCreateSession_ExplicitProvider(t *testing.T) {
	srv := newTestServer(t)

	body := `{"provider": "claude", "model": "claude-sonnet-4-5-20250929"}`
	req := httptest.NewRequest("POST", "/api/sessions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var sess storage.Session
	if err := json.Unmarshal(w.Body.Bytes(), &sess); err != nil {
		t.Fatal(err)
	}

	if sess.Provider != "claude" {
		t.Errorf("expected provider 'claude', got %q", sess.Provider)
	}
	if sess.Model != "claude-sonnet-4-5-20250929" {
		t.Errorf("expected model 'claude-sonnet-4-5-20250929', got %q", sess.Model)
	}
}

func TestCreateSession_InvalidProvider(t *testing.T) {
	srv := newTestServer(t)

	body := `{"provider": "nonexistent"}`
	req := httptest.NewRequest("POST", "/api/sessions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateSession_ChangeModel(t *testing.T) {
	srv := newTestServer(t)

	// Create a session first
	body := `{"provider": "ollama"}`
	req := httptest.NewRequest("POST", "/api/sessions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	var sess storage.Session
	json.Unmarshal(w.Body.Bytes(), &sess)

	// Update to a different provider/model
	updateBody := `{"provider": "claude", "model": "claude-sonnet-4-5-20250929"}`
	req = httptest.NewRequest("PATCH", "/api/sessions/"+sess.ID, bytes.NewBufferString(updateBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated storage.Session
	if err := json.Unmarshal(w.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}

	if updated.Provider != "claude" {
		t.Errorf("expected provider 'claude', got %q", updated.Provider)
	}
	if updated.Model != "claude-sonnet-4-5-20250929" {
		t.Errorf("expected model 'claude-sonnet-4-5-20250929', got %q", updated.Model)
	}
}

func TestUpdateSession_InvalidProvider(t *testing.T) {
	srv := newTestServer(t)

	// Create a session
	body := `{}`
	req := httptest.NewRequest("POST", "/api/sessions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	var sess storage.Session
	json.Unmarshal(w.Body.Bytes(), &sess)

	// Try to update to invalid provider
	updateBody := `{"provider": "nonexistent"}`
	req = httptest.NewRequest("PATCH", "/api/sessions/"+sess.ID, bytes.NewBufferString(updateBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateSession_NotFound(t *testing.T) {
	srv := newTestServer(t)

	body := `{"model": "test"}`
	req := httptest.NewRequest("PATCH", "/api/sessions/nonexistent-id", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateSession_EvictsActiveSession(t *testing.T) {
	srv := newTestServer(t)

	// Create session in store
	sess := &storage.Session{
		ID:       "evict-test",
		Status:   storage.StatusActive,
		Provider: "ollama",
		Model:    "qwen3:14b",
	}
	srv.store.CreateSession(context.Background(), sess)

	// Simulate an active session in the manager
	registry := tools.NewRegistry()
	defer registry.Close()
	srv.sessions.GetOrCreate(context.Background(), sess, srv.cfg, srv.store, registry)

	// Verify it's active
	if _, ok := srv.sessions.Get("evict-test"); !ok {
		t.Fatal("expected active session to exist before update")
	}

	// Update model via PATCH
	body := `{"provider": "claude", "model": "claude-sonnet-4-5-20250929"}`
	req := httptest.NewRequest("PATCH", "/api/sessions/evict-test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Active session should be evicted so the new model takes effect
	if _, ok := srv.sessions.Get("evict-test"); ok {
		t.Error("expected active session to be evicted after model change")
	}
}

func TestUpdateSession_BareProviderName(t *testing.T) {
	srv := newTestServer(t)

	// Create an ollama session
	body := `{}`
	req := httptest.NewRequest("POST", "/api/sessions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	var sess storage.Session
	json.Unmarshal(w.Body.Bytes(), &sess)

	// PATCH with only model="gemini" (no provider) — should resolve to gemini provider
	updateBody := `{"model": "gemini"}`
	req = httptest.NewRequest("PATCH", "/api/sessions/"+sess.ID, bytes.NewBufferString(updateBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated storage.Session
	json.Unmarshal(w.Body.Bytes(), &updated)

	if updated.Provider != "gemini" {
		t.Errorf("expected provider 'gemini', got %q", updated.Provider)
	}
	if updated.Model != "gemini-2.0-flash" {
		t.Errorf("expected model 'gemini-2.0-flash' (default), got %q", updated.Model)
	}
}

func TestListModels_OllamaFallsBackToConfig(t *testing.T) {
	srv := newTestServer(t)

	// Ollama is not running, so live query will fail.
	// The endpoint should fall back to returning configured models.
	req := httptest.NewRequest("GET", "/api/models/ollama", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var models []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &models); err != nil {
		t.Fatal(err)
	}

	// Should return at least the configured default model
	if len(models) == 0 {
		t.Error("expected at least one fallback model when Ollama is unreachable")
	}

	// Check that the default model name is present
	found := false
	for _, m := range models {
		if name, ok := m["name"].(string); ok && name == "qwen3:14b" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected configured default model 'qwen3:14b' in fallback results, got %v", models)
	}
}

func TestListModels_NonOllamaProvider(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/models/claude", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var models []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &models); err != nil {
		t.Fatal(err)
	}

	if len(models) == 0 {
		t.Error("expected at least one model for claude provider")
	}
}

func TestListModels_InvalidProvider(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/models/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSessionCRUD_FullLifecycle(t *testing.T) {
	srv := newTestServer(t)

	// Create
	body := `{"provider": "claude", "model": "claude-sonnet-4-5-20250929", "title": "Test Chat"}`
	req := httptest.NewRequest("POST", "/api/sessions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d", w.Code)
	}

	var sess storage.Session
	json.Unmarshal(w.Body.Bytes(), &sess)

	// Get
	req = httptest.NewRequest("GET", "/api/sessions/"+sess.ID, nil)
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d", w.Code)
	}

	// List
	req = httptest.NewRequest("GET", "/api/sessions", nil)
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", w.Code)
	}

	var sessions []storage.Session
	json.Unmarshal(w.Body.Bytes(), &sessions)
	if len(sessions) == 0 {
		t.Error("list: expected at least one session")
	}

	// Update (PATCH)
	req = httptest.NewRequest("PATCH", "/api/sessions/"+sess.ID, bytes.NewBufferString(`{"model": "gemini-2.0-flash", "provider": "gemini"}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d", w.Code)
	}

	var updated storage.Session
	json.Unmarshal(w.Body.Bytes(), &updated)
	if updated.Provider != "gemini" || updated.Model != "gemini-2.0-flash" {
		t.Errorf("update: expected gemini/gemini-2.0-flash, got %s/%s", updated.Provider, updated.Model)
	}

	// Delete
	req = httptest.NewRequest("DELETE", "/api/sessions/"+sess.ID, nil)
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d", w.Code)
	}

	// Verify deleted
	req = httptest.NewRequest("GET", "/api/sessions/"+sess.ID, nil)
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("get after delete: expected 404, got %d", w.Code)
	}
}
