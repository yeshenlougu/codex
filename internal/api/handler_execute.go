package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/yeshenlougu/codex/internal/workflow"
)

// handleExecuteTask reads a task from PLAN.md and sends it to the agent
// for actual implementation.  The agent uses its tools (shell, edit_file,
// etc.) to carry out the task, and the response is streamed via SSE.
//
//   POST /api/execute/3
//   POST /api/execute  (body: {"task_num": 3})
//
// Query params: session_id, agent_name (optional)
func (s *Server) handleExecuteTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	taskNum := 0

	// Try JSON body
	if r.Header.Get("Content-Type") == "application/json" {
		var body struct {
			TaskNum   int    `json:"task_num"`
			SessionID string `json:"session_id"`
			AgentName string `json:"agent_name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			taskNum = body.TaskNum
		}
	}

	// Fallback: URL path /api/execute/{n}
	if taskNum == 0 {
		path := strings.TrimPrefix(r.URL.Path, "/api/execute/")
		path = strings.TrimSuffix(path, "/")
		if path != "" {
			n, _ := strconv.Atoi(path)
			if n > 0 {
				taskNum = n
			}
		}
	}

	// Fallback: query param
	if taskNum == 0 {
		if q := r.URL.Query().Get("task"); q != "" {
			fmt.Sscanf(q, "%d", &taskNum)
		}
	}

	if taskNum <= 0 {
		writeError(w, http.StatusBadRequest, "task_num required (body, URL path, or query)")
		return
	}

	// Read PLAN.md and find the task
	tl, err := workflow.ParseTasks("PLAN.md")
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{
			"content": "No PLAN.md found. Use /plan to generate one first.",
		})
		return
	}

	if taskNum > len(tl.Tasks) {
		writeJSON(w, http.StatusOK, map[string]string{
			"content": fmt.Sprintf("Task %d not found — PLAN.md has %d tasks.", taskNum, len(tl.Tasks)),
		})
		return
	}

	task := tl.Tasks[taskNum-1]
	if task.Completed {
		writeJSON(w, http.StatusOK, map[string]string{
			"content": fmt.Sprintf("Task %d is already completed: %s", taskNum, task.Content),
		})
		return
	}

	sessionID := r.URL.Query().Get("session_id")
	agentName := r.URL.Query().Get("agent_name")

	if sessionID == "" {
		sessionID = fmt.Sprintf("exec-%d", taskNum)
	}

	// Get or create the agent session
	ag, err := s.manager.GetAgent(sessionID, "")
	if err != nil {
		ag, err = s.manager.CreateSession(sessionID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "create session: "+err.Error())
			return
		}
	}

	if agentName != "" && agentName != "default" {
		if _, err := s.manager.AddAgent(sessionID, agentName); err != nil {
			log.Printf("[execute] add agent %s: %v", agentName, err)
		}
	}

	// Build the implementation prompt
	prompt := fmt.Sprintf(`Execute the following task from PLAN.md:

Task %d: %s

Phase: %s

Instructions:
1. Read any relevant files first using the read_file tool
2. Implement the changes step by step
3. Use edit_file or write_file to make changes
4. Use shell to verify your changes (e.g., build, test)
5. When done, summarize what you changed and why

IMPORTANT: Only implement task %d. Do not work on other tasks.`,
		taskNum, task.Content, task.Phase, taskNum)

	// SSE streaming response
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Send task info first
	taskInfo, _ := json.Marshal(map[string]any{
		"type":     "task_info",
		"task_num": taskNum,
		"content":  task.Content,
		"phase":    task.Phase,
	})
	fmt.Fprintf(w, "data: %s\n\n", taskInfo)
	flusher.Flush()

	result, runErr := ag.Run(prompt, func(chunk string) {
		encoded, _ := json.Marshal(map[string]string{"type": "chunk", "content": chunk})
		fmt.Fprintf(w, "data: %s\n\n", encoded)
		flusher.Flush()
	})

	if runErr != nil {
		errData, _ := json.Marshal(map[string]string{"type": "error", "error": runErr.Error()})
		fmt.Fprintf(w, "data: %s\n\n", errData)
		flusher.Flush()
		return
	}

	// Auto-mark task as done
	marked, markErr := workflow.MarkTaskAsDone("PLAN.md", taskNum)
	if markErr != nil {
		log.Printf("[execute] mark task %d done: %v", taskNum, markErr)
	} else {
		log.Printf("[execute] task %d marked done: %s", taskNum, marked)
	}

	// Finalize: send result + completion
	doneData, _ := json.Marshal(map[string]any{
		"type":       "done",
		"content":    result,
		"task_num":   taskNum,
		"marked_done": markErr == nil,
	})
	fmt.Fprintf(w, "data: %s\n\n", doneData)
	flusher.Flush()

	// Also broadcast via WebSocket
	s.wsHub.broadcastMsg(wsMessage{
		Type:    "task_completed",
		Content: fmt.Sprintf("Task %d completed: %s", taskNum, task.Content),
	})
}
