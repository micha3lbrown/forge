package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/michaelbrown/forge/internal/storage"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Tailscale handles auth
	},
}

// wsIncoming is a message from the client.
type wsIncoming struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

// wsOutgoing is a message to the client.
type wsOutgoing struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
	Name    string `json:"name,omitempty"`
	Args    any    `json:"args,omitempty"`
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Verify session exists
	sess, err := s.store.GetSession(r.Context(), id)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	// Get or create active session
	as, err := s.sessions.GetOrCreate(r.Context(), sess, s.cfg, s.store, s.registry)
	if err != nil {
		wsWriteJSON(conn, wsOutgoing{Type: "error", Content: fmt.Sprintf("initializing agent: %v", err)})
		return
	}

	// Read loop
	for {
		var msg wsIncoming
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return
			}
			log.Printf("websocket read error: %v", err)
			return
		}

		if msg.Type != "message" || msg.Content == "" {
			wsWriteJSON(conn, wsOutgoing{Type: "error", Content: "invalid message"})
			continue
		}

		// Process message with agent
		s.processWebSocketMessage(conn, as, sess, msg.Content)
	}
}

func (s *Server) processWebSocketMessage(conn *websocket.Conn, as *ActiveSession, sess *storage.Session, content string) {
	// Ensure one message at a time
	as.mu.Lock()
	defer as.mu.Unlock()

	// Mutex for thread-safe writes to the WebSocket connection
	var wsMu sync.Mutex

	// Auto-generate title from first message
	if sess.Title == "" {
		sess.Title = generateTitle(content)
		s.store.UpdateSession(context.Background(), sess)
	}

	// Create cancellable context — cancelled on client disconnect
	ctx, cancel := context.WithCancel(context.Background())
	as.Cancel = cancel
	defer func() {
		cancel()
		as.Cancel = nil
	}()

	// Wire agent callbacks to send WebSocket messages
	as.Agent.OnTextDelta = func(delta string) {
		wsMu.Lock()
		wsWriteJSON(conn, wsOutgoing{Type: "text_delta", Content: delta})
		wsMu.Unlock()
	}
	as.Agent.OnToolCall = func(name string, args map[string]any) {
		wsMu.Lock()
		wsWriteJSON(conn, wsOutgoing{Type: "tool_call", Name: name, Args: args})
		wsMu.Unlock()
	}
	as.Agent.OnToolResult = func(name string, result string) {
		wsMu.Lock()
		wsWriteJSON(conn, wsOutgoing{Type: "tool_result", Name: name, Content: result})
		wsMu.Unlock()
	}

	// Run agent with streaming
	response, err := as.Agent.RunStreaming(ctx, content)

	// Save messages regardless of error
	if saveErr := s.store.SaveMessages(context.Background(), sess.ID, as.Agent.History()); saveErr != nil {
		log.Printf("failed to save messages for session %s: %v", sess.ID, saveErr)
	}

	wsMu.Lock()
	defer wsMu.Unlock()

	if err != nil {
		if ctx.Err() != nil {
			wsWriteJSON(conn, wsOutgoing{Type: "error", Content: "interrupted"})
		} else {
			wsWriteJSON(conn, wsOutgoing{Type: "error", Content: err.Error()})
		}
		return
	}

	wsWriteJSON(conn, wsOutgoing{Type: "done", Content: response})
}

func wsWriteJSON(conn *websocket.Conn, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("websocket marshal error: %v", err)
		return
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Printf("websocket write error: %v", err)
	}
}
