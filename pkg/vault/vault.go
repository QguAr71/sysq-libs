// Package vault — Encrypted storage, intrusion detection, access camouflage.
//
// Vault provides ChaCha20-Poly1305 AEAD encryption for secret data, honeypot
// rooms that detect unauthorized access, and mirror responses for silent
// denial. Vault is a filter+transform layer — it does NOT create storage.
//
// Three subsystems:
//  1. Encryption — ChaCha20-Poly1305 AEAD (selective, per-room)
//  2. Honeypots — fake rooms; access triggers alert, never mutates storage
//  3. Mirror — returns fake data for unauthorized access, indistinguishable from empty
//
// Invariants:
//
//	V1: Only designated vault rooms are encrypted
//	V8: Honeypot MUST NOT mutate external state
//	V9: Mirror MUST NOT leak into search/ranking
//	M1: Mirror Non-Detectability — response ≡ empty room
//
// Imports:
//
//	"golang.org/x/crypto/chacha20poly1305"
//
// Usage:
//
//	cfg := vault.NewConfig()
//	cfg.MasterKeyPath = "/secure/path/master.key"
//	mgr, err := vault.NewManager(cfg)
//	if err != nil { ... }
//
//	encrypted, _ := mgr.Encrypt("secret data")
//	plain, _ := mgr.Decrypt(encrypted)
//
//	if result := mgr.CheckHoneypot("wing1", "diary", "citadel.read"); result.Tripped {
//	    log.Printf("INTRUSION: %v", result)
//	}
package vault

import (
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/chacha20poly1305"
)

const vaultPrefix = "LAZARUS_VAULT_V1:"

// VaultRoom is the room name for encrypted entries.
const VaultRoom = "vault"

// Manager handles encryption, honeypots, and mirror gallery.
type Manager struct {
	aead     cipher.AEAD // nil if no key loaded
	cfg      *Config
	logFn    func(level, msg string) // injected logger
}

// NewManager creates a vault manager from config.
// Returns nil, nil if no vault config present.
func NewManager(cfg *Config) (*Manager, error) {
	if cfg == nil {
		return nil, nil
	}

	vm := &Manager{
		cfg:   cfg,
		logFn: defaultLogger,
	}

	if len(cfg.Encrypted) > 0 {
		if err := vm.loadMasterKey(cfg.MasterKeyPath); err != nil {
			// Key missing — continue without encryption.
			// Encrypt will fail, Decrypt will pass through.
			vm.logFn("WARNING", fmt.Sprintf("master key not loaded: %v", err))
		}
	}

	return vm, nil
}

// SetLogger injects a custom logging function.
func (vm *Manager) SetLogger(fn func(level, msg string)) {
	vm.logFn = fn
}

func defaultLogger(level, msg string) {
	fmt.Fprintf(os.Stderr, "[vault] %s: %s\n", level, msg)
}

// loadMasterKey reads the key file and initializes ChaCha20-Poly1305 AEAD.
func (vm *Manager) loadMasterKey(path string) error {
	if path == "" {
		home := os.Getenv("HOME")
		if home == "" {
			return fmt.Errorf("HOME not set and vault.master_key_path not configured")
		}
		path = home + "/.config/sysq/vault/master.key"
	}

	keyBytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read master key %s: %w", path, err)
	}

	if len(keyBytes) < 32 {
		return fmt.Errorf("master key too short (%d bytes, need 32)", len(keyBytes))
	}

	key := make([]byte, 32)
	copy(key, keyBytes[:32])

	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return fmt.Errorf("init chacha20poly1305: %w", err)
	}
	vm.aead = aead

	return nil
}

// Loaded returns true if the encryption key is available.
func (vm *Manager) Loaded() bool {
	return vm.aead != nil
}

// Encrypt encrypts plaintext using ChaCha20-Poly1305.
// Returns LAZARUS_VAULT_V1:<base64> or error.
func (vm *Manager) Encrypt(plaintext string) (string, error) {
	if vm.aead == nil {
		return "", fmt.Errorf("vault: no master key loaded")
	}

	nonce := make([]byte, chacha20poly1305.NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("vault: generate nonce: %w", err)
	}

	ciphertext := vm.aead.Seal(nil, nonce, []byte(plaintext), nil)

	packed := make([]byte, chacha20poly1305.NonceSize+len(ciphertext))
	copy(packed[:chacha20poly1305.NonceSize], nonce)
	copy(packed[chacha20poly1305.NonceSize:], ciphertext)

	return vaultPrefix + base64.StdEncoding.EncodeToString(packed), nil
}

// Decrypt decrypts vault content. Returns plaintext unchanged if not encrypted.
func (vm *Manager) Decrypt(encoded string) (string, error) {
	if vm.aead == nil {
		return encoded, nil
	}

	if len(encoded) < len(vaultPrefix) || encoded[:len(vaultPrefix)] != vaultPrefix {
		return encoded, nil
	}

	b64 := encoded[len(vaultPrefix):]
	packed, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return encoded, fmt.Errorf("vault: base64 decode: %w", err)
	}

	if len(packed) < chacha20poly1305.NonceSize {
		return encoded, fmt.Errorf("vault: ciphertext too short")
	}

	nonce := packed[:chacha20poly1305.NonceSize]
	ciphertext := packed[chacha20poly1305.NonceSize:]

	plaintext, err := vm.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return encoded, fmt.Errorf("vault: decrypt: %w", err)
	}

	return string(plaintext), nil
}

// IsVaultRoom returns true if this wing/room is configured for encryption.
func (vm *Manager) IsVaultRoom(wing, room string) bool {
	if vm.cfg == nil {
		return false
	}
	for _, e := range vm.cfg.Encrypted {
		if e.Room == room && (e.Wing == wing || e.Wing == "" || wing == "") {
			return true
		}
	}
	return false
}

// HoneypotResult describes a triggered honeypot.
type HoneypotResult struct {
	Tripped bool
	Room    string
	Wing    string
	Method  string
}

// CheckHoneypot checks if the access touches a honeypot.
// Returns nil if no honeypot tripped.
// Honeypot check MUST NOT mutate Citadel state (V8).
func (vm *Manager) CheckHoneypot(wing, room, method string) *HoneypotResult {
	if vm.cfg == nil || len(vm.cfg.Honeypots) == 0 {
		return nil
	}

	for _, hp := range vm.cfg.Honeypots {
		if hp.Room != room {
			continue
		}
		if hp.Wing != "" && hp.Wing != wing {
			continue
		}
		if hp.On == "read" && !isReadMethod(method) {
			continue
		}
		// Tripped
		vm.logFn("WARNING", fmt.Sprintf("honeypot tripped: room=%s wing=%s method=%s", room, wing, method))
		return &HoneypotResult{Tripped: true, Room: room, Wing: wing, Method: method}
	}

	return nil
}

// MirrorResult describes a mirror match.
type MirrorResult struct {
	Mirror   bool
	FakeData string
	Room     string
}

// CheckMirror checks if the access should be mirrored.
// Returns fake data for unauthorized access.
// Mirror MUST NOT leak into KG/PSL/ranking (V9).
// Mirror MUST be non-detectable (M1) — response ≡ empty room.
func (vm *Manager) CheckMirror(wing, room, method string) *MirrorResult {
	if vm.cfg == nil || len(vm.cfg.Mirrors) == 0 {
		return nil
	}

	if !isReadMethod(method) {
		return nil
	}

	for _, m := range vm.cfg.Mirrors {
		if m.Room != room {
			continue
		}
		if m.Wing != "" && m.Wing != wing {
			continue
		}

		switch m.MatchRole {
		case "all":
			return &MirrorResult{Mirror: true, FakeData: m.Fake, Room: room}
		case "unauthorized", "":
			return &MirrorResult{Mirror: true, FakeData: m.Fake, Room: room}
		case "authorized":
			return &MirrorResult{Mirror: true, FakeData: m.Fake, Room: room}
		}
	}

	return nil
}

// GenerateMasterKey creates a random 32-byte key and returns hex encoding.
func GenerateMasterKey() (string, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", fmt.Errorf("generate key: %w", err)
	}
	return fmt.Sprintf("%x", key), nil
}

// isReadMethod returns true for read-only operations.
func isReadMethod(method string) bool {
	readMethods := []string{
		"citadel.read", "citadel.read_raw", "citadel.search",
		"identity.validate", "identity.status", "identity.verify",
		"citadel.diary", "citadel.dreams",
	}
	for _, m := range readMethods {
		if strings.EqualFold(method, m) {
			return true
		}
	}
	return false
}
