package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/michaelbrown/forge/internal/llm"
	"github.com/michaelbrown/forge/internal/storage"
)

// --- JSON helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// --- Session handlers ---

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	opts := storage.SessionListOptions{}

	if status := r.URL.Query().Get("status"); status != "" {
		opts.Status = storage.SessionStatus(status)
	}
	if limit := r.URL.Query().Get("limit"); limit != "" {
		if n, err := strconv.Atoi(limit); err == nil {
			opts.Limit = n
		}
	}
	if offset := r.URL.Query().Get("offset"); offset != "" {
		if n, err := strconv.Atoi(offset); err == nil {
			opts.Offset = n
		}
	}

	sessions, err := s.store.ListSessions(r.Context(), opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if sessions == nil {
		sessions = []storage.Session{}
	}
	writeJSON(w, http.StatusOK, sessions)
}

type createSessionRequest struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Profile  string `json:"profile"`
	Title    string `json:"title"`
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req createSessionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	providerName := req.Provider
	if providerName == "" {
		providerName = s.cfg.DefaultProvider
	}

	provider, err := s.cfg.Provider(providerName)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	model := req.Model
	if model == "" {
		model = provider.Models["default"]
	}

	sess := &storage.Session{
		ID:       uuid.New().String(),
		Title:    req.Title,
		Status:   storage.StatusActive,
		Provider: providerName,
		Model:    model,
		Profile:  req.Profile,
	}

	if err := s.store.CreateSession(r.Context(), sess); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, sess)
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sess, err := s.store.GetSession(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "session not found")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, sess)
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Remove from active sessions first
	s.sessions.Remove(id)

	if err := s.store.DeleteSession(r.Context(), id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "session not found")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- Message handlers ---

func (s *Server) handleGetMessages(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	messages, err := s.store.LoadMessages(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if messages == nil {
		messages = []llm.Message{}
	}
	writeJSON(w, http.StatusOK, messages)
}

type sendMessageRequest struct {
	Content string `json:"content"`
}

func (s *Server) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req sendMessageRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	// Get or create active session
	sess, err := s.store.GetSession(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	as, err := s.sessions.GetOrCreate(r.Context(), sess, s.cfg, s.store, s.registry)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("initializing agent: %v", err))
		return
	}

	// Lock to ensure one message at a time
	as.mu.Lock()
	defer as.mu.Unlock()

	// Auto-generate title from first message
	if sess.Title == "" {
		sess.Title = generateTitle(req.Content)
		s.store.UpdateSession(r.Context(), sess)
	}

	// Run agent (non-streaming)
	ctx, cancel := context.WithCancel(r.Context())
	as.Cancel = cancel
	defer func() { as.Cancel = nil }()

	response, err := as.Agent.Run(ctx, req.Content)
	cancel()

	// Save messages
	if saveErr := s.store.SaveMessages(r.Context(), sess.ID, as.Agent.History()); saveErr != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("saving messages: %v", saveErr))
		return
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("agent error: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"content": response})
}

// --- Provider/Model handlers ---

type providerInfo struct {
	Name     string            `json:"name"`
	Models   map[string]string `json:"models"`
	IsOllama bool              `json:"is_ollama"`
}

func (s *Server) handleListProviders(w http.ResponseWriter, r *http.Request) {
	var providers []providerInfo
	for name, p := range s.cfg.Providers {
		providers = append(providers, providerInfo{
			Name:     name,
			Models:   p.Models,
			IsOllama: p.IsOllama(),
		})
	}
	writeJSON(w, http.StatusOK, providers)
}

func (s *Server) handleListModels(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")

	provider, err := s.cfg.Provider(providerName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// For Ollama, query live models
	if provider.IsOllama() {
		client := llm.NewClient(provider.BaseURL, provider.APIKey, "")
		models, err := client.ListModels(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("querying models: %v", err))
			return
		}
		writeJSON(w, http.StatusOK, models)
		return
	}

	// For other providers, return configured models
	var models []llm.ModelInfo
	for key, name := range provider.Models {
		models = append(models, llm.ModelInfo{
			Name:       name,
			ModifiedAt: key,
		})
	}
	writeJSON(w, http.StatusOK, models)
}

// generateTitle creates a session title from the first user message.
func generateTitle(firstMessage string) string {
	t := strings.TrimSpace(firstMessage)
	if len(t) > 80 {
		t = t[:80] + "..."
	}
	return t
}
