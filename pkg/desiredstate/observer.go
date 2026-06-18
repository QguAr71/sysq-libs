package desiredstate

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// ObserverFunc queries a single field's value from the live system.
// Returns (value, nil) or (nil, error). Called concurrently by Observer.
type ObserverFunc func(field Field) (interface{}, error)

// Observer runs parallel observation of all reconciled fields.
// observerFn is called for each field. Returns an ObservedSnapshot
// with individual per-field timestamps and UNKNOWN fallbacks (D6).
//
// Wall-clock time = max(field latency) + goroutine dispatch overhead (D8).
// Each goroutine has a 5s timeout.
type Observer struct {
	observerFn ObserverFunc
	timeout    time.Duration
}

// NewObserver creates an Observer with the given observation function.
// timeout defaults to 5s.
func NewObserver(fn ObserverFunc, timeout time.Duration) *Observer {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Observer{observerFn: fn, timeout: timeout}
}

// Observe runs one observation cycle. Returns a timestamped snapshot.
// Each field is queried in its own goroutine with a per-field timeout.
func (o *Observer) Observe() *ObservedSnapshot {
	fields := ReconciledFields()
	results := make(map[Field]ObservedValue, len(fields))
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(len(fields))

	for _, f := range fields {
		go func(f Field) {
			defer wg.Done()
			ch := make(chan ObservedValue, 1)
			go func() {
				now := time.Now()
				v, err := o.observerFn(f)
				if err != nil {
					ch <- ObservedValue{Value: err.Error(), ObservedAt: now, Status: StatusERROR}
				} else {
					ch <- ObservedValue{Value: v, ObservedAt: now, Status: StatusOK}
				}
			}()

			select {
			case result := <-ch:
				mu.Lock()
				results[f] = result
				mu.Unlock()
			case <-time.After(o.timeout):
				mu.Lock()
				results[f] = ObservedValue{
					Value:      nil,
					ObservedAt: time.Now(),
					Status:     StatusUNKNOWN,
				}
				mu.Unlock()
			}
		}(f)
	}
	wg.Wait()

	return &ObservedSnapshot{
		Fields:  results,
		CycleID: newCycleID(),
		TakenAt: time.Now(),
	}
}

// newCycleID generates a unique ID for this observation cycle.
func newCycleID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("cycle-%d", time.Now().UnixNano())
	}
	return "obs-" + hex.EncodeToString(b)
}
