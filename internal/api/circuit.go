package api

import (
	"errors"
	"sync"
	"time"
)

// ErrCircuitOpen is returned when the circuit breaker is open and has not yet
// reached the half-open recovery window.
var ErrCircuitOpen = errors.New("circuit breaker open: nugs.net API is unavailable, backing off")

type circuitState int

const (
	circuitClosed   circuitState = iota // Normal operation — requests flow through.
	circuitOpen                         // API is failing — all requests rejected immediately.
	circuitHalfOpen                     // Recovery probe — one request allowed through.
)

func (s circuitState) String() string {
	switch s {
	case circuitClosed:
		return "closed"
	case circuitOpen:
		return "open"
	case circuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// circuitBreaker is a three-state circuit breaker.
// It trips open after `threshold` consecutive API-level failures (HTTP 429 or 5xx),
// stays open for `resetTimeout`, then enters half-open to probe recovery.
// All methods are safe for concurrent use.
type circuitBreaker struct {
	mu           sync.Mutex
	state        circuitState
	consecutive  int           // consecutive failure count
	threshold    int           // failures required to open the circuit
	resetTimeout time.Duration // how long to stay open before probing
	openedAt     time.Time
}

func newCircuitBreaker(threshold int, resetTimeout time.Duration) *circuitBreaker {
	return &circuitBreaker{
		state:        circuitClosed,
		threshold:    threshold,
		resetTimeout: resetTimeout,
	}
}

// Allow returns the current state and whether the request should proceed.
// A request is blocked only when the circuit is Open and the reset timeout
// has not yet elapsed.
func (cb *circuitBreaker) Allow() (circuitState, bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case circuitClosed:
		return circuitClosed, true
	case circuitOpen:
		if time.Since(cb.openedAt) >= cb.resetTimeout {
			cb.state = circuitHalfOpen
			return circuitHalfOpen, true
		}
		return circuitOpen, false
	case circuitHalfOpen:
		return circuitHalfOpen, true
	}
	return cb.state, false
}

// RecordSuccess records a successful API response.
// Resets the failure counter and closes the circuit.
// Returns the previous state (so the caller can detect a state change).
func (cb *circuitBreaker) RecordSuccess() (prev circuitState) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	prev = cb.state
	cb.consecutive = 0
	cb.state = circuitClosed
	return prev
}

// RecordFailure records an API-level failure (HTTP 429 or 5xx).
// Opens the circuit when the failure threshold is reached.
// Returns the new state (so the caller can detect a state change).
func (cb *circuitBreaker) RecordFailure() (newState circuitState) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.consecutive++
	if cb.state == circuitHalfOpen || cb.consecutive >= cb.threshold {
		cb.state = circuitOpen
		cb.openedAt = time.Now()
	}
	return cb.state
}

// State returns the current circuit state without side effects.
func (cb *circuitBreaker) State() circuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}
