package provider

import (
	"sync"
	"sync/atomic"
	"time"
)

// PoolEntry is one API key with metadata.
type PoolEntry struct {
	Key      string
	Label    string
	Failures int64
	LastFail time.Time
	Cooldown time.Duration
	BaseURL  string
}

// KeyPool manages multiple API keys with automatic failover.
type KeyPool struct {
	mu       sync.RWMutex
	entries  []*PoolEntry
	index    atomic.Int64
	strategy string // "round_robin" or "fill_first"
}

// NewKeyPool creates a key pool.
func NewKeyPool(strategy string) *KeyPool {
	if strategy == "" {
		strategy = "fill_first"
	}
	return &KeyPool{strategy: strategy}
}

// Add appends a key entry.
func (p *KeyPool) Add(key, label, baseURL string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.entries = append(p.entries, &PoolEntry{
		Key:      key,
		Label:    label,
		Cooldown: 5 * time.Minute,
		BaseURL:  baseURL,
	})
}

// Len returns the number of keys.
func (p *KeyPool) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.entries)
}

// Select returns the next available key, skipping cooldown entries.
func (p *KeyPool) Select() (*PoolEntry, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.entries) == 0 {
		return nil, false
	}

	now := time.Now()
	available := make([]*PoolEntry, 0, len(p.entries))
	for _, e := range p.entries {
		if e.Failures > 0 && now.Sub(e.LastFail) < e.Cooldown {
			continue
		}
		available = append(available, e)
	}

	if len(available) == 0 {
		return nil, false
	}

	switch p.strategy {
	case "round_robin":
		idx := int(p.index.Add(1)-1) % len(available)
		return available[idx], true
	default: // fill_first
		return available[0], true
	}
}

// MarkFailure records a failure on a key, triggering cooldown.
func (p *KeyPool) MarkFailure(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, e := range p.entries {
		if e.Key == key {
			e.Failures++
			e.LastFail = time.Now()
			return
		}
	}
}

// MarkSuccess resets failure count.
func (p *KeyPool) MarkSuccess(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, e := range p.entries {
		if e.Key == key {
			e.Failures = 0
			return
		}
	}
}

// Keys returns all labels (for display).
func (p *KeyPool) Keys() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	labels := make([]string, len(p.entries))
	for i, e := range p.entries {
		labels[i] = e.Label
	}
	return labels
}
