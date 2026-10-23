# minilm — MiniLM ONNX Embedder

> Package: `sysq-libs/pkg/minilm`
> Model: `paraphrase-multilingual-MiniLM-L12-v2`
> Runtime: ONNX CPU

Produces 384-dimensional embeddings from text using a quantized MiniLM
model via onnxruntime. Suitable for semantic search, similarity scoring,
and clustering in multilingual environments.

## Quickstart

```go
import "github.com/QguAr71/sysq-libs/pkg/minilm"

e, err := minilm.NewMiniLMEmbedder()
if err != nil {
    log.Fatal(err)
}
defer e.Close()

vec, _ := e.Embed("Twoje zdanie tutaj")
fmt.Printf("dim=%d first=%f\n", len(vec), vec[0])
```

## API

| Method | Signature | Description |
|--------|-----------|-------------|
| `NewMiniLMEmbedder` | `() (*MiniLMEmbedder, error)` | Downloads model + tokenizer on first call, caches in `~/.cache/minilm/` |
| `Embed` | `(text string) ([]float32, error)` | Single-text embedding |
| `EmbedBatch` | `(texts []string) ([][]float32, error)` | Batch embedding (reuses session) |
| `Dim` | `() int` | Returns 384 |
| `ModelName` | `() string` | Returns model identifier |
| `Close` | `() error` | Releases ONNX session |

## Model

- **Model:** `paraphrase-multilingual-MiniLM-L12-v2` by sentence-transformers
- **Dimensions:** 384
- **Max tokens:** 512
- **Tokenizer:** XLM-Roberta SentencePiece
- **Download:** Automatic from HuggingFace on first `NewMiniLMEmbedder()`
- **Cache:** `~/.cache/minilm/` (model + tokenizer)

## Pooling

Mean pooling over token embeddings, masked by `attention_mask`. Implements
the standard bi-encoder pipeline: tokenize → run ONNX → mean pool over valid
tokens → 384-dim float32 vector.

## Dependencies

- `github.com/yalue/onnxruntime_go` — ONNX runtime bindings
- `github.com/tggo/goSentencePiece` — XLM-R tokenizer
- `onnxruntime` shared library (system-level, not vendored)

## Thread safety

Read-only after initialization. `Embed` and `EmbedBatch` hold a `sync.RWMutex`
read lock on the ONNX session.

## Usage in Lazarus

Previously at `internal/embedder/minilm_onnx.go`. Migrated to `sysq-libs`
as part of BLOK 1.2 (I-34: one library source of truth).

## Files

| File | LOC | Purpose |
|------|-----|---------|
| `onnx.go` | 355 | Full implementation: download, tokenize, ONNX inference, pooling |
