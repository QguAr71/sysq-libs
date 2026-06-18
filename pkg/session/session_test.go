package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLifecycleFSM(t *testing.T) {
	// Terminal states
	for _, s := range []LifecycleState{ExitedZero, ExitedNonZero, ExitedUnknown} {
		if !s.IsTerminal() {
			t.Errorf("%s should be terminal", s)
		}
	}

	// Non-terminal
	for _, s := range []LifecycleState{Unknown, Starting, Alive} {
		if s.IsTerminal() {
			t.Errorf("%s should NOT be terminal", s)
		}
	}

	// Valid transitions
	tests := []struct{ from, to LifecycleState }{
		{Unknown, Starting},
		{Starting, Alive},
		{Alive, ExitedZero},
		{Alive, ExitedNonZero},
	}
	for _, tt := range tests {
		if !ValidTransition(tt.from, tt.to) {
			t.Errorf("transition %s→%s should be valid", tt.from, tt.to)
		}
	}

	// Invalid
	if ValidTransition(ExitedZero, Alive) {
		t.Error("Exited→Alive should be invalid")
	}
}

func TestExitedState(t *testing.T) {
	if ExitedState(0) != ExitedZero {
		t.Error("exit 0 → ExitedZero")
	}
	if ExitedState(1) != ExitedNonZero {
		t.Error("exit 1 → ExitedNonZero")
	}
	if ExitedState(42) != ExitedNonZero {
		t.Error("exit 42 → ExitedNonZero")
	}
	if ExitedState(-1) != ExitedNonZero {
		t.Error("exit -1 → ExitedNonZero")
	}
}

func TestJournal_AppendAndReplay(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "test_journal.jsonl")
	j, err := NewJournalPath(tmp)
	if err != nil {
		t.Fatalf("NewJournalPath: %v", err)
	}

	// Append events
	for i := 0; i < 3; i++ {
		seq, err := j.Append(LifecycleEvent{
			From:   Starting,
			To:     Alive,
			Reason: "test",
		})
		if err != nil {
			t.Fatalf("Append #%d: %v", i, err)
		}
		if seq != int64(i+1) {
			t.Errorf("seq = %d, want %d", seq, i+1)
		}
	}

	// Mark first as applied
	if err := j.MarkApplied(1); err != nil {
		t.Fatalf("MarkApplied: %v", err)
	}

	// Unapplied should return 2 events
	unapplied, err := j.Unapplied()
	if err != nil {
		t.Fatalf("Unapplied: %v", err)
	}
	if len(unapplied) != 2 {
		t.Errorf("unapplied = %d, want 2", len(unapplied))
	}

	// Re-open journal — should recover
	j2, err := NewJournalPath(tmp)
	if err != nil {
		t.Fatalf("re-open: %v", err)
	}
	if j2.NextSeq() != 4 {
		t.Errorf("NextSeq after replay = %d, want 4", j2.NextSeq())
	}

	// Should have same unapplied events
	unapplied2, _ := j2.Unapplied()
	if len(unapplied2) != 2 {
		t.Errorf("unapplied after replay = %d, want 2", len(unapplied2))
	}
}

func TestJournal_Empty(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "empty.jsonl")
	j, _ := NewJournalPath(tmp)
	events, err := j.Unapplied()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Error("empty journal should have no events")
	}
}

func TestJournal_MissingFile(t *testing.T) {
	home, _ := os.UserHomeDir()
	j, err := NewJournalPath(filepath.Join(home, ".cache", "NONEXISTENT_DIR", "test.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if j.NextSeq() != 1 {
		t.Errorf("new journal should start at seq 1, got %d", j.NextSeq())
	}
}

func TestLifecycleEvent_String(t *testing.T) {
	e := LifecycleEvent{
		Seq:      1,
		From:     Starting,
		To:       Alive,
		Reason:   "session_started",
		ExitCode: 0,
	}
	s := e.String()
	if len(s) == 0 {
		t.Error("String() should not be empty")
	}
}
