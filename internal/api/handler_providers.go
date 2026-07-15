package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/yeshenlougu/codex/internal/config"
)

// ── Provider management (cc-switch aligned multi-provider CRUD) ──

func (s *Server) handleProviders(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/providers")
	path = strings.TrimPrefix(path, "/")

	switch {
	case r.Method == http.MethodGet && path == "":
		s.listProviders(w, r)
	case r.Method == http.MethodPost && path == "":
		s.addProvider(w, r)
	case r.Method == http.MethodPost && path == "from-preset":
		s.createFromPreset(w, r)
	case r.Method == http.MethodGet && path == "presets":
		s.listPresets(w, r)
	case strings.Count(path, "/") == 1:
		id := strings.TrimSuffix(path, "/")
		switch {
		case r.Method == http.MethodPut:
			s.updateProvider(w, r, id)
		case r.Method == http.MethodDelete:
			s.deleteProvider(w, r, id)
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/switch"):
			id = strings.TrimSuffix(path, "/switch")
			s.switchProvider(w, r, id)
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/probe"):
			id = strings.TrimSuffix(path, "/probe")
			s.probeProvider(w, r, id)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (s *Server) listProviders(w http.ResponseWriter, r *http.Request) {
	if s.providerStore == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"providers": []interface{}{}, "current": ""})
		return
	}

	all := s.providerStore.All()
	current := s.providerStore.CurrentID()

	type providerSummary struct {
		ID              string `json:"id"`
		Name            string `json:"name"`
		Icon            string `json:"icon,omitempty"`
		IconColor       string `json:"icon_color,omitempty"`
		Category        string `json:"category,omitempty"`
		BackendCount    int    `json:"backend_count"`
		InFailoverQueue bool   `json:"in_failover_queue"`
		IsCurrent       bool   `json:"is_current"`
	}

	result := make([]providerSummary, 0, len(all))
	for _, p := range all {
		result = append(result, providerSummary{
			ID:              p.ID,
			Name:            p.Name,
			Icon:            p.Icon,
			IconColor:       p.IconColor,
			Category:        p.Category,
			BackendCount:    len(p.Backends),
			InFailoverQueue: p.InFailoverQueue,
			IsCurrent:       p.ID == current,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"providers": result,
		"current":   current,
	})
}

func (s *Server) addProvider(w http.ResponseWriter, r *http.Request) {
	if s.providerStore == nil {
		writeError(w, http.StatusInternalServerError, "provider store not initialized")
		return
	}

	var p config.Provider
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	if p.ID == "" {
		p.ID = generateID()
	}
	if p.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if p.Category == "" {
		p.Category = "third_party"
	}
	p.CreatedAt = time.Now().Unix()

	if err := s.providerStore.Add(p); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{"status": "ok", "provider": p})
}

func (s *Server) updateProvider(w http.ResponseWriter, r *http.Request, id string) {
	if s.providerStore == nil {
		writeError(w, http.StatusInternalServerError, "provider store not initialized")
		return
	}

	existing, ok := s.providerStore.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "provider not found: "+id)
		return
	}

	var updates config.Provider
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	// Merge: only overwrite non-zero fields
	if updates.Name != "" {
		existing.Name = updates.Name
	}
	if updates.Icon != "" {
		existing.Icon = updates.Icon
	}
	if updates.IconColor != "" {
		existing.IconColor = updates.IconColor
	}
	if updates.Category != "" {
		existing.Category = updates.Category
	}
	if updates.Notes != "" {
		existing.Notes = updates.Notes
	}
	existing.InFailoverQueue = updates.InFailoverQueue
	if updates.Meta != (config.ProviderMeta{}) {
		existing.Meta = updates.Meta
	}
	if len(updates.Backends) > 0 {
		existing.Backends = updates.Backends
	}

	if err := s.providerStore.Update(id, *existing); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) deleteProvider(w http.ResponseWriter, r *http.Request, id string) {
	if s.providerStore == nil {
		writeError(w, http.StatusInternalServerError, "provider store not initialized")
		return
	}

	if err := s.providerStore.Delete(id); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) switchProvider(w http.ResponseWriter, r *http.Request, id string) {
	if s.providerStore == nil {
		writeError(w, http.StatusInternalServerError, "provider store not initialized")
		return
	}

	if err := s.providerStore.SetCurrent(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Hot-reload: update the global ProviderConfig from the new provider
	p, _ := s.providerStore.Get(id)
	if p != nil {
		s.cfg.Provider.Backends = p.Backends
		s.cfg.Provider.PoolStrategy = "round_robin" // default
		if p.InFailoverQueue {
			s.cfg.Provider.PoolStrategy = "fill_first"
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"current":   id,
		"backends":  len(p.Backends),
	})
}

func (s *Server) probeProvider(w http.ResponseWriter, r *http.Request, id string) {
	// Lightweight probe: check if the provider's backends are reachable
	p, ok := s.providerStore.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "provider not found: "+id)
		return
	}

	type probeResult struct {
		Label  string `json:"label"`
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	}

	results := make([]probeResult, 0, len(p.Backends))
	for _, be := range p.Backends {
		pr := probeResult{Label: be.Label, Status: "unknown"}
		resp, err := httpClient().Get(strings.TrimRight(be.BaseURL, "/") + "/models")
		if err != nil {
			pr.Status = "unreachable"
			pr.Error = err.Error()
		} else {
			resp.Body.Close()
			if resp.StatusCode < 400 {
				pr.Status = "healthy"
			} else {
				pr.Status = "degraded"
				pr.Error = "HTTP " + resp.Status
			}
		}
		results = append(results, pr)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"provider": id,
		"results":  results,
		"total":    len(results),
	})
}

// ── Presets ──

func (s *Server) listPresets(w http.ResponseWriter, r *http.Request) {
	presets := getBuiltinPresets()
	writeJSON(w, http.StatusOK, map[string]interface{}{"presets": presets})
}

func (s *Server) createFromPreset(w http.ResponseWriter, r *http.Request) {
	if s.providerStore == nil {
		writeError(w, http.StatusInternalServerError, "provider store not initialized")
		return
	}

	var req struct {
		PresetName string `json:"preset_name"`
		Name       string `json:"name"`
		APIKey     string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	presets := getBuiltinPresets()
	var preset *config.ProviderPreset
	for i := range presets {
		if presets[i].Name == req.PresetName {
			preset = &presets[i]
			break
		}
	}
	if preset == nil {
		writeError(w, http.StatusNotFound, "preset not found: "+req.PresetName)
		return
	}

	name := req.Name
	if name == "" {
		name = preset.Name
	}

	p := config.Provider{
		ID:              generateID(),
		Name:            name,
		Icon:            preset.Icon,
		IconColor:       preset.IconColor,
		Category:        preset.Category,
		InFailoverQueue: preset.Category == "official",
		CreatedAt:       time.Now().Unix(),
		Backends: []config.BackendConfig{{
			Label:   name,
			Key:     req.APIKey,
			BaseURL: preset.BaseURL,
			Weight:  10,
		}},
	}

	if err := s.providerStore.Add(p); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{"status": "ok", "provider": p})
}

func getBuiltinPresets() []config.ProviderPreset {
	return []config.ProviderPreset{
		{Name: "OpenAI", Category: "official", Icon: "openai", IconColor: "#00A67E", BaseURL: "https://api.openai.com/v1", DefaultModel: "gpt-4o", WireAPI: "chat_completions", WebsiteURL: "https://platform.openai.com", APIKeyURL: "https://platform.openai.com/api-keys"},
		{Name: "Anthropic", Category: "official", Icon: "anthropic", IconColor: "#D97757", BaseURL: "https://api.anthropic.com/v1", DefaultModel: "claude-sonnet-4-6", WireAPI: "chat_completions", WebsiteURL: "https://console.anthropic.com"},
		{Name: "DeepSeek", Category: "official", Icon: "deepseek", IconColor: "#4D6BFE", BaseURL: "https://api.deepseek.com/v1", DefaultModel: "deepseek-v4-pro", WireAPI: "chat_completions", WebsiteURL: "https://platform.deepseek.com"},
		{Name: "Beecode", Category: "partner", Icon: "api", IconColor: "#FF6B35", BaseURL: "https://beecode.cc/v1", DefaultModel: "gpt-5.5", WireAPI: "chat_completions", WebsiteURL: "https://beecode.cc"},
		{Name: "OpenCode Go", Category: "partner", Icon: "api", IconColor: "#5e6ad2", BaseURL: "https://opencode.ai/zen/go/v1", DefaultModel: "deepseek-v4-pro", WireAPI: "chat_completions", WebsiteURL: "https://opencode.ai"},
		{Name: "Google Gemini", Category: "official", Icon: "gemini", IconColor: "#4285F4", BaseURL: "https://generativelanguage.googleapis.com/v1beta", DefaultModel: "gemini-2.5-flash", WireAPI: "chat_completions", WebsiteURL: "https://aistudio.google.com"},
		{Name: "Ollama (Local)", Category: "third_party", Icon: "ollama", IconColor: "#000000", BaseURL: "http://localhost:11434/v1", DefaultModel: "llama3.1", WireAPI: "chat_completions"},
		{Name: "Groq", Category: "third_party", Icon: "api", IconColor: "#F55036", BaseURL: "https://api.groq.com/openai/v1", DefaultModel: "llama-3.3-70b", WireAPI: "chat_completions", WebsiteURL: "https://console.groq.com"},
	}
}

func generateID() string {
	id := make([]byte, 8)
	const hex = "abcdef0123456789"
	now := time.Now().UnixNano()
	for i := range id {
		id[i] = hex[now%16]
		now /= 16
	}
	return string(id)
}

var _httpClient *http.Client

func httpClient() *http.Client {
	if _httpClient == nil {
		_httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return _httpClient
}
