// Package desiredstate defines a generic Desired State engine.
//
// Desired State is the set of conditions a daemon must satisfy to be
// considered healthy. It is derived from invariants, configuration,
// and declarations — never from runtime state (D1).
//
// The Reconciler compares Desired State against Observed State and
// dispatches fix actions. Fix actions respect the current operational
// mode per the mode matrix.
//
// Fields are domain-specific — each project defines its own constants.
package desiredstate

import (
	"fmt"
	"sync"
	"time"
)

// Field identifies a reconcilable condition in the system.
type Field string

// Reconciled fields — observed and automatically fixed.
const (
	FIdentityValidate         Field = "identity.validate"
	FIdentityIntegrity        Field = "identity.integrity"
	FCitadelIntegrity         Field = "citadel.integrity"
	FWingGraph                Field = "wing.graph"
	FWingActive               Field = "wing.active"
	FGCBeyondRetention        Field = "gc.pending_beyond_retention"
	FVaultKeyLoaded           Field = "vault.key.loaded"
	FVaultKeyValid            Field = "vault.key.valid"
	FMode                     Field = "mode"
	FHeartbeatAllOnline       Field = "heartbeat.all_online"
	FHeartbeatMissed          Field = "heartbeat.missed"
	FPortConflicts            Field = "port.conflicts"
	FCitadelWriteAge          Field = "citadel.write_age"
	FSessionWriteAge          Field = "session.write_age"
	FSessionState             Field = "session.state"        // consistent | dirty
	FDocsIndexAge             Field = "docs.index_age"
	FScannerAge               Field = "scanner.age"
	FContextPressure          Field = "context.pressure"   // 0–100, warns at >=80
	FIdentityDeploySync       Field = "identity.deploy_sync" // SYNCED | AHEAD | DRIFT | UNKNOWN
	FCanonicalSync            Field = "canonical.sync"       // SYNCED | AHEAD | UNKNOWN (goose-config → manifest)
	FSocketHealth             Field = "socket.health"        // PASS | FAIL:<reason>
)

// filterIdentityDiffs filters diffs to only identity-related fields (validate, integrity).
// These are the diffs that Phoenix recovery can fix by redeploying the manifest.
func filterIdentityDiffs(diffs []DiffResult) []DiffResult {
	var out []DiffResult
	for _, d := range diffs {
		if d.Field == FIdentityValidate || d.Field == FIdentityIntegrity {
			out = append(out, d)
		}
	}
	return out
}

// ReconciledFields returns all reconciled fields in canonical order.
func ReconciledFields() []Field {
	return []Field{
		FIdentityValidate, FIdentityIntegrity, FCitadelIntegrity,
		FWingGraph, FWingActive, FGCBeyondRetention,
		FVaultKeyLoaded, FVaultKeyValid,
		FMode, FHeartbeatAllOnline, FHeartbeatMissed, FPortConflicts,
		FSocketHealth,         // critical: socket must exist for agents
		FCanonicalSync,       // goose-config → manifest sync (before deploy)
		FIdentityDeploySync, // before session.write_age — auto-deploy has priority
		FCitadelWriteAge,
		FSessionWriteAge,
		FSessionState,
		FDocsIndexAge,
		FScannerAge,
		FContextPressure,
	}
}

// DesiredValue is the expected value for a field.
type DesiredValue struct {
	Value interface{}
	Field Field
}

// DesiredState holds the expected state for all reconciled fields.
type DesiredState struct {
	mu sync.RWMutex
	m  map[Field]DesiredValue
}

// New returns a DesiredState populated with default values for all fields.
func New() *DesiredState {
	ds := &DesiredState{m: make(map[Field]DesiredValue, 17)}
	for _, f := range ReconciledFields() {
		ds.m[f] = DesiredValue{Field: f, Value: desiredDefault(f)}
	}
	return ds
}

// Get returns the desired value for a field.
func (ds *DesiredState) Get(f Field) (DesiredValue, bool) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	v, ok := ds.m[f]
	return v, ok
}

// Set overrides the desired value for a field.
func (ds *DesiredState) Set(f Field, v interface{}) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.m[f] = DesiredValue{Field: f, Value: v}
}

// Snapshot returns a copy of the current desired state.
func (ds *DesiredState) Snapshot() map[Field]DesiredValue {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	out := make(map[Field]DesiredValue, len(ds.m))
	for k, v := range ds.m {
		out[k] = v
	}
	return out
}

// desiredDefault returns the default desired value for a field.
func desiredDefault(f Field) interface{} {
	switch f {
	case FIdentityValidate, FIdentityIntegrity:
		return "PASS"
	case FCitadelIntegrity:
		return "ok"
	case FWingGraph:
		return "PASS"
	case FWingActive:
		return "PASS"
	case FGCBeyondRetention:
		return "PASS"
	case FVaultKeyLoaded, FVaultKeyValid:
		return "PASS"
	case FMode:
		return "ACTIVE"
	case FHeartbeatAllOnline:
		return "PASS"
	case FHeartbeatMissed:
		return "PASS"
	case FPortConflicts:
		return "PASS"
	case FCitadelWriteAge:
		return "<=30m"
	case FSessionWriteAge:
		return "<=30s"
	case FSessionState:
		return "consistent" // session save effects are consistent
	case FDocsIndexAge:
		return "<=60s"
	case FScannerAge:
		return "<=5m"
	case FContextPressure:
		return "<80" // warn when context pressure ≥80%
	case FSocketHealth:
		return "PASS"
	case FCanonicalSync:
		return "SYNCED"
	case FIdentityDeploySync:
		return "SYNCED"
	default:
		return nil
	}
}

// SnapshotStatus represents the observation status of a field.
type SnapshotStatus string

const (
	StatusOK      SnapshotStatus = "OK"
	StatusUNKNOWN SnapshotStatus = "UNKNOWN"
	StatusERROR   SnapshotStatus = "ERROR"
)

// ObservedValue is the observed state of a single field.
type ObservedValue struct {
	Value      interface{}
	ObservedAt time.Time
	Status     SnapshotStatus
}

// ObservedSnapshot holds the observed state for all fields after one cycle.
type ObservedSnapshot struct {
	Fields   map[Field]ObservedValue
	CycleID  string
	TakenAt  time.Time
}

// DiffResult represents a difference between desired and observed state.
type DiffResult struct {
	Field    Field
	Desired  interface{}
	Observed interface{}
	Msg      string
}

// Diff computes the difference between desired and observed state.
// UNKNOWN observed values are treated as mismatches (D6).
func Diff(desired map[Field]DesiredValue, observed *ObservedSnapshot) []DiffResult {
	var diffs []DiffResult
	for _, f := range ReconciledFields() {
		dv, ok := desired[f]
		if !ok {
			continue
		}
		ov, ok := observed.Fields[f]
		if !ok {
			diffs = append(diffs, DiffResult{Field: f, Desired: dv.Value, Observed: nil, Msg: "not observed"})
			continue
		}
		if ov.Status != StatusOK {
			diffs = append(diffs, DiffResult{Field: f, Desired: dv.Value, Observed: ov.Value, Msg: fmt.Sprintf("status=%s", ov.Status)})
			continue
		}
		if !valuesEqual(dv.Value, ov.Value) {
			diffs = append(diffs, DiffResult{Field: f, Desired: dv.Value, Observed: ov.Value, Msg: "mismatch"})
		}
	}
	return diffs
}

// valuesEqual compares two observed/desired values for equality.
func valuesEqual(a, b interface{}) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}
