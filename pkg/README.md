# portfixer — TCP Port Conflict Resolution

**License:** MIT
**Import:** `github.com/QguAr71/sysq-libs/pkg/portfixer`

Resolves TCP port conflicts with graceful degradation.

```
Resolve():
  1. env override → use it
  2. defaultPort free → use it
  3. scan [rangeStart, rangeEnd] → first free
  4. nothing free → defaultPort + 100 (graceful degradation)
```

## Usage

```go
pf := portfixer.New(portfixer.Config{
    DefaultPort: 9595,
    RangeStart:  9595,
    RangeEnd:    9605,
    Bus:         bus, // optional, may be nil
})
port := pf.Resolve()
```

See `portfixer/portfixer.go` for full docs.
