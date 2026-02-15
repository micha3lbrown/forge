package server

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/michaelbrown/forge/internal/agent"
	"github.com/michaelbrown/forge/internal/config"
	"github.com/michaelbrown/forge/internal/llm"
	"github.com/michaelbrown/forge/internal/storage"
	"github.com/michaelbrown/forge/internal/tools"
)

// ActiveSession tracks an in-memory agent for a session.
type ActiveSession struct {
	Agent  *agent.Agent
	Cancel context.CancelFunc // cancels in-flight RunStreaming
	mu     sync.Mutex         // one message at a time per session
}

// SessionManager tracks which sessions have an active Agent in memory.
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*ActiveSession
}

// NewSessionManager creates a new SessionManager.
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*ActiveSession),
	}
}

// Get returns an active session if it exists.
func (sm *SessionManager) Get(sessionID string) (*ActiveSession, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	as, ok := sm.sessions[sessionID]
	return as, ok
}

// GetOrCreate returns an existing active session or creates a new one.
// This encapsulates the full agent initialization pattern from chat.go.
func (sm *SessionManager) GetOrCreate(
	ctx context.Context,
	sess *storage.Session,
	cfg *config.Config,
	store storage.Store,
	registry *tools.Registry,
) (*ActiveSession, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if as, ok := sm.sessions[sess.ID]; ok {
		return as, nil
	}

	// Resolve provider
	providerName := sess.Provider
	if providerName == "" {
		providerName = cfg.DefaultProvider
	}
	provider, err := cfg.Provider(providerName)
	if err != nil {
		return nil, fmt.Errorf("resolving provider: %w", err)
	}

	// Resolve model
	model := sess.Model
	if model == "" {
		model = provider.Models["default"]
	}

	// Load profile if specified
	var profile *agent.Profile
	if sess.Profile != "" {
		profilePath := filepath.Join(cfg.Agent.ProfilesDir, sess.Profile+".yaml")
		profile, err = agent.LoadProfile(profilePath)
		if err != nil {
			return nil, fmt.Errorf("loading profile: %w", err)
		}
	}

	maxIter := cfg.Agent.MaxIterations
	if profile != nil && profile.MaxIter > 0 {
		maxIter = profile.MaxIter
	}

	// Create LLM client and agent
	client := llm.NewClient(provider.BaseURL, provider.APIKey, model)
	a := agent.New(client, registry, maxIter)
	a.SetMaxTokens(cfg.Agent.ContextMaxTokens)

	// Set up utility LLM if configured
	if utilityModel, ok := provider.Models["utility"]; ok && utilityModel != "" {
		utilityClient := llm.NewClient(provider.BaseURL, provider.APIKey, utilityModel)
		a.SetUtilityLLM(utilityClient)
	}

	// Apply profile overrides
	if profile != nil {
		a.SetSystemPrompt(profile.SystemPrompt)
		a.FilterTools(profile.Tools)
	}

	// Load existing history if any
	messages, err := store.LoadMessages(ctx, sess.ID)
	if err != nil {
		return nil, fmt.Errorf("loading messages: %w", err)
	}
	if len(messages) > 0 {
		a.SetHistory(messages)
	}

	as := &ActiveSession{
		Agent: a,
	}
	sm.sessions[sess.ID] = as
	return as, nil
}

// Remove removes an active session and cancels any in-flight work.
func (sm *SessionManager) Remove(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if as, ok := sm.sessions[sessionID]; ok {
		if as.Cancel != nil {
			as.Cancel()
		}
		delete(sm.sessions, sessionID)
	}
}

// CloseAll cancels all active sessions.
func (sm *SessionManager) CloseAll() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	for id, as := range sm.sessions {
		if as.Cancel != nil {
			as.Cancel()
		}
		delete(sm.sessions, id)
	}
}
