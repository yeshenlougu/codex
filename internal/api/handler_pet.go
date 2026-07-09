package api

import (
	"net/http"
)

// handlePetState returns the current agent state for the desktop pet.
func (s *Server) handlePetState(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	agents := s.manager.AllAgents()
	agentCount := len(agents)
	thinkingCount := 0
	for _, ag := range agents {
		if ag.IsThinking() {
			thinkingCount++
		}
	}

	status := "sleeping"
	if agentCount > 0 {
		status = "idle"
	}
	if thinkingCount > 0 {
		status = "thinking"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   status,
		"agents":   agentCount,
		"thinking": thinkingCount,
	})
}
