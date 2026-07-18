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
	list, err := s.sessStore.List(100)
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
		s.handleCreateSession(w, r, id)
	default:
		writeError(w, http.StatusMethodNotAllowed, "GET, POST, DELETE supported")
	}
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request, id string) {
	sess, err := s.sessStore.Load(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// Include agent participants in the response
	agents := s.manager.ListAgents(id)
	writeJSON(w, http.StatusOK, map[string]any{
		"session": sess,
		"agents":  agents,
	})
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request, id string) {
	if err := s.sessStore.Delete(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	// Remove active session and all its agents
	s.manager.RemoveSession(id)
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request, id string) {
	// Create a fresh chat room with default agent
	ag, err := s.manager.CreateSession(id)
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

// handleNewSession creates a new named session.
func (s *Server) handleNewSession(w http.ResponseWriter, r *http.Request) {
	id := agent.NewSessionID()
	ag, err := s.manager.CreateSession(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = ag
	writeJSON(w, http.StatusCreated, map[string]string{"session_id": id})
}
