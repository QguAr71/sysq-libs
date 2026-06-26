// Package lifecycle defines the Unit interface and lifecycle types
// used by the Kernel init system. Shared between sysq-libs and lazarusd.
package lifecycle

import (
	"context"
	"sync"
	"time"
)

// UnitStatus represents the lifecycle state of a unit.
type UnitStatus string

const (
	// UnitDead — not yet started or stopped.
	UnitDead UnitStatus = "dead"
	// UnitStarting — in the process of starting.
	UnitStarting UnitStatus = "starting"
	// UnitActive — running normally.
	UnitActive UnitStatus = "active"
	// UnitDegraded — running but with reduced functionality.
	UnitDegraded UnitStatus = "degraded"
	// UnitFailed — failed to start or crashed.
	UnitFailed UnitStatus = "failed"
)

// HealthReport describes the current health of a unit.
type HealthReport struct {
	Status             UnitStatus `json:"status"`
	Message            string     `json:"message,omitempty"`
	LastCheck          time.Time  `json:"last_check"`
	ConsecutiveFailures int       `json:"consecutive_failures"`
}

// Unit is the fundamental component managed by the Kernel.
type Unit interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Status() UnitStatus
	SetStatus(s UnitStatus)
	Health() HealthReport
	Dependencies() []string
}

// UnitWithStatus is a helper for implementing Status() and SetStatus().
// Thread-safe: uses RWMutex to protect concurrent read/write from watchdog + shutdown.
type UnitWithStatus struct {
	mu     sync.RWMutex
	status UnitStatus
}

// NewUnitWithStatus creates a unit status tracker initialized to dead.
func NewUnitWithStatus() *UnitWithStatus {
	return &UnitWithStatus{status: UnitDead}
}

// Status returns the current status.
func (u *UnitWithStatus) Status() UnitStatus {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.status
}

// SetStatus updates the unit status.
func (u *UnitWithStatus) SetStatus(s UnitStatus) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.status = s
}
