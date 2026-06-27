# vault

Encrypted storage, intrusion detection, access camouflage.

Three subsystems:

## 1. Encryption — ChaCha20-Poly1305 AEAD

Selective, per-room encryption. Not everything is encrypted — only rooms marked as `EncryptedRoom`.

## 2. Honeypots

Fake rooms that trigger alerts on access. Never mutates real storage. Detects unauthorized access attempts.

## 3. Mirror

Returns fake data for unauthorized access. Indistinguishable from empty room. Silent denial.

## API

```go
v, err := vault.New(vault.Config{
    MasterKey:      masterKeyBytes,
    EncryptedRooms: []string{"decisions", "accounts"},
    Honeypots:      []vault.Honeypot{{Room: "secrets", AlertTopic: "vault.intrusion"}},
})

err := v.Encrypt(room, key, plaintext)
plaintext, err := v.Decrypt(room, key, ciphertext)
```

Vault is a filter+transform layer — it does NOT create storage. It wraps existing read/write paths.
