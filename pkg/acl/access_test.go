// access_test.go — Tests for Access Controller v2.7
package access_test

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/QguAr71/sysq-libs/pkg/acl"
)

func tempAccountsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	return dir
}

func tempDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func sha256Hex(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func computeHMAC(payload, key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// TestHMACVerifyValid i TestHMACTampered wyłączone — kod HMAC przeniesiony do _future/hmac.go
// (FUTURE: CERBER_SIGNED_REQUESTS=true dla zdalnych agentów MCP).
// Przywrócić testy razem z kodem gdy będzie blueprint.
// func TestHMACVerifyValid(...) — usunięte 2026-06-21
// func TestHMACTampered(...) — usunięte 2026-06-21

func TestTokenValidate(t *testing.T) {
	token := make([]byte, 32)
	rand.Read(token)
	tokenStr := hex.EncodeToString(token)
	hash := sha256Hex(token)
	if !access.ValidateToken(tokenStr, hash) {
		t.Fatal("valid token should pass")
	}
	if access.ValidateToken("wrong", hash) {
		t.Fatal("wrong token should fail")
	}
}

func TestNonceCacheNew(t *testing.T) {
	cache := access.NewNonceCache(100, 5*time.Minute)
	if err := cache.Check("n1", time.Now()); err != nil {
		t.Fatal("new nonce should be OK")
	}
	cache.Mark("n1", time.Now())
	if err := cache.Check("n1", time.Now()); err == nil {
		t.Fatal("reused nonce should fail")
	}
}

func TestNonceExpiry(t *testing.T) {
	cache := access.NewNonceCache(100, 10*time.Millisecond)
	cache.Mark("n2", time.Now())
	time.Sleep(20 * time.Millisecond)
	if err := cache.Check("n2", time.Now()); err != nil {
		t.Fatalf("expired nonce should be OK: %v", err)
	}
}

func TestClockSkewWithin(t *testing.T) {
	if err := access.CheckTimestamp(time.Now().Unix(), 5*time.Minute); err != nil {
		t.Fatal("current timestamp should pass")
	}
	if err := access.CheckTimestamp(time.Now().Add(-6*time.Minute).Unix(), 5*time.Minute); err == nil {
		t.Fatal("6-min old should fail")
	}
}

func TestACLAllow(t *testing.T) {
	acl := access.NewACL([]string{"wing:echo:write"})
	if err := acl.Check("echo", "write", "", nil); err != nil {
		t.Fatalf("exact allow: %v", err)
	}
}

func TestACLDenyWins(t *testing.T) {
	acl := access.NewACL([]string{"wing:*:write", "wing:*:deny:vault"})
	if err := acl.Check("echo", "write", "", []string{"vault"}); err == nil {
		t.Fatal("deny:vault should override allow")
	}
	if err := acl.Check("echo", "write", "", []string{"gov"}); err != nil {
		t.Fatal("deny:vault should not affect gov")
	}
}

func TestACLRoleScope(t *testing.T) {
	acl := access.NewACL([]string{"wing:role:project:write"})
	if err := acl.Check("w", "write", "project", nil); err != nil {
		t.Fatal("role:project should match")
	}
	if err := acl.Check("w", "write", "other", nil); err == nil {
		t.Fatal("role:project should not match role=other")
	}
}

func TestACLNoMatch(t *testing.T) {
	acl := access.NewACL([]string{"wing:echo:read"})
	if err := acl.Check("projects", "write", "", nil); err == nil {
		t.Fatal("mismatched scope should be denied")
	}
}

func TestCapabilityParseExact(t *testing.T) {
	cap, err := access.ParseCapability("wing:echo:write")
	if err != nil {
		t.Fatal(err)
	}
	if cap.Namespace != "wing" || cap.Scope != "echo" || cap.Permission != "write" {
		t.Fatalf("bad parse: %s/%s/%s", cap.Namespace, cap.Scope, cap.Permission)
	}
}

func TestCapabilityParseDeny(t *testing.T) {
	cap, err := access.ParseCapability("wing:*:deny:vault")
	if err != nil {
		t.Fatal(err)
	}
	if !cap.IsDeny() || cap.DenyKind() != "vault" {
		t.Fatal("deny parse failed")
	}
}

func TestCapabilityParseRoleRef(t *testing.T) {
	cap, err := access.ParseCapability("wing:role:project:write")
	if err != nil {
		t.Fatal(err)
	}
	if cap.Scope != "role:project" {
		t.Fatalf("scope should be role:project, got %s", cap.Scope)
	}
}

func TestAccountCreateAndRead(t *testing.T) {
	dir := tempAccountsDir(t)
	am := access.NewAccountManager(dir)

	acct, token, secret, err := am.CreateAccount("echo", []string{"wing:echo:*"})
	if err != nil {
		t.Fatal(err)
	}
	if acct.Account != "echo" || acct.TokenVersion != 1 {
		t.Fatal("wrong account data")
	}
	if token == "" || secret == "" {
		t.Fatal("token/secret must not be empty")
	}

	acct2, _ := am.ReadAccount("echo")
	if !access.ValidateToken(token, acct2.TokenHash) {
		t.Fatal("returned token should match stored hash")
	}
}

func TestAccountRevoke(t *testing.T) {
	dir := tempAccountsDir(t)
	am := access.NewAccountManager(dir)
	am.CreateAccount("rtest", []string{"wing:*:read"})
	if err := am.RevokeAccount("rtest"); err != nil {
		t.Fatal(err)
	}
	acct, _ := am.ReadAccount("rtest")
	if !acct.IsRevoked() {
		t.Fatal("account should be revoked")
	}
	if err := am.RevokeAccount("rtest"); err == nil {
		t.Fatal("double revoke should fail")
	}
}

func TestSessionSubsetInvariant(t *testing.T) {
	master := []string{"wing:echo:write", "wing:echo:read", "wing:system:read"}
	// Valid: subset
	_, _, err := access.CreateSession("echo", master, []string{"wing:echo:read"}, 1*time.Hour)
	if err != nil {
		t.Fatal("subset should pass", err)
	}
	// Invalid: not subset
	_, _, err = access.CreateSession("echo", master, []string{"wing:dev:write"}, 1*time.Hour)
	if err == nil {
		t.Fatal("non-subset should fail")
	}
}

func TestSessionExpiry(t *testing.T) {
	master := []string{"wing:echo:*"}
	_, token, err := access.CreateSession("echo", master, []string{"wing:echo:*"}, 1*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)

	dir := tempAccountsDir(t)
	am := access.NewAccountManager(dir)
	am.CreateAccount("echo", master)

	_, err = access.ValidateSession(token, am)
	if err == nil {
		t.Fatal("expired session should be rejected")
	}
}

func TestTokenRotationRace(t *testing.T) {
	dir := tempAccountsDir(t)
	am := access.NewAccountManager(dir)

	mk := make([]byte, 32)
	rand.Read(mk)
	masterKey := &access.MasterKey{Key: mk, Path: filepath.Join(dir, "master.key")}

	acct, oldTokenStr, _, _ := am.CreateAccount("race", []string{"wing:*:read"})
	oldToken, _ := hex.DecodeString(oldTokenStr)

	rotator := access.NewTokenRotator(am, masterKey)

	result, err := rotator.RotateToken("race", oldToken, acct.TokenVersion)
	if err != nil {
		t.Fatal(err)
	}
	if result.Version != 2 {
		t.Fatalf("version should be 2, got %d", result.Version)
	}

	// CAS: old version should fail
	_, err = rotator.RotateToken("race", oldToken, acct.TokenVersion)
	if err == nil {
		t.Fatal("CAS should reject old version")
	}
}

func TestPresetPrivilegeEscalation(t *testing.T) {
	worker := access.NewACL([]string{"wing:role:project:write", "wing:role:system:read"})
	if err := worker.ValidatePresetCreation("operator"); err == nil {
		t.Fatal("worker should not create operator preset")
	}

	coordinator := access.NewACL([]string{
		"wing:role:project:write", "wing:role:system:read",
		"wing:*:read", "workspace:create", "contract:propose",
	})
	if err := coordinator.ValidatePresetCreation("coordinator"); err != nil {
		t.Fatalf("coordinator should pass: %v", err)
	}
}

func TestAccountStorageIsolation(t *testing.T) {
	dir := tempAccountsDir(t)
	access.NewAccountManager(dir).CreateAccount("iso", []string{"wing:iso:read"})
	info, _ := os.Stat(dir)
	if info.Mode()&0007 != 0 {
		t.Fatal("accounts dir should be 0700")
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		fi, _ := e.Info()
		if fi != nil && fi.Mode()&0077 != 0 {
			t.Fatalf("account file %s should be 0600", e.Name())
		}
	}
}

func TestAuditChain(t *testing.T) {
	dir := tempDir(t)
	al, err := access.NewAuditLog(dir, "auditagent")
	if err != nil {
		t.Fatal(err)
	}
	al.Append("2026-06-20T10:00:00Z|n1|auditagent|mw|w|ok")
	h1 := al.ChainHash()
	if h1 == "" {
		t.Fatal("hash should not be empty")
	}
	al.Append("2026-06-20T10:00:01Z|n2|auditagent|mw|w|ok")
	if al.ChainHash() == h1 {
		t.Fatal("hash should change")
	}
	if err := al.VerifyIntegrity(); err != nil {
		t.Fatal(err)
	}
}

func TestAuditTruncation(t *testing.T) {
	dirAcc := tempAccountsDir(t)
	am := access.NewAccountManager(dirAcc)
	am.CreateAccount("trunc", []string{"wing:*:read"})

	dirAud := tempDir(t)
	al, _ := access.NewAuditLog(dirAud, "trunc")
	for i := 0; i < 5; i++ {
		al.Append("2026-06-20T10:0" + string(rune('0'+i)) + "|nx|trunc|mw|w|ok")
	}

	hash := al.ChainHash()
	acct, _ := am.ReadAccount("trunc")
	acct.AuditCheckpointHash = &hash
	am.WriteAccount(acct)

	if err := al.VerifyTruncation(am); err != nil {
		t.Fatal("truncation should pass", err)
	}

	acct2, _ := am.ReadAccount("trunc")
	wrong := "badhash"
	acct2.AuditCheckpointHash = &wrong
	am.WriteAccount(acct2)

	if err := al.VerifyTruncation(am); err == nil {
		t.Fatal("tampered checkpoint should be detected")
	}
}
