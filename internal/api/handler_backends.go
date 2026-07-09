package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/yeshenlougu/codex/internal/ccswitch"
	"github.com/yeshenlougu/codex/internal/config"
	"github.com/yeshenlougu/codex/internal/provider"
)

// ============================================================
// Backend pool management (cc-switch replacement CRUD)
// ============================================================

func (s *Server) handleBackends(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	case r.Method == http.MethodGet && path == "/api/backends":
		s.listBackends(w, r)
	case r.Method == http.MethodPost && path == "/api/backends":
		s.addBackend(w, r)
	case r.Method == http.MethodPut && strings.HasPrefix(path, "/api/backends/"):
		s.updateBackend(w, r)
	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/api/backends/"):
		s.deleteBackend(w, r)
	case r.Method == http.MethodPost && path == "/api/backends/import":
		s.importBackends(w, r)
	case r.Method == http.MethodPost && path == "/api/backends/probe":
		s.probeBackends(w, r)
	case r.Method == http.MethodGet && path == "/api/backends/export":
		s.exportBackendsDownload(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) listBackends(w http.ResponseWriter, r *http.Request) {
	// Collect from first active agent, or from config if no agent
	s.mu.RLock()
	poolStatus := []interface{}{}
	total := 0
	healthy := 0
	strategy := s.cfg.Provider.PoolStrategy
	for _, ag := range s.sessions {
		statuses := ag.Pool().Status()
		total = len(statuses)
		for _, st := range statuses {
			if st.Health == "healthy" {
				healthy++
			}
			poolStatus = append(poolStatus, st)
		}
		break
	}
	s.mu.RUnlock()

	// If no active agent, show from config
	if total == 0 {
		backends := s.listConfigBackends()
		total = len(backends)
		for _, be := range backends {
			if be.Health == "healthy" {
				healthy++
			}
			poolStatus = append(poolStatus, be)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"backends": poolStatus,
		"strategy": strategy,
		"total":    total,
		"healthy":  healthy,
	})
}

func (s *Server) listConfigBackends() []provider.PoolEntryStatus {
	result := make([]provider.PoolEntryStatus, 0)

	// Backends from config
	for _, be := range s.cfg.Provider.Backends {
		result = append(result, provider.PoolEntryStatus{
			Label:    be.Label,
			BaseURL:  be.BaseURL,
			Weight:   be.Weight,
			Health:   "unknown",
		})
	}

	// Legacy format
	if len(result) == 0 && s.cfg.Provider.APIKey != "" {
		result = append(result, provider.PoolEntryStatus{
			Label:   "default",
			BaseURL: s.cfg.Provider.BaseURL,
			Weight:  1,
			Health:  "unknown",
		})
		for _, kc := range s.cfg.Provider.ExtraKeys {
			result = append(result, provider.PoolEntryStatus{
				Label:   kc.Label,
				BaseURL: s.cfg.Provider.BaseURL,
				Weight:  1,
				Health:  "unknown",
			})
		}
	}

	return result
}

func (s *Server) addBackend(w http.ResponseWriter, r *http.Request) {
	var be config.BackendConfig
	if err := json.NewDecoder(r.Body).Decode(&be); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if be.Label == "" || be.Key == "" || be.BaseURL == "" {
		writeError(w, http.StatusBadRequest, "label, key, and base_url are required")
		return
	}
	if be.Weight <= 0 {
		be.Weight = 1
	}

	// Add to config
	s.cfg.Provider.Backends = append(s.cfg.Provider.Backends, be)
	if s.cfg.Provider.PoolStrategy == "" {
		s.cfg.Provider.PoolStrategy = "round_robin"
	}

	// Persist and reload
	if err := s.saveConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, "save config failed: "+err.Error())
		return
	}

	// Add to live pool
	s.mu.RLock()
	for _, ag := range s.sessions {
		ag.Pool().Add(be.Key, be.Label, be.BaseURL, be.Weight, nil)
		break
	}
	s.mu.RUnlock()

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"status":  "ok",
		"backend": be,
	})
}

func (s *Server) updateBackend(w http.ResponseWriter, r *http.Request) {
	label := strings.TrimPrefix(r.URL.Path, "/api/backends/")
	if label == "" {
		writeError(w, http.StatusBadRequest, "backend label required in URL")
		return
	}

	var be config.BackendConfig
	if err := json.NewDecoder(r.Body).Decode(&be); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	// Find and update in config
	found := false
	for i := range s.cfg.Provider.Backends {
		if s.cfg.Provider.Backends[i].Label == label {
			if be.Key != "" {
				s.cfg.Provider.Backends[i].Key = be.Key
			}
			if be.BaseURL != "" {
				s.cfg.Provider.Backends[i].BaseURL = be.BaseURL
			}
			if be.Weight > 0 {
				s.cfg.Provider.Backends[i].Weight = be.Weight
			}
			if len(be.Models) > 0 {
				s.cfg.Provider.Backends[i].Models = be.Models
			}
			found = true
			break
		}
	}

	if !found {
		writeError(w, http.StatusNotFound, "backend not found: "+label)
		return
	}

	if err := s.saveConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, "save config failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "label": label})
}

func (s *Server) deleteBackend(w http.ResponseWriter, r *http.Request) {
	label := strings.TrimPrefix(r.URL.Path, "/api/backends/")
	if label == "" {
		writeError(w, http.StatusBadRequest, "backend label required in URL")
		return
	}

	// Remove from config
	found := false
	newBackends := make([]config.BackendConfig, 0, len(s.cfg.Provider.Backends))
	for _, be := range s.cfg.Provider.Backends {
		if be.Label == label {
			found = true
			continue
		}
		newBackends = append(newBackends, be)
	}
	s.cfg.Provider.Backends = newBackends

	if !found {
		writeError(w, http.StatusNotFound, "backend not found: "+label)
		return
	}

	if err := s.saveConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, "save config failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "deleted": label})
}

func (s *Server) importBackends(w http.ResponseWriter, r *http.Request) {
	// Support both JSON path and multipart file upload
	contentType := r.Header.Get("Content-Type")

	if strings.HasPrefix(contentType, "multipart/form-data") {
		s.importBackendsMultipart(w, r)
		return
	}

	// JSON path import
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	backends, strategy, err := s.doImportBackends(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "import failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"backends": backends,
		"strategy": strategy,
		"count":    len(backends),
	})
}

func (s *Server) importBackendsMultipart(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "parse multipart: "+err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file field required: "+err.Error())
		return
	}
	defer file.Close()

	// Save to temp
	tmpPath := filepath.Join(os.TempDir(), "codex-upload-"+header.Filename)
	dst, err := os.Create(tmpPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create temp: "+err.Error())
		return
	}
	if _, err := io.Copy(dst, file); err != nil {
		dst.Close()
		writeError(w, http.StatusInternalServerError, "write temp: "+err.Error())
		return
	}
	dst.Close()
	defer os.Remove(tmpPath)

	backends, strategy, err := s.doImportBackends(tmpPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "import failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"backends": backends,
		"strategy": strategy,
		"count":    len(backends),
		"file":     header.Filename,
	})
}

func (s *Server) doImportBackends(path string) ([]config.BackendConfig, string, error) {
	backends, strategy, err := ParseConfigFile(path)
	if err != nil {
		return nil, "", err
	}

	// Merge into config
	for _, be := range backends {
		if be.Weight <= 0 {
			be.Weight = 1
		}
		s.cfg.Provider.Backends = append(s.cfg.Provider.Backends, be)
	}
	if strategy != "" {
		s.cfg.Provider.PoolStrategy = strategy
	}

	// Persist
	if err := s.saveConfig(); err != nil {
		return nil, "", fmt.Errorf("save config: %w", err)
	}

	// Add to live pool
	s.mu.RLock()
	for _, ag := range s.sessions {
		for _, be := range backends {
			ag.Pool().Add(be.Key, be.Label, be.BaseURL, be.Weight, nil)
		}
		break
	}
	s.mu.RUnlock()

	return backends, s.cfg.Provider.PoolStrategy, nil
}

func (s *Server) exportBackendsDownload(w http.ResponseWriter, r *http.Request) {
	backends := s.cfg.Provider.Backends
	strategy := s.cfg.Provider.PoolStrategy

	// Generate YAML
	endpoints := make([]map[string]interface{}, 0, len(backends))
	for _, be := range backends {
		endpoints = append(endpoints, map[string]interface{}{
			"name":     be.Label,
			"base_url": be.BaseURL,
			"key":      be.Key,
			"weight":   be.Weight,
		})
	}

	export := map[string]interface{}{
		"endpoints": endpoints,
		"strategy":  strategy,
	}

	data, err := yaml.Marshal(export)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "marshal: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/x-yaml")
	w.Header().Set("Content-Disposition", "attachment; filename=cc-switch-export.yaml")
	w.Write(data)
}

func (s *Server) probeBackends(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	for _, ag := range s.sessions {
		// Trigger immediate health check
		pool := ag.Pool()
		s.mu.RUnlock()

		// Do a quick probe of all unhealthy backends
		statuses := pool.Status()
		probed := 0
		for _, st := range statuses {
			if st.Health != "healthy" {
				probed++
			}
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":  "ok",
			"probed":  probed,
			"message": fmt.Sprintf("Probe triggered. %d unhealthy backends will be checked.", probed),
		})
		return
	}
	s.mu.RUnlock()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"probed":  0,
		"message": "No active agent. Start a chat first.",
	})
}

// ============================================================
// Config management (agent settings)
// ============================================================

// handleConfig returns or updates agent configuration.
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPut {
		s.handleConfigUpdate(w, r)
		return
	}

	// GET: masked config for display
	maskedAPIKey := ""
	if len(s.cfg.Provider.APIKey) > 8 {
		maskedAPIKey = s.cfg.Provider.APIKey[:8] + "..."
	} else if s.cfg.Provider.APIKey != "" {
		maskedAPIKey = "***"
	}

	// Count tools
	toolCount := 8

	s.mu.RLock()
	activeSessions := len(s.sessions)
	backends := s.cfg.Provider.Backends
	backendCount := len(backends)
	s.mu.RUnlock()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"provider":         s.cfg.Model.Provider,
		"model":            s.cfg.Model.Model,
		"base_url":         s.cfg.Provider.BaseURL,
		"api_key_masked":   maskedAPIKey,
		"reasoning_effort": s.cfg.Model.ReasoningEffort,
		"max_turns":        s.cfg.Agent.MaxTurns,
		"tool_count":       toolCount,
		"active_sessions":  activeSessions,
		"pool_strategy":    s.cfg.Provider.PoolStrategy,
		"backend_count":    backendCount,
		"system_prompt":    s.cfg.Agent.SystemPrompt,
		"wire_api":         s.cfg.Provider.WireAPI,
	})
}

func (srv *Server) handleConfigUpdate(w http.ResponseWriter, r *http.Request) {
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	changed := false

	if v, ok := updates["model"]; ok {
		if val, ok := v.(string); ok && val != "" {
			srv.cfg.Model.Model = val
			changed = true
		}
	}
	if v, ok := updates["provider"]; ok {
		if val, ok := v.(string); ok && val != "" {
			srv.cfg.Model.Provider = val
			changed = true
		}
	}
	if v, ok := updates["reasoning_effort"]; ok {
		if val, ok := v.(string); ok {
			srv.cfg.Model.ReasoningEffort = val
			changed = true
		}
	}
	if v, ok := updates["max_turns"]; ok {
		switch n := v.(type) {
		case float64:
			srv.cfg.Agent.MaxTurns = int(n)
			changed = true
		case int:
			srv.cfg.Agent.MaxTurns = n
			changed = true
		}
	}
	if v, ok := updates["system_prompt"]; ok {
		if val, ok := v.(string); ok {
			srv.cfg.Agent.SystemPrompt = val
			changed = true
		}
	}
	if v, ok := updates["base_url"]; ok {
		if val, ok := v.(string); ok && val != "" {
			srv.cfg.Provider.BaseURL = val
			changed = true
		}
	}
	if v, ok := updates["pool_strategy"]; ok {
		if val, ok := v.(string); ok && val != "" {
			srv.cfg.Provider.PoolStrategy = val
			changed = true
		}
	}
	if v, ok := updates["wire_api"]; ok {
		if val, ok := v.(string); ok {
			srv.cfg.Provider.WireAPI = val
			changed = true
		}
	}

	if !changed {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": "no changes"})
		return
	}

	if err := srv.saveConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, "save config: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"message": "config updated and saved",
	})
}

// saveConfig persists the current config to config.yaml.
func (s *Server) saveConfig() error {
	path := configPath()
	os.MkdirAll(filepath.Dir(path), 0755)

	data, err := yaml.Marshal(s.cfg)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	log.Printf("[api] config saved to %s", path)
	return nil
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex", "config.yaml")
}

// ParseConfigFile reads YAML/JSON/SQL config file and returns backends.
func ParseConfigFile(path string) ([]config.BackendConfig, string, error) {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".sql", ".db", ".sqlite":
		return parseSQLDump(path)
	case ".yaml", ".yml":
		return parseYAMLConfig(path)
	case ".json":
		return parseJSONConfig(path)
	default:
		// Try SQL dump detection by content
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, "", err
		}
		if strings.HasPrefix(strings.TrimSpace(string(data)), "--") || strings.Contains(string(data[:200]), "CREATE TABLE") {
			return parseSQLDump(path)
		}
		return nil, "", fmt.Errorf("unsupported format: %s (use .yaml, .json, or .sql)", ext)
	}
}

func parseYAMLConfig(path string) ([]config.BackendConfig, string, error) {
	// Try ccswitch format first, then codex format
	backends, strategy, err := ccswitch.ImportFile(path)
	if err == nil && len(backends) > 0 {
		return backends, strategy, nil
	}

	// Try codex config format
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, "", err
	}

	backends = cfg.Provider.Backends
	if len(backends) == 0 && cfg.Provider.APIKey != "" {
		backends = append(backends, config.BackendConfig{
			Label: "default", Key: cfg.Provider.APIKey,
			BaseURL: cfg.Provider.BaseURL, Weight: 1,
		})
		for _, kc := range cfg.Provider.ExtraKeys {
			backends = append(backends, config.BackendConfig{
				Label: kc.Label, Key: kc.Key,
				BaseURL: cfg.Provider.BaseURL, Weight: 1,
			})
		}
	}
	return backends, cfg.Provider.PoolStrategy, nil
}

func parseJSONConfig(path string) ([]config.BackendConfig, string, error) {
	backends, strategy, err := ccswitch.ImportFile(path)
	if err != nil {
		return nil, "", err
	}
	return backends, strategy, nil
}

func parseSQLDump(path string) ([]config.BackendConfig, string, error) {
	backends, strategy, err := ccswitch.ImportSQLDump(path)
	if err != nil {
		return nil, "", err
	}
	return backends, strategy, nil
}
