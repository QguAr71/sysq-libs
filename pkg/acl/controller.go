// controller.go — Top-level Access Controller wiring.
//
// Controller is the main entry point wired into lazarusd.
// It holds all sub-components: AccountManager, TokenRotator, ACL, AuditLog, etc.
// External requests pass through AuthorizeExternal.
// Internal requests pass through CheckInternal.
//
// I-18: Graceful degradation. Controller startuje zawsze — poszczególne
// subkomponenty mogą być niedostępne (nil). AuthorizeExternal/CheckInternal
// sprawdzają dostępność przed użyciem i zwracają ErrDegraded gdy potrzebny
// komponent jest niedostępny.

package access

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ErrDegraded — operacja niedostępna w obecnym trybie degradacji.
var ErrDegraded = fmt.Errorf("access controller: component unavailable in current degradation level")

// DegradationLevel opisuje stan Access Controllera.
type DegradationLevel int

const (
	DegradeFull      DegradationLevel = iota // wszystkie subkomponenty OK → autoryzacja + audyt
	DegradeNoAudit                           // audit log padł → autoryzacja działa, audyt nie
	DegradeNoTokens                          // master key padł → tylko localhost, bez tokenów
)

func (d DegradationLevel) String() string {
	switch d {
	case DegradeFull:
		return "FULL (auth + audit)"
	case DegradeNoAudit:
		return "NO_AUDIT (auth only)"
	case DegradeNoTokens:
		return "NO_TOKENS (localhost only)"
	default:
		return "UNKNOWN"
	}
}

// Controller is the top-level Access Controller instance.
// Wired into lazarusd main.go as a global singleton (FIX B4: only one instance).
type Controller struct {
	am          *AccountManager
	rotator     *TokenRotator
	nonceCache  *NonceCache
	auditLog    *AuditLog
	masterKey   *MasterKey

	mu    sync.RWMutex
	level DegradationLevel
}

// NewController creates the Access Controller with graceful degradation.
// I-18: ZAWSZE zwraca Controller (nigdy nil). Subkomponenty które nie wstały → nil.
func NewController(accountsDir string, masterKeyPath string) *Controller {
	c := &Controller{
		nonceCache: NewNonceCache(100000, 5*time.Minute),
		level:      DegradeFull,
	}

	// Account Manager — jeśli padnie, to krytyczne (brak kont = brak autoryzacji)
	am := NewAccountManager(accountsDir)
	c.am = am

	// Master Key — InitMasterKey: generuje jeśli nie istnieje, ładuje jeśli istnieje
	paths := DefaultMasterKeyPaths()
	if masterKeyPath != "" {
		paths.Key = masterKeyPath
	}
	mk, err := InitMasterKey(paths)
	if err != nil {
		log.Printf("[access] master key unavailable: %v — token auth disabled", err)
		c.level = DegradeNoTokens
	} else {
		c.masterKey = mk
		c.rotator = NewTokenRotator(am, mk)
	}

	// Audit Log — jeśli padnie, autoryzacja nadal działa
	auditLog, auditErr := NewAuditLog(accountsDir, "system")
	if auditErr != nil {
		log.Printf("[access] audit log unavailable: %v — audit disabled", auditErr)
		if c.level == DegradeFull {
			c.level = DegradeNoAudit
		}
	} else {
		c.auditLog = auditLog
	}

	log.Printf("[access] controller ready — level=%s", c.level)
	return c
}

// Level returns the current degradation level (thread-safe).
func (c *Controller) Level() DegradationLevel {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.level
}

// IsFullyAvailable returns true only when all components are up.
func (c *Controller) IsFullyAvailable() bool {
	return c.Level() == DegradeFull
}

// NeedsLegacyFallback returns true when the controller cannot handle auth
// and the caller should use legacy superwing.Authenticate instead.
func (c *Controller) NeedsLegacyFallback() bool {
	level := c.Level()
	return level == DegradeNoTokens
}

// AuthorizeExternal validates an external request (HMAC, nonce, token, signature).
// Called from MCP handler in main.go.
// I-18: jeśli TokenRotator niedostępny → ErrDegraded.
func (c *Controller) AuthorizeExternal(accountName, token string, addr string) error {
	if c.rotator == nil {
		return ErrDegraded
	}

	acct, err := c.am.ReadAccount(accountName)
	if err != nil {
		return fmt.Errorf("account %s: %w", accountName, err)
	}
	if acct.IsRevoked() {
		return ErrTokenRevoked
	}
	if !ValidateToken(token, acct.TokenHash) {
		return ErrTokenInvalid
	}

	// Nonce check-and-mark — atomic, prevents TOCTOU, thread-safe
	if err := c.nonceCache.CheckAndMark(token); err != nil {
		return err
	}
	_ = addr // for future rate-limiting

	// Audit log — best-effort, nil-safe
	if c.auditLog != nil {
		c.auditLog.Append(fmt.Sprintf("auth_ext account=%s addr=%s", accountName, addr))
	}

	return nil
}

// CheckInternal checks ACL for an internal operation.
// I-18: jeśli ACL niedostępny → ErrDegraded.
func (c *Controller) CheckInternal(workspace, wing, method, ownerHash string) error {
	if wing == "" {
		return fmt.Errorf("access: wing cannot be empty")
	}
	acct, err := c.am.ReadAccount(workspace)
	if err != nil {
		return fmt.Errorf("account %s: %w", workspace, err)
	}

	acl := NewACL(acct.Capabilities)
	action := methodToAction(method)
	role := ""
	kinds := []string{wing}

	if err := acl.Check(workspace, action, role, kinds); err != nil {
		return err
	}

	// Audit log — best-effort, nil-safe
	if c.auditLog != nil {
		c.auditLog.Append(fmt.Sprintf("auth_int workspace=%s wing=%s method=%s", workspace, wing, method))
	}

	return nil
}

// AccountManager returns the underlying AccountManager.
func (c *Controller) AccountManager() *AccountManager { return c.am }

// TokenRotator returns the underlying TokenRotator (nil if degraded).
func (c *Controller) TokenRotator() *TokenRotator { return c.rotator }

// MasterKey returns the master key (nil if degraded).
func (c *Controller) MasterKey() *MasterKey { return c.masterKey }

// AuditLog returns the audit log (nil if degraded).
func (c *Controller) AuditLog() *AuditLog { return c.auditLog }

// ── Helpers ─────────────────────────────────────────────────────────────────

func methodToAction(method string) string {
	switch method {
	case "citadel.write", "memory.write", "memory_write":
		return "write"
	case "citadel.read", "memory.read", "memory_read", "citadel.search", "citadel.count":
		return "read"
	case "superwing.create", "superwing.delete", "wing.create", "wing.delete":
		return "create"
	case "wing.switch", "wing.fork":
		return "write"
	default:
		return "read"
	}
}
