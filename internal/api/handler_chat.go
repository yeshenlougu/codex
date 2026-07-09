package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/yeshenlougu/codex/internal/agent"
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
		s.handleStreamingChat(w, r, ag, req.Message)
		return
	}

	// Non-streaming: use manager routing (handles @mentions)
	result, respondingAgent, err := s.manager.SendMessage(sessionID, req.Message, func(chunk string) {
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
