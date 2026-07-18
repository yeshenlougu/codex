package api

import (
	"log"

	"github.com/yeshenlougu/codex/internal/config"
	"github.com/yeshenlougu/codex/internal/store"
)

// migrateProvidersToSQLite imports config.yaml backends into SQLite if the
// providers table is empty but config.yaml has configured backends.
func (s *Server) migrateProvidersToSQLite() {
	if s.store == nil {
		return
	}

	providers, err := s.store.ListProviders()
	if err != nil {
		log.Printf("[api] SQLite provider scan: %v", err)
		return
	}
	if len(providers) > 0 {
		log.Printf("[api] SQLite: %d providers already present — skipping config migration", len(providers))
		return
	}

	// No providers in SQLite — import from config.yaml
	backendCount := len(s.cfg.Provider.Backends)

	// If config has no backends but has a base_url + api_key, use that as a default backend
	if backendCount == 0 && s.cfg.Provider.APIKey != "" && s.cfg.Provider.BaseURL != "" {
		backendCount = 1
	}

	if backendCount == 0 {
		log.Printf("[api] no backends in config.yaml or SQLite — starting fresh")
		return
	}

	// Create a "cc-switch" provider from config
	backends := s.cfg.Provider.Backends
	if len(backends) == 0 {
		backends = []config.BackendConfig{{
			Label:   "default",
			Key:     s.cfg.Provider.APIKey,
			BaseURL: s.cfg.Provider.BaseURL,
			Weight:  10,
		}}
	}

	var beInputs []store.BackendInput
	for _, be := range backends {
		models := make([]store.ModelInput, len(be.Models))
		for i, m := range be.Models {
			models[i] = store.ModelInput{
				Name:          m.Name,
				Type:          m.Type,
				ContextLength: m.ContextLength,
			}
		}
		beInputs = append(beInputs, store.BackendInput{
			Label:   be.Label,
			APIKey:  be.Key,
			BaseURL: be.BaseURL,
			Weight:  be.Weight,
			Models:  models,
		})
	}

	name := "cc-switch"
	category := "partner"
	icon := "ccswitch"
	iconColor := "#6366f1"

	// Use model provider name if it looks like a provider name
	if s.cfg.Model.Provider != "" && s.cfg.Model.Provider != "openai" {
		name = s.cfg.Model.Provider
	}

	provider, err := s.store.CreateProvider(name, icon, iconColor, category, "", beInputs)
	if err != nil {
		log.Printf("[api] SQLite migration failed: %v", err)
		return
	}

	// Set as current
	if err := s.store.SwitchProvider(provider.ID); err != nil {
		log.Printf("[api] switch provider after migration: %v", err)
	}

	log.Printf("[api] SQLite migration: imported %d backends into provider %q (%s)", backendCount, name, provider.ID)
}

// syncProvidersFromSQLite syncs the current SQLite provider's backends into
// the runtime config so the Provider Pool can use them.
func (s *Server) syncProvidersFromSQLite() {
	if s.store == nil {
		return
	}

	providers, err := s.store.ListProviders()
	if err != nil {
		log.Printf("[api] SQLite provider sync list: %v", err)
		return
	}

	// Find current provider
	var currentProvider *store.ProviderRow
	for i := range providers {
		if providers[i].IsCurrent {
			currentProvider = &providers[i]
			break
		}
	}

	if currentProvider == nil {
		log.Printf("[api] SQLite: no current provider set — %d providers available", len(providers))
		return
	}

	// Sync backends to config pool
	backends, err := s.store.ListBackends(currentProvider.ID)
	if err != nil {
		log.Printf("[api] SQLite backend sync: %v", err)
		return
	}

	if len(backends) > 0 {
		bes := make([]config.BackendConfig, 0, len(backends))
		for _, be := range backends {
			bes = append(bes, config.BackendConfig{
				Label:   be.Label,
				Key:     be.APIKey,
				BaseURL: be.BaseURL,
				Weight:  be.Weight,
			})
		}
		s.cfg.Provider.Backends = bes
		log.Printf("[api] SQLite → Pool: %d backends synced from provider %q", len(bes), currentProvider.Name)
	}

	// Sync wire_api if set
	if currentProvider.APIFormat != "" {
		s.cfg.Provider.WireAPI = currentProvider.APIFormat
	}

	// Update model provider name
	s.cfg.Model.Provider = currentProvider.Name
}
