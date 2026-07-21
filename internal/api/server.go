// Package api provides the HTTP + WebSocket server for the Codex agent.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/yeshenlougu/codex/internal/agent"
	"github.com/yeshenlougu/codex/internal/config"
	"github.com/yeshenlougu/codex/internal/mcp"
	"github.com/yeshenlougu/codex/internal/sandbox"
	"github.com/yeshenlougu/codex/internal/schedule"
	"github.com/yeshenlougu/codex/internal/session"
	"github.com/yeshenlougu/codex/internal/skill"
	"github.com/yeshenlougu/codex/internal/store"
	"github.com/yeshenlougu/codex/internal/tool"
)

// Server is the HTTP/WebSocket API server.
type Server struct {
	cfg       *config.Config
	sessStore *session.Store
	manager   *agent.Manager    // multi-agent session manager
	scheduler *schedule.Engine  // cron scheduler
	mu        sync.RWMutex
	httpSrv   *http.Server
	wsHub     *wsHub
	addr      string

	// SQLite-backed data store (§SPEC Phase 0)
	store *store.Store

	// MCP runtime management
	mcpStore    *store.MCPStore
	mcpClients  map[string]*mcp.MCPClient
	mcpMu       sync.Mutex
	mcpRegistry *tool.Registry // shared tool registry for MCP tools

	// Skill management
	skillStore     *store.SkillStore
	skillInstaller *skill.Installer

	// Multi-Provider management (§SPEC-CCSWITCH Phase 1)
	providerStore *store.ProviderStore
}

// New creates a new API server.
func New(cfg *config.Config, dataStore *store.Store, sessStore *session.Store, addr string) *Server {
	s := &Server{
		cfg:         cfg,
		sessStore:   sessStore,
		store:       dataStore,
		addr:        addr,
		wsHub:       newWSHub(),
		mcpClients:  make(map[string]*mcp.MCPClient),
		mcpRegistry: tool.NewRegistry(),
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
	s.manager = agent.NewManager(s.cfg, s.sessStore, agRegistry)
	// Inject SQLite store for agent config loading (per SPEC Phase 0.5)
	s.manager.SetDataStore(&agentStoreAdapter{s.store})
	// Build and inject ProviderRouter for failover (per SPEC §3.5)
	s.buildProviderRouter()
	// Inject shared MCP tool registry into manager for auto-injection into new agents
	s.manager.SetMCPRegistry(s.mcpRegistry)
	log.Printf("[api] agent manager ready — %d profiles loaded", len(agRegistry.List()))

	// Initialize MCP store from persistent file
	mcpStorePath := filepath.Join(expandHome("~/.codex"), "mcp-servers.json")
	mcpStore, mcpErr := store.NewMCPStore(mcpStorePath)
	if mcpErr != nil {
		log.Printf("[api] MCP store init: %v", mcpErr)
	} else {
		s.mcpStore = mcpStore
		// Start all enabled MCP servers
		for _, def := range s.mcpStore.All() {
			if def.Enabled {
				s.startMCPClient(def)
			}
		}
		log.Printf("[api] MCP store ready — %d servers loaded", len(s.mcpStore.All()))
	}

	// Initialize skill store and installer
	skillStorePath := filepath.Join(expandHome("~/.codex"), "skill-store.json")
	skillsDir := filepath.Join(expandHome("~/.codex"), "skills")
	if skillStore, skErr := store.NewSkillStore(skillStorePath); skErr == nil {
		s.skillStore = skillStore
		s.skillInstaller = skill.NewInstaller(skillStore, skillsDir)
		log.Printf("[api] skill store ready — %d installed, %d repos", len(skillStore.Skills()), len(skillStore.Repos()))
		// Auto-index skills into SQLite on startup
		go func() {
			dirs := []string{
				filepath.Join(expandHome("~/.codex"), "skills"),
				filepath.Join(expandHome("~/.claude"), "skills"),
				filepath.Join(expandHome("~/.agents"), "skills"),
			}
			for _, dir := range dirs {
				reg := skill.NewRegistry()
				reg.AddDir(dir)
				if err := reg.LoadAll(); err != nil {
					continue
				}
				for _, sk := range reg.All() {
					tagsJSON := "[]"
					if len(sk.Tags) > 0 {
						b, _ := json.Marshal(sk.Tags)
						tagsJSON = string(b)
					}
					s.store.UpsertSkill(sk.Name, sk.Description, tagsJSON, sk.Path, dir)
				}
			}
			if indexed, _ := s.store.ListIndexedSkills(); len(indexed) > 0 {
				log.Printf("[api] skills indexed — %d in SQLite", len(indexed))
			}
		}()
	} else {
		log.Printf("[api] skill store init: %v", skErr)
	}

	// ── SQLite → Config sync: if providers exist in SQLite, use them as the
	// authoritative source for the Provider Pool.  Otherwise fall back to
	// config.yaml backends, and import them into SQLite if none exist yet.
	s.migrateProvidersToSQLite()
	s.syncProvidersFromSQLite()

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
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
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
	mux.HandleFunc("/api/agents/sync-to-yaml", cors(s.handleSyncAgentsToYAML))
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

	// Multi-Provider management (§SPEC-CCSWITCH Phase 1 — now SQLite-backed)
	mux.HandleFunc("/api/providers", cors(s.handleProviders))

	// Model Aliases (runtime mapping — must be before /api/providers/ prefix)
	mux.HandleFunc("/api/providers/aliases", cors(s.handleModelAliases))
	mux.HandleFunc("/api/providers/aliases/", cors(s.handleModelAliases))
	mux.HandleFunc("/api/providers/", cors(func(w http.ResponseWriter, r *http.Request) {
		// Route /api/providers/:id/backends/:label to handleProviderBackends
		path := strings.TrimPrefix(r.URL.Path, "/api/providers/")
		if strings.Contains(path, "/backends") {
			s.handleProviderBackends(w, r)
			return
		}
		s.handleProviders(w, r)
	}))

	// Model capabilities (auto-discovery)
	mux.HandleFunc("/api/capabilities", cors(s.handleCapabilities))
	mux.HandleFunc("/api/backends/models", cors(s.handleBackendModels))

	// Usage stats (§SPEC Phase 2.4)
	mux.HandleFunc("/api/usage", cors(s.handleUsage))
	mux.HandleFunc("/api/usage/", cors(s.handleUsage))

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
	mux.HandleFunc("/api/skills/", cors(s.handleSkillsExtended))
	mux.HandleFunc("/api/skills", cors(s.handleSkillsExtended))

	// Tools management (§SPEC Phase 3.2)
	mux.HandleFunc("/api/tools", cors(s.handleTools))
	mux.HandleFunc("/api/tools/", cors(s.handleTools))

	// MCP servers (runtime management)
	mux.HandleFunc("/api/mcp/", cors(s.handleMCPServers))
	mux.HandleFunc("/api/mcp", cors(s.handleMCPServers))

	// Terminal
	mux.HandleFunc("/api/terminal", cors(s.handleTerminal))

	// Git review
	mux.HandleFunc("/api/git/status", cors(s.handleGitStatus))
	mux.HandleFunc("/api/git/diff", cors(s.handleGitDiff))

	// WebSocket
	mux.HandleFunc("/ws", s.handleWebSocket)

	// Image Generation — proxy to shenfeng
	mux.HandleFunc("/v1/images/generations", cors(s.handleImageGeneration))

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
	// Close all MCP clients
	s.mcpMu.Lock()
	for id, client := range s.mcpClients {
		client.Close()
		delete(s.mcpClients, id)
	}
	s.mcpMu.Unlock()

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

// handleModelAliases manages model aliases for the current provider.
func (s *Server) handleModelAliases(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeError(w, http.StatusInternalServerError, "store not available")
		return
	}

	// Find current provider
	providers, _ := s.store.ListProviders()
	var providerID string
	for _, prov := range providers {
		if prov.IsCurrent {
			providerID = prov.ID
			break
		}
	}
	if providerID == "" && len(providers) > 0 {
		providerID = providers[0].ID
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/providers/aliases")
	path = strings.TrimPrefix(path, "/")

	switch {
	case r.Method == http.MethodGet && path == "":
		aliases, err := s.store.ListModelAliases(providerID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"aliases": aliases, "provider_id": providerID})

	case r.Method == http.MethodPost && path == "":
		var input struct {
			Alias    string `json:"alias"`
			RealName string `json:"real_name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if input.Alias == "" || input.RealName == "" {
			writeError(w, http.StatusBadRequest, "alias and real_name required")
			return
		}
		if err := s.store.UpsertModelAlias(providerID, input.Alias, input.RealName); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"alias": input.Alias, "real_name": input.RealName})

	case r.Method == http.MethodDelete && path != "":
		var id int
		if _, err := fmt.Sscanf(path, "%d", &id); err != nil {
			writeError(w, http.StatusBadRequest, "invalid alias ID")
			return
		}
		if err := s.store.DeleteModelAlias(id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"deleted": path})

	default:
		writeError(w, http.StatusMethodNotAllowed, "use GET/POST/DELETE")
	}
}

// handleImageGeneration proxies image generation requests to the configured image API.
// Reads credentials from ~/.hermes/config.yaml image_gen section.
func (s *Server) handleImageGeneration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}

	// Read Hermes config for image API credentials
	home, err := os.UserHomeDir()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot find home dir")
		return
	}

	cfgPath := filepath.Join(home, ".hermes", "config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot read image config: "+err.Error())
		return
	}

	var hermesCfg struct {
		ImageGen struct {
			BaseURL string `yaml:"base_url"`
			OpenAI  struct {
				APIKey  string `yaml:"api_key"`
				BaseURL string `yaml:"base_url"`
			} `yaml:"openai"`
		} `yaml:"image_gen"`
	}
	if err := yaml.Unmarshal(data, &hermesCfg); err != nil {
		writeError(w, http.StatusInternalServerError, "parse config: "+err.Error())
		return
	}

	apiKey := hermesCfg.ImageGen.OpenAI.APIKey
	baseURL := hermesCfg.ImageGen.BaseURL
	if baseURL == "" {
		baseURL = hermesCfg.ImageGen.OpenAI.BaseURL
	}
	if apiKey == "" || baseURL == "" {
		writeError(w, http.StatusInternalServerError, "image API not configured (set image_gen in Hermes config)")
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "cannot read body")
		return
	}
	defer r.Body.Close()

	// Forward to image API
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(baseURL, "/")+"/images/generations", io.NopCloser(strings.NewReader(string(body))))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "build request: "+err.Error())
		return
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "image API error: "+err.Error())
		return
	}
	defer resp.Body.Close()

	// Copy response back
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "read response: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	if resp.Header.Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}
