package api

import (
	"net/http"

	"github.com/yeshenlougu/codex/internal/provider"
)

// handleCapabilities returns model capability summary across all backends.
func (s *Server) handleCapabilities(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Collect capabilities from the first active agent's pool
	var caps map[provider.ModelType]bool
	var byCap map[provider.ModelType][]provider.PoolEntryStatus

	for _, ag := range s.sessions {
		pool := ag.Pool()
		caps = pool.Capabilities()
		byCap = pool.ModelsByCapability()
		break
	}

	if caps == nil {
		caps = make(map[provider.ModelType]bool)
		for _, t := range provider.AllModelTypes {
			caps[t] = false
		}
	}

	// Build response
	type capInfo struct {
		Type     string                  `json:"type"`
		Label    string                  `json:"label"`
		Icon     string                  `json:"icon"`
		Desc     string                  `json:"desc"`
		Enabled  bool                    `json:"enabled"`
		Backends []provider.PoolEntryStatus `json:"backends,omitempty"`
	}

	result := make([]capInfo, 0, len(provider.AllModelTypes))
	for _, t := range provider.AllModelTypes {
		meta := provider.ModelTypeMetaMap[t]
		ci := capInfo{
			Type:    string(t),
			Label:   meta.Label,
			Icon:    meta.Icon,
			Desc:    meta.Desc,
			Enabled: caps[t],
		}
		if byCap != nil {
			ci.Backends = byCap[t]
		}
		result = append(result, ci)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"capabilities": result,
	})
}

// handleBackendModels returns models for all backends with their auto-detected types.
func (s *Server) handleBackendModels(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ag := range s.sessions {
		pool := ag.Pool()
		statuses := pool.Status()

		// Enrich with model type metadata
		type backendModelInfo struct {
			provider.PoolEntryStatus
			ModelsGrouped map[string][]provider.ModelInfo `json:"models_grouped"`
		}

		result := make([]backendModelInfo, 0, len(statuses))
		for _, st := range statuses {
			grouped := provider.ModelsByType(st.Models)
			// Convert ModelType keys to strings
			strGrouped := make(map[string][]provider.ModelInfo)
			for mt, models := range grouped {
				strGrouped[string(mt)] = models
			}
			result = append(result, backendModelInfo{
				PoolEntryStatus: st,
				ModelsGrouped:   strGrouped,
			})
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"backends": result,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"backends": []interface{}{},
	})
}
