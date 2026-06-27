# psl

Palace Semantic Layer — deterministic re-ranker over Citadel FTS5 results.

PSL does NOT change memory. It changes the order of access to memory.

## Architecture

```
Citadel (FTS5) → truth layer (retrieval baseline)
PSL             → ordering layer (interpretation bias)
```

## API

```go
psl := psl.New(psl.Config{
    EpistemicWeight:   0.40,  // trust-based weighting
    AgeWeight:         0.60,  // recency bias
    WingContext:       "sysq",
})

results, err := psl.Rerank(query, fts5Results)
```

## Key invariant

PSL never filters data. Only re-ranks. Dependency inversion: uses `types.Store` interface, not `*citadel.Store` — breaks circular PSL ↔ citadel dependency.

## Tests

Tests covering re-ranking order, epistemic weighting, age decay.
