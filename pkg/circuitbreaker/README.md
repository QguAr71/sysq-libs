# circuitbreaker — Generic 3-State Circuit Breaker

> Package: `sysq-libs/pkg/circuitbreaker`

Protects distributed systems from cascading failures by stopping
requests to unavailable backends. Three states: Closed (normal),
Open (blocked → fast fail), HalfOpen (probe recovery).

Thread-safe. Zero external dependencies.

## API

| Method | Signature | Description |
|--------|-----------|-------------|
| `New` | `(maxFailures int, timeout time.Duration) *CircuitBreaker` | Creates a CB |
| `State` | `() State` | Returns Closed/Open/HalfOpen |
| `RecordSuccess` | `()` | Reset failure count |
| `RecordFailure` | `()` | Increment failure count, may trip |
| `TryHalfOpen` | `() bool` | CAS from Open→HalfOpen for recovery probe |
| `ReleaseHalfOpen` | `()` | Release probe slot |
| `BeforeRequest` | `()` | Wait if Open, proceed if Closed/HalfOpen |

## States

```
    RecordFailure → opens
Closed ────────HalfOpen ──────── Open
              (probe)   (fast fail)
    │    RecordSuccess → closes    │
    └──────────────────────────────┘
```

## Dependencies

None (stdlib only).

## Files

| File | LOC | Purpose |
|------|-----|---------|
| `circuitbreaker.go` | ~90 | Core implementation |

## History

Part of sysq-libs since initial release. Used by ModelRouter (pro/flash routing).
