// session/lifecycle.go — Lifecycle FSM + event types for session lifecycle observer.
//
// Design: docs/blueprints/SESSION_LIFECYCLE_v1.md
//
// Lifecycle transitions are events, not Reconciler fields.
// EXITED:0 is a success, not a drift.

package session

import (
	"fmt"
	"time"
)

// LifecycleState represents the lifecycle phase of an agent session.
type LifecycleState string

const (
	Unknown       LifecycleState = "UNKNOWN"
	Starting      LifecycleState = "STARTING"
	Alive         LifecycleState = "ALIVE"
	Terminating   LifecycleState = "TERMINATING"
	ExitedZero    LifecycleState = "EXITED:0"
	ExitedNonZero LifecycleState = "EXITED:N"
	ExitedUnknown LifecycleState = "EXITED:UNKNOWN"
)

// ValidTransition checks if the state transition is allowed.
func ValidTransition(from, to LifecycleState) bool {
	switch from {
	case Unknown:
		return to == Starting
	case Starting:
		return to == Alive || to == ExitedUnknown // ESRCH at pidfd_open
	case Alive:
		return to == Terminating || to == ExitedZero || to == ExitedNonZero || to == ExitedUnknown
	case Terminating:
		return to == ExitedZero || to == ExitedNonZero || to == ExitedUnknown
	default:
		return false // EXITED:* are terminal states
	}
}

// LifecycleEvent is emitted when the lifecycle state changes.
type LifecycleEvent struct {
	Seq       int64          `json:"seq"`
	Timestamp time.Time      `json:"ts"`
	From      LifecycleState `json:"from"`
	To        LifecycleState `json:"to"`
	ExitCode  int            `json:"exit_code"`
	Reason    string         `json:"reason"`
}

func (e LifecycleEvent) String() string {
	return fmt.Sprintf("lifecycle: %s→%s (exit=%d, reason=%s)", e.From, e.To, e.ExitCode, e.Reason)
}

// IsTerminal returns true if the state is a terminal EXITED state.
func (s LifecycleState) IsTerminal() bool {
	return s == ExitedZero || s == ExitedNonZero || s == ExitedUnknown
}

// ExitedState maps exit code to the appropriate terminal state.
func ExitedState(exitCode int) LifecycleState {
	if exitCode == 0 {
		return ExitedZero
	}
	return ExitedNonZero
}
