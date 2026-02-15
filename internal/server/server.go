package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/michaelbrown/forge/internal/config"
	"github.com/michaelbrown/forge/internal/storage"
	"github.com/michaelbrown/forge/internal/tools"
)

// Server is the HTTP server for the Forge web API.
type Server struct {
	cfg      *config.Config
	store    storage.Store
	registry *tools.Registry
	sessions *SessionManager
	router   chi.Router
	http     *http.Server
}

// New creates a new Server.
func New(cfg *config.Config, store storage.Store, registry *tools.Registry) *Server {
	s := &Server{
		cfg:      cfg,
		store:    store,
		registry: registry,
		sessions: NewSessionManager(),
		router:   chi.NewRouter(),
	}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	r := s.router

	// Global middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	// API routes
	r.Route("/api", func(r chi.Router) {
		r.Use(jsonContentType)

		// Sessions
		r.Get("/sessions", s.handleListSessions)
		r.Post("/sessions", s.handleCreateSession)
		r.Get("/sessions/{id}", s.handleGetSession)
		r.Delete("/sessions/{id}", s.handleDeleteSession)

		// Messages
		r.Get("/sessions/{id}/messages", s.handleGetMessages)
		r.Post("/sessions/{id}/messages", s.handleSendMessage)

		// WebSocket (no JSON content-type)
		r.Get("/sessions/{id}/ws", s.handleWebSocket)

		// Providers & models
		r.Get("/providers", s.handleListProviders)
		r.Get("/models/{provider}", s.handleListModels)
	})

	// SPA fallback
	r.Handle("/*", spaHandler())
}

// jsonContentType sets Content-Type to application/json for API routes.
func jsonContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

// Start begins listening on the given port.
func (s *Server) Start(port int) error {
	addr := fmt.Sprintf(":%d", port)
	s.http = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}

	log.Printf("Forge server starting on http://localhost%s", addr)
	return s.http.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("Shutting down server...")
	s.sessions.CloseAll()

	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return s.http.Shutdown(shutdownCtx)
}
