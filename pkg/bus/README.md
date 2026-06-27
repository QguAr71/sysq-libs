# bus

EventBus interface for SysQ projects.

## API

```go
type Bus interface {
    Publish(topic string, msg interface{})
    Subscribe(topic string, handler func(msg interface{}))
    Request(topic string, msg interface{}, timeout time.Duration) (interface{}, error)
}
```

## Backends

- **InMemoryBus** (`pkg/bus/memory/`) — goroutine-safe, single-process
- **NATSBus** — external, JetStream, persistent, multi-node (Supervisor)

## Usage

Lazarus uses `Publish` + `Subscribe`. Supervisor adds `Request` (ACK/NACK).

## Invariants

- B1: Bus is NOT source of truth. Citadel/StateStore is.
