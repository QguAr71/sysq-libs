package memory

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/QguAr71/sysq-libs/pkg/bus"
)

// InMemoryBus is a goroutine-safe, single-process event bus.
// Messages are delivered synchronously to all matching subscribers.
// No persistence, no retry, no request-response.
//
// Suitable for Lazarus MVP. Supervisor uses this for tests,
// then swaps to NATSBus in production.
type InMemoryBus struct {
	mu   sync.RWMutex
	subs []subscription
}

type subscription struct {
	topic   string
	handler func(bus.Message)
	closed  bool
}

// New creates a ready-to-use InMemoryBus.
func New() *InMemoryBus {
	return &InMemoryBus{}
}

// Publish sends a message synchronously to all matching subscribers.
// Slow handlers (>5s) are dropped with a log warning.
func (b *InMemoryBus) Publish(ctx context.Context, topic string, msg bus.Message) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	msg.Topic = topic
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	if msg.ID == "" {
		msg.ID = fmt.Sprintf("%x", time.Now().UnixNano())
	}

	for _, sub := range b.subs {
		if sub.closed {
			continue
		}
		if !matchTopic(sub.topic, topic) {
			continue
		}
		b.deliver(ctx, sub, msg)
	}
	return nil
}

func (b *InMemoryBus) deliver(ctx context.Context, sub subscription, msg bus.Message) {
	done := make(chan struct{}, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[bus] subscriber panic on topic=%s: %v", msg.Topic, r)
			}
			done <- struct{}{}
		}()
		sub.handler(msg)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		log.Printf("[bus] WARNING: slow subscriber on topic=%s — dropped", msg.Topic)
	case <-ctx.Done():
		log.Printf("[bus] context cancelled during delivery on topic=%s", msg.Topic)
	}
}

// Subscribe registers a handler for the given topic glob.
func (b *InMemoryBus) Subscribe(ctx context.Context, topic string, handler func(bus.Message)) (bus.Subscription, error) {
	s := subscription{topic: topic, handler: handler}
	b.mu.Lock()
	b.subs = append(b.subs, s)
	b.mu.Unlock()
	return &subHandle{b: b, s: &b.subs[len(b.subs)-1]}, nil
}

// Request is NOT supported by InMemoryBus. Returns an error.
// Use NATSBus for request-response patterns.
func (b *InMemoryBus) Request(ctx context.Context, topic string, msg bus.Message, timeout time.Duration) (bus.Message, error) {
	return bus.Message{}, fmt.Errorf("InMemoryBus does not support Request — use NATSBus")
}

// Close unsubscribes all handlers.
func (b *InMemoryBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := range b.subs {
		b.subs[i].closed = true
	}
	b.subs = nil
	return nil
}

type subHandle struct {
	b *InMemoryBus
	s *subscription
}

func (h *subHandle) Unsubscribe() error {
	h.b.mu.Lock()
	defer h.b.mu.Unlock()
	h.s.closed = true
	return nil
}

// matchTopic checks if a topic matches a glob pattern.
// Supported: exact match, trailing '*', mid '*' (e.g., "wing.*.deployed").
func matchTopic(pattern, topic string) bool {
	if pattern == topic {
		return true
	}
	if pattern == "*" {
		return true
	}
	// Simple glob: split both, compare segment by segment
	pp := splitTopic(pattern)
	tp := splitTopic(topic)
	if len(pp) != len(tp) {
		return false
	}
	for i := range pp {
		if pp[i] == "*" || pp[i] == tp[i] {
			continue
		}
		return false
	}
	return true
}

func splitTopic(topic string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(topic); i++ {
		if topic[i] == '.' {
			parts = append(parts, topic[start:i])
			start = i + 1
		}
	}
	parts = append(parts, topic[start:])
	return parts
}
