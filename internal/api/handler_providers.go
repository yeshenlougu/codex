package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/yeshenlougu/codex/internal/config"
	"github.com/yeshenlougu/codex/internal/store"
)

// ── Provider management (SQLite-backed, per PLAN Phase 0.2) ──

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
	if s.store == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"providers": []interface{}{}, "current": ""})
		return
	}

	all, err := s.store.ListProviders()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list providers: "+err.Error())
		return
	}

	// Find current provider
	current := ""
	for _, p := range all {
		if p.IsCurrent {
			current = p.ID
			break
		}
	}

	type providerSummary struct {
		ID              string `json:"id"`
		Name            string `json:"name"`
		Icon            string `json:"icon,omitempty"`
		IconColor       string `json:"icon_color,omitempty"`
		Category        string `json:"category,omitempty"`
		BackendCount    int    `json:"backend_count"`
		HealthyCount    int    `json:"healthy_count"`
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
			BackendCount:    p.BackendCount,
			HealthyCount:    p.HealthyCount,
			InFailoverQueue: p.InFailoverQueue,
			IsCurrent:       p.IsCurrent,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"providers": result,
		"current":   current,
	})
}

func (s *Server) addProvider(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeError(w, http.StatusInternalServerError, "store not initialized")
		return
	}

	var req struct {
		Name       string              `json:"name"`
		Icon       string              `json:"icon"`
		IconColor  string              `json:"icon_color"`
		Category   string              `json:"category"`
		APIFormat  string              `json:"api_format"`
		Backends   []store.BackendInput `json:"backends"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Category == "" {
		req.Category = "third_party"
	}
	if req.APIFormat == "" {
		req.APIFormat = "openai_chat"
	}

	provider, err := s.store.CreateProvider(req.Name, req.Icon, req.IconColor, req.Category, req.APIFormat, req.Backends)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"status":   "ok",
		"provider": provider,
	})
}

func (s *Server) updateProvider(w http.ResponseWriter, r *http.Request, id string) {
	if s.store == nil {
		writeError(w, http.StatusInternalServerError, "store not initialized")
		return
	}

	existing, err := s.store.GetProvider(id)
	if err != nil || existing == nil {
		writeError(w, http.StatusNotFound, "provider not found: "+id)
		return
	}

	var req struct {
		Name       string `json:"name"`
		Icon       string `json:"icon"`
		IconColor  string `json:"icon_color"`
		Category   string `json:"category"`
		Notes      string `json:"notes"`
		APIFormat  string `json:"api_format"`
		InFailover bool   `json:"in_failover_queue"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	// Merge with existing
	name := req.Name
	if name == "" {
		name = existing.Name
	}
	icon := req.Icon
	if icon == "" {
		icon = existing.Icon
	}
	iconColor := req.IconColor
	if iconColor == "" {
		iconColor = existing.IconColor
	}
	category := req.Category
	if category == "" {
		category = existing.Category
	}
	notes := req.Notes
	if notes == "" {
		notes = existing.Notes
	}
	apiFormat := req.APIFormat
	if apiFormat == "" {
		apiFormat = existing.APIFormat
	}

	if err := s.store.UpdateProvider(id, name, icon, iconColor, category, notes, apiFormat, req.InFailover); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) deleteProvider(w http.ResponseWriter, r *http.Request, id string) {
	if s.store == nil {
		writeError(w, http.StatusInternalServerError, "store not initialized")
		return
	}

	if err := s.store.DeleteProvider(id); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) switchProvider(w http.ResponseWriter, r *http.Request, id string) {
	if s.store == nil {
		writeError(w, http.StatusInternalServerError, "store not initialized")
		return
	}

	if err := s.store.SwitchProvider(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Hot-reload: sync current provider's backends to config Pool
	p, err := s.store.GetProvider(id)
	if err == nil && p != nil {
		// Build BackendConfigs from SQLite backends for the pool
		backends, _ := s.store.ListBackends(id)
		bes := make([]config.BackendConfig, 0, len(backends))
		for _, be := range backends {
			bes = append(bes, config.BackendConfig{
				Label:   be.Label,
				Key:     be.APIKey,
				BaseURL: be.BaseURL,
				Weight:  be.Weight,
			})
		}
		if len(bes) > 0 {
			s.cfg.Provider.Backends = bes
		}
		// Set API format from provider if set
		if p.APIFormat != "" {
			s.cfg.Provider.WireAPI = p.APIFormat
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"current": id,
	})
}

func (s *Server) probeProvider(w http.ResponseWriter, r *http.Request, id string) {
	if s.store == nil {
		writeError(w, http.StatusInternalServerError, "store not initialized")
		return
	}

	backends, err := s.store.ListBackends(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list backends: "+err.Error())
		return
	}

	type probeResult struct {
		Label  string `json:"label"`
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	}

	results := make([]probeResult, 0, len(backends))
	for _, be := range backends {
		pr := probeResult{Label: be.Label, Status: "unknown"}
		resp, err := httpClient().Get(strings.TrimRight(be.BaseURL, "/") + "/models")
		if err != nil {
			pr.Status = "unreachable"
			pr.Error = err.Error()
		} else {
			resp.Body.Close()
			if resp.StatusCode < 400 {
				pr.Status = "healthy"
				// Update health in store
				s.store.UpdateBackendHealth(be.ID, "healthy", 0)
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
	if s.store == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"presets": []interface{}{}})
		return
	}

	presets, err := s.store.ListPresets()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list presets: "+err.Error())
		return
	}

	// Convert PresetRow to frontend-compatible shape
	type presetOut struct {
		ID         int    `json:"id"`
		Name       string `json:"name"`
		Category   string `json:"category"`
		Icon       string `json:"icon"`
		IconColor  string `json:"icon_color"`
		WebsiteURL string `json:"website_url"`
		APIKeyURL  string `json:"api_key_url"`
		SortOrder  int    `json:"sort_order"`
	}

	out := make([]presetOut, 0, len(presets))
	for _, p := range presets {
		out = append(out, presetOut{
			ID:         p.ID,
			Name:       p.Name,
			Category:   p.Category,
			Icon:       p.Icon,
			IconColor:  p.IconColor,
			WebsiteURL: p.WebsiteURL,
			APIKeyURL:  p.APIKeyURL,
			SortOrder:  p.SortOrder,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"presets": out})
}

func (s *Server) createFromPreset(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeError(w, http.StatusInternalServerError, "store not initialized")
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

	// Find the preset
	presets, err := s.store.ListPresets()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list presets: "+err.Error())
		return
	}

	var preset *store.PresetRow
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

	// Create provider with one default backend
	provider, err := s.store.CreateProvider(name, preset.Icon, preset.IconColor, preset.Category, "", []store.BackendInput{
		{
			Label:   name,
			APIKey:  req.APIKey,
			BaseURL: preset.WebsiteURL,
			Weight:  10,
		},
	})
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"status":   "ok",
		"provider": provider,
	})
}

// ── Backend sub-resource (under a provider) ──

// handleProviderBackends handles /api/providers/:id/backends and /api/providers/:id/backends/:label
func (s *Server) handleProviderBackends(w http.ResponseWriter, r *http.Request) {
	// Path: /api/providers/{id}/backends[/{label}]
	path := strings.TrimPrefix(r.URL.Path, "/api/providers/")
	parts := strings.SplitN(path, "/", 3)

	if len(parts) < 2 || parts[1] != "backends" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	providerID := parts[0]

	if len(parts) == 2 {
		// /api/providers/:id/backends
		switch r.Method {
		case http.MethodGet:
			s.listProviderBackends(w, r, providerID)
		case http.MethodPost:
			s.addProviderBackend(w, r, providerID)
		default:
			writeError(w, http.StatusMethodNotAllowed, "GET, POST supported")
		}
		return
	}

	// /api/providers/:id/backends/:label
	label := parts[2]
	switch r.Method {
	case http.MethodPut:
		s.updateProviderBackend(w, r, providerID, label)
	case http.MethodDelete:
		s.deleteProviderBackend(w, r, providerID, label)
	default:
		writeError(w, http.StatusMethodNotAllowed, "PUT, DELETE supported")
	}
}

func (s *Server) listProviderBackends(w http.ResponseWriter, r *http.Request, providerID string) {
	if s.store == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"backends": []interface{}{}})
		return
	}

	backends, err := s.store.ListBackends(providerID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list backends: "+err.Error())
		return
	}

	// Enrich with model details
	type backendOut struct {
		store.BackendRow
		ModelDetails []store.ModelInput `json:"models"`
	}

	out := make([]backendOut, 0, len(backends))
	for _, be := range backends {
		// Parse models from the comma-separated string
		var modelDetails []store.ModelInput
		if be.Models != "" {
			for _, name := range strings.Split(be.Models, ", ") {
				modelDetails = append(modelDetails, store.ModelInput{
					Name: strings.TrimSpace(name),
					Type: "chat",
				})
			}
		}
		out = append(out, backendOut{
			BackendRow:   be,
			ModelDetails: modelDetails,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"backends": out,
		"total":    len(out),
	})
}

func (s *Server) addProviderBackend(w http.ResponseWriter, r *http.Request, providerID string) {
	if s.store == nil {
		writeError(w, http.StatusInternalServerError, "store not initialized")
		return
	}

	var req struct {
		Label   string            `json:"label"`
		APIKey  string            `json:"key"`
		BaseURL string            `json:"base_url"`
		Weight  int               `json:"weight"`
		Models  []store.ModelInput `json:"models"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	if req.Label == "" || req.APIKey == "" || req.BaseURL == "" {
		writeError(w, http.StatusBadRequest, "label, key, and base_url are required")
		return
	}
	if req.Weight <= 0 {
		req.Weight = 1
	}

	be, err := s.store.CreateBackend(providerID, req.Label, req.APIKey, req.BaseURL, req.Weight, req.Models)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"status":  "ok",
		"backend": be,
	})
}

func (s *Server) updateProviderBackend(w http.ResponseWriter, r *http.Request, providerID, label string) {
	// For now, delete + re-create since UpdateBackend doesn't support label changes
	// We'll add UpdateBackend to store.go in a follow-up

	if s.store == nil {
		writeError(w, http.StatusInternalServerError, "store not initialized")
		return
	}

	var req struct {
		Label   string            `json:"label"`
		APIKey  string            `json:"key"`
		BaseURL string            `json:"base_url"`
		Weight  int               `json:"weight"`
		Models  []store.ModelInput `json:"models"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	// Find existing backend by label and delete it, then recreate
	backends, err := s.store.ListBackends(providerID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list backends: "+err.Error())
		return
	}

	var existingID int
	for _, be := range backends {
		if be.Label == label {
			existingID = be.ID
			break
		}
	}
	if existingID == 0 {
		writeError(w, http.StatusNotFound, "backend not found: "+label)
		return
	}

	// Delete old, create new
	if err := s.store.DeleteBackend(existingID); err != nil {
		writeError(w, http.StatusInternalServerError, "delete backend: "+err.Error())
		return
	}

	newLabel := req.Label
	if newLabel == "" {
		newLabel = label
	}
	newKey := req.APIKey
	if newKey == "" {
		// Use the old key if not provided (re-read from the old backend)
		for _, be := range backends {
			if be.ID == existingID {
				newKey = be.APIKey
				break
			}
		}
	}
	newURL := req.BaseURL
	if newURL == "" {
		for _, be := range backends {
			if be.ID == existingID {
				newURL = be.BaseURL
				break
			}
		}
	}
	weight := req.Weight
	if weight <= 0 {
		for _, be := range backends {
			if be.ID == existingID {
				weight = be.Weight
				break
			}
		}
	}

	_, err = s.store.CreateBackend(providerID, newLabel, newKey, newURL, weight, req.Models)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create backend: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "label": newLabel})
}

func (s *Server) deleteProviderBackend(w http.ResponseWriter, r *http.Request, providerID, label string) {
	if s.store == nil {
		writeError(w, http.StatusInternalServerError, "store not initialized")
		return
	}

	backends, err := s.store.ListBackends(providerID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list backends: "+err.Error())
		return
	}

	var targetID int
	for _, be := range backends {
		if be.Label == label {
			targetID = be.ID
			break
		}
	}
	if targetID == 0 {
		writeError(w, http.StatusNotFound, "backend not found: "+label)
		return
	}

	if err := s.store.DeleteBackend(targetID); err != nil {
		writeError(w, http.StatusInternalServerError, "delete backend: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "deleted": label})
}

// ── Helpers ──

var _httpClient *http.Client

func httpClient() *http.Client {
	if _httpClient == nil {
		_httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return _httpClient
}
