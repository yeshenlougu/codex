package api

import (
	"net/http"

	"github.com/yeshenlougu/codex/internal/update"
)

const (
	updateOwner = "yeshenlougu"
	updateRepo  = "codex"
	currentVer  = "1.0.0"
)

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	latest, hasUpdate, err := update.CheckUpdate(updateOwner, updateRepo, currentVer)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"current":  currentVer,
		"latest":   latest.TagName,
		"name":     latest.Name,
		"has_update": hasUpdate,
		"url":      latest.HTMLURL,
		"body":     latest.Body,
		"published": latest.PublishedAt.Format("2006-01-02"),
	})
}
