# portfixer

TCP port conflict resolver with graceful degradation.

## Algorithm

1. `PORT_FIXER_PORT` env var → use it
2. Default port (e.g. 9595) → try it
3. Scan `[start, end]` range → first free
4. Emergency fallback → random port in range

## API

```go
fixer := portfixer.New(portfixer.Config{
    DefaultPort: 9595,
    RangeStart:  9595,
    RangeEnd:    9605,
    Bus:         eventBus,  // nil-safe
})

port, err := fixer.Resolve()
// port.Assigned event published on bus
```

## Invariants

- I-18: Graceful degradation — system never refuses to start
- III-31: Nil-safe bus, panic recovery
- III-38: Publishes `port.assigned`, `port.conflict`

## Tests

10 tests PASS: default port, env override, range scan, emergency fallback, bus events, cache, concurrent.
