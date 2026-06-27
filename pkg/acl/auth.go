// auth.go — Token validation + Nonce cache + Clock skew checks.
//
// External path (MCP Bridge → AuthorizeExternal):
//   - Verify token: SHA256(token_plaintext) == token_hash
//   - Check nonce: CheckAndMark (atomic check+mark), TTL 5min
//
// HMAC signing + signature verification przeniesione do _future/hmac.go
// (FUTURE: CERBER_SIGNED_REQUESTS=true dla zdalnych agentów MCP).

package access

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// ── Errors ──────────────────────────────────────────────────────────────────

var (
	ErrTokenInvalid = errors.New("access: token invalid")
	ErrTokenRevoked = errors.New("access: token revoked")
	ErrNonceReused  = errors.New("access: nonce already used")
	ErrClockSkew    = errors.New("access: clock skew exceeded")
	ErrTokenExpired = errors.New("access: token expired")
)

// ── Token validation ────────────────────────────────────────────────────────

// ValidateToken checks that SHA256(decoded_token_bytes) == token_hash.
// token_plaintext is hex-encoded — we decode it first, then hash.
// C1: token is always stored as SHA256 hash of the raw bytes.
func ValidateToken(tokenHex, tokenHash string) bool {
	decoded, err := hex.DecodeString(tokenHex)
	if err != nil {
		return false
	}
	hash := sha256.Sum256(decoded)
	return subtle.ConstantTimeCompare(
		[]byte(hex.EncodeToString(hash[:])),
		[]byte(tokenHash),
	) == 1
}

// ── Nonce cache ─────────────────────────────────────────────────────────────

// NonceCache holds recently seen nonces to prevent replay.
// Default size: 100k entries, configurable via CERBER_NONCE_CACHE_SIZE.
type NonceCache struct {
	mu      sync.Mutex
	entries map[string]time.Time
	maxSize int
	ttl     time.Duration
}

// NewNonceCache creates a cache with the given max entries and TTL.
func NewNonceCache(maxEntries int, ttl time.Duration) *NonceCache {
	return &NonceCache{
		entries: make(map[string]time.Time, maxEntries),
		maxSize: maxEntries,
		ttl:     ttl,
	}
}

// CheckAndMark atomically checks if the nonce is valid and marks it as used.
// Atomic — no TOCTOU window between check and mark (audyt B, runda 2, D5).
func (nc *NonceCache) CheckAndMark(nonce string) error {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	now := time.Now()

	if ts, ok := nc.entries[nonce]; ok {
		if now.Sub(ts) < nc.ttl {
			return ErrNonceReused
		}
		// Expired — remove and allow
		delete(nc.entries, nonce)
	}

	// Mark (inline to avoid double-lock)
	if len(nc.entries) >= nc.maxSize {
		for k, v := range nc.entries {
			if now.Sub(v) >= nc.ttl {
				delete(nc.entries, k)
			}
		}
	}
	nc.entries[nonce] = now
	return nil
}

// Check checks if the nonce is already in the cache or expired (thread-safe).
// Returns nil if the nonce is valid (new or expired).
func (nc *NonceCache) Check(nonce string, now time.Time) error {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	if ts, ok := nc.entries[nonce]; ok {
		if now.Sub(ts) < nc.ttl {
			return ErrNonceReused
		}
		// Expired — remove and allow
		delete(nc.entries, nonce)
	}
	return nil
}

// Mark adds the nonce to the cache (thread-safe).
func (nc *NonceCache) Mark(nonce string, now time.Time) {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	if len(nc.entries) >= nc.maxSize {
		// Purge expired entries
		for k, v := range nc.entries {
			if now.Sub(v) >= nc.ttl {
				delete(nc.entries, k)
			}
		}
	}
	nc.entries[nonce] = now
}

// Add is a convenience method that calls Mark (public, thread-safe).
func (nc *NonceCache) Add(nonce string) {
	nc.Mark(nonce, time.Now())
}

// ── Clock skew ──────────────────────────────────────────────────────────────

// CheckTimestamp verifies that the request timestamp is within allowed skew.
func CheckTimestamp(ts int64, skew time.Duration) error {
	now := time.Now().Unix()
	delta := now - ts
	if delta < 0 {
		delta = -delta
	}
	if delta > int64(skew.Seconds()) {
		return ErrClockSkew
	}
	return nil
}
