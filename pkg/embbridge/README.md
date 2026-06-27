# embbridge

Semantic re-ranking bridge over FTS5 results.

Pre-filter: widens candidate recall through semantic similarity, then hands off to PSL for epistemic ranking. Never replaces FTS5, never changes PSL ordering.

## API

```go
emb := embbridge.NewMockEmbedder(384) // or ONNX in Lazarus
bridge := embbridge.NewBridge(emb, 50)

results, err := bridge.Filter("deploy script", []embbridge.TextEntry{
    {ID: "c1", Content: "deploy automation build pipeline"},
    {ID: "c2", Content: "GDM freeze NVIDIA DRM fix"},
})
```

## TextEntry

```go
type TextEntry struct {
    ID      string
    Content string
}
```

Minimalny struct — bez zależności od Citadel. Lazarus adapter: `embbridge.TextEntry{ID: e.ID, Content: e.Content}`.

## Embedder

```go
type Embedder interface {
    Embed(text string) ([]float32, error)
    EmbedBatch(texts []string) ([][]float32, error)
    Dim() int
    ModelName() string
    Close() error
}
```

Implementacje: `NewMockEmbedder` (testy), `MiniLMEmbedder` (ONNX, w Lazarusie).

## Architektura

```
FTS5 (100 results)
  → EmbBridge (re-rank do top-50, cosine similarity)
    → PSL (epistemic bucket sort)
      → final results
```

Graceful degradation: jeśli embedding niedostępny → zwraca oryginalne candidates bez re-rankingu.

## Tests

11 testów PASS: empty, basic re-rank, topK, order stability, cosine similarity (same/opposite/mismatch/zero), normalize, benchmarks.

## Dependencies

- stdlib only (`math`, `sort`, `log/slog`)
- Zero zależności zewnętrznych — ONNX/SentencePiece tylko w implementacji Embedder (w Lazarusie)
