package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/yeshenlougu/codex/internal/workflow"
)

// handleListTasks returns the parsed PLAN.md task list as JSON.
func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	tl, err := workflow.ParseTasks("PLAN.md")
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"content": "No PLAN.md found. Use /plan to generate one.",
			"tasks":   []workflow.Task{},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"content": workflow.FormatTasksForChat(tl),
		"tasks":   tl.Tasks,
	})
}

// handleImplementTask marks a task as done in PLAN.md.
// Accepts POST with JSON body {"task_num": 3} or query param ?task=3
func (s *Server) handleImplementTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	taskNum := 0

	// Try JSON body first
	if r.Header.Get("Content-Type") == "application/json" {
		var body struct {
			TaskNum int `json:"task_num"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil && body.TaskNum > 0 {
			taskNum = body.TaskNum
		}
	}

	// Fallback: query param
	if taskNum == 0 {
		q := r.URL.Query().Get("task")
		if q != "" {
			fmt.Sscanf(q, "%d", &taskNum)
		}
	}

	// Fallback: extract from URL path /api/implement/{n}
	if taskNum == 0 {
		path := strings.TrimPrefix(r.URL.Path, "/api/implement/")
		path = strings.TrimSuffix(path, "/")
		if path != "" {
			fmt.Sscanf(path, "%d", &taskNum)
		}
	}

	if taskNum <= 0 {
		writeError(w, http.StatusBadRequest, "task_num required (body, query, or URL path)")
		return
	}

	content, err := workflow.MarkTaskAsDone("PLAN.md", taskNum)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{
			"content": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"content": "✅ Task " + strconv.Itoa(taskNum) + " marked as done: " + content,
	})
}
