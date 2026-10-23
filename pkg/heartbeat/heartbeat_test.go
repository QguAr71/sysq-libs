package heartbeat

import (
	"testing"
	"time"

	"github.com/QguAr71/sysq-core/pkg/modes"
)

func TestHeartbeat_Ping(t *testing.T) {
	mm := modes.NewManager()
	mm.Transition(modes.Guarded, "test")
	mm.Transition(modes.Active, "test")

	w := NewWatcher(mm, 100*time.Millisecond)
	w.Ping("echo")

	lp, missed, ok := w.Status("echo")
	if !ok {
		t.Fatal("echo should be tracked")
	}
	if lp.IsZero() {
		t.Error("last ping should be set")
	}
	if missed != 0 {
		t.Errorf("missed should reset to 0, got %d", missed)
	}
}

func TestHeartbeat_TimeoutDegrades(t *testing.T) {
	mm := modes.NewManager()
	mm.Transition(modes.Guarded, "test")
	mm.Transition(modes.Active, "test")

	w := NewWatcher(mm, 10*time.Millisecond)
	w.Ping("echo")

	// Wait for timeout (3 × 10ms = 30ms)
	time.Sleep(60 * time.Millisecond)
	w.check()

	if mm.Current() != modes.Degraded {
		t.Errorf("timeout should degrade mode, got %s", mm.Current())
	}

	_, missed, _ := w.Status("echo")
	if missed < 1 {
		t.Errorf("missed should be >= 1, got %d", missed)
	}
}

func TestHeartbeat_PanicAfterDegraded(t *testing.T) {
	mm := modes.NewManager()
	mm.Transition(modes.Guarded, "test")
	mm.Transition(modes.Active, "test")

	w := NewWatcher(mm, 10*time.Millisecond)
	w.Ping("echo")

	// First timeout → DEGRADED
	time.Sleep(50 * time.Millisecond)
	w.check()
	if mm.Current() != modes.Degraded {
		t.Fatalf("first timeout should degrade, got %s", mm.Current())
	}

	// Second + third timeout → PANIC
	time.Sleep(100 * time.Millisecond)
	w.check()
	time.Sleep(50 * time.Millisecond)
	w.check()

	if mm.Current() != modes.Panic {
		t.Errorf("3× degraded should panic, got %s", mm.Current())
	}
}

func TestHeartbeat_Recover(t *testing.T) {
	mm := modes.NewManager()
	mm.Transition(modes.Guarded, "test")
	mm.Transition(modes.Active, "test")

	w := NewWatcher(mm, 10*time.Millisecond)
	w.Ping("echo")

	// Force degrade
	time.Sleep(50 * time.Millisecond)
	w.check()
	if mm.Current() != modes.Degraded {
		t.Fatalf("should degrade, got %s", mm.Current())
	}

	// Ping → recover to ACTIVE
	w.Ping("echo")
	if mm.Current() != modes.Active {
		t.Errorf("ping should recover to ACTIVE, got %s", mm.Current())
	}
}

func TestHeartbeat_All(t *testing.T) {
	mm := modes.NewManager()
	mm.Transition(modes.Guarded, "test")
	mm.Transition(modes.Active, "test")

	w := NewWatcher(mm, time.Second)
	w.Ping("echo")
	w.Ping("mini")

	all := w.All()
	if len(all) != 2 {
		t.Errorf("should track 2 superwings, got %d", len(all))
	}
}
