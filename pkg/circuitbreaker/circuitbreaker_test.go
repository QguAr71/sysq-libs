package circuitbreaker

import (
	"sync"
	"testing"
	"time"
)

func TestStates(t *testing.T) {
	cb := New(3, 100*time.Millisecond)

	if cb.State() != Closed {
		t.Error("new CircuitBreaker should be Closed")
	}

	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != Closed {
		t.Error("should still be Closed after 2 failures")
	}
	cb.RecordFailure()
	if cb.State() != Open {
		t.Error("should be Open after 3 failures")
	}

	time.Sleep(150 * time.Millisecond)
	cb.BeforeRequest()
	if cb.State() != HalfOpen {
		t.Errorf("should be HalfOpen after resetAfter, got %v", cb.State())
	}

	cb.RecordSuccess()
	if cb.State() != Closed {
		t.Error("should be Closed after success in HalfOpen")
	}
}

func TestHalfOpenCAS(t *testing.T) {
	cb := New(1, 50*time.Millisecond)

	cb.RecordFailure()
	time.Sleep(100 * time.Millisecond)
	cb.BeforeRequest()

	if cb.State() != HalfOpen {
		t.Fatalf("expected HalfOpen, got %v", cb.State())
	}

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if cb.TryHalfOpen() {
				mu.Lock()
				successCount++
				mu.Unlock()
				time.Sleep(10 * time.Millisecond)
				cb.RecordSuccess()
				cb.ReleaseHalfOpen()
			}
		}()
	}
	wg.Wait()

	if successCount != 1 {
		t.Errorf("TryHalfOpen CAS: %d goroutines entered, want exactly 1", successCount)
	}
}

func TestStateTransitions(t *testing.T) {
	cb := New(2, 100*time.Millisecond)

	// Closed → Open
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != Open {
		t.Error("should be Open")
	}

	// Open → HalfOpen
	time.Sleep(150 * time.Millisecond)
	cb.BeforeRequest()
	if cb.State() != HalfOpen {
		t.Error("should be HalfOpen")
	}

	// HalfOpen + failure → Open
	cb.RecordFailure()
	if cb.State() != Open {
		t.Error("should be Open after HalfOpen failure")
	}

	// Open → HalfOpen again
	time.Sleep(150 * time.Millisecond)
	cb.BeforeRequest()
	if cb.State() != HalfOpen {
		t.Error("should be HalfOpen after second reset")
	}

	// HalfOpen + success → Closed
	cb.RecordSuccess()
	if cb.State() != Closed {
		t.Error("should be Closed after HalfOpen success")
	}
}
