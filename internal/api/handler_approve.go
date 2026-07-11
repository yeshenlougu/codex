package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/yeshenlougu/codex/internal/sandbox"
)

// handleApprove resolves a pending sandbox approval check.
// POST /api/approve
// Body: {"check_id": 123, "approved": true}
// URL path: /api/approve/123?approved=true
func (s *Server) handleApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	checkID := 0
	approved := false

	// Try JSON body
	if r.Header.Get("Content-Type") == "application/json" {
		var body struct {
			CheckID  int  `json:"check_id"`
			Approved bool `json:"approved"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil && body.CheckID > 0 {
			checkID = body.CheckID
			approved = body.Approved
		}
	}

	// Fallback: URL path /api/approve/{id}
	if checkID == 0 {
		path := strings.TrimPrefix(r.URL.Path, "/api/approve/")
		path = strings.TrimSuffix(path, "/")
		if path != "" {
			n, _ := strconv.Atoi(path)
			if n > 0 {
				checkID = n
			}
		}
		// Query param
		if q := r.URL.Query().Get("approved"); q != "" {
			approved = strings.EqualFold(q, "true") || q == "1"
		}
	}

	if checkID <= 0 {
		writeError(w, http.StatusBadRequest, "check_id required (body, URL path, or query)")
		return
	}

	if !sandbox.ApproveCheck(checkID, approved) {
		writeError(w, http.StatusNotFound, "approval check not found or already resolved")
		return
	}

	action := "rejected"
	if approved {
		action = "approved"
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status":   "ok",
		"check_id": strconv.Itoa(checkID),
		"action":   action,
	})
}
