// Package retrymanager — singleflight retry dla CircuitBreaker.
//
// Wzorzec RetryManager: tylko jedna goroutine na backend wykonuje retry.
// Zapobiega thundering herd przy masowej awarii backendu.
// Używa health-check probe zamiast powtarzania żądania użytkownika.
//
// EventBus: N/A.

package retrymanager

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/QguAr71/sysq-libs/pkg/circuitbreaker"
)

// Retryable — interfejs dla backendu który można retry'ować.
type Retryable interface {
	// Name zwraca nazwę backendu (np. "pro", "flash").
	Name() string
	// Ping wysyła health-check probe (lekki payload).
	Ping(ctx context.Context) error
}

// RetryManager zarządza retry dla niedostępnych backendów.
// Singleflight — jedna goroutine na backend.
type RetryManager struct {
	mu      sync.Mutex
	pending map[string]*retryJob
}

type retryJob struct {
	done chan struct{}
}

// New tworzy RetryManager.
func New() *RetryManager {
	return &RetryManager{
		pending: make(map[string]*retryJob),
	}
}

// Retry uruchamia retry dla niedostępnego backendu.
// Jeśli retry już trwa — czeka na jego zakończenie.
// Probe używa lekkiego health-check, nie oryginalnego requestu.
func (rm *RetryManager) Retry(ctx context.Context, backend Retryable, cb *circuitbreaker.CircuitBreaker) {
	rm.mu.Lock()
	if job, ok := rm.pending[backend.Name()]; ok {
		rm.mu.Unlock()
		<-job.done
		return
	}
	job := &retryJob{done: make(chan struct{})}
	rm.pending[backend.Name()] = job
	rm.mu.Unlock()

	defer func() {
		close(job.done)
		rm.mu.Lock()
		delete(rm.pending, backend.Name())
		rm.mu.Unlock()
	}()

	checkTimeout := 45 * time.Second
	checkCtx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		cb.BeforeRequest()

		if cb.TryHalfOpen() {
			err := backend.Ping(checkCtx)
			if err == nil {
				cb.RecordSuccess()
				log.Printf("[retrymanager] %s recovered — circuit breaker closed", backend.Name())
				cb.ReleaseHalfOpen()
				return
			}
			cb.RecordFailure()
			cb.ReleaseHalfOpen()
		}

		if cb.State() == circuitbreaker.Closed {
			return
		}

		select {
		case <-checkCtx.Done():
			return
		case <-ticker.C:
		}
	}
}
