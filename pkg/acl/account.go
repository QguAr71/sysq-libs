// account.go — Account YAML CRUD + atomic writes + flock.
//
// Account storage: ~/.lazarus/accounts/<name>.yaml (outside Citadel, III-36).
// Kanoniczny schemat (F1): account, token_hash, signing_secret_enc,
// token_version, token_revoked_at, capabilities, audit_checkpoint_hash.
//
// Protected by: directory 0700, file 0600, flock on write.

package access

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

// ── Types ───────────────────────────────────────────────────────────────────

// Account represents an account stored in accounts/<name>.yaml.
type Account struct {
	Account              string   `yaml:"account"`
	TokenHash            string   `yaml:"token_hash"`
	SigningSecretEnc     string   `yaml:"signing_secret_enc"` // AES-256-GCM encrypted
	TokenVersion         int      `yaml:"token_version"`
	TokenRevokedAt       *string  `yaml:"token_revoked_at"` // null = active, ISO8601 = revoked
	Capabilities         []string `yaml:"capabilities"`
	AuditCheckpointHash  *string  `yaml:"audit_checkpoint_hash"`
}

// ── Account Manager ─────────────────────────────────────────────────────────

type AccountManager struct {
	dir   string // ~/.lazarus/accounts/
	mu    sync.Mutex // write lock
}

// NewAccountManager creates the account manager.
func NewAccountManager(dir string) *AccountManager {
	return &AccountManager{dir: dir}
}

// ── Read ─────────────────────────────────────────────────────────────────────

// ReadAccount reads an account from disk.
func (am *AccountManager) ReadAccount(name string) (*Account, error) {
	path := filepath.Join(am.dir, name+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("account read %s: %w", name, err)
	}

	var acct Account
	if err := yaml.Unmarshal(data, &acct); err != nil {
		return nil, fmt.Errorf("account parse %s: %w", name, err)
	}
	return &acct, nil
}

// ── Write (atomic + flock) ──────────────────────────────────────────────────

// atomicWrite writes data to path atomically using tmp+rename.
func atomicWrite(path string, data []byte, perms os.FileMode) error {
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, ".tmp-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	if err := os.WriteFile(tmpFile.Name(), data, perms); err != nil {
		return err
	}
	return os.Rename(tmpFile.Name(), path)
}

// WriteAccount saves an account atomically with flock protection.
func (am *AccountManager) WriteAccount(acct *Account) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	path := filepath.Join(am.dir, acct.Account+".yaml")

	// Open for locking
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("account open %s: %w", acct.Account, err)
	}
	defer f.Close()

	// flock (exclusive)
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("account flock %s: %w", acct.Account, err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	data, err := yaml.Marshal(acct)
	if err != nil {
		return fmt.Errorf("account marshal %s: %w", acct.Account, err)
	}
	return atomicWrite(path, data, 0600)
}

// ── Create ───────────────────────────────────────────────────────────────────

// CreateAccount creates a new account with a token and signing_secret.
// Returns the plaintext token and signing_secret (sent to agent once).
func (am *AccountManager) CreateAccount(name string, capabilities []string) (*Account, string, string, error) {
	token := generateRandom256()
	signingSecret := generateRandom256()

	tokenHash := sha256Hex(token)

	// Signing_secret will be encrypted later by the caller (needs master key).
	// For now, store placeholder.
	signingSecretEnc := "PLACEHOLDER_encrypt_with_master_key"

	acct := &Account{
		Account:          name,
		TokenHash:        tokenHash,
		SigningSecretEnc: signingSecretEnc,
		TokenVersion:     1,
		Capabilities:     capabilities,
	}

	if err := am.WriteAccount(acct); err != nil {
		return nil, "", "", err
	}

	return acct, hex.EncodeToString(token), hex.EncodeToString(signingSecret), nil
}

// ── Revoke ───────────────────────────────────────────────────────────────────

// RevokeAccount sets the token_revoked_at timestamp.
func (am *AccountManager) RevokeAccount(name string) error {
	acct, err := am.ReadAccount(name)
	if err != nil {
		return err
	}
	if acct.TokenRevokedAt != nil {
		return ErrTokenRevoked // already revoked
	}

	now := time.Now().UTC().Format(time.RFC3339)
	acct.TokenRevokedAt = &now
	return am.WriteAccount(acct)
}

// IsRevoked checks if an account's token is revoked.
func (acct *Account) IsRevoked() bool {
	return acct.TokenRevokedAt != nil
}

// ── Master Key ──────────────────────────────────────────────────────────────

type MasterKey struct {
	Key    []byte   // raw AES-256 key (32 bytes)
	Path   string   // ~/.lazarus/master.key
}

// LoadMasterKey reads the master key from disk.
func LoadMasterKey(path string) (*MasterKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("master key read %s: %w", path, err)
	}
	key := make([]byte, hex.DecodedLen(len(data)))
	n, err := hex.Decode(key, data)
	if err != nil {
		return nil, fmt.Errorf("master key decode: %w", err)
	}
	if n != 32 {
		return nil, errors.New("master key: invalid key size, expected 32 bytes")
	}
	return &MasterKey{Key: key, Path: path}, nil
}

// GenerateMasterKey creates a new random 256-bit master key.
func GenerateMasterKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}

// SaveMasterKey saves the master key to disk (0600, atomic write).
func SaveMasterKey(path string, key []byte) error {
	encoded := []byte(hex.EncodeToString(key))
	return atomicWrite(path, encoded, 0600)
}

// ── AES-256-GCM encrypt/decrypt ─────────────────────────────────────────────

// EncryptSecret encrypts a signing_secret with AES-256-GCM.
func EncryptSecret(plaintext []byte, masterKey *MasterKey) (string, error) {
	block, err := aes.NewCipher(masterKey.Key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	// ciphertext = nonce + encrypted_data
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptSecret decrypts a signing_secret_enc back to plaintext.
func DecryptSecret(encoded string, masterKey *MasterKey) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(masterKey.Key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce := ciphertext[:nonceSize]
	encrypted := ciphertext[nonceSize:]

	return gcm.Open(nil, nonce, encrypted, nil)
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func generateRandom256() []byte {
	b := make([]byte, 32)
	rand.Read(b)
	return b
}

func sha256Hex(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
