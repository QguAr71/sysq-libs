// token.go — Token lifecycle: CAS rotation, session tokens, grace period.
//
// Token = what you are (bearer, C1: hash).
// Signing_secret = what you can prove (MAC key, AES-256-GCM encrypted).
//
// Rotation uses CAS (compare-and-swap) with expected_version.
// Grace period: 5s for both old token AND old signing_secret.

package access

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrTokenVersionMismatch = errors.New("access: token version mismatch — another process already rotated")
	ErrTokenGraceExpired    = errors.New("access: token grace expired")
)

// ── Token Rotator ───────────────────────────────────────────────────────────

type TokenRotator struct {
	am        *AccountManager
	masterKey *MasterKey
	mu        sync.Mutex // per-account rotation lock
}

// NewTokenRotator creates the token rotator.
func NewTokenRotator(am *AccountManager, masterKey *MasterKey) *TokenRotator {
	return &TokenRotator{am: am, masterKey: masterKey}
}

// RotateResult is returned on successful token rotation.
type RotateResult struct {
	Token          string // new plaintext token (sent once)
	SigningSecret  string // new plaintext signing_secret (sent once)
	Version        int
}

// RotateToken performs CAS rotation.
// oldToken: current plaintext token
// expectedVersion: what the agent thinks is the current version
//
// If expected_version != actual → ErrTokenVersionMismatch → agent retries.
// On success: new token + new signing_secret returned once.
// Old token + old signing_secret remain valid for 5s grace period.
func (tr *TokenRotator) RotateToken(accountName string, oldToken []byte, expectedVersion int) (*RotateResult, error) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	acct, err := tr.am.ReadAccount(accountName)
	if err != nil {
		return nil, err
	}

	if acct.IsRevoked() {
		return nil, ErrTokenRevoked
	}

	// Verify old token
	oldTokenHash := hex.EncodeToString(hashBytes(oldToken))
	if oldTokenHash != acct.TokenHash {
		return nil, ErrTokenInvalid
	}

	// CAS: check version
	if expectedVersion != acct.TokenVersion {
		return nil, ErrTokenVersionMismatch
	}

	// Generate new
	newToken := generateRandom256()
	newSigningSecret := generateRandom256()

	// Encrypt new signing_secret
	newSigningSecretEnc, err := EncryptSecret(newSigningSecret, tr.masterKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt signing secret: %w", err)
	}

	newTokenHash := hex.EncodeToString(hashBytes(newToken))

	// Store old for grace period
	oldSigningSecretEnc := acct.SigningSecretEnc

	// Update account — atomic write
	acct.TokenHash = newTokenHash
	acct.SigningSecretEnc = newSigningSecretEnc
	acct.TokenVersion = expectedVersion + 1

	if err := tr.am.WriteAccount(acct); err != nil {
		return nil, err
	}

	// Register grace period for old credentials (5s)
	graceDuration := 5 * time.Second
	registerGrace(accountName, oldTokenHash, oldSigningSecretEnc, graceDuration)

	return &RotateResult{
		Token:         hex.EncodeToString(newToken),
		SigningSecret: hex.EncodeToString(newSigningSecret),
		Version:       acct.TokenVersion,
	}, nil
}

// ── Grace period ────────────────────────────────────────────────────────────

type graceEntry struct {
	account          string
	oldTokenHash     string
	oldSigningSecretEnc string
	expiresAt        time.Time
}

var (
	graceMu    sync.Mutex
	graceTable = make(map[string]*graceEntry)
)

func registerGrace(account, oldTokenHash, oldSigningSecretEnc string, dur time.Duration) {
	graceMu.Lock()
	defer graceMu.Unlock()
	graceTable[account] = &graceEntry{
		account:            account,
		oldTokenHash:       oldTokenHash,
		oldSigningSecretEnc: oldSigningSecretEnc,
		expiresAt:           time.Now().Add(dur),
	}
}

// CheckGrace checks if the old token/signing_secret is still valid under grace.
func CheckGrace(account string) (*graceEntry, bool) {
	graceMu.Lock()
	defer graceMu.Unlock()
	entry, ok := graceTable[account]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		delete(graceTable, account)
		return nil, false
	}
	return entry, true
}

// ── Session Token ───────────────────────────────────────────────────────────

type Session struct {
	ID             string
	Account        string
	Capabilities   []string // subset of master capabilities
	IssuedAt       time.Time
	ExpiresAt      time.Time
	LastValidated  time.Time
}

var (
	sessionMu     sync.RWMutex
	sessionCache  = make(map[string]*Session) // key: session_token
)

// CreateSession creates a new session token with the given capabilities subset.
// Capabilities must be a subset of master (C7).
func CreateSession(account string, masterCaps []string, requestedCaps []string, ttl time.Duration) (*Session, string, error) {
	// Validate requested ⊆ master
	masterSet := make(map[string]bool)
	for _, cap := range masterCaps {
		masterSet[cap] = true
	}
	for _, cap := range requestedCaps {
		if !masterSet[cap] {
			return nil, "", fmt.Errorf("session: requested capability %s not in master", cap)
		}
	}

	token := make([]byte, 32)
	rand.Read(token)
	tokenStr := hex.EncodeToString(token)

	now := time.Now()
	sess := &Session{
		ID:            tokenStr[:8],
		Account:       account,
		Capabilities:  requestedCaps,
		IssuedAt:      now,
		ExpiresAt:     now.Add(ttl),
		LastValidated: now,
	}

	sessionMu.Lock()
	sessionCache[tokenStr] = sess
	sessionMu.Unlock()

	return sess, tokenStr, nil
}

// ValidateSession validates a session token and checks if it's still valid.
// Also triggers revalidation if 30min passed since last validation.
func ValidateSession(token string, am *AccountManager) (*Session, error) {
	sessionMu.RLock()
	sess, ok := sessionCache[token]
	sessionMu.RUnlock()

	if !ok {
		return nil, errors.New("session: token not found")
	}
	if time.Now().After(sess.ExpiresAt) {
		sessionMu.Lock()
		delete(sessionCache, token)
		sessionMu.Unlock()
		return nil, errors.New("session: token expired")
	}

	// Revalidation: every 30 minutes, check master capabilities haven't changed
	if time.Since(sess.LastValidated) > 30*time.Minute {
		acct, err := am.ReadAccount(sess.Account)
		if err != nil {
			return nil, err
		}
		if acct.IsRevoked() {
			sessionMu.Lock()
			delete(sessionCache, token)
			sessionMu.Unlock()
			return nil, ErrTokenRevoked
		}

		// Check that session caps are still subset of master
		masterSet := make(map[string]bool)
		for _, cap := range acct.Capabilities {
			masterSet[cap] = true
		}
		for _, cap := range sess.Capabilities {
			if !masterSet[cap] {
				// Master capability removed — session invalid
				sessionMu.Lock()
				delete(sessionCache, token)
				sessionMu.Unlock()
				return nil, errors.New("session: master capability revoked")
			}
		}

		sess.LastValidated = time.Now()
	}

	return sess, nil
}

// RevokeSession removes a session token from cache.
func RevokeSession(token string) {
	sessionMu.Lock()
	delete(sessionCache, token)
	sessionMu.Unlock()
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func hashBytes(data []byte) []byte {
	hash := sha256.Sum256(data)
	return hash[:]
}
