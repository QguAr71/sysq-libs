// masterkey.go — Master key generation, load, backup, recovery.
//
// Master key: 256-bit AES key stored at ~/.lazarus/master.key (0600).
// Backup: ~/.lazarus/master.key.bak (copy before rotation).
// Encrypted backup: ~/.lazarus/master.key.enc (AES-GCM encrypted with argon2id derived key).
//
// Recovery: if master.key is lost, operator enters passphrase →
// argon2id(passphrase, salt) → decrypt master.key.enc → restore master.key.

package access

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/argon2"
)

const (
	masterKeySize   = 32 // 256-bit
	saltSize        = 16 // 128-bit
	argon2Time      = 3
	argon2Mem       = 64 * 1024 // 64MB
	argon2Threads   = 4
)

// ── Master key init ─────────────────────────────────────────────────────────

// MasterKeyPaths holds the paths for master key files.
type MasterKeyPaths struct {
	Dir        string // ~/.lazarus/
	Key        string // master.key
	KeyBak     string // master.key.bak
	KeyEnc     string // master.key.enc
}

// DefaultMasterKeyPaths returns paths under ~/.lazarus/.
func DefaultMasterKeyPaths() *MasterKeyPaths {
	dir := filepath.Join(homeDir(), ".lazarus")
	return &MasterKeyPaths{
		Dir:    dir,
		Key:    filepath.Join(dir, "master.key"),
		KeyBak: filepath.Join(dir, "master.key.bak"),
		KeyEnc: filepath.Join(dir, "master.key.enc"),
	}
}

// InitMasterKey generates a new master key if none exists.
// If master.key exists, loads it.
// If master.key is missing but master.key.enc exists, enters recovery mode.
func InitMasterKey(paths *MasterKeyPaths) (*MasterKey, error) {
	if _, err := os.Stat(paths.Key); err == nil {
		// Key exists — load it
		return LoadMasterKey(paths.Key)
	}

	// Key doesn't exist — check for encrypted backup
	if _, err := os.Stat(paths.KeyEnc); err == nil {
		// Encrypted backup exists — need passphrase for recovery
		return nil, fmt.Errorf("master key lost — use recovery with passphrase")
	}

	// Fresh start — generate new key
	key, err := GenerateMasterKey()
	if err != nil {
		return nil, err
	}

	if err := SaveMasterKey(paths.Key, key); err != nil {
		return nil, err
	}

	return &MasterKey{Key: key, Path: paths.Key}, nil
}

// ── Master key backup ──────────────────────────────────────────────────────

// BackupMasterKeyWithPassphrase encrypts the master key with argon2id(passphrase).
// The encrypted key is stored at master.key.enc.
// Returns the salt (hex-encoded, stored separately or with the file).
func BackupMasterKeyWithPassphrase(masterKey *MasterKey, passphrase string, paths *MasterKeyPaths) (string, error) {
	// Generate salt
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	// Derive key from passphrase
	derivedKey := argon2.IDKey([]byte(passphrase), salt, argon2Time, argon2Mem, argon2Threads, masterKeySize)

	// Encrypt master key
	block, err := aes.NewCipher(derivedKey)
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

	// Format: salt + nonce + ciphertext
	ciphertext := make([]byte, len(salt)+len(nonce)+len(masterKey.Key)+gcm.Overhead())
	copy(ciphertext[:saltSize], salt)
	copy(ciphertext[saltSize:saltSize+gcm.NonceSize()], nonce)
	gcm.Seal(ciphertext[saltSize+gcm.NonceSize():][:0], nonce, masterKey.Key, nil)

	if err := atomicWrite(paths.KeyEnc, []byte(hex.EncodeToString(ciphertext)), 0600); err != nil {
		return "", err
	}

	return hex.EncodeToString(salt), nil
}

// ── Recovery ───────────────────────────────────────────────────────────────

// RecoverMasterKey decrypts master.key.enc using a passphrase.
func RecoverMasterKey(passphrase string, paths *MasterKeyPaths) (*MasterKey, error) {
	data, err := os.ReadFile(paths.KeyEnc)
	if err != nil {
		return nil, fmt.Errorf("encrypted key file not found: %w", err)
	}

	ciphertext, err := hex.DecodeString(string(data))
	if err != nil {
		return nil, fmt.Errorf("decode encrypted key: %w", err)
	}

	if len(ciphertext) < saltSize+12+masterKeySize { // 12 = GCM nonce minimum
		return nil, errors.New("encrypted key file corrupted: too short")
	}

	salt := ciphertext[:saltSize]
	nonce := ciphertext[saltSize : saltSize+12]
	encrypted := ciphertext[saltSize+12:]

	// Derive key from passphrase + salt
	derivedKey := argon2.IDKey([]byte(passphrase), salt, argon2Time, argon2Mem, argon2Threads, masterKeySize)

	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plaintextKey, err := gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt failed — wrong passphrase or corrupted file")
	}

	// Restore master.key
	if err := SaveMasterKey(paths.Key, plaintextKey); err != nil {
		return nil, err
	}

	return &MasterKey{Key: plaintextKey, Path: paths.Key}, nil
}

// ── Master key rotation ────────────────────────────────────────────────────

// RotateMasterKey re-encrypts all signing secrets with a new master key.
func RotateMasterKey(am *AccountManager, oldMasterKey *MasterKey, paths *MasterKeyPaths) (*MasterKey, error) {
	// Generate new key
	newKey, err := GenerateMasterKey()
	if err != nil {
		return nil, err
	}

	newMasterKey := &MasterKey{Key: newKey, Path: paths.Key}

	// Backup old key
	oldData, err := os.ReadFile(paths.Key)
	if err != nil {
		return nil, err
	}
	if err := atomicWrite(paths.KeyBak, oldData, 0600); err != nil {
		return nil, err
	}

	// Re-encrypt all accounts
	entries, err := os.ReadDir(am.dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		name := entry.Name()[:len(entry.Name())-5] // strip .yaml
		acct, err := am.ReadAccount(name)
		if err != nil {
			return nil, fmt.Errorf("rotate: read %s: %w", name, err)
		}

		if acct.SigningSecretEnc != "" && acct.SigningSecretEnc != "PLACEHOLDER_encrypt_with_master_key" {
			// Decrypt with old key
			secret, err := DecryptSecret(acct.SigningSecretEnc, oldMasterKey)
			if err != nil {
				return nil, fmt.Errorf("rotate: decrypt %s: %w", name, err)
			}
			// Re-encrypt with new key
			newEnc, err := EncryptSecret(secret, newMasterKey)
			if err != nil {
				return nil, fmt.Errorf("rotate: encrypt %s: %w", name, err)
			}
			acct.SigningSecretEnc = newEnc
			if err := am.WriteAccount(acct); err != nil {
				return nil, fmt.Errorf("rotate: write %s: %w", name, err)
			}
		}
	}

	// Save new key atomically
	if err := SaveMasterKey(paths.Key, newKey); err != nil {
		return nil, err
	}

	return newMasterKey, nil
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return "/home/sysq"
}
