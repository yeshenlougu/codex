package api

import (
	"net/http"
	"strings"

	"github.com/yeshenlougu/codex/internal/agent"
)

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	list, err := s.store.List(100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(list) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"sessions": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": list})
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	// Extract session ID from path: /api/sessions/{id}
	id := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	if id == "" || id == "/api/sessions" {
		s.handleListSessions(w, r)
		return
	}

	switch r.Method {
	case "GET":
		s.handleGetSession(w, r, id)
	case "DELETE":
		s.handleDeleteSession(w, r, id)
	case "POST":
		// POST /api/sessions/{id} creates/resumes
		s.handleCreateSession(w, r, id)
	default:
		writeError(w, http.StatusMethodNotAllowed, "GET, POST, DELETE supported")
	}
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request, id string) {
	sess, err := s.store.Load(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request, id string) {
	if err := s.store.Delete(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	// Also remove active agent
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request, id string) {
	// Create a fresh session
	ag, err := s.getOrCreateAgent(id, true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sid := ag.SessionID()
	writeJSON(w, http.StatusCreated, map[string]string{
		"session_id": sid,
		"status":     "active",
	})
}

// handleGetConfig returns the current model/provider/key settings (keys masked).
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	cfg := s.cfg
	keyMasked := cfg.Provider.APIKey
	if len(keyMasked) > 8 {
		keyMasked = keyMasked[:4] + "****" + keyMasked[len(keyMasked)-4:]
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"provider":         cfg.Model.Provider,
		"model":            cfg.Model.Model,
		"base_url":         cfg.Provider.BaseURL,
		"api_key_masked":   keyMasked,
		"reasoning_effort": cfg.Model.ReasoningEffort,
		"max_turns":        cfg.Agent.MaxTurns,
		"tool_count":       8,
		"active_sessions":  len(s.sessions),
	})
}

// handleNewSession creates a new named session.
func (s *Server) handleNewSession(w http.ResponseWriter, r *http.Request) {
	id := agent.NewSessionID()
	_, err := s.getOrCreateAgent(id, true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"session_id": id})
}
