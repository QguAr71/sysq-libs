// Package circuitbreaker — generyczny Circuit Breaker dla systemów rozproszonych.
//
// Wzorzec Circuit Breaker chroni przed wysyłaniem requestów do niedostępnego
// backendu. Trzy stany: Closed (normalny), Open (niedostępny → fallback),
// HalfOpen (testowanie recovery). Thread-safe.
//
// Przykład:
//
//	cb := circuitbreaker.New(5, 30*time.Second)
//	if cb.State() == circuitbreaker.Closed {
//	    resp, err := backend.Call(ctx, req)
//	    if err != nil { cb.RecordFailure() } else { cb.RecordSuccess() }
//	}
//	// W tle:
//	for cb.State() == circuitbreaker.Open {
//	    time.Sleep(5 * time.Second)
//	    cb.BeforeRequest()
//	}
//	if cb.TryHalfOpen() {
//	    defer cb.ReleaseHalfOpen()
//	    // test recovery
//	}
//
// EventBus: N/A.

package circuitbreaker

import (
	"sync"
	"sync/atomic"
	"time"
)

// State — stan circuit breakera.
type State int

const (
	Closed    State = iota // normal operation
	Open                    // unavailable → fallback
	HalfOpen                // testing recovery
)

// CircuitBreaker chroni przed wysyłaniem requestów do niedostępnego backendu.
// maxFailures kolejnych failures → Open → resetAfter → HalfOpen → test → Closed/Open.
type CircuitBreaker struct {
	failures     atomic.Int64
	lastFailure  time.Time
	state        State
	stateMu      sync.RWMutex  // protects state and lastFailure
	halfOpenLock atomic.Bool   // CAS guard — only one goroutine enters HalfOpen

	maxFailures int64
	resetAfter  time.Duration
}

// New tworzy CircuitBreaker w stanie Closed.
func New(maxFailures int64, resetAfter time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:       Closed,
		maxFailures: maxFailures,
		resetAfter:  resetAfter,
	}
}

// State zwraca obecny stan.
func (cb *CircuitBreaker) State() State {
	cb.stateMu.RLock()
	defer cb.stateMu.RUnlock()
	return cb.state
}

// RecordFailure rejestruje failure.
func (cb *CircuitBreaker) RecordFailure() {
	cb.failures.Add(1)
	cb.stateMu.Lock()
	cb.lastFailure = time.Now()
	if cb.failures.Load() >= cb.maxFailures {
		cb.state = Open
	}
	cb.stateMu.Unlock()
}

// RecordSuccess resetuje do Closed.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.failures.Store(0)
	cb.stateMu.Lock()
	cb.state = Closed
	cb.stateMu.Unlock()
}

// BeforeRequest sprawdza czy czas na przejście Open → HalfOpen.
func (cb *CircuitBreaker) BeforeRequest() {
	cb.stateMu.Lock()
	defer cb.stateMu.Unlock()
	if cb.state == Open {
		if time.Since(cb.lastFailure) > cb.resetAfter {
			cb.state = HalfOpen
		}
	}
}

// TryHalfOpen — atomic CAS. Tylko jedna goroutine przechodzi.
func (cb *CircuitBreaker) TryHalfOpen() bool {
	cb.stateMu.RLock()
	st := cb.state
	cb.stateMu.RUnlock()
	if st != HalfOpen {
		return false
	}
	return cb.halfOpenLock.CompareAndSwap(false, true)
}

// ReleaseHalfOpen zwalnia lock po teście recovery.
func (cb *CircuitBreaker) ReleaseHalfOpen() {
	cb.halfOpenLock.Store(false)
}
