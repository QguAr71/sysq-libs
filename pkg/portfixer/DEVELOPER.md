# Port Fixer — Developer Documentation

**Package:** `github.com/QguAr71/sysq-libs/pkg/portfixer`
**License:** MIT
**Version:** v1.0.0
**LOC:** ~190 (+ ~130 test)

---

## Algorithm

```
Resolve():
  1. ENV override     → return env value
  2. defaultPort free → return defaultPort
  3. scan [start,end] → return first free
  4. emergency        → return defaultPort + 100 (graceful degradation)
```

Port check: `net.DialTimeout("tcp", "127.0.0.1:PORT", 50ms)` — lightweight, cross-platform, no root required.

---

## API

### `type Config`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `DefaultPort` | `int` | `9595` | Primary port to try |
| `RangeStart` | `int` | `9595` | Start of fallback scan range |
| `RangeEnd` | `int` | `9605` | End of fallback scan range (inclusive) |
| `EnvVar` | `string` | `"LAZARUSD_MCP_PORT"` | Env var name for override |
| `Bus` | `Bus` | `nil` | Optional event bus for publishing port events |

Defaults via `DefaultConfig()`.

### `type PortFixer`

| Method | Returns | Description |
|--------|---------|-------------|
| `Resolve()` | `int` | Resolves the port. Idempotent — second call returns cached value. |
| `Port()` | `int` | Returns last resolved port, or `0` if not resolved yet. |
| `ClearCache()` | — | Clears port check cache. Use after a port has been freed. |

### `type Bus` (interface)

```go
type Bus interface {
    Publish(ctx context.Context, topic string, msg Message) error
}
```

Nil-safe — if `Bus` is nil, all publishes are no-ops. No panics.

---

## Events

| Topic | When | Payload |
|-------|------|---------|
| `port.assigned` | Every resolution | `{"port": N, "source": "env"\|"default"\|"fallback"}` |
| `port.conflict` | Default port was busy | `{"port": N, "previous": M, "reason": "default busy"}` |

Events are fire-and-forget — never block startup.

---

## Usage

### Basic (no bus)

```go
pf := portfixer.New(portfixer.DefaultConfig())
port := pf.Resolve()
// port is the port to bind to
```

### With env override

```go
pf := portfixer.New(portfixer.Config{
    DefaultPort: 1420,
    RangeStart:  1420,
    RangeEnd:    1430,
    EnvVar:      "MYAPP_PORT",
})
port := pf.Resolve()
```

### With event bus

```go
bus := mybus.New()
pf := portfixer.New(portfixer.Config{
    DefaultPort: 9595,
    Bus:         bus,
})
port := pf.Resolve()
// bus now has "port.assigned" event
```

### Emergency cleanup

```go
pf.ClearCache()
port := pf.Resolve() // re-scans
```

---

## Behavior

| Scenario | Result |
|----------|--------|
| `PORT=8888` in env | Returns `8888` |
| Default port free | Returns default port |
| Default port busy, `9600` free | Returns `9600` + publishes `port.conflict` |
| All ports in range busy | Returns `defaultPort + 100` + publishes `port.conflict` with reason `"no free ports in range"` |
| Called twice | Second call returns cached value (O(1)) |
| `Bus` is `nil` | No events published, no panic |

---

## Invariants

- **Nil-safe** — nil Bus, nil config fields, concurrent calls — nothing panics
- **Graceful degradation** — system never refuses to start (returns a port no matter what)
- **Idempotent** — `Resolve()` always returns the same port after first call
- **Cache with eviction** — `ClearCache()` lets you re-scan after freeing a port
- **Sets env var** — on fallback, sets `EnvVar` so child processes see the resolved port

---

## Testing

```bash
go test ./pkg/portfixer/ -v -race
```

9 tests:
- Env override
- Default resolution
- Range scan (`findAvailable`)
- Range exhausted (`-1`)
- Port cache hit
- Cache clear
- Idempotent resolve
- Event publishing (assigned + source)
- Nil-safe bus

---

## Integration example (lazarusd)

```go
// cmd/lazarusd/port_fixer.go
func resolveMCPPort() int {
    cfg := portfixerLib.DefaultConfig()
    cfg.DefaultPort = 9595
    cfg.RangeStart = 9595
    cfg.RangeEnd = 9605
    cfg.EnvVar = "LAZARUSD_MCP_PORT"

    if portFixBus != nil {
        cfg.Bus = &portfixerBusAdapter{portFixBus}
    }

    globalPortFixer = portfixerLib.New(cfg)
    return globalPortFixer.Resolve()
}
```

---

## Porting to other languages

Algorithm is small enough to port:

```javascript
async function resolvePort(preferred, rangeStart, rangeEnd) {
  if (await isPortFree(preferred)) return preferred;
  for (let p = rangeStart; p <= rangeEnd; p++) {
    if (p === preferred) continue;
    if (await isPortFree(p)) return p;
  }
  return preferred + 100;
}
```

The key insight: `net.DialTimeout("127.0.0.1:PORT", 50ms)` — a failed connect means the port is free. This works on any OS with a TCP stack.
