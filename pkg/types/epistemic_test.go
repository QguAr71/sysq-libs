package types

import "testing"

func TestEpistemicConstants(t *testing.T) {
	if EpistemicExperienced != "experienced" {
		t.Errorf("expected 'experienced', got %q", EpistemicExperienced)
	}
	if EpistemicReferenced != "referenced" {
		t.Errorf("expected 'referenced', got %q", EpistemicReferenced)
	}
	if EpistemicReported != "reported" {
		t.Errorf("expected 'reported', got %q", EpistemicReported)
	}
	if EpistemicInferred != "inferred" {
		t.Errorf("expected 'inferred', got %q", EpistemicInferred)
	}
}

func TestEntryFields(t *testing.T) {
	e := Entry{
		ID:         "abc123",
		Content:    "Go is statically typed",
		Epistemic:  EpistemicExperienced,
		Confidence: 1.0,
		Timestamp:  1718636400,
	}
	if e.ID != "abc123" {
		t.Errorf("ID mismatch")
	}
	if e.Content != "Go is statically typed" {
		t.Errorf("Content mismatch")
	}
	if e.Epistemic != EpistemicExperienced {
		t.Errorf("Epistemic mismatch")
	}
	if e.Confidence != 1.0 {
		t.Errorf("Confidence mismatch")
	}
	if len(e.Content) == 0 {
		t.Errorf("Content should not be empty")
	}
}

func TestSearchResult(t *testing.T) {
	sr := SearchResult{
		Entry: Entry{
			ID:      "def456",
			Content: "test",
		},
		Rank: 0.95,
		Room: "kg",
	}
	if sr.Rank != 0.95 {
		t.Errorf("Rank mismatch")
	}
	if sr.Room != "kg" {
		t.Errorf("Room mismatch")
	}
}
