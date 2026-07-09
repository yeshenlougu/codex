// Package api provides the HTTP + WebSocket server for the Codex agent.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/yeshenlougu/codex/internal/agent"
	"github.com/yeshenlougu/codex/internal/config"
	"github.com/yeshenlougu/codex/internal/session"
)

// Server is the HTTP/WebSocket API server.
type Server struct {
	cfg      *config.Config
	store    *session.Store
	sessions map[string]*agent.Agent // active agents keyed by session ID
	mu       sync.RWMutex
	httpSrv  *http.Server
	wsHub    *wsHub
	addr     string
}

// New creates a new API server.
func New(cfg *config.Config, store *session.Store, addr string) *Server {
	s := &Server{
		cfg:      cfg,
		store:    store,
		addr:     addr,
		sessions: make(map[string]*agent.Agent),
		wsHub:    newWSHub(),
	}
	return s
}

// Start begins listening and returns immediately.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// CORS middleware wrapper
	cors := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			h(w, r)
		}
	}

	// Health
	mux.HandleFunc("/health", cors(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}))

	// Session CRUD
	mux.HandleFunc("/api/sessions", cors(s.handleListSessions))
	mux.HandleFunc("/api/sessions/", cors(s.handleSession))

	// Chat (non-streaming)
	mux.HandleFunc("/api/chat", cors(s.handleChat))

	// Config
	mux.HandleFunc("/api/config", cors(s.handleGetConfig))

	// Release update
	mux.HandleFunc("/api/update", cors(s.handleUpdate))

	// File browser
	mux.HandleFunc("/api/files", cors(s.handleListFiles))
	mux.HandleFunc("/api/files/content", cors(s.handleReadFile))
	mux.HandleFunc("/api/files/diff", cors(s.handleDiff))

	// Pet state
	mux.HandleFunc("/api/pet-state", cors(s.handlePetState))

	// WebSocket
	mux.HandleFunc("/ws", s.handleWebSocket)

	// Static files (frontend)
	mux.Handle("/", http.FileServer(http.Dir("web/dist")))

	s.httpSrv = &http.Server{
		Addr:         s.addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute, // long for streaming
		IdleTimeout:  120 * time.Second,
	}

	// Start WebSocket hub
	go s.wsHub.run()

	log.Printf("[api] listening on %s", s.addr)
	return s.httpSrv.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpSrv.Shutdown(ctx)
}

// getOrCreateAgent returns an agent for a session ID, creating one if needed.
func (s *Server) getOrCreateAgent(sessionID string, create bool) (*agent.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ag, ok := s.sessions[sessionID]; ok {
		return ag, nil
	}
	if !create {
		return nil, fmt.Errorf("session %s not active", sessionID)
	}

	ag := agent.New(s.cfg).WithStore(s.store)

	// Try to load from disk
	if _, err := s.store.Load(sessionID); err == nil {
		if err := ag.LoadSession(sessionID); err != nil {
			return nil, err
		}
	} else {
		ag.SetSessionID(sessionID)
	}

	s.sessions[sessionID] = ag
	return ag, nil
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
