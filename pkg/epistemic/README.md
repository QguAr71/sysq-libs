# epistemic

Deterministic knowledge hashing — EntryID.

Same fact + same origin + same epistemic trust = same ID. Always.

## API

```go
id := epistemic.GenerateEntryID("diary", "Lazarus is alive", epistemic.OriginSelf, epistemic.TrustExperienced)
// id = "abc123def456"
```

## Types

| Type | Description |
|------|-------------|
| `Origin` | Where knowledge came from (Self, Operator, External, Unknown) |
| `Epistemic` | Trust level (Experienced, Observed, Inferred, Reported, Speculative) |

## Formula

`SHA256(namespace || content || origin || epistemic)[:16]`

Used by Citadel for deduplication and cross-referencing.

## Tests

2 tests: determinism, different content.
