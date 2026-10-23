// Package heartbeat monitors superwing agent liveness.
//
// Agents ping lazarusd at regular intervals. If a superwing misses its
// heartbeat window, it transitions to DEGRADED. Three consecutive DEGRADED
// intervals trigger PANIC.
//
// Timeout NEVER deletes data — only transitions mode (H1 invariant).
//
// Adapted from SysQ Supervisor blueprint §22 (health-check).
package heartbeat

import (
	"sync"
	"time"

	"github.com/QguAr71/sysq-core/pkg/modes"
)

// HeartbeatReader exposes read-only access to watcher state.
// Used by consumers (RPC handlers, reconciler) that need Ping/Status/All
// but must never stop the watcher or mutate its internal state.
//
// *Watcher implements HeartbeatReader (see Watcher.Ping, .Status, .All).
type HeartbeatReader interface {
	Ping(superwing string)
	Status(superwing string) (lastPing time.Time, missed int, ok bool)
	All() map[string]struct {
		LastPing time.Time `json:"last_ping"`
		Missed   int       `json:"missed"`
	}
}

// Watcher tracks liveness per superwing.
// Implements HeartbeatReader.
type Watcher struct {
	mu         sync.RWMutex
	lastPing   map[string]time.Time
	missed     map[string]int     // consecutive missed heartbeats
	modeMgr    *modes.Manager
	interval   time.Duration
	done       chan struct{}
}

// NewWatcher creates a heartbeat watcher with the given mode manager.
// interval: expected heartbeat period (default 10s).
func NewWatcher(modeMgr *modes.Manager, interval time.Duration) *Watcher {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	return &Watcher{
		lastPing: make(map[string]time.Time),
		missed:   make(map[string]int),
		modeMgr:  modeMgr,
		interval: interval,
		done:     make(chan struct{}),
	}
}

// Ping records a heartbeat from a superwing.
func (w *Watcher) Ping(superwing string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.lastPing[superwing] = time.Now()
	w.missed[superwing] = 0

	// If mode is DEGRADED due to heartbeat, attempt recovery
	if w.modeMgr.Current() == modes.Degraded {
		w.modeMgr.Transition(modes.Active, "heartbeat recovered: "+superwing)
	}
}

// Start begins periodic checking for missed heartbeats.
// Runs until Stop() is called.
func (w *Watcher) Start() {
	go func() {
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				w.check()
			case <-w.done:
				return
			}
		}
	}()
}

// Stop terminates the periodic checker.
func (w *Watcher) Stop() {
	close(w.done)
}

// check walks all superwings and transitions mode on timeout.
func (w *Watcher) check() {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	timeout := 3 * w.interval // 3 missed intervals → DEGRADED

	for sw, lastPing := range w.lastPing {
		if now.Sub(lastPing) > timeout {
			w.missed[sw]++
			if w.missed[sw] >= 3 {
				// 3 × DEGRADED → PANIC
				if w.modeMgr.Current() != modes.Panic {
					w.modeMgr.Transition(modes.Panic, "heartbeat lost: "+sw)
				}
			} else if w.modeMgr.Current() == modes.Active {
				w.modeMgr.Transition(modes.Degraded, "heartbeat timeout: "+sw)
			}
		}
	}
}

// Status returns the last ping time and missed count for a superwing.
func (w *Watcher) Status(superwing string) (lastPing time.Time, missed int, ok bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	lp, found := w.lastPing[superwing]
	return lp, w.missed[superwing], found
}

// All returns the status of all monitored superwings.
func (w *Watcher) All() map[string]struct {
	LastPing time.Time `json:"last_ping"`
	Missed   int       `json:"missed"`
} {
	w.mu.RLock()
	defer w.mu.RUnlock()

	out := make(map[string]struct {
		LastPing time.Time `json:"last_ping"`
		Missed   int       `json:"missed"`
	})
	for sw, lp := range w.lastPing {
		out[sw] = struct {
			LastPing time.Time `json:"last_ping"`
			Missed   int       `json:"missed"`
		}{LastPing: lp, Missed: w.missed[sw]}
	}
	return out
}
