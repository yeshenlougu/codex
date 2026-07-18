package provider

import (
	"log"
	"sync"
)

// ProviderRef is a lightweight reference to a provider with its backends.
type ProviderRef struct {
	ID              string
	Name            string
	InFailoverQueue bool
	Backends        []*PoolEntry
}

// ProviderRouter manages multi-provider failover per SPEC §3.5.
//
// When the current provider has no available backends, the router iterates
// providers with in_failover_queue = true and switches to the first one
// that has healthy backends.
type ProviderRouter struct {
	mu           sync.RWMutex
	providers    []*ProviderRef
	currentIdx   int // index into providers
	currentPool  *Pool
}

// NewProviderRouter creates an empty router.
func NewProviderRouter() *ProviderRouter {
	return &ProviderRouter{}
}

// SetProviders replaces the provider list and optionally sets the current.
func (r *ProviderRouter) SetProviders(providers []*ProviderRef, currentID string, currentPool *Pool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers = providers
	r.currentPool = currentPool

	for i, p := range providers {
		if p.ID == currentID {
			r.currentIdx = i
			break
		}
	}
}

// Current returns the current provider reference.
func (r *ProviderRouter) Current() *ProviderRef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.currentIdx >= 0 && r.currentIdx < len(r.providers) {
		return r.providers[r.currentIdx]
	}
	return nil
}

// SelectBackend tries to select a backend from the current provider.
// If all backends are unavailable, triggers failover to the next provider.
// Returns the selected entry and a boolean indicating if a failover occurred.
func (r *ProviderRouter) SelectBackend(currentPool *Pool) (*PoolEntry, bool) {
	// Try current provider first
	entry, ok := currentPool.Select()
	if ok {
		return entry, false
	}

	// ── Failover: all backends in current provider are down ──
	r.mu.Lock()
	defer r.mu.Unlock()

	// Collect failover candidates in order
	var candidates []*ProviderRef
	for _, p := range r.providers {
		if p.InFailoverQueue && p.ID != r.currentProviderID() {
			candidates = append(candidates, p)
		}
	}

	for _, candidate := range candidates {
		// Build a temporary pool from this provider's backends
		tempPool := NewPool("fill_first")
		for _, be := range candidate.Backends {
			if be.IsAvailable() {
				tempPool.Add(be.Key, be.Label, be.BaseURL, be.Weight, nil)
			}
		}

		if entry, ok := tempPool.Select(); ok {
			// Switch current provider
			for i, p := range r.providers {
				if p.ID == candidate.ID {
					r.currentIdx = i
					break
				}
			}
			log.Printf("[router] FAILOVER: switched to provider %q (%s) — %d healthy backends",
				candidate.Name, candidate.ID, tempPool.Available())
			return entry, true
		}
	}

	log.Printf("[router] FAILOVER FAILED: no available provider out of %d candidates", len(candidates))
	return nil, false
}

func (r *ProviderRouter) currentProviderID() string {
	if r.currentIdx >= 0 && r.currentIdx < len(r.providers) {
		return r.providers[r.currentIdx].ID
	}
	return ""
}

// AvailableProviders returns the count of providers with at least one healthy backend.
func (r *ProviderRouter) AvailableProviders() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, p := range r.providers {
		for _, be := range p.Backends {
			if be.IsAvailable() {
				count++
				break
			}
		}
	}
	return count
}
