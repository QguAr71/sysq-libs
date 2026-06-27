// Package portfixer resolves TCP port conflicts with graceful degradation.
//
// Adapted from Citadel Advanced Port Manager v1.0 (bash).
// Algorithm: env override → default → next free in range → emergency fallback.
//
// Port Fixer is a generyczny Unit — nie zależy od Lazarusa.
// Importowany jako github.com/QguAr71/sysq-libs/pkg/portfixer.
//
// CONSTITUTION invarianty:
//
//	I-18  — graceful degradation (system never refuses to start)
//	III-31 — klocek nie może rozwalić innych (nil-safe bus, panic recovery)
//	III-38 — EventBus: publikuje port.assigned, port.conflict
//
// Bus integration:
//   - Publishes "port.assigned" — always, after every port resolution.
//     Subscribers: heartbeat, reconciler, session_unit, auto-discover.
//   - Publishes "port.conflict" — only when default port was busy.
//     Subscribers: scanner (risk_gap), mode_manager (GUARDED), reconciler (cleanup).
//   - Bus is nil-safe — all publish calls check before sending.
//
// Usage:
//
//	pf := portfixer.New(portfixer.Config{
//	    DefaultPort: 9595,
//	    RangeStart:  9595,
//	    RangeEnd:    9605,
//	    Bus:         bus, // optional, may be nil
//	})
//	port := pf.Resolve()
package portfixer

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

// ────────────────────────────────────────────────────────────
// EventBus — minimal interface, nil-safe
// ────────────────────────────────────────────────────────────

// Bus is a minimal event publisher interface.
// Must be nil-safe — all methods check before calling.
type Bus interface {
	Publish(ctx context.Context, topic string, msg Message) error
}

// Message wraps payload for bus delivery.
type Message struct {
	Payload []byte
}

// ────────────────────────────────────────────────────────────
// Config
// ────────────────────────────────────────────────────────────

// Config holds port resolution parameters.
type Config struct {
	// DefaultPort is the primary port to try.
	DefaultPort int

	// RangeStart and RangeEnd define the fallback scan range (inclusive).
	// If both are 0, defaults to [DefaultPort, DefaultPort+10].
	RangeStart int
	RangeEnd   int

	// EnvVar is the environment variable name that can override the port.
	// Default: "LAZARUSD_MCP_PORT".
	EnvVar string

	// Bus is an optional EventBus for publishing port events.
	// nil-safe — all publish calls check before sending.
	Bus Bus
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		DefaultPort: 9595,
		RangeStart:  9595,
		RangeEnd:    9605,
		EnvVar:      "LAZARUSD_MCP_PORT",
	}
}

// ────────────────────────────────────────────────────────────
// PortFixer
// ────────────────────────────────────────────────────────────

// PortFixer resolves a port with graceful degradation.
// nil-safe for Bus — all publish calls check before sending.
type PortFixer struct {
	cfg           Config
	cache         map[int]bool
	cacheMu       sync.RWMutex
	resolvedPort  int
	resolvedOnce  bool
	mu            sync.Mutex
}

// New creates a new PortFixer with the given config.
// Bus may be nil — all publishes will be no-ops.
func New(cfg Config) *PortFixer {
	if cfg.DefaultPort == 0 {
		cfg.DefaultPort = 9595
	}
	if cfg.RangeStart == 0 && cfg.RangeEnd == 0 {
		cfg.RangeStart = cfg.DefaultPort
		cfg.RangeEnd = cfg.DefaultPort + 10
	}
	if cfg.EnvVar == "" {
		cfg.EnvVar = "LAZARUSD_MCP_PORT"
	}
	return &PortFixer{
		cfg:   cfg,
		cache: make(map[int]bool),
	}
}

// Port returns the last resolved port, or 0 if not resolved yet.
func (pf *PortFixer) Port() int {
	pf.mu.Lock()
	defer pf.mu.Unlock()
	return pf.resolvedPort
}

// ────────────────────────────────────────────────────────────
// Resolution
// ────────────────────────────────────────────────────────────

// Resolve returns the port to bind to.
// Tries: env var → default → next free in range → emergency fallback.
//
// Publishes "port.assigned" always, "port.conflict" when fallback was needed.
// Idempotent — second call returns cached value.
func (pf *PortFixer) Resolve() int {
	pf.mu.Lock()
	if pf.resolvedOnce {
		port := pf.resolvedPort
		pf.mu.Unlock()
		return port
	}
	pf.mu.Unlock()

	port := pf.resolve()
	pf.mu.Lock()
	pf.resolvedPort = port
	pf.resolvedOnce = true
	pf.mu.Unlock()
	return port
}

func (pf *PortFixer) resolve() int {
	// 1. Environment variable override
	if pf.cfg.EnvVar != "" {
		if env := os.Getenv(pf.cfg.EnvVar); env != "" {
			if p, err := strconv.Atoi(env); err == nil && p > 0 {
				pf.publish("port.assigned", map[string]interface{}{
					"port":   p,
					"source": "env",
				})
				return p
			}
		}
	}

	// 2. Default free
	if !pf.isInUse(pf.cfg.DefaultPort) {
		pf.publish("port.assigned", map[string]interface{}{
			"port":   pf.cfg.DefaultPort,
			"source": "default",
		})
		return pf.cfg.DefaultPort
	}

	// 3. Conflict — find fallback in range (skip default)
	start := pf.cfg.RangeStart
	if start == pf.cfg.DefaultPort {
		start++
	}
	p := findAvailable(start, pf.cfg.RangeEnd, pf.isInUse)
	if p > 0 {
		pf.publish("port.conflict", map[string]interface{}{
			"port":     p,
			"previous": pf.cfg.DefaultPort,
			"reason":   "default busy",
		})
		pf.publish("port.assigned", map[string]interface{}{
			"port":   p,
			"source": "fallback",
		})

		os.Setenv(pf.cfg.EnvVar, strconv.Itoa(p))
		return p
	}

	// 4. I-18: graceful degradation — emergency fallback
	p = pf.cfg.DefaultPort + 100
	pf.publish("port.conflict", map[string]interface{}{
		"port":     p,
		"previous": pf.cfg.DefaultPort,
		"reason":   "no free ports in range",
	})

	os.Setenv(pf.cfg.EnvVar, strconv.Itoa(p))
	return p
}

// isInUse checks whether a TCP port is already listened on.
// Uses net.Dial with 50ms timeout. Results are cached until ClearCache.
func (pf *PortFixer) isInUse(port int) bool {
	pf.cacheMu.RLock()
	if inUse, ok := pf.cache[port]; ok {
		pf.cacheMu.RUnlock()
		return inUse
	}
	pf.cacheMu.RUnlock()

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
	if err != nil {
		pf.cacheMu.Lock()
		pf.cache[port] = false
		pf.cacheMu.Unlock()
		return false
	}
	conn.Close()

	pf.cacheMu.Lock()
	pf.cache[port] = true
	pf.cacheMu.Unlock()
	return true
}

// findAvailable scans [start, end] for the first free port.
func findAvailable(start, end int, check func(int) bool) int {
	for p := start; p <= end; p++ {
		if !check(p) {
			return p
		}
	}
	return -1
}

// ClearCache clears the port check cache so next check re-scans.
func (pf *PortFixer) ClearCache() {
	pf.cacheMu.Lock()
	for k := range pf.cache {
		delete(pf.cache, k)
	}
	pf.cacheMu.Unlock()
}

// ────────────────────────────────────────────────────────────
// Bus publish (nil-safe)
// ────────────────────────────────────────────────────────────

func (pf *PortFixer) publish(topic string, payload map[string]interface{}) {
	if pf.cfg.Bus == nil {
		return
	}
	data, _ := json.Marshal(payload)
	// Fire-and-forget — nie blokujemy startu systemu
	_ = pf.cfg.Bus.Publish(context.Background(), topic, Message{
		Payload: data,
	})
}
