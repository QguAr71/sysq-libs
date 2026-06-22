// Package bus defines the shared EventBus interface for SysQ projects.
//
// Design: unified contract for Lazarus (in-process signals) and Supervisor
// (multi-agent pub/sub). The interface is a superset — each project
// implements the methods it needs.
//
// Lazarus uses: Publish, Subscribe.
// Supervisor uses: Publish, Subscribe, Request (ACK/NACK).
//
// Backends:
//   - InMemoryBus (pkg/bus/memory/) — goroutine-safe, single-process
//   - NATSBus (Supervisor, external) — JetStream, persistent, multi-node
//
// Invariants:
//   B1: Bus is NOT source of truth. Citadel/StateStore is the only truth.
//   B2: Bus NEVER modifies state directly. Only signals.
//   B3: Bus is optional — systems operate fully without it.
//   B4: Subscribers must not block (handler timeout 5s).
package bus

import (
	"context"
	"time"
)

// Priority levels (0-4) for message ordering and QoS.
const (
	PriorityLow      = 0
	PriorityNormal   = 1
	PriorityHigh     = 2
	PriorityCritical = 3
	PrioritySystem   = 4
)

// ACK/NACK codes for request-response pattern.
// Used by Supervisor; Lazarus may use them in future.
const (
	ACKApplied     = "APPLIED"
	NACKTimeout    = "TIMEOUT"
	NACKBusy       = "BUSY"
	NACKInvalid    = "INVALID"
	NACKForbidden  = "FORBIDDEN"
	NACKNotFound   = "NOT_FOUND"
	NACKInternal   = "INTERNAL_ERROR"
	NACKRateLimit  = "RATE_LIMITED"
	NACKUnsupported = "UNSUPPORTED"
)

// Message is a bus event or command. Both publish-subscribe and
// request-response use the same structure.
type Message struct {
	ID            string    // unique message id (UUID or short hash)
	CorrelationID string    // for request-response chaining
	Topic         string    // where the message was published
	Timestamp     time.Time // when the message was created
	Priority      int       // 0-4, higher = more urgent
	TTL           time.Duration // max lifetime; 0 = no expiry
	Signature     []byte    // optional signature for critical commands
	TraceID       string    // for distributed tracing
	Mode          string    // operational mode of the sender
	Payload       []byte    // arbitrary payload (JSON recommended)
}

// Subscription represents an active topic subscription.
type Subscription interface {
	Unsubscribe() error
}

// Bus is the unified event bus interface.
//
// Real-world usage:
//
//	bus.Publish(ctx, "wing.abc123.deployed", msg)       // fire-and-forget
//	bus.Subscribe(ctx, "wing.*.deployed", handler)      // react to events
//	resp, _ := bus.Request(ctx, "system/mode/change", msg, 5*time.Second)  // ask + wait
type Bus interface {
	// Publish sends a message to a topic. Fire-and-forget.
	// Returns immediately; delivery is best-effort.
	Publish(ctx context.Context, topic string, msg Message) error

	// Subscribe registers a handler for messages on a topic.
	// Topic supports glob patterns (e.g., "wing.*.deployed").
	// Handler MUST NOT block longer than 5 seconds.
	Subscribe(ctx context.Context, topic string, handler func(Message)) (Subscription, error)

	// Request publishes a message and waits for a single response.
	// Used for commands that need acknowledgment.
	// Timeout after the given duration. Returns the response message.
	// Not all backends support this (InMemoryBus does not).
	Request(ctx context.Context, topic string, msg Message, timeout time.Duration) (Message, error)

	// RetainLast stores the last message for a topic so new subscribers
	// receive it immediately upon Subscribe. Similar to MQTT retained
	// messages. Only the most recent message per topic is retained.
	// Passing a nil Message clears the retained message.
	RetainLast(topic string, msg *Message)

	// Close shuts down the bus, unsubscribes all handlers,
	// and releases resources.
	Close() error
}
