package api

import (
	"encoding/json"
	"net/http"

	"github.com/yeshenlougu/codex/internal/ccswitch"
	"github.com/yeshenlougu/codex/internal/config"
)

// handleBackends returns pool health status (cc-switch replacement dashboard).
func (s *Server) handleBackends(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		s.listBackends(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/api/backends/import" {
		s.importCCSwitch(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/api/backends/export" {
		s.exportCCSwitch(w, r)
		return
	}
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func (s *Server) listBackends(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	poolStatus := []interface{}{}
	total := 0
	healthy := 0
	for _, ag := range s.sessions {
		statuses := ag.Pool().Status()
		total = len(statuses)
		for _, st := range statuses {
			if st.Health == "healthy" {
				healthy++
			}
			poolStatus = append(poolStatus, st)
		}
		break // first agent's pool is representative
	}
	s.mu.RUnlock()

	if total == 0 {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"backends": []interface{}{},
			"strategy": "none",
			"total":    0,
			"healthy":  0,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"backends": poolStatus,
		"strategy": s.cfg.Provider.PoolStrategy,
		"total":    total,
		"healthy":  healthy,
	})
}

func (s *Server) importCCSwitch(w http.ResponseWriter, r *http.Request) {
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

	backends, strategy, err := ccswitch.ImportFile(req.Path)
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

func (s *Server) exportCCSwitch(w http.ResponseWriter, r *http.Request) {
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

	backends := s.cfg.Provider.Backends
	strategy := s.cfg.Provider.PoolStrategy

	// If no backends configured, generate from legacy config
	if len(backends) == 0 && s.cfg.Provider.APIKey != "" {
		backends = append(backends, config.BackendConfig{
			Label:   "default",
			Key:     s.cfg.Provider.APIKey,
			BaseURL: s.cfg.Provider.BaseURL,
			Weight:  1,
		})
		for _, kc := range s.cfg.Provider.ExtraKeys {
			backends = append(backends, config.BackendConfig{
				Label:   kc.Label,
				Key:     kc.Key,
				BaseURL: s.cfg.Provider.BaseURL,
				Weight:  1,
			})
		}
	}

	if err := ccswitch.ExportToCCSwitch(backends, strategy, req.Path); err != nil {
		writeError(w, http.StatusInternalServerError, "export failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"path":   req.Path,
		"count":  len(backends),
	})
}
