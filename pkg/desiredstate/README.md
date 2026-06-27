# desiredstate

Generic Desired State engine — defines conditions a daemon must satisfy to be healthy.

## API

```go
type Engine struct { ... }
func (e *Engine) AddField(name string, desired interface{}, observer ObserverFunc)
func (e *Engine) Reconcile() []FixAction
```

## Concepts

- **Desired State** — derived from invariants, configuration, declarations. Never from runtime state (D1).
- **Reconciler** — compares Desired State vs Observed State, dispatches fix actions.
- **Fix actions** respect current operational mode per mode matrix.

## Integration

Used by Lazarus Kernel: 17 observed fields, 8 automatic fixes. Cycle: 60s.

## Files

| File | Description |
|------|-------------|
| `engine.go` | Core engine + Reconciler |
| `fields.go` | Field definition types |
| `observer.go` | ObserverFunc interface |
