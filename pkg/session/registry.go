// session/registry.go — Session Registry + Event Stream.
//
// Design: docs/blueprints/SESSION_LIFECYCLE_v1.md
//
// The Registry manages the lifecycle of one active agent session.
// Lifecycle transitions are emitted as events through the Event Stream.
// Handlers react to events (save, log, alert) — the Reconciler does NOT
// observe lifecycle directly; it observes session.state effects.

package session

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// EventHandler is called when a lifecycle event occurs.
type EventHandler func(e LifecycleEvent)

// SessionInfo contains the identity of a managed agent process.
type SessionInfo struct {
	PID       int   `json:"pid"`
	StartTime uint64 `json:"start_time"` // /proc/PID/stat field 22 (ticks since boot)
}

// Registry manages the lifecycle of one active session.
// Thread-safe.
type Registry struct {
	mu        sync.Mutex
	state     LifecycleState
	info      *SessionInfo
	observers []LifecycleObserver
	handlers  []EventHandler
	journal          *Journal
	seq              int64
	reportedExitCode atomic.Int32 // -1 = not reported yet (session.exit RPC)
}

// LifecycleObserver watches for process termination.
type LifecycleObserver interface {
	Start(session *SessionInfo) error
	Stop() error
}

// NewRegistry creates a new session registry.
func NewRegistry(j *Journal) *Registry {
	r := &Registry{
		state:   Unknown,
		journal: j,
	}
	r.reportedExitCode.Store(-1)
	return r
}

// State returns the current lifecycle state.
func (r *Registry) State() LifecycleState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state
}

// HasActiveSession returns true if there's an active agent session.
func (r *Registry) HasActiveSession() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state == Alive || r.state == Starting || r.state == Terminating
}

// RegisterObserver adds a lifecycle observer (e.g., pidfd/epoll).
func (r *Registry) RegisterObserver(obs LifecycleObserver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.observers = append(r.observers, obs)
}

// OnEvent registers a lifecycle event handler.
func (r *Registry) OnEvent(h EventHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers = append(r.handlers, h)
}

// StartSessionByFd starts tracking using a pidfd received via SCM_RIGHTS.
// pidfd is already open and owned by the daemon after this call.
// PID is operational metadata only — used for /proc/PID/stat exit code reading.
func (r *Registry) StartSessionByFd(pid int, fd int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.state == Alive || r.state == Starting || r.state == Terminating {
		return fmt.Errorf("session already active (state=%s)", r.state)
	}
	// Allow new session after previous one reached terminal state (EXITED:*)
	if r.state.IsTerminal() {
		r.state = Unknown
	}
	if !ValidTransition(r.state, Starting) {
		return fmt.Errorf("invalid transition: %s→STARTING", r.state)
	}

	r.state = Starting
	r.info = &SessionInfo{PID: pid}

	for _, obs := range r.observers {
		if fdObs, ok := obs.(interface{ StartByFd(int, int) error }); ok {
			if err := fdObs.StartByFd(pid, fd); err != nil {
				return fmt.Errorf("observer StartByFd: %w", err)
			}
		}
	}
	return nil
}

// StartSession initiates session lifecycle tracking.
// Returns error if a session is already active.
func (r *Registry) StartSession(info SessionInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.state == Alive || r.state == Starting || r.state == Terminating {
		return fmt.Errorf("session already active (state=%s)", r.state)
	}
	// Allow new session after previous one reached terminal state (EXITED:*)
	if r.state.IsTerminal() {
		r.state = Unknown
	}
	if !ValidTransition(r.state, Starting) {
		return fmt.Errorf("invalid transition: %s→STARTING", r.state)
	}

	r.state = Starting
	r.info = &info

	// Start all observers
	for _, obs := range r.observers {
		if err := obs.Start(&info); err != nil {
			return fmt.Errorf("observer start: %w", err)
		}
	}
	return nil
}

// Transition advances the lifecycle to a new state and emits an event.
func (r *Registry) Transition(to LifecycleState, exitCode int, reason string) error {
	r.mu.Lock()
	from := r.state
	if !ValidTransition(from, to) {
		r.mu.Unlock()
		return fmt.Errorf("invalid transition: %s→%s", from, to)
	}
	r.state = to
	r.info = nil // clear on exit
	r.seq++
	seq := r.seq
	r.mu.Unlock()

	e := LifecycleEvent{
		Seq:       seq,
		Timestamp: time.Now().UTC(),
		From:      from,
		To:        to,
		ExitCode:  exitCode,
		Reason:    reason,
	}

	// Emit: journal → dispatch
	if err := r.emit(e); err != nil {
		return fmt.Errorf("emit: %w", err)
	}
	return nil
}

// emit journals and dispatches the event to all registered handlers.
func (r *Registry) emit(e LifecycleEvent) error {
	// 1. Append to journal (durability)
	journalSeq, err := r.journal.Append(e)
	if err != nil {
		return fmt.Errorf("journal append: %w", err)
	}

	log.Printf("[session] event seq=%d: %s", journalSeq, e.String())

	// 2. Dispatch to all handlers
	for _, h := range r.handlers {
		h(e)
	}
	return nil
}

// StoreReportedExitCode stores the exit code received via session.exit RPC.
// Thread-safe (atomic). Called by handleSessionExit before the observer fires.
func (r *Registry) StoreReportedExitCode(code int) {
	r.reportedExitCode.Store(int32(code))
}

// GetReportedExitCode returns the exit code from session.exit RPC, or -1 if not set.
// Thread-safe (atomic). Called by the observer callback to check for authoritative code.
func (r *Registry) GetReportedExitCode() int {
	return int(r.reportedExitCode.Load())
}

// Stop terminates all observers (called during daemon shutdown).
func (r *Registry) Stop() {
	r.mu.Lock()
	obs := r.observers
	r.mu.Unlock()

	for _, o := range obs {
		o.Stop()
	}
}
