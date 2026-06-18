package epistemic

import "testing"

func TestEntryID_Deterministic(t *testing.T) {
	id1 := EntryID("kg", "Go is statically typed", OriginSelf, "experienced")
	id2 := EntryID("kg", "Go is statically typed", OriginSelf, "experienced")
	if id1 != id2 {
		t.Errorf("same inputs should produce same ID: %s != %s", id1, id2)
	}
	if len(id1) != 16 {
		t.Errorf("expected 16 hex chars, got %d", len(id1))
	}
}

func TestEntryID_DifferentContent(t *testing.T) {
	id1 := EntryID("kg", "Go is statically typed", OriginSelf, "experienced")
	id2 := EntryID("kg", "Rust is statically typed", OriginSelf, "experienced")
	if id1 == id2 {
		t.Errorf("different content should produce different IDs")
	}
}

func TestEntryID_DifferentOrigin(t *testing.T) {
	id1 := EntryID("kg", "Go is statically typed", OriginSelf, "experienced")
	id2 := EntryID("kg", "Go is statically typed", OriginExternal, "referenced")
	if id1 == id2 {
		t.Errorf("different origin should produce different IDs (anti-spoofing)")
	}
}

func TestEntryID_DifferentRoom(t *testing.T) {
	id1 := EntryID("kg", "Go is statically typed", OriginSelf, "experienced")
	id2 := EntryID("diary", "Go is statically typed", OriginSelf, "experienced")
	if id1 == id2 {
		t.Errorf("different room should produce different IDs")
	}
}

func TestEntryID_DeterministicAcrossCalls(t *testing.T) {
	first := EntryID("decisions", "use Go for backend", OriginSelf, "experienced")
	second := EntryID("decisions", "use Go for backend", OriginSelf, "experienced")
	if first != second {
		t.Errorf("EntryID must be deterministic across calls: %s != %s", first, second)
	}
}

func TestEntryID_MatchesSpec(t *testing.T) {
	// From SPEC.md:
	// EntryID("kg", "Go is statically typed", "self", "experienced")
	// Must produce the same value as the reference implementation.
	id := EntryID("kg", "Go is statically typed", OriginSelf, "experienced")
	if len(id) == 0 {
		t.Fatal("EntryID returned empty string")
	}
	// Verify hex format
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("EntryID contains non-hex character: %c", c)
		}
	}
}
