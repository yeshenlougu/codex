package provider

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// HealthState tracks a backend's health.
type HealthState string

const (
	HealthUnknown   HealthState = "unknown"
	HealthHealthy   HealthState = "healthy"
	HealthDegraded  HealthState = "degraded"  // some failures but still usable
	HealthUnhealthy HealthState = "unhealthy" // exceeded failure threshold, in cooldown
)

// PoolEntry is one API endpoint with health metadata.
type PoolEntry struct {
	Key         string
	Label       string
	BaseURL     string
	Provider    string
	Weight      int
	Health      HealthState
	Failures    int64
	Successes   int64
	LastFail    time.Time
	LastSuccess time.Time
	Cooldown    time.Duration
	mu          sync.RWMutex
}

// IsAvailable returns true if the backend can accept requests.
func (e *PoolEntry) IsAvailable() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.Weight <= 0 {
		return false
	}
	if e.Health == HealthUnhealthy {
		if time.Since(e.LastFail) > e.Cooldown {
			return true // cooldown expired, will be probed
		}
		return false
	}
	return e.Health != HealthUnknown || e.Failures == 0
}

// MarkSuccess records a successful request.
func (e *PoolEntry) MarkSuccess() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Failures = 0
	e.Successes++
	e.LastSuccess = time.Now()
	if e.Health == HealthUnhealthy || e.Health == HealthDegraded {
		e.Health = HealthHealthy
		log.Printf("[pool] %s recovered → healthy", e.Label)
	}
}

// MarkFailure records a failure. isRetryable=true for rate-limit/5xx, false for 4xx/auth.
func (e *PoolEntry) MarkFailure(isRetryable bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Failures++
	e.LastFail = time.Now()

	if e.Failures >= 3 || !isRetryable {
		e.Health = HealthUnhealthy
		e.Cooldown = time.Duration(minInt(int(e.Failures)*30, 300)) * time.Second
		log.Printf("[pool] %s unhealthy (failures=%d, cooldown=%v)", e.Label, e.Failures, e.Cooldown)
	} else if e.Failures >= 1 {
		e.Health = HealthDegraded
	}
}

// Status returns a snapshot for API reporting.
func (e *PoolEntry) Status() PoolEntryStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return PoolEntryStatus{
		Label:       e.Label,
		Provider:    e.Provider,
		BaseURL:     e.BaseURL,
		Weight:      e.Weight,
		Health:      string(e.Health),
		Failures:    e.Failures,
		Successes:   e.Successes,
		LastFail:    e.LastFail,
		LastSuccess: e.LastSuccess,
		Cooldown:    e.Cooldown.String(),
	}
}

// PoolEntryStatus is the JSON-safe snapshot.
type PoolEntryStatus struct {
	Label       string    `json:"label"`
	Provider    string    `json:"provider"`
	BaseURL     string    `json:"base_url"`
	Weight      int       `json:"weight"`
	Health      string    `json:"health"`
	Failures    int64     `json:"failures"`
	Successes   int64     `json:"successes"`
	LastFail    time.Time `json:"last_fail"`
	LastSuccess time.Time `json:"last_success"`
	Cooldown    string    `json:"cooldown"`
}

// Pool manages multiple API backends with automatic failover.
// It replaces the need for an external cc-switch proxy.
type Pool struct {
	mu          sync.RWMutex
	entries     []*PoolEntry
	index       atomic.Int64
	strategy    string
	healthCheck time.Duration
	stopCh      chan struct{}
	started     bool
}

// NewPool creates a backend pool.
func NewPool(strategy string) *Pool {
	if strategy == "" {
		strategy = "round_robin"
	}
	return &Pool{
		strategy:    strategy,
		healthCheck: 30 * time.Second,
		stopCh:      make(chan struct{}),
	}
}

// Add appends a backend entry.
func (p *Pool) Add(key, label, baseURL, providerType string, weight int) {
	if weight <= 0 {
		weight = 1
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.entries = append(p.entries, &PoolEntry{
		Key:      key,
		Label:    label,
		BaseURL:  baseURL,
		Provider: providerType,
		Weight:   weight,
		Health:   HealthHealthy,
		Cooldown: 30 * time.Second,
	})
}

// Len returns the number of backends.
func (p *Pool) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.entries)
}

// Available returns count of healthy backends.
func (p *Pool) Available() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	count := 0
	for _, e := range p.entries {
		if e.IsAvailable() {
			count++
		}
	}
	return count
}

// Select returns the next available backend based on strategy.
func (p *Pool) Select() (*PoolEntry, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Collect available entries
	available := make([]*PoolEntry, 0, len(p.entries))
	for _, e := range p.entries {
		if e.IsAvailable() {
			available = append(available, e)
		}
	}

	if len(available) == 0 {
		// All backends down — try to find an expired cooldown as last resort
		for _, e := range p.entries {
			if e.Weight > 0 {
				available = append(available, e)
			}
		}
		if len(available) == 0 {
			return nil, false
		}
	}

	switch p.strategy {
	case "random":
		return available[rand.Intn(len(available))], true
	case "fill_first":
		return available[0], true
	default: // round_robin (weighted)
		return p.weightedSelect(available), true
	}
}

func (p *Pool) weightedSelect(available []*PoolEntry) *PoolEntry {
	// Build a weighted flat list
	totalWeight := 0
	for _, e := range available {
		e.mu.RLock()
		totalWeight += e.Weight
		e.mu.RUnlock()
	}

	if totalWeight <= 0 {
		return available[0]
	}

	idx := int(p.index.Add(1)-1) % totalWeight
	cumulative := 0
	for _, e := range available {
		e.mu.RLock()
		w := e.Weight
		e.mu.RUnlock()
		cumulative += w
		if idx < cumulative {
			return e
		}
	}
	return available[len(available)-1]
}

// StartHealthCheck begins periodic probing of unhealthy backends.
func (p *Pool) StartHealthCheck() {
	if p.started {
		return
	}
	p.started = true
	go func() {
		ticker := time.NewTicker(p.healthCheck)
		defer ticker.Stop()
		for {
			select {
			case <-p.stopCh:
				return
			case <-ticker.C:
				p.probeAll()
			}
		}
	}()
	log.Printf("[pool] health checker started (interval=%v)", p.healthCheck)
}

// Stop terminates the health check goroutine.
func (p *Pool) Stop() {
	if p.started {
		close(p.stopCh)
		p.started = false
	}
}

func (p *Pool) probeAll() {
	p.mu.RLock()
	entries := make([]*PoolEntry, len(p.entries))
	copy(entries, p.entries)
	p.mu.RUnlock()

	for _, e := range entries {
		e.mu.RLock()
		isUnhealthy := e.Health == HealthUnhealthy
		e.mu.RUnlock()

		if !isUnhealthy {
			continue
		}

		// Lightweight probe: GET the base URL, expect any non-5xx response
		if p.probeBackend(e) {
			e.MarkSuccess()
		}
	}
}

func (p *Pool) probeBackend(e *PoolEntry) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", e.BaseURL+"/models", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+e.Key)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode < 500
}

// Status returns health snapshots for all backends.
func (p *Pool) Status() []PoolEntryStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	statuses := make([]PoolEntryStatus, len(p.entries))
	for i, e := range p.entries {
		statuses[i] = e.Status()
	}
	return statuses
}

// Entries returns all backend entries (for migration from old pool).
func (p *Pool) Entries() []*PoolEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]*PoolEntry, len(p.entries))
	copy(result, p.entries)
	return result
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ============================================================
// Legacy KeyPool (kept for backward compat — deprecated)
// ============================================================

// KeyPool is the legacy single-endpoint key pool.
// Deprecated: use Pool with BackendConfig instead.
type KeyPool struct {
	mu       sync.RWMutex
	entries  []*PoolEntry
	index    atomic.Int64
	strategy string
}

func NewKeyPool(strategy string) *KeyPool {
	if strategy == "" {
		strategy = "fill_first"
	}
	return &KeyPool{strategy: strategy}
}

func (p *KeyPool) Add(key, label, baseURL string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.entries = append(p.entries, &PoolEntry{
		Key:      key,
		Label:    label,
		Cooldown: 5 * time.Minute,
		BaseURL:  baseURL,
		Health:   HealthHealthy,
		Weight:   1,
	})
}

func (p *KeyPool) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.entries)
}

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
	default:
		return available[0], true
	}
}

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

func (p *KeyPool) Keys() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	labels := make([]string, len(p.entries))
	for i, e := range p.entries {
		labels[i] = e.Label
	}
	return labels
}

// ToPool migrates legacy KeyPool to new Pool for unified use.
func (p *KeyPool) ToPool() *Pool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	newPool := NewPool(p.strategy)
	for _, e := range p.entries {
		newPool.Add(e.Key, e.Label, e.BaseURL, "openai", 1)
	}
	return newPool
}

// Ensure fmt is used (it is, but go compiler needs to see it)
var _ = fmt.Sprintf
