package modes

import (
	"testing"
	"time"
)

func TestModes_InitialState(t *testing.T) {
	m := NewManager()
	if m.Current() != Init {
		t.Errorf("initial mode should be INIT, got %s", m.Current())
	}
	if m.state.Since.IsZero() {
		t.Error("timestamp should be set")
	}
}

func TestModes_ValidTransitions(t *testing.T) {
	tests := []struct {
		from, to Mode
	}{
		{Init, Guarded},
		{Guarded, Active},
		{Active, Degraded},
		{Degraded, Active},
		{Active, Emergency},
		{Emergency, Degraded},
		{Emergency, Panic},
		{Panic, Recovery},
		{Recovery, Active},
		{Active, Maintenance},
		{Maintenance, Active},
		{Degraded, Panic},
		{Init, Degraded},
	}
	for _, tc := range tests {
		t.Run(string(tc.from)+"→"+string(tc.to), func(t *testing.T) {
			m := NewManager()
			// Walk to starting mode
			walkTo(m, tc.from, t)
			err := m.Transition(tc.to, "test")
			if err != nil {
				t.Fatalf("valid transition %s→%s rejected: %v", tc.from, tc.to, err)
			}
			if m.Current() != tc.to {
				t.Errorf("expected %s, got %s", tc.to, m.Current())
			}
		})
	}
}

func TestModes_InvalidTransitions(t *testing.T) {
	tests := []struct {
		from, to Mode
	}{
		{Active, Init},       // can't go back to init
		{Degraded, Init},     // can't go back to init
		{Panic, Active},      // panic requires recovery first
		{Panic, Init},        // panic only goes to recovery
		{Init, Active},       // must go through guarded
		{Guarded, Emergency}, // guarded can't go directly to emergency
	}
	for _, tc := range tests {
		t.Run(string(tc.from)+"→"+string(tc.to), func(t *testing.T) {
			m := NewManager()
			walkTo(m, tc.from, t)
			err := m.Transition(tc.to, "test")
			if err == nil {
				t.Errorf("invalid transition %s→%s was accepted", tc.from, tc.to)
			}
		})
	}
}

func TestModes_StateMetadata(t *testing.T) {
	m := NewManager()
	walkTo(m, Guarded, t)
	m.Transition(Active, "boot completed")

	s := m.State()
	if s.Mode != Active {
		t.Errorf("mode should be ACTIVE, got %s", s.Mode)
	}
	if s.PrevMode != Guarded {
		t.Errorf("prev mode should be GUARDED, got %s", s.PrevMode)
	}
	if s.Reason != "boot completed" {
		t.Errorf("reason should be recorded, got %s", s.Reason)
	}
	if s.Since.IsZero() || s.PrevSince.IsZero() {
		t.Error("timestamps should be set")
	}
}

func TestModes_IsOperational(t *testing.T) {
	m := NewManager()
	if m.IsOperational() {
		t.Error("INIT should not be operational")
	}
	walkTo(m, Guarded, t)
	if !m.IsOperational() {
		t.Error("GUARDED should be operational")
	}
	m.Transition(Active, "ok")
	if !m.IsOperational() {
		t.Error("ACTIVE should be operational")
	}
	m.Transition(Degraded, "invariant broken")
	if m.IsOperational() {
		t.Error("DEGRADED should not be operational")
	}
}

func TestModes_IsReadOnly(t *testing.T) {
	m := NewManager()
	walkTo(m, Active, t)
	if m.IsReadOnly() {
		t.Error("ACTIVE should not be read-only")
	}
	walkTo(m, Emergency, t)
	if !m.IsReadOnly() {
		t.Error("EMERGENCY should be read-only")
	}
	walkTo(m, Panic, t)
	if !m.IsReadOnly() {
		t.Error("PANIC should be read-only")
	}
}

func TestModes_IsRecovering(t *testing.T) {
	m := NewManager()
	walkTo(m, Recovery, t)
	if !m.IsRecovering() {
		t.Error("RECOVERY should be recovering")
	}
}

// walkTo advances the state machine to the desired starting mode.
func walkTo(m *Manager, target Mode, t *testing.T) {
	t.Helper()
	if m.Current() == target {
		return
	}
	switch target {
	case Guarded:
		m.Transition(Guarded, "test")
	case Active:
		walkTo(m, Guarded, t)
		m.Transition(Active, "test")
	case Degraded:
		walkTo(m, Active, t)
		m.Transition(Degraded, "test")
	case Emergency:
		walkTo(m, Active, t)
		m.Transition(Emergency, "test")
	case Panic:
		walkTo(m, Emergency, t)
		m.Transition(Panic, "test")
	case Recovery:
		walkTo(m, Panic, t)
		m.Transition(Recovery, "test")
	case Maintenance:
		walkTo(m, Active, t)
		m.Transition(Maintenance, "test")
	default:
		t.Fatalf("walkTo: unsupported target %s", target)
	}
	if m.Current() != target {
		t.Fatalf("walkTo: failed to reach %s (at %s)", target, m.Current())
	}
	time.Sleep(1 * time.Millisecond) // ensure timestamps differ
}
