package api

import (
	"net/http"
)

// handlePetState returns the current agent state for the desktop pet.
// This is polled by the Electron pet window to update animations.
func (s *Server) handlePetState(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	// Derive pet state from active sessions
	status := "idle"
	agentCount := 0
	thinkingCount := 0

	s.mu.RLock()
	for _, ag := range s.sessions {
		agentCount++
		if ag.IsThinking() {
			thinkingCount++
		}
	}
	s.mu.RUnlock()

	if agentCount == 0 {
		status = "sleeping"
	} else if thinkingCount > 0 {
		status = "thinking"
	} else if agentCount > 0 {
		status = "idle"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   status,
		"agents":   agentCount,
		"thinking": thinkingCount,
	})
}
