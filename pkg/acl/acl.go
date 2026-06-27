// acl.go — Capability engine with deny>allow priority and pattern matching.
//
// Grammar (FIX D5):
//   Capability = Namespace ":" Scope ":" Permission
//   Namespace  = "wing" | "workspace" | "operator" | "account"
//   Scope      = ConcreteRef | RoleRef | KindRef | Wildcard
//   ConcreteRef = <workspace_name>     // "echo", "sysq"
//   RoleRef     = "role:" <tag_value>  // "role:project"
//   KindRef     = "kind:" <wing_kind>  // "kind:gov"
//   Wildcard    = "*"
//   Permission = "read" | "write" | "*" | "deny:" <wing_kind>
//
// Deny is always global: "wing:*:deny:" <wing_kind>
// Deny is checked first. If any deny matches → 403.
// Then allow is checked. If any allow matches → OK.
// If none → 403.

package access

import (
	"errors"
	"strings"
)

var (
	ErrACLDenied = errors.New("access: ACL denied")
)

// ── Capability ──────────────────────────────────────────────────────────────

// Capability is a parsed capability string.
type Capability struct {
	Namespace  string // "wing", "workspace", "operator", "account"
	Scope      string // ConcreteRef, RoleRef, KindRef, Wildcard
	Permission string // "read", "write", "*", or "deny:<kind>"
	Raw        string // original string
}

// ParseCapability parses a capability string.
// Grammar: namespace:scope:perm
// Scope can be: "echo", "role:project", "kind:gov", "*"
// Perm can be: "write", "read", "*", "deny:vault"
//
// Deny format: namespace:*:deny:kind  (scope is always "*" for deny)
// Generic: namespace:scope:perm
func ParseCapability(raw string) (*Capability, error) {
	first := strings.IndexByte(raw, ':')
	if first < 0 {
		return nil, errors.New("capability: missing colon")
	}
	namespace := raw[:first]
	rest := raw[first+1:]

	// Detect deny format: rest = "*:deny:<kind>" → scope=*, perm=deny:<kind>
	if strings.HasPrefix(rest, "*:deny:") {
		kind := rest[len("*:deny:"):]
		return &Capability{
			Namespace:  namespace,
			Scope:      "*",
			Permission: "deny:" + kind,
			Raw:        raw,
		}, nil
	}

	// Generic: find last colon, scope=everything before, perm=everything after
	last := strings.LastIndex(rest, ":")
	if last < 0 {
		return nil, errors.New("capability: missing second colon")
	}
	scope := rest[:last]
	perm := rest[last+1:]
	return &Capability{
		Namespace:  namespace,
		Scope:      scope,
		Permission: perm,
		Raw:        raw,
	}, nil
}

// IsDeny checks if this is a deny capability.
func (c *Capability) IsDeny() bool {
	return strings.HasPrefix(c.Permission, "deny:")
}

// DenyKind returns the wing kind that is denied, or empty string.
func (c *Capability) DenyKind() string {
	if c.IsDeny() {
		return strings.TrimPrefix(c.Permission, "deny:")
	}
	return ""
}

// ── Matching ────────────────────────────────────────────────────────────────

// matchScope checks if the capability scope matches a concrete workspace name
// and optional metadata (role tag, wing kinds).
func (c *Capability) matchScope(workspace, role string, kinds []string) bool {
	switch {
	case c.Scope == "*":
		return true
	case c.Scope == workspace:
		return true
	case strings.HasPrefix(c.Scope, "role:") && c.Scope[5:] == role:
		return true
	case strings.HasPrefix(c.Scope, "kind:"):
		kind := c.Scope[5:]
		for _, k := range kinds {
			if k == kind {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// matches checks if a capability applies to a concrete operation.
func (c *Capability) matches(workspace, action, role string, kinds []string) bool {
	if !c.matchScope(workspace, role, kinds) {
		return false
	}
	if c.Permission == "*" || c.Permission == action {
		return true
	}
	return false
}

// ── ACL ──────────────────────────────────────────────────────────────────────

// ACL holds capabilities for an account.
type ACL struct {
	Capabilities []string `yaml:"capabilities"`
}

// NewACL creates an ACL from a list of capability strings.
func NewACL(caps []string) *ACL {
	return &ACL{Capabilities: caps}
}

// Check verifies whether an operation is allowed.
// Order: deny first, then allow.
func (acl *ACL) Check(workspace, action, role string, kinds []string) error {
	// Phase 1: DENY CHECK (always first)
	for _, raw := range acl.Capabilities {
		cap, err := ParseCapability(raw)
		if err != nil {
			continue // malformed capability → skip (logged elsewhere)
		}
		if !cap.IsDeny() {
			continue
		}
		// Deny is always global: namespace must be "wing", scope must be "*"
		if cap.Namespace == "wing" && cap.Scope == "*" {
			for _, kind := range kinds {
				if kind == cap.DenyKind() {
					return ErrACLDenied
				}
			}
		}
	}

	// Phase 2: ALLOW CHECK
	for _, raw := range acl.Capabilities {
		cap, err := ParseCapability(raw)
		if err != nil {
			continue
		}
		if cap.IsDeny() {
			continue
		}
		if cap.Namespace == "wing" && cap.matches(workspace, action, role, kinds) {
			return nil
		}
		if cap.Namespace == "workspace" && action == "create" {
			return nil
		}
		if cap.Namespace == "operator" && action == "backup" {
			return nil
		}
		if cap.Namespace == "account" && action == "manage" {
			return nil
		}
	}

	return ErrACLDenied
}

// HasCapability checks if the account has a specific capability (prefix match).
func (acl *ACL) HasCapability(prefix string) bool {
	for _, cap := range acl.Capabilities {
		if cap == prefix {
			return true
		}
		// Allow prefix match for wildcards
		if strings.HasSuffix(prefix, "*") {
			pattern := strings.TrimSuffix(prefix, "*")
			if strings.HasPrefix(cap, pattern) {
				return true
			}
		}
	}
	return false
}

// ── Preset validation ──────────────────────────────────────────────────────

// PresetCapabilities returns the default capabilities for a given preset.
func PresetCapabilities(preset string) ([]string, error) {
	switch preset {
	case "coordinator":
		return []string{
			"wing:role:project:write",
			"wing:role:system:read",
			"wing:*:read",
			"workspace:create",
			"contract:propose",
		}, nil
	case "worker":
		return []string{
			"wing:role:project:write",
			"wing:role:system:read",
		}, nil
	case "auditor":
		return []string{
			"wing:*:read",
			"wing:role:system:read",
		}, nil
	case "operator":
		return []string{
			"wing:*:*",
			"workspace:*",
			"operator:backup",
			"account:manage",
		}, nil
	default:
		return nil, errors.New("unknown preset: " + preset)
	}
}

// ValidatePresetCreation checks that creating a workspace with preset
// does not grant capabilities the account doesn't already have (FIX B2, C10).
func (acl *ACL) ValidatePresetCreation(preset string) error {
	presetCaps, err := PresetCapabilities(preset)
	if err != nil {
		return err
	}
	for _, cap := range presetCaps {
		if !acl.HasCapability(cap) {
			return errors.New("preset validation: preset requires capability " + cap + " not held by account")
		}
	}
	return nil
}
