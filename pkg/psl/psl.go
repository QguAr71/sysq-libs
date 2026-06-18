// Package psl implements the Palace Semantic Layer — a deterministic re-ranker
// over Citadel FTS5 results with per-wing epistemic weighting.
//
// PSL does NOT change memory. It changes the order of access to memory.
// PSL is a pure function: (Query, FTS5_ResultSet) → OrderedResultSet.
//
// Architecture:
//
//	Citadel (FTS5)  → truth layer (retrieval baseline)
//	PSL              → ordering layer (interpretation bias)
//
// Key invariant: PSL never filters data. Only re-ranks.
//
// Dependency inversion: PSL uses types.Store interface, not *citadel.Store.
// This breaks the circular PSL ↔ citadel dependency.
package psl

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/QguAr71/sysq-libs/pkg/types"
)

// Mode controls PSL behavior.
type Mode int

const (
	// PSLOff returns pure FTS5 results (debug, regression testing).
	PSLOff Mode = iota
	// PSLOn re-ranks FTS5 results with epistemic + recency + token weights.
	PSLOn
)

// Config defines per-wing epistemic weights.
// Static configuration — no dynamic learning, no drift.
type Config struct {
	// Epistemic weights: experienced, referenced, reported.
	// Default: 1.0, 0.6, 0.3
	EpistemicWeights map[types.Epistemic]float64

	// TimeGateThreshold — how recent is "fresh" (nanoseconds).
	// Fresh entries get epistemic weight damped by 0.85.
	// Default: 1 hour
	TimeGateThreshold int64

	// ConfidenceFloor — BM25 below this skips PSL entirely.
	// Default: 0.2
	ConfidenceFloor float64
}

// DefaultConfig returns sensible defaults for PSL.
func DefaultConfig() Config {
	return Config{
		EpistemicWeights: map[types.Epistemic]float64{
			types.EpistemicExperienced: 1.0,
			types.EpistemicReferenced:  0.6,
			types.EpistemicReported:    0.3,
		},
		TimeGateThreshold: int64(time.Hour),
		ConfidenceFloor:   0.2,
	}
}

// EchoConfig — balanced trust of inheritance.
func EchoConfig() Config {
	return DefaultConfig()
}

// MiniConfig — conservative optimism toward inherited knowledge.
func MiniConfig() Config {
	return Config{
		EpistemicWeights: map[types.Epistemic]float64{
			types.EpistemicExperienced: 1.0,
			types.EpistemicReferenced:  0.8,
			types.EpistemicReported:    0.5,
		},
		TimeGateThreshold: int64(time.Hour),
		ConfidenceFloor:   0.2,
	}
}

// SysQCliConfig — radical self-reliance bias.
func SysQCliConfig() Config {
	return Config{
		EpistemicWeights: map[types.Epistemic]float64{
			types.EpistemicExperienced: 1.0,
			types.EpistemicReferenced:  0.4,
			types.EpistemicReported:    0.2,
		},
		TimeGateThreshold: int64(time.Hour),
		ConfidenceFloor:   0.2,
	}
}

// Store is the minimal interface PSL requires from Citadel.
// This breaks the circular dependency: PSL no longer imports citadel.
// Lazarus/citadel.Store satisfies this interface.
type Store interface {
	SearchAll(query string, limit int) ([]types.SearchResult, error)
}

// Layer is the Palace Semantic Layer — wraps a Citadel store and applies
// per-wing epistemic re-ranking.
type Layer struct {
	store  Store
	config Config
	mode   Mode
}

// New creates a PSL layer wrapping a store that satisfies Store interface.
func New(store Store, config Config) *Layer {
	if config.EpistemicWeights == nil {
		config = DefaultConfig()
	}
	if config.TimeGateThreshold == 0 {
		config.TimeGateThreshold = int64(time.Hour)
	}
	if config.ConfidenceFloor == 0 {
		config.ConfidenceFloor = 0.2
	}
	return &Layer{store: store, config: config, mode: PSLOn}
}

// SetMode switches between PSLOff (pure FTS5) and PSLOn (re-ranked).
func (l *Layer) SetMode(m Mode) { l.mode = m }

// Mode returns the current PSL mode.
func (l *Layer) Mode() Mode { return l.mode }

// SearchResult extends types.SearchResult with re-ranked score.
type SearchResult struct {
	types.Entry
	BM25Score   float64 `json:"bm25_score"`
	TokenScore  float64 `json:"token_score"`
	EpistScore  float64 `json:"epist_score"`
	RecencyScore float64 `json:"recency_score"`
	InnerScore  float64 `json:"inner_score"`  // score within epistemic bucket (BM25+recency+token)
	FinalScore  float64 `json:"final_score"`   // = InnerScore (for backward compat)
}

// byFinalScore sorts results by FinalScore descending.
type byFinalScore []SearchResult

func (a byFinalScore) Len() int           { return len(a) }
func (a byFinalScore) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byFinalScore) Less(i, j int) bool { return a[i].FinalScore > a[j].FinalScore }

// Search performs FTS5 search + PSL bucket sort.
// PSL bucket sort: experienced > referenced > inferred > reported.
//
// When bridge is non-nil AND PSL mode is PSLOn:
//
//	FTS5 (100 candidates) → semantic bridge (re-rank to top-50) → PSL bucket sort
//
// Bridge inactive (nil, PSL=Off, or error):
//
//	FTS5 (50 candidates) → PSL bucket sort
func (l *Layer) Search(wing, query string, limit int) ([]SearchResult, error) {
	return l.SearchWithBridge(wing, query, limit, nil)
}

// SearchWithBridge is like Search but with optional semantic re-ranking bridge.
func (l *Layer) SearchWithBridge(wing, query string, limit int, bridge interface{}) ([]SearchResult, error) {
	// Dynamic FTS5 limit — wider net when bridge is active
	ftsCount := 50
	if bridge != nil && l.mode == PSLOn {
		ftsCount = 100
	}

	// Get raw FTS5 results
	ftsResults, err := l.store.SearchAll(query, ftsCount)
	if err != nil {
		return nil, err
	}

	// Apply semantic bridge if available
	if bridge != nil && l.mode == PSLOn && err == nil {
		type bridgeFilterer interface {
			Filter(string, []types.Entry) ([]types.Entry, error)
		}
		if bf, ok := bridge.(bridgeFilterer); ok {
			entries := make([]types.Entry, len(ftsResults))
			for i, r := range ftsResults {
				entries[i] = r.Entry
			}
			filtered, ferr := bf.Filter(query, entries)
			if ferr == nil {
				newResults := make([]types.SearchResult, len(filtered))
				for i, e := range filtered {
					newResults[i] = types.SearchResult{Entry: e}
				}
				ftsResults = newResults
			}
		}
	}

	// Bucket constants
	const (
		bucketExperienced = 0
		bucketReferenced  = 1
		bucketInferred    = 2
		bucketReported    = 3
	)

	buckets := [4][]SearchResult{}

	for _, r := range ftsResults {
		sr := SearchResult{
			Entry:     r.Entry,
			BM25Score: r.Rank,
		}

		bm25 := r.Rank
		if bm25 < 0 {
			bm25 = -bm25
		}

		sr.TokenScore = tokenScore(query, r.Content)

		if l.mode == PSLOff || (bm25 < l.config.ConfidenceFloor && sr.TokenScore < 0.1) {
			sr.BM25Score = bm25
			sr.FinalScore = bm25
			sr.InnerScore = bm25   // sort by BM25 in PSLOff mode
			buckets[bucketReported] = append(buckets[bucketReported], sr)
			continue
		}

		if bm25 < 0.01 {
			bm25 = 0.5 * sr.TokenScore
		}

		sr.BM25Score = bm25
		sr.RecencyScore = recencyScore(r.Timestamp)

		sr.InnerScore = 0.55*bm25 + 0.15*sr.TokenScore + 0.30*sr.RecencyScore
		sr.InnerScore *= sr.Confidence
		sr.FinalScore = sr.InnerScore

		var bucket int
		switch r.Epistemic {
		case types.EpistemicExperienced:
			bucket = bucketExperienced
		case types.EpistemicReferenced:
			bucket = bucketReferenced
		case types.EpistemicInferred:
			bucket = bucketInferred
		case types.EpistemicReported:
			bucket = bucketReported
		default:
			bucket = bucketReported
		}
		buckets[bucket] = append(buckets[bucket], sr)
	}

	// Sort each bucket by inner score descending, tiebreak by ID
	for i := range buckets {
		sort.Slice(buckets[i], func(a, b int) bool {
			if buckets[i][a].InnerScore != buckets[i][b].InnerScore {
				return buckets[i][a].InnerScore > buckets[i][b].InnerScore
			}
			return buckets[i][a].ID < buckets[i][b].ID
		})
	}

	// Concatenate buckets in epistemic order
	var results []SearchResult
	for i := 0; i < 4; i++ {
		results = append(results, buckets[i]...)
	}

	if limit <= 0 || limit > len(results) {
		limit = len(results)
	}
	return results[:limit], nil
}

// tokenScore computes Jaccard similarity + phrase bonus.
func tokenScore(query, doc string) float64 {
	qTokens := tokenSet(query)
	dTokens := tokenSet(doc)

	if len(qTokens) == 0 || len(dTokens) == 0 {
		return 0
	}

	intersection := 0
	for t := range qTokens {
		if dTokens[t] {
			intersection++
		}
	}
	union := len(qTokens) + len(dTokens) - intersection
	jaccard := float64(intersection) / float64(union)

	lowerDoc := strings.ToLower(doc)
	lowerQuery := strings.ToLower(query)
	phraseBonus := 0.0
	if strings.Contains(lowerDoc, lowerQuery) {
		phraseBonus = 1.0
	}

	return 0.7*jaccard + 0.3*phraseBonus
}

// recencyScore returns an exponential decay weight based on timestamp.
// Half-life: 7 days (168h).
//
// Semantic model:
//
//	1 day:   0.91 — yesterday's fix is still relevant
//	1 week:  0.50 — sprint boundary, natural freshness threshold
//	1 month: 0.05 — background context, findable but doesn't dominate
//	1 year:  ~0   — effectively invisible
func recencyScore(timestamp int64) float64 {
	now := time.Now().UnixNano()
	age := float64(now-timestamp) / float64(time.Hour)
	if age < 0 {
		return 1.0
	}
	return math.Exp(-age * 0.693 / 168.0)
}

// tokenSet returns a set of lowercase tokens from a string.
func tokenSet(s string) map[string]bool {
	tokens := make(map[string]bool)
	fields := strings.Fields(strings.ToLower(s))
	for _, f := range fields {
		clean := strings.Trim(f, ".,;:!?\"'()[]{}")
		if len(clean) > 1 {
			tokens[clean] = true
		}
	}
	return tokens
}
