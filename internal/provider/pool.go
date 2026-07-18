package provider

import (
	"context"
	"fmt"
	"io"
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
	Weight      int
	Health      HealthState
	Failures    int64
	Successes   int64
	LastFail    time.Time
	LastSuccess time.Time
	Cooldown    time.Duration
	Models      []ModelInfo // auto-discovered + manual override models
	mu          sync.RWMutex

	// Circuit breaker (per SPEC §3.4)
	breaker *CircuitBreaker
}

// IsAvailable returns true if the backend can accept requests.
// Checks both health status and circuit breaker state.
func (e *PoolEntry) IsAvailable() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.Weight <= 0 {
		return false
	}

	// Circuit breaker check (per SPEC §3.4)
	if e.breaker != nil && !e.breaker.Allow() {
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
	// Circuit breaker: record success
	if e.breaker != nil {
		e.breaker.RecordSuccess()
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

	// Circuit breaker: record failure
	if e.breaker != nil {
		e.breaker.RecordFailure()
	}
}

// Status returns a snapshot for API reporting.
func (e *PoolEntry) Status() PoolEntryStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return PoolEntryStatus{
		Label:       e.Label,
		BaseURL:     e.BaseURL,
		Weight:      e.Weight,
		Health:      string(e.Health),
		Failures:    e.Failures,
		Successes:   e.Successes,
		LastFail:    e.LastFail,
		LastSuccess: e.LastSuccess,
		Cooldown:    e.Cooldown.String(),
		Models:      e.Models,
	}
}

// PoolEntryStatus is the JSON-safe snapshot.
type PoolEntryStatus struct {
	Label       string      `json:"label"`
	BaseURL     string      `json:"base_url"`
	Weight      int         `json:"weight"`
	Health      string      `json:"health"`
	Failures    int64       `json:"failures"`
	Successes   int64       `json:"successes"`
	LastFail    time.Time   `json:"last_fail"`
	LastSuccess time.Time   `json:"last_success"`
	Cooldown    string      `json:"cooldown"`
	Models      []ModelInfo `json:"models"`
}

// Pool manages multiple API backends with automatic failover.
// It replaces the need for an external cc-switch proxy.
// Each backend auto-discovers its available models via the /models endpoint
// during health checks, and models are classified by capability (chat, vision, image_gen, etc.).
type Pool struct {
	mu           sync.RWMutex
	entries      []*PoolEntry
	manualModels map[string][]ModelInfo // label → manual model overrides from config
	index        atomic.Int64
	strategy     string
	healthCheck  time.Duration
	stopCh       chan struct{}
	started      bool
}

// NewPool creates a backend pool.
func NewPool(strategy string) *Pool {
	if strategy == "" {
		strategy = "round_robin"
	}
	return &Pool{
		strategy:     strategy,
		healthCheck:  30 * time.Second,
		stopCh:       make(chan struct{}),
		manualModels: make(map[string][]ModelInfo),
	}
}

// Add appends a backend entry with optional manual model overrides.
func (p *Pool) Add(key, label, baseURL string, weight int, models []ModelInfo) {
	if weight <= 0 {
		weight = 1
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	p.manualModels[label] = models

	entry := &PoolEntry{
		Key:      key,
		Label:    label,
		BaseURL:  baseURL,
		Weight:   weight,
		Health:   HealthHealthy,
		Cooldown: 30 * time.Second,
		Models:   models, // start with manual models, auto-discovered models merged later
		breaker:  NewCircuitBreaker(),
	}
	// Apply manual model overrides to breaker's health check
	if len(models) > 0 {
		entry.breaker.SetMaxFailures(5)
	}

	p.entries = append(p.entries, entry)
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
// Also performs an immediate model discovery on all backends.
func (p *Pool) StartHealthCheck() {
	if p.started {
		return
	}
	p.started = true

	// Immediate model discovery
	go func() {
		time.Sleep(500 * time.Millisecond) // brief delay for pool to settle
		p.probeAll()
	}()

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
		// Always probe — update models on every cycle, not just for unhealthy
		if p.probeBackend(e) {
			e.MarkSuccess()
		}
		// Discover models regardless of health (on first probe or periodically)
		if len(e.Models) == 0 || e.Health != HealthUnhealthy {
			p.discoverModels(e)
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

	if resp.StatusCode >= 500 {
		return false
	}

	// Parse model names from response body
	if resp.StatusCode == 200 {
		p.discoverModelsFromResponse(e, resp)
	}

	return true
}

// discoverModelsFromResponse parses /models response and updates PoolEntry.Models.
func (p *Pool) discoverModelsFromResponse(e *PoolEntry, resp *http.Response) {
	// Read body (limit to 1MB)
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return
	}

	names, err := ParseModelsResponse(body)
	if err != nil || len(names) == 0 {
		return
	}

	// Convert to ModelInfo with auto-detection
	autoModels := make([]ModelInfo, 0, len(names))
	for _, name := range names {
		mt, _ := DetectModelType(name)
		autoModels = append(autoModels, ModelInfo{
			Name: name,
			Type: mt,
			Auto: true,
		})
	}

	// Merge with manual overrides (manual wins over auto for same name)
	manual := p.manualModels[e.Label]
	merged := MergeModels(autoModels, manual)

	e.mu.Lock()
	e.Models = merged
	e.mu.Unlock()

	log.Printf("[pool] %s: discovered %d models (%d auto, %d manual)",
		e.Label, len(names), len(autoModels), len(manual))
}

// discoverModels tries to fetch /models to populate auto-discovered models.
func (p *Pool) discoverModels(e *PoolEntry) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", e.BaseURL+"/models", nil)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+e.Key)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return
	}

	p.discoverModelsFromResponse(e, resp)
}

// SelectFor returns the best available backend that supports the given model types.
// Returns nil if no backend matches.
func (p *Pool) SelectFor(types ...ModelType) (*PoolEntry, ModelInfo, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Collect backends that have at least one matching model
	type candidate struct {
		entry *PoolEntry
		model ModelInfo
	}
	var candidates []candidate

	for _, e := range p.entries {
		if !e.IsAvailable() {
			continue
		}
		for _, m := range e.Models {
			for _, t := range types {
				if m.Type == t {
					candidates = append(candidates, candidate{entry: e, model: m})
					goto nextEntry
				}
			}
		}
	nextEntry:
	}

	if len(candidates) == 0 {
		// Fallback: return first available backend regardless of type
		for _, e := range p.entries {
			if e.IsAvailable() {
				if len(e.Models) > 0 {
					return e, e.Models[0], true
				}
				return e, ModelInfo{Name: "unknown", Type: ModelChat, Auto: true}, true
			}
		}
		return nil, ModelInfo{}, false
	}

	// Use pool strategy for selection
	switch p.strategy {
	case "random":
		idx := rand.Intn(len(candidates))
		return candidates[idx].entry, candidates[idx].model, true
	case "fill_first":
		return candidates[0].entry, candidates[0].model, true
	default: // round_robin
		idx := int(p.index.Add(1)-1) % len(candidates)
		return candidates[idx].entry, candidates[idx].model, true
	}
}

// HasCapability checks if any backend supports the given model type.
func (p *Pool) HasCapability(t ModelType) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, e := range p.entries {
		for _, m := range e.Models {
			if m.Type == t {
				return true
			}
		}
	}
	return false
}

// Capabilities returns a summary of which model types are available across all backends.
func (p *Pool) Capabilities() map[ModelType]bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make(map[ModelType]bool)
	for _, t := range AllModelTypes {
		result[t] = false
	}
	for _, e := range p.entries {
		for _, m := range e.Models {
			result[m.Type] = true
		}
	}
	return result
}

// ModelsByCapability returns all backends grouped by their model capabilities.
func (p *Pool) ModelsByCapability() map[ModelType][]PoolEntryStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make(map[ModelType][]PoolEntryStatus)
	for _, t := range AllModelTypes {
		result[t] = []PoolEntryStatus{}
	}
	for _, e := range p.entries {
		status := e.Status()
		hasCap := make(map[ModelType]bool)
		for _, m := range e.Models {
			if !hasCap[m.Type] {
				hasCap[m.Type] = true
				result[m.Type] = append(result[m.Type], status)
			}
		}
	}
	return result
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

// ForceDiscover triggers immediate model discovery on all backends.
func (p *Pool) ForceDiscover() {
	p.mu.RLock()
	entries := make([]*PoolEntry, len(p.entries))
	copy(entries, p.entries)
	p.mu.RUnlock()

	for _, e := range entries {
		p.discoverModels(e)
	}
	log.Printf("[pool] force-discovered models on %d backends", len(entries))
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
		newPool.Add(e.Key, e.Label, e.BaseURL, 1, nil)
	}
	return newPool
}

// Ensure fmt is used (it is, but go compiler needs to see it)
var _ = fmt.Sprintf
