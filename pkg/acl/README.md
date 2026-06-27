# acl

Access Control Layer — capability engine with DENY>ALLOW priority, pattern matching, JWT tokens.

## API

```go
// ACL
acl := access.NewACL([]string{"wing:sysq:read", "wing:sysq:write"})
err := acl.Check("sysq", "write", "", nil)

// Capability parsing
cap := access.ParseCapability("wing:projects:read")
cap := access.ParseCapability("wing:*:deny:gov")

// TokenManager
tm := access.NewTokenManager(masterKey)
token, err := tm.Issue("echo", "sysq", 3600)
claims, err := tm.Validate(token)

// AccountManager
am := access.NewAccountManager(storageDir)
acct, err := am.Create("echo", "agent")
acct, err := am.Get("echo")
```

## Key invariants

- DENY wins over ALLOW (checked first)
- Audit trail: append-only, never deleted
- Tokens: JWT HMAC-SHA256 with expiry + nonce
- Clock skew tolerance: 30s
- Per-account key rotation (graceful transition)

## Files

| File | Description |
|------|-------------|
| `acl.go` | Capability engine, ACL struct, Check() |
| `token.go` | JWT HMAC-SHA256, issue/validate, nonce cache |
| `account.go` | Per-agent accounts, YAML storage |
| `auth.go` | External auth (HMAC + nonce + token) |
| `audit.go` | Append-only audit trail |
| `controller.go` | Access controller (MCP bridge gate) |
| `masterkey.go` | Master key from Vault |
| `doc.go` | Package documentation |

## Tests

20 tests, all PASS. Coverage: token lifecycle, ACL matching, DENY priority, account CRUD, audit chain, race conditions.

## Dependencies

- `gopkg.in/yaml.v3` — account storage
- `golang.org/x/crypto` — HMAC-SHA256
