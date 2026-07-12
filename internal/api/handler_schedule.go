package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/yeshenlougu/codex/internal/schedule"
)

// handleSchedules handles GET/POST on /api/schedules and DELETE on /api/schedules/{id}.
func (s *Server) handleSchedules(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/schedules")
	path = strings.TrimPrefix(path, "/")

	switch {
	case r.Method == http.MethodGet && path == "":
		s.listSchedules(w)
	case r.Method == http.MethodPost && path == "":
		s.createSchedule(w, r)
	case r.Method == http.MethodDelete && path != "":
		s.deleteSchedule(w, path)
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/toggle"):
		id := strings.TrimSuffix(path, "/toggle")
		s.toggleSchedule(w, r, id)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) listSchedules(w http.ResponseWriter) {
	tasks := s.scheduler.List()
	writeJSON(w, http.StatusOK, map[string]any{"schedules": tasks})
}

func (s *Server) createSchedule(w http.ResponseWriter, r *http.Request) {
	var task schedule.Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if task.Name == "" || task.Prompt == "" || task.CronExpr == "" {
		writeError(w, http.StatusBadRequest, "name, prompt, and cron_expr are required")
		return
	}
	if task.Category == "" {
		task.Category = "daily"
	}

	created, err := s.scheduler.Create(task)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) deleteSchedule(w http.ResponseWriter, id string) {
	if err := s.scheduler.Delete(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func (s *Server) toggleSchedule(w http.ResponseWriter, r *http.Request, id string) {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if err := s.scheduler.Toggle(id, body.Enabled); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "id": id})
}
