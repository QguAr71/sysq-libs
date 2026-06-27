# lifecycle

Unit interface and lifecycle types used by Kernel init system.

## API

```go
type Unit interface {
    Name() string
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Health() UnitStatus
    Dependencies() []string
}
```

## Status

| Status | Description |
|--------|-------------|
| `STARTING` | Init in progress |
| `RUNNING` | Healthy, operational |
| `DEGRADED` | Running but limited |
| `FAILED` | Crashed, recovery needed |
| `STOPPED` | Gracefully shut down |

## Usage

Shared between sysq-libs and lazarusd. Every kernel component implements this interface to be pluggable into the Rack (I-15).
