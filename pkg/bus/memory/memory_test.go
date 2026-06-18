package memory

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/QguAr71/sysq-libs/pkg/bus"
)

func TestInMemoryBus_PublishSubscribe(t *testing.T) {
	b := New()
	ctx := context.Background()

	var received []bus.Message
	var mu sync.Mutex

	_, err := b.Subscribe(ctx, "wing.*.deployed", func(msg bus.Message) {
		mu.Lock()
		received = append(received, msg)
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// Publish
	msg := bus.Message{
		ID:      "test-1",
		Payload: []byte(`{"wing":"abc123"}`),
	}
	if err := b.Publish(ctx, "wing.abc123.deployed", msg); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	// Allow goroutine to process
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("expected 1 message, got %d", len(received))
	}
	if received[0].ID != "test-1" {
		t.Errorf("msg ID: got %s, want test-1", received[0].ID)
	}
	if received[0].Topic != "wing.abc123.deployed" {
		t.Errorf("topic: got %s, want wing.abc123.deployed", received[0].Topic)
	}
}

func TestInMemoryBus_NoMatch(t *testing.T) {
	b := New()
	ctx := context.Background()

	count := 0
	b.Subscribe(ctx, "system.health", func(msg bus.Message) {
		count++
	})

	b.Publish(ctx, "wing.deployed", bus.Message{ID: "test"})
	time.Sleep(50 * time.Millisecond)

	if count != 0 {
		t.Errorf("expected 0 messages, got %d", count)
	}
}

func TestInMemoryBus_GlobMatch(t *testing.T) {
	b := New()
	ctx := context.Background()

	count := 0
	b.Subscribe(ctx, "wing.*.deployed", func(msg bus.Message) { count++ })

	tests := []string{
		"wing.abc123.deployed",      // match
		"wing.xyz.deployed",         // match
		"wing.abc123.switched",      // no match
		"superwing.echo.deployed",   // no match (4 segments vs 3)
	}
	expected := 0
	for _, topic := range tests {
		b.Publish(ctx, topic, bus.Message{ID: "test"})
		if topic == "wing.abc123.deployed" || topic == "wing.xyz.deployed" {
			expected++
		}
	}
	time.Sleep(50 * time.Millisecond)

	if count != expected {
		t.Errorf("expected %d matches, got %d", expected, count)
	}
}

func TestInMemoryBus_CloseUnsubscribesAll(t *testing.T) {
	b := New()
	ctx := context.Background()

	count := 0
	b.Subscribe(ctx, "test.topic", func(msg bus.Message) { count++ })

	b.Publish(ctx, "test.topic", bus.Message{ID: "before-close"})
	time.Sleep(50 * time.Millisecond)

	b.Close()

	b.Publish(ctx, "test.topic", bus.Message{ID: "after-close"})
	time.Sleep(50 * time.Millisecond)

	if count != 1 {
		t.Errorf("expected 1 message (before close), got %d", count)
	}
}

func TestInMemoryBus_SlowSubscriber(t *testing.T) {
	b := New()
	ctx := context.Background()

	b.Subscribe(ctx, "slow.topic", func(msg bus.Message) {
		time.Sleep(6 * time.Second) // exceeds 5s timeout
	})

	// Should not block or panic — just log warning
	done := make(chan bool, 1)
	go func() {
		b.Publish(ctx, "slow.topic", bus.Message{ID: "slow"})
		done <- true
	}()

	select {
	case <-done:
		// OK — Publish didn't block
	case <-time.After(8 * time.Second):
		t.Fatal("Publish blocked forever on slow subscriber")
	}
}

func TestInMemoryBus_RequestNotSupported(t *testing.T) {
	b := New()
	_, err := b.Request(context.Background(), "test", bus.Message{}, 1*time.Second)
	if err == nil {
		t.Error("expected error for Request on InMemoryBus")
	}
}

func TestMatchTopic(t *testing.T) {
	tests := []struct{ pattern, topic string; want bool }{
		{"*", "anything", true},
		{"wing.*.deployed", "wing.abc.deployed", true},
		{"wing.*.deployed", "wing.abc.switched", false},
		{"wing.*.deployed", "wing.abc.xyz.deployed", false},
		{"system.health", "system.health", true},
		{"system.health", "system.mode", false},
		{"*.created", "anything.created", true},
	}
	for _, tt := range tests {
		got := matchTopic(tt.pattern, tt.topic)
		if got != tt.want {
			t.Errorf("matchTopic(%q, %q) = %v, want %v", tt.pattern, tt.topic, got, tt.want)
		}
	}
}
