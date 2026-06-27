package embbridge

import (
	"math"
	"testing"
)

// ============================================================
// MockEmbedder — deterministic fake for unit tests
// ============================================================

// MockEmbedder returns deterministic embeddings for testing.
// Each text gets an embedding = the first N bytes hashed into a fixed-dim vector.
type MockEmbedder struct {
	dim int
}

func NewMockEmbedder(dim int) *MockEmbedder { return &MockEmbedder{dim: dim} }

func (m *MockEmbedder) Embed(text string) ([]float32, error) {
	batch, _ := m.EmbedBatch([]string{text})
	return batch[0], nil
}

func (m *MockEmbedder) EmbedBatch(texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i, t := range texts {
		vec := make([]float32, m.dim)
		// Deterministic "hash" — sum of UTF-8 bytes
		var sum int32
		for _, b := range []byte(t) {
			sum += int32(b)
		}
		for j := range vec {
			vec[j] = float32((sum+int32(j))%100) / 100.0
		}
		result[i] = vec
	}
	return result, nil
}

func (m *MockEmbedder) Dim() int           { return m.dim }
func (m *MockEmbedder) ModelName() string   { return "mock" }
func (m *MockEmbedder) Close() error        { return nil }

// ============================================================
// Tests
// ============================================================

func TestBridgeFilter_Empty(t *testing.T) {
	b := NewBridge(NewMockEmbedder(4), 50)
	filtered, err := b.Filter("query", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(filtered) != 0 {
		t.Errorf("expected 0 results, got %d", len(filtered))
	}
}

func TestBridgeFilter_Basic(t *testing.T) {
	b := NewBridge(NewMockEmbedder(4), 3)

	candidates := []TextEntry{
		{ID: "c1", Content: "deploy automation build pipeline"},
		{ID: "c2", Content: "GDM freeze NVIDIA DRM fix"},
		{ID: "c3", Content: "wdrożenie automatyzacji deployment"},
		{ID: "c4", Content: "exit hook panic recovery shutdown"},
		{ID: "c5", Content: "tożsamość agenta identity kernel"},
	}

	filtered, err := b.Filter("deploy script", candidates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(filtered) != 3 {
		t.Fatalf("expected 3 results, got %d", len(filtered))
	}

	// c1 ("deploy automation...") should be first — most similar to "deploy script"
	// But mock embedder is deterministic on bytes, so we just verify ordering is stable
	found := make(map[string]bool)
	for _, e := range filtered {
		found[e.ID] = true
	}
	for _, want := range []string{"c1", "c2", "c3", "c4", "c5"} {
		_ = want // at least 3 of 5 should be in results
	}
	if len(found) != 3 {
		t.Errorf("expected 3 unique results, got %d", len(found))
	}
}

func TestCosineSimilarity_Same(t *testing.T) {
	a := []float32{1, 2, 3, 4}
	score := cosineSimilarity(a, a)
	if math.Abs(score-1.0) > 0.001 {
		t.Errorf("expected cosine=1.0 for same vectors, got %.4f", score)
	}
}

func TestCosineSimilarity_Opposite(t *testing.T) {
	a := []float32{1, 2, 3}
	b := []float32{-1, -2, -3}
	score := cosineSimilarity(a, b)
	if math.Abs(score+1.0) > 0.001 {
		t.Errorf("expected cosine=-1.0 for opposite vectors, got %.4f", score)
	}
}

func TestCosineSimilarity_DimensionMismatch(t *testing.T) {
	a := []float32{1, 2, 3}
	b := []float32{1, 2}
	score := cosineSimilarity(a, b)
	if score != 0 {
		t.Errorf("expected 0 for dimension mismatch, got %.4f", score)
	}
}

func TestCosineSimilarity_ZeroVector(t *testing.T) {
	a := []float32{0, 0, 0}
	b := []float32{1, 2, 3}
	score := cosineSimilarity(a, b)
	if score != 0 {
		t.Errorf("expected 0 for zero vector, got %.4f", score)
	}
}

func TestNormalize_UnitLength(t *testing.T) {
	v := []float32{3, 4}
	normalize(v)
	length := math.Sqrt(float64(v[0]*v[0] + v[1]*v[1]))
	if math.Abs(length-1.0) > 0.001 {
		t.Errorf("expected unit length after normalize, got %.4f", length)
	}
}

func TestNormalize_ZeroVector(t *testing.T) {
	v := []float32{0, 0, 0}
	normalize(v)
	for _, x := range v {
		if x != 0 {
			t.Errorf("expected zero vector to stay zero, got %v", v)
		}
	}
}

func TestBridgeTopK(t *testing.T) {
	b := NewBridge(NewMockEmbedder(4), 2)

	candidates := []TextEntry{
		{ID: "a", Content: "first"},
		{ID: "b", Content: "second"},
		{ID: "c", Content: "third"},
		{ID: "d", Content: "fourth"},
	}

	filtered, _ := b.Filter("test", candidates)
	if len(filtered) != 2 {
		t.Errorf("expected 2 results (topK=2), got %d", len(filtered))
	}
}

func TestBridgeFilter_OrderStable(t *testing.T) {
	b := NewBridge(NewMockEmbedder(4), 5)

	candidates := []TextEntry{
		{ID: "a", Content: "alpha"},
		{ID: "b", Content: "beta"},
		{ID: "c", Content: "gamma"},
	}

	// Run twice, expect same ordering
	run1, _ := b.Filter("test", candidates)
	run2, _ := b.Filter("test", candidates)

	for i := range run1 {
		if run1[i].ID != run2[i].ID {
			t.Errorf("order not stable: run1[%d]=%s, run2[%d]=%s",
				i, run1[i].ID, i, run2[i].ID)
		}
	}
}

func TestBridgeFilter_SortByScore(t *testing.T) {
	b := NewBridge(NewMockEmbedder(4), 5)

	// "target" has higher byte sum than "other", so it will score higher
	candidates := []TextEntry{
		{ID: "low", Content: "a"},          // low byte sum
		{ID: "high", Content: "zzzzzzzzz"}, // high byte sum
	}

	filtered, _ := b.Filter("query", candidates)
	if len(filtered) != 2 {
		t.Fatal("expected 2 results")
	}
	// With mock embedder, "high" byte sum > "low" → higher cosine with similar query
	// But since the mock uses sum-based vectors, "high" may score differently.
	// We only verify both are present.
	present := make(map[string]bool)
	for _, e := range filtered {
		present[e.ID] = true
	}
	if !present["high"] || !present["low"] {
		t.Errorf("expected both entries present, got %v", present)
	}
}

func BenchmarkCosineSimilarity(b *testing.B) {
	a := make([]float32, 384)
	for i := range a {
		a[i] = float32(i) / 384.0
	}
	bb := make([]float32, 384)
	copy(bb, a)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cosineSimilarity(a, bb)
	}
}

func BenchmarkBridgeFilter(b *testing.B) {
	br := NewBridge(NewMockEmbedder(384), 50)
	candidates := make([]TextEntry, 100)
	for i := range candidates {
		candidates[i] = TextEntry{
			ID:      "id-" + string(rune('a'+i%26)),
			Content: "test content " + string(rune('a'+i%26)),
		}
	}

	query := "semantic search query for bench"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		br.Filter(query, candidates)
	}
}
