package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/yeshenlougu/codex/internal/agent"
	"github.com/yeshenlougu/codex/internal/store"
)

// ===================== Agent Profile CRUD (SQLite-backed + YAML fallback) =====================

// handleAgents handles GET (list) and POST (create) on /api/agents.
func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		s.listAgents(w, r)
	case "POST":
		s.handleCreateAgent(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "GET, POST supported")
	}
}

// handleAgentByID handles /api/agents/{name} — GET, PUT, DELETE.
// Also handles /api/agents/copy (POST).
func (s *Server) handleAgentByID(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/agents/")
	if name == "" {
		s.handleAgents(w, r)
		return
	}

	// POST /api/agents/copy — special route for agent copy
	if name == "copy" && r.Method == "POST" {
		s.handleCopyAgent(w, r)
		return
	}

	// Sub-paths: /api/agents/{name}/soul, /api/agents/{name}/memory, etc.
	if idx := strings.Index(name, "/"); idx >= 0 {
		subPath := name[idx+1:]
		name = name[:idx]
		switch {
		case subPath == "soul" || subPath == "soul.md":
			if r.Method == "GET" {
				s.handleGetAgentSoul(w, r, name)
			} else if r.Method == "PUT" {
				s.handleUpdateAgentSoul(w, r, name)
			}
			return
		case subPath == "memory":
			s.handleAgentMemory(w, r, name)
			return
		case subPath == "sessions":
			s.handleAgentSessions(w, r, name)
			return
		case subPath == "clone":
			s.handleCloneAgent(w, r, name)
			return
		}
	}

	switch r.Method {
	case "GET":
		s.handleGetAgent(w, r, name)
	case "PUT":
		s.handleUpdateAgent(w, r, name)
	case "DELETE":
		s.handleDeleteAgent(w, r, name)
	default:
		writeError(w, http.StatusMethodNotAllowed, "GET, PUT, DELETE supported")
	}
}

// ── List ──

func (s *Server) listAgents(w http.ResponseWriter, r *http.Request) {
	// Merge SQLite agents + YAML registry agents
	agentMap := make(map[string]interface{})

	// 1. Load from SQLite Store (authoritative for user-created agents)
	if s.store != nil {
		dbAgents, err := s.store.ListAgents()
		if err == nil {
			for _, a := range dbAgents {
				agentMap[a.Name] = map[string]interface{}{
					"name":             a.Name,
					"display_name":     a.DisplayName,
					"provider":         a.Provider,
					"model":            a.Model,
					"system_prompt":    a.SystemPrompt,
					"max_turns":        a.MaxTurns,
					"reasoning_effort": a.ReasoningEffort,
					"tools_mode":       a.ToolsMode,
					"tools_list":       store.ParseJSONList(a.ToolsList),
					"mcp_mode":         a.MCPMode,
					"mcp_list":         store.ParseJSONList(a.MCPList),
					"skills_mode":      a.SkillsMode,
					"skills_list":      store.ParseJSONList(a.SkillsList),
					"session_count":    a.SessionCount,
					"is_builtin":       a.Name == "default",
					"source":           "sqlite",
				}
			}
		}
	}

	// 2. Load from YAML Registry (fallback — profiles that may not be in SQLite yet)
	profiles := s.manager.Registry().List()
	for _, p := range profiles {
		if _, exists := agentMap[p.Name]; !exists {
			agentMap[p.Name] = p
		}
	}

	// Convert map to slice
	agents := make([]interface{}, 0, len(agentMap))
	for _, v := range agentMap {
		agents = append(agents, v)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"agents": agents})
}

// ── Get ──

func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request, name string) {
	// Try SQLite first
	if s.store != nil {
		a, err := s.store.GetAgent(name)
		if err == nil && a != nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"name":             a.Name,
				"display_name":     a.DisplayName,
				"provider":         a.Provider,
				"model":            a.Model,
				"system_prompt":    a.SystemPrompt,
				"max_turns":        a.MaxTurns,
				"reasoning_effort": a.ReasoningEffort,
				"tools_mode":       a.ToolsMode,
				"tools_list":       store.ParseJSONList(a.ToolsList),
				"mcp_mode":         a.MCPMode,
				"mcp_list":         store.ParseJSONList(a.MCPList),
				"skills_mode":      a.SkillsMode,
				"skills_list":      store.ParseJSONList(a.SkillsList),
				"is_builtin":       a.Name == "default",
				"source":           "sqlite",
			})
			return
		}
	}

	// Fallback to YAML registry
	p := s.manager.Registry().Get(name)
	if p == nil {
		writeError(w, http.StatusNotFound, "agent profile not found")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// ── Create ──

func (s *Server) handleCreateAgent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string `json:"name"`
		DisplayName  string `json:"display_name"`
		Provider     string `json:"provider"`
		Model        string `json:"model"`
		SystemPrompt string `json:"system_prompt"`
		CloneFrom    string `json:"clone_from"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}

	// Use Store (SQLite) as primary
	if s.store != nil {
		if req.DisplayName == "" {
			req.DisplayName = req.Name
		}

		if req.CloneFrom != "" {
			// Copy from source agent
			if err := s.store.CopyAgent(req.CloneFrom, req.Name); err != nil {
				writeError(w, http.StatusBadRequest, "copy failed: "+err.Error())
				return
			}
			// Copy file system artifacts
			s.copyAgentFiles(req.CloneFrom, req.Name)
		} else {
			if err := s.store.CreateAgent(req.Name, req.DisplayName, req.Provider, req.Model, req.SystemPrompt); err != nil {
				writeError(w, http.StatusBadRequest, "create failed: "+err.Error())
				return
			}
			// Create agent directory with soul.md
			s.ensureAgentDir(req.Name)
		}

		// Return the newly created agent
		a, _ := s.store.GetAgent(req.Name)
		if a != nil {
			writeJSON(w, http.StatusCreated, map[string]interface{}{
				"name":             a.Name,
				"display_name":     a.DisplayName,
				"provider":         a.Provider,
				"model":            a.Model,
				"system_prompt":    a.SystemPrompt,
				"is_builtin":       false,
				"source":           "sqlite",
			})
			return
		}
	}

	// Fallback to YAML registry
	profile, err := s.manager.Registry().Create(req.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, profile)
}

// ── Update ──

func (s *Server) handleUpdateAgent(w http.ResponseWriter, r *http.Request, name string) {
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	// Try SQLite update
	if s.store != nil {
		existing, err := s.store.GetAgent(name)
		if err == nil && existing != nil {
			displayName := getString(updates, "display_name", existing.DisplayName)
			provider := getString(updates, "provider", existing.Provider)
			model := getString(updates, "model", existing.Model)
			systemPrompt := getString(updates, "system_prompt", existing.SystemPrompt)
			maxTurns := getInt(updates, "max_turns", existing.MaxTurns)

			if err := s.store.UpdateAgent(name, displayName, provider, model, systemPrompt, maxTurns); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"updated": name})
			return
		}
	}

	// Fallback to YAML
	if err := s.manager.Registry().Update(name, &agent.AgentProfile{}); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"updated": name})
}

// ── Delete ──

func (s *Server) handleDeleteAgent(w http.ResponseWriter, r *http.Request, name string) {
	if name == "default" {
		writeError(w, http.StatusBadRequest, "cannot delete the default agent")
		return
	}

	// Try SQLite delete (cascades sessions/messages/memory)
	if s.store != nil {
		if err := s.store.DeleteAgent(name); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		// Also remove file system artifacts
		s.removeAgentDir(name)
		writeJSON(w, http.StatusOK, map[string]string{"deleted": name})
		return
	}

	// Fallback to YAML
	if err := s.manager.Registry().Delete(name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"deleted": name})
}

// ── Copy ──

func (s *Server) handleCopyAgent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Source string `json:"source"`
		Target string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Source == "" || req.Target == "" {
		writeError(w, http.StatusBadRequest, "source and name are required")
		return
	}
	if req.Target == "default" {
		writeError(w, http.StatusBadRequest, "cannot overwrite the default agent")
		return
	}

	if s.store != nil {
		if err := s.store.CopyAgent(req.Source, req.Target); err != nil {
			writeError(w, http.StatusBadRequest, "copy failed: "+err.Error())
			return
		}
		// Copy file system artifacts (soul.md, rules/, skills/)
		s.copyAgentFiles(req.Source, req.Target)

		a, _ := s.store.GetAgent(req.Target)
		if a != nil {
			writeJSON(w, http.StatusCreated, map[string]interface{}{
				"name":         a.Name,
				"display_name": a.DisplayName,
				"source":       "sqlite",
			})
			return
		}
	}

	// Fallback to YAML clone
	profile, err := s.manager.Registry().CloneFrom(req.Source, req.Target)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, profile)
}

func (s *Server) handleCloneAgent(w http.ResponseWriter, r *http.Request, sourceName string) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}

	if s.store != nil {
		if err := s.store.CopyAgent(sourceName, req.Name); err != nil {
			writeError(w, http.StatusBadRequest, "copy failed: "+err.Error())
			return
		}
		s.copyAgentFiles(sourceName, req.Name)
		a, _ := s.store.GetAgent(req.Name)
		if a != nil {
			writeJSON(w, http.StatusCreated, a)
			return
		}
	}

	profile, err := s.manager.Registry().CloneFrom(sourceName, req.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, profile)
}

// ── Soul.md management ──

func (s *Server) handleGetAgentSoul(w http.ResponseWriter, r *http.Request, name string) {
	path := s.agentSoulPath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, map[string]string{"soul": "", "path": path})
			return
		}
		writeError(w, http.StatusInternalServerError, "read soul.md: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"soul": string(data), "path": path})
}

func (s *Server) handleUpdateAgentSoul(w http.ResponseWriter, r *http.Request, name string) {
	var req struct {
		Soul string `json:"soul"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	path := s.agentSoulPath(name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "mkdir: "+err.Error())
		return
	}
	if err := os.WriteFile(path, []byte(req.Soul), 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "write soul.md: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ── Agent Memory ──

func (s *Server) handleAgentMemory(w http.ResponseWriter, r *http.Request, name string) {
	switch r.Method {
	case "GET":
		if s.store == nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{"memory": map[string]string{}})
			return
		}
		mem, err := s.store.ListMemory(name)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if mem == nil {
			mem = map[string]string{}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"memory": mem})

	case "PUT":
		var req struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if s.store == nil {
			writeError(w, http.StatusInternalServerError, "store not initialized")
			return
		}
		if err := s.store.SetMemory(name, req.Key, req.Value); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "GET, PUT supported")
	}
}

// ── Agent Sessions ──

func (s *Server) handleAgentSessions(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != "GET" {
		writeError(w, http.StatusMethodNotAllowed, "GET supported")
		return
	}
	if s.store == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"sessions": []interface{}{}})
		return
	}
	sessions, err := s.store.ListSessions(name, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"sessions": sessions, "agent": name})
}

// ── Helpers ──

func (s *Server) agentDir(name string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex", "agents", name)
}

func (s *Server) agentSoulPath(name string) string {
	return filepath.Join(s.agentDir(name), "soul.md")
}

func (s *Server) ensureAgentDir(name string) {
	dir := s.agentDir(name)
	os.MkdirAll(filepath.Join(dir, "rules"), 0755)
	os.MkdirAll(filepath.Join(dir, "skills"), 0755)
	soulPath := filepath.Join(dir, "soul.md")
	if _, err := os.Stat(soulPath); os.IsNotExist(err) {
		os.WriteFile(soulPath, []byte(fmt.Sprintf("# %s\n\nA custom AI agent.\n", name)), 0644)
	}
}

func (s *Server) copyAgentFiles(source, target string) {
	srcDir := s.agentDir(source)
	dstDir := s.agentDir(target)
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		s.ensureAgentDir(target)
		return
	}

	// Use recursive copy for rules/ and skills/
	os.MkdirAll(dstDir, 0755)
	copyFile(filepath.Join(srcDir, "soul.md"), filepath.Join(dstDir, "soul.md"))
	copyDir(filepath.Join(srcDir, "rules"), filepath.Join(dstDir, "rules"))
	copyDir(filepath.Join(srcDir, "skills"), filepath.Join(dstDir, "skills"))
}

func (s *Server) removeAgentDir(name string) {
	dir := s.agentDir(name)
	os.RemoveAll(dir)
}

func copyFile(src, dst string) {
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return
	}
	os.MkdirAll(filepath.Dir(dst), 0755)
	os.WriteFile(dst, data, 0644)
}

func copyDir(src, dst string) {
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return
	}
	os.MkdirAll(dst, 0755)
	entries, err := os.ReadDir(src)
	if err != nil {
		return
	}
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		if e.IsDir() {
			copyDir(srcPath, dstPath)
		} else {
			copyFile(srcPath, dstPath)
		}
	}
}

func getString(m map[string]interface{}, key, fallback string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return fallback
}

func getInt(m map[string]interface{}, key string, fallback int) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		case int64:
			return int(n)
		}
	}
	return fallback
}

// ===================== Session Agent Management =====================

// handleSessionAgents handles /api/sessions/{id}/agents
//   GET  /api/sessions/{id}/agents        — list agents in session
//   POST /api/sessions/{id}/agents        — add agent (body: {"agent_name":"..."})
//   DELETE /api/sessions/{id}/agents/{name} — remove agent
func (s *Server) handleSessionAgents(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	sessionID := parts[0]

	if len(parts) == 2 && parts[1] == "agents" {
		switch r.Method {
		case "GET":
			agents := s.manager.ListAgents(sessionID)
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"session_id": sessionID,
				"agents":     agents,
			})
		case "POST":
			var req struct {
				AgentName string `json:"agent_name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid JSON")
				return
			}
			if req.AgentName == "" {
				writeError(w, http.StatusBadRequest, "agent_name required")
				return
			}
			ag, err := s.manager.AddAgent(sessionID, req.AgentName)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{
				"session_id": sessionID,
				"agent":      req.AgentName,
				"status":     "joined",
				"session":    ag.SessionID(),
			})
		default:
			writeError(w, http.StatusMethodNotAllowed, "GET, POST supported")
		}
		return
	}

	if len(parts) >= 3 && parts[1] == "agents" {
		agentName := parts[2]
		if r.Method != "DELETE" {
			writeError(w, http.StatusMethodNotAllowed, "DELETE supported")
			return
		}
		if err := s.manager.RemoveAgent(sessionID, agentName); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"session_id": sessionID,
			"agent":      agentName,
			"status":     "removed",
		})
	}
}

// handleSyncAgentsToYAML exports all SQLite agent configs to ~/.codex/agents/*.yaml.
// This bridges the gap between Web UI (SQLite) and CLI mode (YAML).
func (s *Server) handleSyncAgentsToYAML(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	if s.store == nil {
		writeError(w, http.StatusServiceUnavailable, "no SQLite store")
		return
	}

	agents, err := s.store.ListAgents()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list agents: "+err.Error())
		return
	}

	home, err := os.UserHomeDir()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "home dir: "+err.Error())
		return
	}
	agentDir := filepath.Join(home, ".codex", "agents")
	if err := os.MkdirAll(agentDir, 0700); err != nil {
		writeError(w, http.StatusInternalServerError, "mkdir: "+err.Error())
		return
	}

	synced := 0
	for _, a := range agents {
		if a.Name == "default" {
			continue
		}
		profile := agent.BuiltinDefaultProfile().Clone(a.Name)
		profile.Description = a.DisplayName
		if a.Provider != "" {
			profile.Model.Provider = a.Provider
		}
		if a.Model != "" {
			profile.Model.Model = a.Model
		}
		if a.SystemPrompt != "" {
			profile.Agent.SystemPrompt = a.SystemPrompt
		}
		if a.MaxTurns > 0 {
			profile.Agent.MaxTurns = a.MaxTurns
		}
		profile.FilePath = filepath.Join(agentDir, a.Name+".yaml")
		if err := profile.Save(); err != nil {
			log.Printf("[api] sync agent %q → yaml failed: %v", a.Name, err)
			continue
		}
		synced++
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "synced",
		"count":   synced,
		"dir":     agentDir,
		"message": fmt.Sprintf("Synced %d agents to %s/*.yaml — CLI mode can now pick them up on next start.", synced, agentDir),
	})
}
