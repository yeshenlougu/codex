package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/yeshenlougu/codex/internal/store"
)

// handleTools routes GET /api/tools and PUT /api/tools/:name.
func (s *Server) handleTools(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/tools")
	path = strings.TrimPrefix(path, "/")

	if path == "" {
		switch r.Method {
		case "GET":
			s.listTools(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "GET supported")
		}
		return
	}

	// /api/tools/:name
	name := path
	switch r.Method {
	case "PUT":
		s.updateTool(w, r, name)
	default:
		writeError(w, http.StatusMethodNotAllowed, "PUT supported")
	}
}

func (s *Server) listTools(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"tools": []interface{}{}})
		return
	}

	category := r.URL.Query().Get("category")

	tools, err := s.store.ListTools(category)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list tools: "+err.Error())
		return
	}
	if tools == nil {
		tools = []store.ToolRow{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tools": tools,
		"total": len(tools),
	})
}

func (s *Server) updateTool(w http.ResponseWriter, r *http.Request, name string) {
	if s.store == nil {
		writeError(w, http.StatusInternalServerError, "store not initialized")
		return
	}

	var req struct {
		Enabled          *bool   `json:"enabled"`
		Description      *string `json:"description"`
		ApprovalRequired *bool   `json:"approval_required"`
		Risk             *string `json:"risk"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	// Get existing tool
	allTools, err := s.store.ListTools("")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var existing *store.ToolRow
	for i := range allTools {
		if allTools[i].Name == name {
			existing = &allTools[i]
			break
		}
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "tool not found: "+name)
		return
	}

	// Update fields via direct SQL
	enabled := existing.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	desc := existing.Description
	if req.Description != nil {
		desc = *req.Description
	}
	risk := existing.Risk
	if req.Risk != nil {
		risk = *req.Risk
	}
	approval := existing.ApprovalRequired
	if req.ApprovalRequired != nil {
		approval = *req.ApprovalRequired
	}

	// Use raw SQL update (Store doesn't have UpdateTool yet — add it)
	_, err = s.store.UpdateTool(name, desc, risk, approval, enabled)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "updated": name})
}
