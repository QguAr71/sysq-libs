# modes

Operational state machine for SysQ daemons.

## States

| State | Description |
|-------|-------------|
| `INIT` | Initial boot, validation in progress |
| `GUARDED` | Validated, first reconcile pending |
| `ACTIVE` | Normal operation, all invariants passing |
| `DEGRADED` | Invariant broken, limited operations |
| `EMERGENCY` | Critical failure, read-only |
| `PANIC` | Kernel unreachable, waiting for recovery |
| `MAINTENANCE` | Operator-requested, no heartbeats |
| `RECOVERY` | Recovering from PANIC, restoring state |

## API

```go
mgr := modes.NewManager(target)
mgr.RegisterTransition("GUARDED", "ACTIVE", func() error { ... })
err := mgr.Transition(modes.Active)
state := mgr.State()
```

## Usage

Lazarus Kernel boots through modes: STARTING → GUARDED → ACTIVE. Degradation: ACTIVE → DEGRADED on port.conflict (3/5min). Recovery: port free → GUARDED.

## Tests

Tests for mode transitions, degradation, boot recovery.
