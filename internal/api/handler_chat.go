package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/yeshenlougu/codex/internal/agent"
	"github.com/yeshenlougu/codex/internal/workflow"
)

// ChatRequest is the incoming chat request.
type ChatRequest struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
	Stream    bool   `json:"stream"`
	New       bool   `json:"new"`       // create new session
	AgentName string `json:"agent_name"` // which agent profile to use (empty = default)
}

// ChatResponse is the non-streaming chat response.
type ChatResponse struct {
	SessionID       string `json:"session_id"`
	Content         string `json:"content"`
	TurnCount       int    `json:"turn_count"`
	ToolCalls       int    `json:"tool_calls"`
	RespondingAgent string `json:"responding_agent"`
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Message == "" {
		writeError(w, http.StatusBadRequest, "message required")
		return
	}

	// ---- Slash command interception ----
	msg := strings.TrimSpace(req.Message)
	if handled := s.interceptSlashCommand(w, r, &req, msg); handled {
		return
	}
	// Re-read msg in case intercept modified req.Message (e.g. /spec → crafted prompt)
	msg = strings.TrimSpace(req.Message)

	// Determine session ID
	sessionID := req.SessionID
	if req.New || sessionID == "" {
		sessionID = agent.NewSessionID()
	}

	// Ensure the session/chat-room exists
	ag, err := s.manager.GetAgent(sessionID, "")
	if err != nil {
		// Create new chat room with default agent
		ag, err = s.manager.CreateSession(sessionID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	// If agent_name specified and different from current, add to room
	if req.AgentName != "" && req.AgentName != "default" {
		if _, err := s.manager.AddAgent(sessionID, req.AgentName); err != nil {
			log.Printf("[api] add agent %s: %v", req.AgentName, err)
			// Don't fail; continue with existing agents
		}
	}

	if req.Stream {
		s.handleStreamingChat(w, r, ag, msg)
		return
	}

	// Non-streaming: use manager routing (handles @mentions)
	result, respondingAgent, err := s.manager.SendMessage(sessionID, msg, func(chunk string) {
		// accumulate only
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, ChatResponse{
		SessionID:       sessionID,
		Content:         result,
		TurnCount:       ag.TurnCount(),
		ToolCalls:       0,
		RespondingAgent: respondingAgent,
	})
}

// interceptSlashCommand handles workflow slash commands before they reach the agent.
// Returns true if the command was handled (no further processing needed).
func (s *Server) interceptSlashCommand(w http.ResponseWriter, r *http.Request, req *ChatRequest, msg string) bool {
	switch {
	case strings.HasPrefix(msg, "/spec"):
		return s.handleSlashSpec(w, r, req, msg)

	case strings.HasPrefix(msg, "/plan"):
		return s.handleSlashPlan(w, r, req, msg)

	case msg == "/tasks":
		s.handleSlashTasks(w)
		return true

	case strings.HasPrefix(msg, "/implement"):
		return s.handleSlashImplement(w, msg)

	case strings.HasPrefix(msg, "/steer"):
		return s.handleSlashSteer(w, r, req, msg)
	}
	return false
}

func (s *Server) handleSlashSpec(w http.ResponseWriter, r *http.Request, req *ChatRequest, msg string) bool {
	desc := strings.TrimSpace(strings.TrimPrefix(msg, "/spec"))
	if desc == "" {
		writeJSON(w, http.StatusOK, ChatResponse{
			SessionID: req.SessionID,
			Content:   "Usage: /spec <feature description>\nExample: /spec 添加暗色模式支持",
		})
		return true
	}

	slug := workflow.Slugify(desc)
	filename := workflow.SpecFilename(desc)
	worktreePath := filepath.Join("..", slug)

	// Try to create a git worktree for isolated development
	worktreeCreated := false
	absWorktreePath := ""
	if abs, err := filepath.Abs(worktreePath); err == nil {
		absWorktreePath = abs
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "worktree", "add", "-b", slug, worktreePath).CombinedOutput()
	if err == nil {
		worktreeCreated = true
		log.Printf("[spec] worktree created: %s (branch: %s)", worktreePath, slug)
	} else {
		log.Printf("[spec] worktree creation skipped: %v — %s", err, strings.TrimSpace(string(out)))
		worktreePath = "." // fallback to current directory
		absWorktreePath, _ = os.Getwd()
	}

	// Craft prompt: worktree-aware
	var prompt string
	if worktreeCreated {
		prompt = fmt.Sprintf(workflow.SpecPromptTemplateWorktree, desc, absWorktreePath, filename, slug)
	} else {
		prompt = fmt.Sprintf(workflow.SpecPromptTemplate, desc, filename)
	}

	req.Message = prompt
	return false // let normal chat processing handle the prompt
}

func (s *Server) handleSlashPlan(w http.ResponseWriter, r *http.Request, req *ChatRequest, msg string) bool {
	specFile := strings.TrimSpace(strings.TrimPrefix(msg, "/plan"))
	if specFile == "" || !fileExists(specFile) {
		specFile = "SPEC.md"
	}
	if !fileExists(specFile) {
		// Auto-discover: find any SPEC-*.md file
		if found := discoverSpecFile(); found != "" {
			specFile = found
		}
	}

	prompt := fmt.Sprintf(workflow.PlanPromptTemplate, specFile)
	req.Message = prompt
	return false // let normal chat processing handle the prompt
}

// fileExists checks if a file exists on disk.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// discoverSpecFile finds the first SPEC-*.md file in the current directory.
func discoverSpecFile() string {
	entries, err := os.ReadDir(".")
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "SPEC-") && strings.HasSuffix(e.Name(), ".md") {
			return e.Name()
		}
	}
	return ""
}

func (s *Server) handleSlashTasks(w http.ResponseWriter) {
	tl, err := workflow.ParseTasks("PLAN.md")
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{
			"type":    "tasks",
			"content": "No PLAN.md found. Use /plan to generate one.",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"type":  "tasks",
		"tasks": tl.Tasks,
		"text":  workflow.FormatTasksForChat(tl),
	})
}

func (s *Server) handleSlashImplement(w http.ResponseWriter, msg string) bool {
	taskIDStr := strings.TrimSpace(strings.TrimPrefix(msg, "/implement"))
	if taskIDStr == "" {
		writeJSON(w, http.StatusOK, map[string]string{
			"type":    "implement",
			"content": "Usage: /implement <task-number>\nExample: /implement 3",
		})
		return true
	}

	// Parse task number from the message (may have extra text after the number)
	parts := strings.Fields(taskIDStr)
	if len(parts) == 0 {
		writeJSON(w, http.StatusOK, map[string]string{
			"type":    "implement",
			"content": "Usage: /implement <task-number>",
		})
		return true
	}

	taskNum := 0
	fmt.Sscanf(parts[0], "%d", &taskNum)
	if taskNum <= 0 {
		writeJSON(w, http.StatusOK, map[string]string{
			"type":    "implement",
			"content": "Invalid task number: " + parts[0],
		})
		return true
	}

	content, err := workflow.MarkTaskAsDone("PLAN.md", taskNum)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{
			"type":    "implement",
			"content": err.Error(),
		})
		return true
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"type":    "implement",
		"content": fmt.Sprintf("✅ Task %d marked as done: %s", taskNum, content),
	})
	return true
}

// handleSlashSteer implements the unified /steer command — one message that
// drives the agent through spec→plan→tasks→implement in a single turn.
func (s *Server) handleSlashSteer(w http.ResponseWriter, r *http.Request, req *ChatRequest, msg string) bool {
	desc := strings.TrimSpace(strings.TrimPrefix(msg, "/steer"))
	if desc == "" {
		writeJSON(w, http.StatusOK, ChatResponse{
			SessionID: req.SessionID,
			Content:   "Usage: /steer <feature description>\nExample: /steer 添加实时通知系统\n\nThe agent will run through spec → plan → implement in one turn.",
		})
		return true
	}

	// Try to create a git worktree for isolated development
	slug := workflow.Slugify(desc)
	worktreePath := filepath.Join("..", slug)
	worktreeCreated := false
	if out, err := exec.CommandContext(context.Background(), "git", "worktree", "add", "-b", slug, worktreePath).CombinedOutput(); err == nil {
		worktreeCreated = true
		log.Printf("[steer] worktree created: %s (branch: %s)", worktreePath, slug)
		_ = out
	}

	// Build the steer prompt
	prompt := fmt.Sprintf(workflow.SteerPromptTemplate, desc)
	if worktreeCreated {
		prompt = fmt.Sprintf("Working directory: %s\n\n%s", worktreePath, prompt)
	}

	// Replace the message — let normal streaming chat handle execution
	req.Message = prompt
	return false
}

func (s *Server) handleStreamingChat(w http.ResponseWriter, r *http.Request, ag *agent.Agent, message string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	sessionID := ag.SessionID()

	// Send session_id first
	fmt.Fprintf(w, "data: {\"type\":\"session\",\"session_id\":\"%s\"}\n\n", sessionID)
	flusher.Flush()

	result, err := ag.Run(message, func(chunk string) {
		// SSE format
		encoded, _ := json.Marshal(map[string]string{"type": "chunk", "content": chunk})
		fmt.Fprintf(w, "data: %s\n\n", encoded)
		flusher.Flush()
	})

	if err != nil {
		encoded, _ := json.Marshal(map[string]string{"type": "error", "error": err.Error()})
		fmt.Fprintf(w, "data: %s\n\n", encoded)
		flusher.Flush()
		return
	}

	encoded, _ := json.Marshal(map[string]string{"type": "done", "content": result})
	fmt.Fprintf(w, "data: %s\n\n", encoded)
	flusher.Flush()

	// Also broadcast via WebSocket
	s.wsHub.broadcastMsg(wsMessage{
		Type:      "done",
		SessionID: sessionID,
		Content:   result,
	})
}
