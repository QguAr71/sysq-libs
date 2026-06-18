package psl

import (
	"testing"
	"time"

	"github.com/QguAr71/sysq-libs/pkg/types"
)

// mockStore implements Store for testing.
type mockStore struct {
	results []types.SearchResult
	err     error
}

func (m *mockStore) SearchAll(query string, limit int) ([]types.SearchResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	if limit > len(m.results) {
		limit = len(m.results)
	}
	return m.results[:limit], m.err
}

func TestPSL_BucketOrder(t *testing.T) {
	store := &mockStore{
		results: []types.SearchResult{
			{Entry: types.Entry{ID: "1", Epistemic: types.EpistemicReported, Content: "reported fact", Timestamp: 1, Confidence: 1.0}, Rank: 0.9},
			{Entry: types.Entry{ID: "2", Epistemic: types.EpistemicInferred, Content: "inferred fact", Timestamp: 1, Confidence: 1.0}, Rank: 0.8},
			{Entry: types.Entry{ID: "3", Epistemic: types.EpistemicReferenced, Content: "referenced fact", Timestamp: 1, Confidence: 1.0}, Rank: 0.7},
			{Entry: types.Entry{ID: "4", Epistemic: types.EpistemicExperienced, Content: "experienced fact", Timestamp: 1, Confidence: 1.0}, Rank: 0.6},
		},
	}

	layer := New(store, DefaultConfig())
	results, err := layer.Search("wing", "fact", 4)
	if err != nil {
		t.Fatal(err)
	}

	// Experienced first, then referenced, inferred, reported — regardless of BM25
	expected := []string{"4", "3", "2", "1"}
	for i, want := range expected {
		if results[i].ID != want {
			t.Errorf("position %d: expected ID %s, got %s", i, want, results[i].ID)
		}
	}
}

func TestPSL_PSLOff(t *testing.T) {
	store := &mockStore{
		results: []types.SearchResult{
			{Entry: types.Entry{ID: "rep", Epistemic: types.EpistemicReported, Content: "x", Timestamp: 1, Confidence: 1.0}, Rank: 0.9},
			{Entry: types.Entry{ID: "exp", Epistemic: types.EpistemicExperienced, Content: "x", Timestamp: 1, Confidence: 1.0}, Rank: 0.3},
		},
	}

	layer := New(store, DefaultConfig())
	layer.SetMode(PSLOff)
	results, err := layer.Search("wing", "x", 2)
	if err != nil {
		t.Fatal(err)
	}

	// Off mode — no bucket sort, all go to reported bucket
	// Both entries end up in reported bucket, sorted by BM25 descending
	if results[0].ID != "rep" {
		t.Errorf("PSLOff: expected 'rep' first (BM25=0.9), got %s", results[0].ID)
	}
}

func TestPSL_StoreInterface(t *testing.T) {
	// Verify that mockStore satisfies Store interface at compile time
	var _ Store = (*mockStore)(nil)

	// Verify nil store produces no panic at type level
	var s Store
	if s != nil {
		t.Error("nil Store should be nil")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.EpistemicWeights == nil {
		t.Fatal("DefaultConfig: EpistemicWeights should not be nil")
	}
	if cfg.EpistemicWeights[types.EpistemicExperienced] != 1.0 {
		t.Errorf("Experienced weight should be 1.0")
	}
}

func TestTokenScore_Jaccard(t *testing.T) {
	score := tokenScore("Go static typing", "Go is a statically typed language")
	if score <= 0 {
		t.Errorf("tokenScore should be > 0 for overlapping tokens")
	}
}

func TestTokenScore_PhraseBonus(t *testing.T) {
	withPhrase := tokenScore("statically typed", "Go is statically typed")
	noPhrase := tokenScore("statically typed", "The language has types and is static")
	// Exact phrase match should score higher
	if withPhrase <= noPhrase {
		t.Errorf("phrase match should score higher than token-only: %.3f <= %.3f", withPhrase, noPhrase)
	}
}

func TestRecencyScore_Range(t *testing.T) {
	// Now (timestamp = now) should be close to 1.0
	now := time.Now().UnixNano()
	s := recencyScore(now)
	if s < 0.95 || s > 1.0 {
		t.Errorf("recency score for 'now' should be ~1.0, got %.3f", s)
	}

	// 7 days ago should be ~0.5
	weekAgo := now - int64(168*time.Hour)
	s = recencyScore(weekAgo)
	if s < 0.4 || s > 0.6 {
		t.Errorf("recency score for 7d ago should be ~0.5, got %.3f", s)
	}
}

func TestConfigDefaults(t *testing.T) {
	layer := New(&mockStore{}, Config{})
	if layer.config.TimeGateThreshold == 0 {
		t.Error("TimeGateThreshold should default to non-zero")
	}
	if layer.config.ConfidenceFloor == 0 {
		t.Error("ConfidenceFloor should default to non-zero")
	}
	if layer.mode != PSLOn {
		t.Error("Mode should default to PSLOn")
	}
}
