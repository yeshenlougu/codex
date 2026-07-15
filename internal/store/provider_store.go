package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/yeshenlougu/codex/internal/config"
)

// ProviderStore persists and manages the multi-provider registry.
type ProviderStore struct {
	mu       sync.RWMutex
	path     string
	registry *config.ProviderStorage
}

// NewProviderStore loads or creates a provider registry.
func NewProviderStore(path string) (*ProviderStore, error) {
	ps := &ProviderStore{path: path, registry: &config.ProviderStorage{}}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		// No file yet — try migration from config.yaml
		os.MkdirAll(filepath.Dir(path), 0755)
		if err := ps.migrateFromConfig(); err != nil {
			// Config migration failed (or no config) — start empty
			ps.registry = &config.ProviderStorage{}
		}
		if err := ps.save(); err != nil {
			return nil, fmt.Errorf("provider store init: %w", err)
		}
		return ps, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read providers: %w", err)
	}

	var storage config.ProviderStorage
	if err := json.Unmarshal(data, &storage); err != nil {
		return nil, fmt.Errorf("parse providers: %w", err)
	}
	ps.registry = &storage
	return ps, nil
}

func (ps *ProviderStore) save() error {
	data, err := json.MarshalIndent(ps.registry, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ps.path, data, 0644)
}

// ── CRUD ──

// All returns every provider (read-only snapshot).
func (ps *ProviderStore) All() []config.Provider {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	out := make([]config.Provider, len(ps.registry.Providers))
	copy(out, ps.registry.Providers)
	return out
}

// Get returns a single provider by ID.
func (ps *ProviderStore) Get(id string) (*config.Provider, bool) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	for i := range ps.registry.Providers {
		if ps.registry.Providers[i].ID == id {
			return &ps.registry.Providers[i], true
		}
	}
	return nil, false
}

// CurrentID returns the active provider ID.
func (ps *ProviderStore) CurrentID() string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.registry.Current
}

// Current returns the active provider (or nil).
func (ps *ProviderStore) Current() *config.Provider {
	id := ps.CurrentID()
	if id == "" {
		return nil
	}
	p, _ := ps.Get(id)
	return p
}

// SetCurrent switches the active provider.
func (ps *ProviderStore) SetCurrent(id string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if _, ok := ps.findIdx(id); !ok {
		return fmt.Errorf("provider %s not found", id)
	}
	ps.registry.Current = id
	return ps.save()
}

// Add inserts a new provider.
func (ps *ProviderStore) Add(p config.Provider) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if _, ok := ps.findIdx(p.ID); ok {
		return fmt.Errorf("provider %s already exists", p.ID)
	}
	ps.registry.Providers = append(ps.registry.Providers, p)
	// Auto-select first provider
	if ps.registry.Current == "" {
		ps.registry.Current = p.ID
	}
	return ps.save()
}

// Update modifies an existing provider by ID.
func (ps *ProviderStore) Update(id string, p config.Provider) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	idx, ok := ps.findIdx(id)
	if !ok {
		return fmt.Errorf("provider %s not found", id)
	}
	ps.registry.Providers[idx] = p
	return ps.save()
}

// Delete removes a provider. Fails if it is the current one.
func (ps *ProviderStore) Delete(id string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.registry.Current == id {
		return fmt.Errorf("cannot delete current provider — switch first")
	}
	idx, ok := ps.findIdx(id)
	if !ok {
		return fmt.Errorf("provider %s not found", id)
	}
	ps.registry.Providers = append(ps.registry.Providers[:idx], ps.registry.Providers[idx+1:]...)
	return ps.save()
}

func (ps *ProviderStore) findIdx(id string) (int, bool) {
	for i := range ps.registry.Providers {
		if ps.registry.Providers[i].ID == id {
			return i, true
		}
	}
	return -1, false
}

// ── Backend sub-operations (scoped to a provider) ──

func (ps *ProviderStore) AddBackend(providerID string, be config.BackendConfig) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	idx, ok := ps.findIdx(providerID)
	if !ok {
		return fmt.Errorf("provider %s not found", providerID)
	}
	ps.registry.Providers[idx].Backends = append(ps.registry.Providers[idx].Backends, be)
	return ps.save()
}

func (ps *ProviderStore) UpdateBackend(providerID, label string, be config.BackendConfig) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	idx, ok := ps.findIdx(providerID)
	if !ok {
		return fmt.Errorf("provider %s not found", providerID)
	}
	for j := range ps.registry.Providers[idx].Backends {
		if ps.registry.Providers[idx].Backends[j].Label == label {
			ps.registry.Providers[idx].Backends[j] = be
			return ps.save()
		}
	}
	return fmt.Errorf("backend %s not found in provider %s", label, providerID)
}

func (ps *ProviderStore) DeleteBackend(providerID, label string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	idx, ok := ps.findIdx(providerID)
	if !ok {
		return fmt.Errorf("provider %s not found", providerID)
	}
	bes := ps.registry.Providers[idx].Backends
	for j := range bes {
		if bes[j].Label == label {
			ps.registry.Providers[idx].Backends = append(bes[:j], bes[j+1:]...)
			return ps.save()
		}
	}
	return fmt.Errorf("backend %s not found in provider %s", label, providerID)
}

// ── Migration ──

// migrateFromConfig reads config.yaml and creates a default provider from legacy backends.
func (ps *ProviderStore) migrateFromConfig() error {
	configPath := filepath.Join(filepath.Dir(ps.path), "config.yaml")
	cfg, err := config.Load(configPath)
	if err != nil || len(cfg.Provider.Backends) == 0 {
		return nil // nothing to migrate
	}

	id := "default"
	ps.registry = &config.ProviderStorage{
		Providers: []config.Provider{{
			ID:              id,
			Name:            "Default Provider",
			Category:        "third_party",
			Backends:        cfg.Provider.Backends,
			InFailoverQueue: true,
		}},
		Current: id,
	}
	return nil
}
