# heartbeat — Agent Liveness Watcher

> Package: `sysq-libs/pkg/heartbeat`

Monitors agent liveness across superwings. Agents ping lazarusd at
regular intervals. Missed pings transition the superwing to DEGRADED;
three consecutive DEGRADED intervals trigger PANIC.

Timeout NEVER deletes data — only transitions mode (H1 invariant).

## API

| Method | Signature | Description |
|--------|-----------|-------------|
| `NewWatcher` | `(window time.Duration, modeMgr Modes) *Watcher` | Creates a watcher |
| `Ping` | `(superwing string)` | Register a heartbeat |
| `Status` | `(superwing string) (lastPing time.Time, missed int, ok bool)` | Query current state |
| `All` | `() map[string]Status` | Snapshot of all superwings |
| `Stop` | `()` | Stop the watcher goroutine |

## Architecture

- One goroutine per watcher, ticks every `window/3`
- Mode transitions via `Modes` interface (DEGRADED / PANIC)
- Thread-safe: internal `sync.RWMutex`

## Dependencies

- `sysq-core/pkg/modes` — mode manager interface

## Files

| File | LOC | Purpose |
|------|-----|---------|
| `heartbeat.go` | 147 | Watcher, HeartbeatReader interface, Ping/Status/All |
| `heartbeat_test.go` | 113 | Watcher lifecycle, mode transitions |

## History

Migrated from `lazarus/heartbeat/` as part of BLOK 1.1 (2026-07-02).
