// Package modes defines operational mode types shared across SysQ projects.
//
// Each daemon (lazarusd, supervisor, etc.) defines its own valid transitions
// and mode constants. This package provides the shared types and minimal
// interface that the Kernel requires.
//
// Lazy loading support: SetMode and ModeActive are added for Kernel.
package modes

import (
	"sync"
	"time"
)

// Mode represents an operational state.
type Mode string

const (
	ModeInit   Mode = "INIT"
	ModeActive Mode = "ACTIVE"
)

// State holds the current mode with metadata.
type State struct {
	Mode      Mode      `json:"mode"`
	Since     time.Time `json:"since"`
	Reason    string    `json:"reason"`
	PrevMode  Mode      `json:"prev_mode,omitempty"`
	PrevSince time.Time `json:"prev_since,omitempty"`
}

// Manager tracks mode state.
type Manager struct {
	mu    sync.RWMutex
	state State
}

// NewManager creates a mode manager in INIT state.
func NewManager() *Manager {
	return &Manager{
		state: State{
			Mode:  ModeInit,
			Since: time.Now().UTC(),
		},
	}
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

// SetMode sets the current mode.
func (m *Manager) SetMode(mode Mode) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = State{
		Mode:     mode,
		Since:    time.Now().UTC(),
		PrevMode: m.state.Mode,
	}
}
