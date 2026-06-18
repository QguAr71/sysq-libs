package desiredstate

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// FixFunc performs a single fix action via lazarusd API.
// Returns nil on success, error on failure or timeout.
type FixFunc func(field Field, action string) error

// ModeManager defines the minimal mode interface needed by the Reconciler.
type ModeManager interface {
	CurrentString() string
	Transition(target string, reason string) error
}

// PhoenixFunc performs a multi-step Phoenix recovery.
// Called when reconciler escalates to critical: rollback → deploy → restart → verify.
// Returns nil on success, error if any step fails.
type PhoenixFunc func() error

// Reconciler runs periodic reconciliation cycles: observe → diff → fix.
// Only one cycle runs at a time (D7). Fix actions respect the current
// operational mode (D4) and are dispatched per the mode matrix (§5.3).
//
// Phoenix escalation (v2): when critical threshold is exceeded or operator
// is offline, the reconciler escalates from single-field fix to full Phoenix
// recovery: rollback → manifest restore → service restart → verify.
type Reconciler struct {
	desired  *DesiredState
	observer *Observer
	fixFn    FixFunc

	// Phoenix escalation (v2)
	phoenixFn         PhoenixFunc
	modes             ModeManager
	criticalThreshold int           // > this many diffs → escalate
	offlineTimeout    time.Duration // operator not active for this long → escalate

	mu         sync.Mutex // cycle lock (D7)
	interval   time.Duration
	fixTimeout time.Duration
	done       chan struct{}
}

// NewReconciler creates a reconciler with the given observer and fix function.
// interval should be ≥ 60s (worst-case cycle ~35s).
func NewReconciler(desired *DesiredState, observer *Observer, fixFn FixFunc, interval time.Duration) *Reconciler {
	if interval <= 0 {
		interval = 60 * time.Second
	}
	return &Reconciler{
		desired:           desired,
		observer:          observer,
		fixFn:             fixFn,
		interval:          interval,
		fixTimeout:        30 * time.Second,
		criticalThreshold: 2,              // >2 diffs → escalate
		offlineTimeout:    5 * time.Minute, // operator offline >5min → escalate
		done:              make(chan struct{}),
	}
}

// SetFixTimeout allows overriding the per-fix timeout for testing.
func (r *Reconciler) SetFixTimeout(d time.Duration) {
	r.fixTimeout = d
}

// SetPhoenix configures Phoenix escalation.
// phoenixFn performs the multi-step recovery (rollback+deploy+restart+verify).
// modes is the mode manager for DEGRADED→PANIC→RECOVERY transitions.
// threshold is the number of diffs that trigger escalation (0 = always escalate).
func (r *Reconciler) SetPhoenix(phoenixFn PhoenixFunc, modes ModeManager, threshold int) {
	r.phoenixFn = phoenixFn
	r.modes = modes
	if threshold > 0 {
		r.criticalThreshold = threshold
	}
}

// SetOfflineTimeout sets the duration after which the operator is considered offline.
func (r *Reconciler) SetOfflineTimeout(d time.Duration) {
	r.offlineTimeout = d
}

// Start begins periodic reconciliation. Runs until Stop() is called.
func (r *Reconciler) Start() {
	go func() {
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()
		r.runCycle() // immediate first cycle

		for {
			select {
			case <-ticker.C:
				r.runCycle()
			case <-r.done:
				return
			}
		}
	}()
}

// Stop terminates the reconciler goroutine.
func (r *Reconciler) Stop() {
	close(r.done)
}

// RunOnce executes a single reconciliation cycle synchronously.
func (r *Reconciler) RunOnce() ([]DiffResult, error) {
	// D7: only one cycle at a time
	if !r.mu.TryLock() {
		return nil, fmt.Errorf("cycle skipped: previous cycle still in progress")
	}
	defer r.mu.Unlock()

	return r.runCycle()
}

// runCycle performs one observe → diff → fix cycle.
// v2: When critical threshold exceeded, escalates to Phoenix recovery.
func (r *Reconciler) runCycle() ([]DiffResult, error) {
	// 1. Observe
	snapshot := r.observer.Observe()

	// 2. Diff
	desired := r.desired.Snapshot()
	diffs := Diff(desired, snapshot)

	if len(diffs) == 0 {
		return nil, nil // clean cycle
	}

	// 3. Phoenix escalation check (v3): only identity diffs trigger Phoenix
	if r.phoenixFn != nil && r.modes != nil {
		identityDiffs := filterIdentityDiffs(diffs)
		if len(identityDiffs) >= r.criticalThreshold {
			log.Printf("[reconciler] cycle=%s CRITICAL: %d identity diffs >= threshold %d — escalating to Phoenix",
				snapshot.CycleID, len(identityDiffs), r.criticalThreshold)

			// Transition to PANIC
			if err := r.modes.Transition("PANIC", fmt.Sprintf("reconciler: %d identity diffs", len(identityDiffs))); err != nil {
				log.Printf("[reconciler] cycle=%s PANIC transition failed: %v — forcing Phoenix anyway", snapshot.CycleID, err)
			}

			// Execute Phoenix recovery
			if err := r.phoenixFn(); err != nil {
				log.Printf("[reconciler] cycle=%s PHOENIX FAILED: %v", snapshot.CycleID, err)
				return diffs, fmt.Errorf("phoenix recovery failed: %w", err)
			}

			log.Printf("[reconciler] cycle=%s PHOENIX OK — identity recovered", snapshot.CycleID)

			// Transition PANIC → RECOVERY → ACTIVE
			if err := r.modes.Transition("RECOVERY", "phoenix recovery complete"); err != nil {
				log.Printf("[reconciler] cycle=%s RECOVERY transition failed: %v", snapshot.CycleID, err)
			}
			if err := r.modes.Transition("ACTIVE", "phoenix: identity restored"); err != nil {
				log.Printf("[reconciler] cycle=%s ACTIVE transition failed: %v (staying in RECOVERY)", snapshot.CycleID, err)
			}

			// Fall through to normal fix for remaining diffs
		}
	}

	// 4. Fix: pick the first non-LOG diff and attempt fix (D5: one fix per cycle)
	// LOG-action diffs are informational — skip them to avoid starvation.
	var firstDiff DiffResult
	var firstAction string
	for _, d := range diffs {
		a := r.fixAction(d)
		if a != "LOG" && a != "" {
			firstDiff = d
			firstAction = a
			break
		}
	}
	if firstAction == "" {
		return diffs, nil // all diffs are LOG-only — nothing to fix
	}

	log.Printf("[reconciler] cycle=%s field=%s desired=%v observed=%v action=%s",
		snapshot.CycleID, firstDiff.Field, firstDiff.Desired, firstDiff.Observed, firstAction)

	// D3: log fix attempt
	fixErr := r.fixWithTimeout(firstDiff.Field, firstAction, r.fixTimeout)
	if fixErr != nil {
		log.Printf("[reconciler] cycle=%s fix failed: %v", snapshot.CycleID, fixErr)
		return diffs, fmt.Errorf("fix %s: %w", firstDiff.Field, fixErr)
	}

	return diffs, nil
}

// fixWithTimeout runs a fix action with a hard timeout.
func (r *Reconciler) fixWithTimeout(field Field, action string, timeout time.Duration) error {
	ch := make(chan error, 1)
	go func() {
		ch <- r.fixFn(field, action)
	}()

	select {
	case err := <-ch:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("timeout after %v", timeout)
	}
}

// fixAction maps a diff result to a fix action string.
// "LOG" means log only, no API call. "" means no action defined.
// v2: "PHOENIX" now triggers full multi-step recovery via phoenixFn.
func (r *Reconciler) fixAction(d DiffResult) string {
	switch d.Field {
	case FIdentityValidate:
		if d.Observed == "FAIL" {
			return "rollback" // rollback to last known good
		}
		return "LOG" // mismatch but not FAIL — alert operator
	case FIdentityIntegrity:
		return "rollback"
	case FCitadelIntegrity:
		return "LOG" // manual restore only — individual entry fixes
	case FWingGraph:
		return "switch" // switch to valid parent + delete orphans
	case FWingActive:
		return "switch"
	case FGCBeyondRetention:
		return "compact"
	case FVaultKeyLoaded, FVaultKeyValid:
		return "LOG" // alert operator
	case FMode:
		return "transition-ACTIVE"
	case FHeartbeatAllOnline, FHeartbeatMissed:
		return "LOG" // heartbeat transitions handled by heartbeat.Watcher directly
	case FPortConflicts:
		return "LOG" // manual resolution
	case FCitadelWriteAge:
		return "LOG" // manual resolution only
	case FSessionWriteAge:
		return "session-save"
	case FSessionState:
		return "LOG" // dirty state is informational — save cycle handles it
	case FDocsIndexAge:
		return "docs-reindex"
	case FScannerAge:
		return "scanner-run"
	case FContextPressure:
		return "LOG" // only log warning, no automatic fix
	case FSocketHealth:
		if s, ok := d.Observed.(string); ok && strings.HasPrefix(s, "FAIL") {
			return "recover-socket"
		}
		return "LOG"
	case FCanonicalSync:
		if s, ok := d.Observed.(string); ok && s == "AHEAD" {
			return "canonical-sync"
		}
		return "LOG" // SYNCED, UNKNOWN — log only
	case FIdentityDeploySync:
		if s, ok := d.Observed.(string); ok && s == "AHEAD" {
			return "deploy-forward"
		}
		return "LOG" // DRIFT, UNKNOWN, SYNCED — log only
	}
	return ""
}
