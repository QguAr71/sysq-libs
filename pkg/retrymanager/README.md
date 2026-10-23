# retrymanager — Singleflight Retry for Circuit Breakers

> Package: `sysq-libs/pkg/retrymanager`

Prevents thundering herd when multiple callers hit an open circuit breaker.
Only one goroutine per backend probes recovery; others wait for the result.
Uses lightweight health-check pings instead of replaying user requests.

## API

| Method | Signature | Description |
|--------|-----------|-------------|
| `New` | `(cb *circuitbreaker.CircuitBreaker, probeInterval time.Duration) *RetryManager` | Creates a manager |
| `Retry` | `(ctx context.Context, backend Retryable) error` | Probes backend, returns on first success or ctx cancel |
| `Stop` | `()` | Stops all active retry loops |

## Retryable Interface

```go
type Retryable interface {
    Name() string
    Ping(ctx context.Context) error
}
```

## Behavior

- `Retry()` blocks until backend responds or context expires
- Only **one** goroutine per `Name()` actually sends probes
- Others block on a `sync.Cond` and get the result when the probe completes
- Probe interval: configurable (default 5s)
- Thread-safe: internal mutex per backend

## Dependencies

- `sysq-libs/pkg/circuitbreaker` — CircuitBreaker state

## Files

| File | LOC | Purpose |
|------|-----|---------|
| `retrymanager.go` | ~100 | Core implementation |

## History

Part of sysq-libs since initial release. Used by ModelRouter (pro/flash recovery).
