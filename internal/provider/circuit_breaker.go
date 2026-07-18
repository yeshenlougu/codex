package provider

import (
	"sync"
	"time"
)

// CircuitState represents the three states of a circuit breaker.
type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"
	CircuitOpen     CircuitState = "open"
	CircuitHalfOpen CircuitState = "half_open"
)

// CircuitBreaker implements a 3-state circuit breaker per SPEC §3.4.
//
// State transitions:
//
//	Closed  ──(5 consecutive failures)──→ Open
//	Open    ──(cooldown 30s elapsed)───→ HalfOpen
//	HalfOpen ──(probe success)─────────→ Closed
//	HalfOpen ──(probe failure)─────────→ Open (reset timer)
type CircuitBreaker struct {
	mu sync.RWMutex

	state        CircuitState
	failCount    int
	lastFailTime time.Time
	cooldown     time.Duration

	// Thresholds
	maxFailures int // consecutive failures before opening (default 5)
}

// NewCircuitBreaker creates a new circuit breaker in Closed state.
func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{
		state:       CircuitClosed,
		cooldown:    30 * time.Second,
		maxFailures: 5,
	}
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Allow returns true if the request should be allowed through.
//   - Closed: always allow
//   - Open: check if cooldown expired → transition to HalfOpen, then allow
//   - HalfOpen: allow (one probe at a time — caller should serialize)
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return true

	case CircuitOpen:
		if time.Since(cb.lastFailTime) >= cb.cooldown {
			cb.state = CircuitHalfOpen
			return true // allow one probe
		}
		return false

	case CircuitHalfOpen:
		return true // allow one probe (caller must ensure single probe)
	}

	return true
}

// RecordSuccess records a successful request.
//   - HalfOpen → Closed (probe passed)
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failCount = 0

	switch cb.state {
	case CircuitHalfOpen:
		cb.state = CircuitClosed
	}
}

// RecordFailure records a failed request.
//   - Closed: increment failCount; if >= maxFailures → Open
//   - HalfOpen: probe failed → Open (reset timer)
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failCount++
	cb.lastFailTime = time.Now()

	switch cb.state {
	case CircuitClosed:
		if cb.failCount >= cb.maxFailures {
			cb.state = CircuitOpen
		}
	case CircuitHalfOpen:
		// Probe failed — go back to Open
		cb.state = CircuitOpen
	}
}

// SetCooldown overrides the default cooldown duration.
func (cb *CircuitBreaker) SetCooldown(d time.Duration) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.cooldown = d
}

// SetMaxFailures overrides the default failure threshold.
func (cb *CircuitBreaker) SetMaxFailures(n int) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.maxFailures = n
}

// Reset forces the circuit back to Closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = CircuitClosed
	cb.failCount = 0
}
