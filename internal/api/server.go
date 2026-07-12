// Package api provides the HTTP + WebSocket server for the Codex agent.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/yeshenlougu/codex/internal/agent"
	"github.com/yeshenlougu/codex/internal/config"
	"github.com/yeshenlougu/codex/internal/sandbox"
	"github.com/yeshenlougu/codex/internal/schedule"
	"github.com/yeshenlougu/codex/internal/session"
)

// Server is the HTTP/WebSocket API server.
type Server struct {
	cfg       *config.Config
	store     *session.Store
	manager   *agent.Manager    // multi-agent session manager
	scheduler *schedule.Engine  // cron scheduler
	mu        sync.RWMutex
	httpSrv   *http.Server
	wsHub     *wsHub
	addr      string
}

// New creates a new API server.
func New(cfg *config.Config, store *session.Store, addr string) *Server {
	s := &Server{
		cfg:   cfg,
		store: store,
		addr:  addr,
		wsHub: newWSHub(),
	}
	return s
}

// Start begins listening and returns immediately.
func (s *Server) Start() error {
	// Initialize agent registry and manager
	agentsDir := s.cfg.Agents.Dir
	if agentsDir == "" {
		agentsDir = expandHome("~/.codex/agents")
	}
	agRegistry := agent.NewRegistry(agentsDir)
	if err := agRegistry.LoadAll(); err != nil {
		log.Printf("[api] agent registry: %v", err)
	}
	s.manager = agent.NewManager(s.cfg, s.store, agRegistry)
	log.Printf("[api] agent manager ready — %d profiles loaded", len(agRegistry.List()))

	// Sandbox approval
	sandbox.OnApprovalRequested = func(check sandbox.Check) {
		data, _ := json.Marshal(check)
		s.wsHub.broadcastMsg(wsMessage{
			Type:    "approval_request",
			Content: string(data),
		})
	}

	// Schedule engine
	schedDir := filepath.Join(expandHome("~/.codex"), "schedules")
	var schedErr error
	s.scheduler, schedErr = schedule.NewEngine(schedDir)
	if schedErr != nil {
		log.Printf("[api] schedule engine: %v", schedErr)
	} else {
		s.scheduler.OnTrigger = func(task schedule.Task) {
			log.Printf("[api] schedule trigger: %s — executing via agent", task.Name)
			go func() {
				sessionID := fmt.Sprintf("sched-%s-%d", task.ID, time.Now().Unix())
				ag, err := s.manager.CreateSession(sessionID)
				if err != nil {
					log.Printf("[api] schedule session error: %v", err)
					return
				}
				result, err := ag.Run(task.Prompt, nil)
				if err != nil {
					log.Printf("[api] schedule run error: %v", err)
					s.scheduler.UpdateLastRun(task.ID, "ERROR: "+err.Error())
				} else {
					log.Printf("[api] schedule run done: %s — %d chars", task.Name, len(result))
					s.scheduler.UpdateLastRun(task.ID, result)
				}
				s.manager.RemoveSession(sessionID)
			}()
		}
		s.scheduler.Start()
		log.Printf("[api] schedule engine ready")
	}

	mux := http.NewServeMux()

	// CORS middleware wrapper
	cors := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			h(w, r)
		}
	}

	// Health
	mux.HandleFunc("/health", cors(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}))

	// Session CRUD
	mux.HandleFunc("/api/sessions", cors(s.handleListSessions))
	mux.HandleFunc("/api/sessions/", cors(s.handleSessionRoute))

	// Agent profiles
	mux.HandleFunc("/api/agents", cors(s.handleAgents))
	mux.HandleFunc("/api/agents/", cors(s.handleAgentByID))

	// Chat (non-streaming)
	mux.HandleFunc("/api/chat", cors(s.handleChat))

	// Config
	mux.HandleFunc("/api/config", cors(s.handleConfig))

	// Release update
	mux.HandleFunc("/api/update", cors(s.handleUpdate))

	// File browser
	mux.HandleFunc("/api/files", cors(s.handleListFiles))
	mux.HandleFunc("/api/files/content", cors(s.handleReadFile))
	mux.HandleFunc("/api/files/diff", cors(s.handleDiff))

	// Pet state
	mux.HandleFunc("/api/pet-state", cors(s.handlePetState))

	// Backend pool (cc-switch replacement)
	mux.HandleFunc("/api/backends", cors(s.handleBackends))
	mux.HandleFunc("/api/backends/", cors(s.handleBackends))

	// Model capabilities (auto-discovery)
	mux.HandleFunc("/api/capabilities", cors(s.handleCapabilities))
	mux.HandleFunc("/api/backends/models", cors(s.handleBackendModels))

	// Workflow tasks (spec/plan/tasks)
	mux.HandleFunc("/api/tasks", cors(s.handleListTasks))
	mux.HandleFunc("/api/implement/", cors(s.handleImplementTask))
	mux.HandleFunc("/api/implement", cors(s.handleImplementTask))

	// Task execution (actual agent-driven implementation)
	mux.HandleFunc("/api/execute/", cors(s.handleExecuteTask))
	mux.HandleFunc("/api/execute", cors(s.handleExecuteTask))

	// Sandbox approval (resolve pending checks)
	mux.HandleFunc("/api/approve/", cors(s.handleApprove))
	mux.HandleFunc("/api/approve", cors(s.handleApprove))

	// Schedules (cron-based agent tasks)
	mux.HandleFunc("/api/schedules", cors(s.handleSchedules))
	mux.HandleFunc("/api/schedules/", cors(s.handleSchedules))

	// Plugins
	mux.HandleFunc("/api/plugins", cors(s.handlePlugins))
	mux.HandleFunc("/api/plugins/", cors(s.handlePlugins))

	// Skills
	mux.HandleFunc("/api/skills", cors(s.handleSkills))

	// Terminal
	mux.HandleFunc("/api/terminal", cors(s.handleTerminal))

	// Git review
	mux.HandleFunc("/api/git/status", cors(s.handleGitStatus))
	mux.HandleFunc("/api/git/diff", cors(s.handleGitDiff))

	// WebSocket
	mux.HandleFunc("/ws", s.handleWebSocket)

	// Static files (frontend — embedded in binary)
	mux.Handle("/", http.FileServer(WebFS()))

	s.httpSrv = &http.Server{
		Addr:         s.addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute, // long for streaming
		IdleTimeout:  120 * time.Second,
	}

	// Start WebSocket hub
	go s.wsHub.run()

	log.Printf("[api] listening on %s", s.addr)
	return s.httpSrv.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	// Close all agents
	if s.manager != nil {
		s.manager.ActiveSessions() // no-op, just ensuring it exists
	}
	return s.httpSrv.Shutdown(ctx)
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// expandHome resolves ~ to the user's home directory.
func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return home + path[1:]
		}
	}
	return path
}

// handleSessionRoute dispatches /api/sessions/{id} and /api/sessions/{id}/agents.
func (s *Server) handleSessionRoute(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) >= 2 && parts[1] == "agents" {
		s.handleSessionAgents(w, r)
		return
	}
	s.handleSession(w, r)
}
