// session/journal.go — Append-only event journal with replay.
//
// Events are journaled before dispatch. On restart, unapplied events
// are replayed to recover causality. The journal is a plain newline-delimited
// JSON file.
//
// Path is configured via NewJournalPath(); default fallback uses
// ~/.cache/goose/session_journal.jsonl for backward compatibility.

package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Journal is an append-only event log for session lifecycle events.
// Thread-safe.
type Journal struct {
	mu       sync.Mutex
	path     string
	applied  map[int64]bool // seq → applied
	nextSeq  int64
}

// NewJournal opens or creates the event journal at the default path
// (~/.cache/goose/session_journal.jsonl). Prefer NewJournalPath for
// library usage.
func NewJournal() (*Journal, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("home dir: %w", err)
	}
	return NewJournalPath(filepath.Join(home, ".cache", "goose", "session_journal.jsonl"))
}

// NewJournalPath opens or creates an event journal at the given path.
func NewJournalPath(path string) (*Journal, error) {
	j := &Journal{
		path:    path,
		applied: make(map[int64]bool),
		nextSeq: 1,
	}
	if _, err := os.Stat(path); err == nil {
		if err := j.loadApplied(); err != nil {
			return nil, fmt.Errorf("load journal: %w", err)
		}
	}
	return j, nil
}

// Load applied markers from existing journal lines.
// Format: {"seq":N, ...} — seq=0 means applied marker.
func (j *Journal) loadApplied() error {
	data, err := os.ReadFile(j.path)
	if err != nil {
		return err
	}
	lines := splitLines(string(data))
	for _, line := range lines {
		if line == "" {
			continue
		}
		var marker struct {
			Seq        int64 `json:"seq"`
			SeqApplied int64 `json:"seq_applied,omitempty"`
		}
		if err := json.Unmarshal([]byte(line), &marker); err != nil {
			continue
		}
		if marker.SeqApplied > 0 {
			j.applied[marker.SeqApplied] = true
			if marker.SeqApplied >= j.nextSeq {
				j.nextSeq = marker.SeqApplied + 1
			}
		} else if marker.Seq >= j.nextSeq {
			j.nextSeq = marker.Seq + 1
		}
	}
	return nil
}

// Append writes an event to the journal and returns its sequence number.
func (j *Journal) Append(e LifecycleEvent) (int64, error) {
	j.mu.Lock()
	defer j.mu.Unlock()

	seq := j.nextSeq
	j.nextSeq++
	e.Seq = seq

	f, err := os.OpenFile(j.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return 0, fmt.Errorf("journal append: %w", err)
	}
	defer f.Close()

	b, err := json.Marshal(e)
	if err != nil {
		return 0, fmt.Errorf("journal marshal: %w", err)
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		return 0, fmt.Errorf("journal write: %w", err)
	}
	return seq, nil
}

// MarkApplied records that an event has been successfully handled.
func (j *Journal) MarkApplied(seq int64) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	j.applied[seq] = true

	f, err := os.OpenFile(j.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("journal mark: %w", err)
	}
	defer f.Close()

	marker := struct {
		SeqApplied int64 `json:"seq_applied"`
	}{SeqApplied: seq}
	b, _ := json.Marshal(marker)
	if _, err := f.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("journal mark write: %w", err)
	}
	return nil
}

// Unapplied returns all events that haven't been marked as applied.
func (j *Journal) Unapplied() ([]LifecycleEvent, error) {
	j.mu.Lock()
	defer j.mu.Unlock()

	data, err := os.ReadFile(j.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var events []LifecycleEvent
	lines := splitLines(string(data))
	for _, line := range lines {
		if line == "" {
			continue
		}
		var e LifecycleEvent
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		if e.Seq == 0 {
			continue
		}
		if !j.applied[e.Seq] {
			events = append(events, e)
		}
	}
	return events, nil
}

// NextSeq returns the next sequence number (for testing).
func (j *Journal) NextSeq() int64 {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.nextSeq
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}