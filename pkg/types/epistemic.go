// Package types provides shared epistemic types used across the SysQ ecosystem.
//
// These types are the canonical definitions of Entry, Epistemic, and SearchResult.
// Lazarus/citadel embeds these types. PSL imports only this package (not citadel),
// breaking the circular dependency between PSL and Lazarus.
//
// Invariant: types in this package MUST NOT import any Lazarus or PSL package.
package types

// Epistemic is the knowledge source classification.
// Every entry carries exactly one epistemic tag — how the agent knows this fact.
type Epistemic string

const (
	// EpistemicExperienced — the agent lived it. First-hand knowledge.
	EpistemicExperienced Epistemic = "experienced"

	// EpistemicReferenced — trusted source told the agent.
	EpistemicReferenced Epistemic = "referenced"

	// EpistemicReported — heard somewhere. Second-hand.
	EpistemicReported Epistemic = "reported"

	// EpistemicInferred — the agent deduced it. Possibly wrong.
	EpistemicInferred Epistemic = "inferred"
)

// Entry is a single memory item — the minimal representation shared
// between Citadel and PSL. Lazarus/citadel embeds this type and adds
// Citadel-specific fields (Origin, Owner, Room, Tombstone, etc.).
type Entry struct {
	ID         string    `json:"id"`         // stable identity hash
	Content    string    `json:"content"`    // the actual text
	Epistemic  Epistemic `json:"epistemic"`  // knowledge source
	Confidence float64   `json:"confidence"` // 0.0–1.0, default 1.0
	Timestamp  int64     `json:"timestamp"`  // nanoseconds
}

// SearchResult is a search hit with rank — shared between FTS5 and PSL.
type SearchResult struct {
	Entry
	Rank float64 `json:"rank"` // FTS5 BM25 rank
	Room string  `json:"room"` // which room this entry belongs to
}
