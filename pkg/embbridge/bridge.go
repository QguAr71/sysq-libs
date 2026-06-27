// Package embbridge provides semantic re-ranking of FTS5 search results
// using embedding models (MiniLM ONNX CPU, ollama, or llama.cpp).
//
// Bridge is a pre-filter: it widens candidate recall through semantic
// similarity, then hands off to PSL for epistemic ranking.
// It never replaces FTS5, never changes PSL ordering.
package embbridge

import (
	"log/slog"
	"math"
	"sort"
)

// Embedder is a pluggable embedding model with batch support.
type Embedder interface {
	Embed(text string) ([]float32, error)
	EmbedBatch(texts []string) ([][]float32, error)
	Dim() int
	ModelName() string
	Close() error
}

// Bridge wraps an Embedder to filter FTS5 candidates by semantic similarity.
type Bridge struct {
	embedder Embedder
	topK     int // default 50
}

// NewBridge creates a semantic re-ranking bridge.
func NewBridge(e Embedder, topK int) *Bridge {
	if topK <= 0 {
		topK = 50
	}
	return &Bridge{embedder: e, topK: topK}
}

// Embedder returns the underlying embedding model (may be nil if bridge is disabled).
func (b *Bridge) Embedder() Embedder { return b.embedder }

// TextEntry is the minimal entry type for semantic filtering.
// EmbBridge only needs Content + ID — no dependency on Citadel or any specific Entry type.
type TextEntry struct {
	ID      string
	Content string
}

// GetContent satisfies the minimal text interface (for compatibility with adapters).
func (t TextEntry) GetContent() string { return t.Content }

// Filter re-ranks FTS5 candidates by cosine similarity to the query.
//
// NOTES (for future maintainers):
//   - Normalizes vectors IN PLACE — mutates input slices from EmbedBatch.
//     No other code should hold references to raw embeddings after Filter().
//   - Query is normalized once, candidates each once, then pure dot product
//     (dot = cosine for unit vectors — zero divisions, zero allocations).
//   - Batch embeds query + candidates in single call (101 texts for 100 candidates).
//
// On error: returns original candidates (graceful degradation to FTS5-only).
func (b *Bridge) Filter(query string, candidates []TextEntry) ([]TextEntry, error) {
	if len(candidates) == 0 {
		return candidates, nil
	}

	// Batch: query + all candidates
	texts := make([]string, len(candidates)+1)
	texts[0] = query
	for i, c := range candidates {
		texts[i+1] = c.GetContent()
	}

	vecs, err := b.embedder.EmbedBatch(texts)
	if err != nil {
		return candidates, err // graceful degradation
	}

	// Normalize query vector in place (reciprocal multiply)
	queryVec := normalize(vecs[0])

	// Score each candidate by cosine = dot product (unit vectors)
	type scored struct {
		entry TextEntry
		score float64
	}
	items := make([]scored, len(candidates))
	for i := range candidates {
		candVec := normalize(vecs[i+1])
		var dot float64
		for j := range queryVec {
			dot += float64(queryVec[j]) * float64(candVec[j])
		}
		items[i] = scored{entry: candidates[i], score: dot}
	}

	// Sort by score descending
	sort.Slice(items, func(i, j int) bool {
		return items[i].score > items[j].score
	})

	// Keep top-K
	limit := b.topK
	if limit > len(items) {
		limit = len(items)
	}
	result := make([]TextEntry, limit)
	for i := 0; i < limit; i++ {
		result[i] = items[i].entry
	}
	return result, nil
}

// normalize scales a vector to unit length IN PLACE.
// Uses reciprocal multiply: 1 div per vector, then N multiplications.
// Returns the same slice (mutated).
func normalize(v []float32) []float32 {
	var sq float64
	for _, x := range v {
		sq += float64(x) * float64(x)
	}
	if sq == 0 {
		return v
	}
	invNorm := float32(1.0 / math.Sqrt(float64(sq)))
	for i := range v {
		v[i] *= invNorm
	}
	return v
}

// cosineSimilarity computes cosine similarity between two vectors.
// Guards against dimension mismatch (logs warning, returns 0).
// Uses single-pass dot + squared norms, one division at the end.
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		if len(a) != len(b) && len(a) > 0 {
			slog.Warn("cosineSimilarity: dimension mismatch",
				"len_a", len(a),
				"len_b", len(b),
			)
		}
		return 0
	}

	var dot, sqA, sqB float64
	for i := range a {
		ai, bi := float64(a[i]), float64(b[i])
		dot += ai * bi
		sqA += ai * ai
		sqB += bi * bi
	}

	normA, normB := math.Sqrt(sqA), math.Sqrt(sqB)
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (normA * normB)
}
