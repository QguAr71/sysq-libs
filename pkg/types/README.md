# types

Shared epistemic types for SysQ ecosystem.

## Types

```go
type Entry struct {
    ID        string
    Room      string
    Content   string
    Origin    Epistemic
    Timestamp int64
    Tombstone bool
}

type Epistemic int
const (
    TrustExperienced Epistemic = iota  // agent's own experience
    TrustObserved                      // observed external event
    TrustInferred                      // derived from other facts
    TrustReported                      // reported by external source
    TrustSpeculative                   // hypothetical, not confirmed
)

type SearchResult struct {
    Entry
    Score     float64
    FTS5Rank  float64
    PSLRank   float64
}
```

## Invariant

Types in this package MUST NOT import any Lazarus or PSL package. This package is the bottom layer of the dependency graph — everyone depends on it, it depends on nothing (except stdlib).
