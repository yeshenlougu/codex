package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/yeshenlougu/codex/internal/agent"
)

// ===================== Agent Profile CRUD =====================

// handleAgents handles GET (list) and POST (create) on /api/agents.
func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		profiles := s.manager.Registry().List()
		writeJSON(w, http.StatusOK, map[string]any{
			"agents": profiles,
		})
	case "POST":
		s.handleCreateAgent(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "GET, POST supported")
	}
}

// handleAgentByID handles /api/agents/{name} — GET, PUT, DELETE.
func (s *Server) handleAgentByID(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/agents/")
	if name == "" || name == "/api/agents" {
		s.handleAgents(w, r)
		return
	}

	// Sub-paths: /api/agents/{name}/clone
	if strings.HasSuffix(name, "/clone") {
		name = strings.TrimSuffix(name, "/clone")
		s.handleCloneAgent(w, r, name)
		return
	}

	switch r.Method {
	case "GET":
		s.handleGetAgent(w, r, name)
	case "PUT":
		s.handleUpdateAgent(w, r, name)
	case "DELETE":
		s.handleDeleteAgent(w, r, name)
	default:
		writeError(w, http.StatusMethodNotAllowed, "GET, PUT, DELETE supported")
	}
}

func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request, name string) {
	p := s.manager.Registry().Get(name)
	if p == nil {
		writeError(w, http.StatusNotFound, "agent profile not found")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleCreateAgent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string `json:"name"`
		CloneFrom string `json:"clone_from"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}

	var profile *agent.AgentProfile
	var err error

	if req.CloneFrom != "" && req.CloneFrom != "default" {
		profile, err = s.manager.Registry().CloneFrom(req.CloneFrom, req.Name)
	} else {
		profile, err = s.manager.Registry().Create(req.Name)
	}

	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, profile)
}

func (s *Server) handleUpdateAgent(w http.ResponseWriter, r *http.Request, name string) {
	var updates agent.AgentProfile
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if err := s.manager.Registry().Update(name, &updates); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"updated": name})
}

func (s *Server) handleDeleteAgent(w http.ResponseWriter, r *http.Request, name string) {
	if err := s.manager.Registry().Delete(name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"deleted": name})
}

func (s *Server) handleCloneAgent(w http.ResponseWriter, r *http.Request, sourceName string) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}

	profile, err := s.manager.Registry().CloneFrom(sourceName, req.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, profile)
}

// ===================== Session Agent Management =====================

// handleSessionAgents handles /api/sessions/{id}/agents
//   GET  /api/sessions/{id}/agents        — list agents in session
//   POST /api/sessions/{id}/agents        — add agent (body: {"agent_name":"..."})
//   DELETE /api/sessions/{id}/agents/{name} — remove agent
func (s *Server) handleSessionAgents(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	sessionID := parts[0]

	if len(parts) == 2 && parts[1] == "agents" {
		// /api/sessions/{id}/agents
		switch r.Method {
		case "GET":
			agents := s.manager.ListAgents(sessionID)
			writeJSON(w, http.StatusOK, map[string]any{
				"session_id": sessionID,
				"agents":     agents,
			})
		case "POST":
			var req struct {
				AgentName string `json:"agent_name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid JSON")
				return
			}
			if req.AgentName == "" {
				writeError(w, http.StatusBadRequest, "agent_name required")
				return
			}
			ag, err := s.manager.AddAgent(sessionID, req.AgentName)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{
				"session_id": sessionID,
				"agent":      req.AgentName,
				"status":     "joined",
				"session":    ag.SessionID(),
			})
		default:
			writeError(w, http.StatusMethodNotAllowed, "GET, POST supported")
		}
		return
	}

	if len(parts) >= 3 && parts[1] == "agents" {
		// /api/sessions/{id}/agents/{agentName}
		agentName := parts[2]
		if r.Method != "DELETE" {
			writeError(w, http.StatusMethodNotAllowed, "DELETE supported")
			return
		}
		if err := s.manager.RemoveAgent(sessionID, agentName); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"session_id": sessionID,
			"agent":      agentName,
			"status":     "removed",
		})
	}
}
