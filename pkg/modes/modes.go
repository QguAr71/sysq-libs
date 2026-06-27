// Package modes defines the operational state machine for SysQ daemons.
//
// Eight operational modes govern what operations are allowed and how
// the kernel responds to invariants, heartbeats, and reconciliation.
//
//	INIT        → initial boot, validation in progress
//	GUARDED     → validated, first reconcile pending
//	ACTIVE      → normal operation, all invariants passing
//	DEGRADED    → invariant broken, limited operations
//	EMERGENCY   → critical failure, read-only mode
//	PANIC       → kernel unreachable, waiting for recovery
//	MAINTENANCE → operator-requested, no heartbeats
//	RECOVERY    → recovering from PANIC, restoring state
//
// Transitions are validated per-mode. Reconciler may override in RECOVERY.
// Mode NEVER influences identity or search ranking.
//
// Usage:
//
//	mgr := modes.NewManager()
//	mgr.Transition(modes.Guarded, "first heartbeat received")
//	if mgr.IsOperational() { ... }
//
// Package is shared across Lazarus, Supervisor, and Narada.
// Each project defines its own mode progression policy.
package modes

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Message is a generic bus message (avoid circular dependency on bus package).
type Message struct {
	Payload []byte
}

// Bus is a minimal event bus interface. Mode Manager only needs Publish.
type Bus interface {
	Publish(ctx context.Context, topic string, msg Message) error
}

// TransitionEvent is published on mode.transition after every successful transition.
type TransitionEvent struct {
	From      Mode      `json:"from"`
	To        Mode      `json:"to"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
}

// Mode represents an operational state.
type Mode string

const (
	Init        Mode = "INIT"
	Guarded     Mode = "GUARDED"
	Active      Mode = "ACTIVE"
	Degraded    Mode = "DEGRADED"
	Emergency   Mode = "EMERGENCY"
	Panic       Mode = "PANIC"
	Maintenance Mode = "MAINTENANCE"
	Recovery    Mode = "RECOVERY"
)

// State holds the current mode with metadata.
type State struct {
	Mode      Mode      `json:"mode"`
	Since     time.Time `json:"since"`
	Reason    string    `json:"reason"`
	PrevMode  Mode      `json:"prev_mode,omitempty"`
	PrevSince time.Time `json:"prev_since,omitempty"`
}

// Manager tracks mode state and validates transitions.
type Manager struct {
	mu    sync.RWMutex
	state State
	bus   Bus // optional: set via SetBus for publishing mode.transition events
}

// NewManager creates a mode manager in INIT state.
func NewManager() *Manager {
	return &Manager{
		state: State{
			Mode:  Init,
			Since: time.Now().UTC(),
		},
	}
}

// SetBus injects an event bus for publishing mode transitions.
// If nil, events are not published (backward compatible).
func (m *Manager) SetBus(b Bus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bus = b
}

// State returns a copy of the current mode state.
func (m *Manager) State() State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// Current returns the current mode.
func (m *Manager) Current() Mode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state.Mode
}

// Transition attempts to change the operational mode.
// Returns error if the transition is not allowed.
func (m *Manager) Transition(target Mode, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isValidTransition(target) {
		return fmt.Errorf("invalid transition: %s → %s (allowed: %v)", m.state.Mode, target, allowedFrom(m.state.Mode))
	}

	prev := m.state
	m.state = State{
		Mode:      target,
		Since:     time.Now().UTC(),
		Reason:    reason,
		PrevMode:  prev.Mode,
		PrevSince: prev.Since,
	}

	// Publish mode.transition event if bus is configured
	if m.bus != nil {
		ev := TransitionEvent{
			From:      prev.Mode,
			To:        target,
			Reason:    reason,
			Timestamp: m.state.Since,
		}
		data, err := json.Marshal(ev)
		if err == nil {
			_ = m.bus.Publish(context.Background(), "mode.transition", Message{Payload: data})
		}
	}

	return nil
}

// isValidTransition checks if the target mode is reachable from current.
func (m *Manager) isValidTransition(target Mode) bool {
	switch m.state.Mode {
	case Init:
		return target == Guarded || target == Degraded || target == Panic || target == Recovery
	case Guarded:
		return target == Active || target == Degraded || target == Maintenance
	case Active:
		return target == Degraded || target == Emergency || target == Maintenance
	case Degraded:
		return target == Active || target == Emergency || target == Maintenance || target == Panic
	case Emergency:
		return target == Degraded || target == Panic || target == Maintenance || target == Recovery
	case Panic:
		return target == Recovery
	case Maintenance:
		return target == Active || target == Degraded || target == Emergency || target == Init
	case Recovery:
		return target == Active || target == Degraded || target == Guarded || target == Panic
	default:
		return false
	}
}

// allowedFrom returns the list of reachable modes from current.
func allowedFrom(current Mode) []Mode {
	switch current {
	case Init:
		return []Mode{Guarded, Degraded, Panic, Recovery}
	case Guarded:
		return []Mode{Active, Degraded, Maintenance}
	case Active:
		return []Mode{Degraded, Emergency, Maintenance}
	case Degraded:
		return []Mode{Active, Emergency, Maintenance, Panic}
	case Emergency:
		return []Mode{Degraded, Panic, Maintenance, Recovery}
	case Panic:
		return []Mode{Recovery}
	case Maintenance:
		return []Mode{Active, Degraded, Emergency, Init}
	case Recovery:
		return []Mode{Active, Degraded, Guarded, Panic}
	default:
		return nil
	}
}

// IsOperational returns true if the mode allows normal agent operations.
func (m *Manager) IsOperational() bool {
	mode := m.Current()
	return mode == Active || mode == Guarded
}

// IsDegraded returns true if the kernel is in a degraded state.
func (m *Manager) IsDegraded() bool {
	mode := m.Current()
	return mode == Degraded
}

// IsReadOnly returns true if write operations should be blocked.
func (m *Manager) IsReadOnly() bool {
	mode := m.Current()
	return mode == Emergency || mode == Panic
}

// IsRecovering returns true if the kernel is recovering from PANIC.
func (m *Manager) IsRecovering() bool {
	return m.Current() == Recovery
}

// SetMode sets the current mode without transition validation.
// Used by Kernel for programmatic mode changes (boot, degrade).
func (m *Manager) SetMode(modeStr string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = State{
		Mode:     Mode(modeStr),
		Since:    time.Now().UTC(),
		PrevMode: m.state.Mode,
	}
}

// String returns the string representation.
func (m Mode) String() string { return string(m) }
