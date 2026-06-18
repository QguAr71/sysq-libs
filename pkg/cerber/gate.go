// Package cerber — Agent Identity Gate.
//
// Three-headed gatekeeper for AI agent memory access.
// Implements the Cerber Protocol v1.0 (spec: github.com/QguAr71/cerber).
//
// Heads:
//   1. Token auth — validate bearer token (future)
//   2. Write gate — IsWriteTool returns true for state-modifying operations
//   3. ACL — superwing/wing ownership check (future)
//
// Currently implements Head 2 (write gate) as extracted from Lazarus mcp/auth.go.
// Head 1 (token auth) and Head 3 (ACL) will follow as Lazarus dispatch evolves.
package cerber

// WriteTool lists all methods that modify state and require client approval.
var WriteTool = map[string]bool{
	"citadel.write":     true,
	"identity.deploy":   true,
	"identity.rollback": true,
	"superwing.create":  true,
	"superwing.delete":  true,
	"wing.create":       true,
	"wing.switch":       true,
	"wing.deploy":       true,
	"gc.run":            true,
}

// ReadTool lists methods that do NOT modify state. Always allowed without approval.
// These match Lazarus/alwaysAllow and are here for forward compatibility
// when token auth (Head 1) is implemented.
var ReadTool = map[string]bool{
	"lazarus.version":      true,
	"lazarus.health":       true,
	"session.save":         true,
	"session.status":       true,
	"session.warn":         true,
	"session.start":        true,
	"session.exit":         true,
	"session.start_by_fd":  true,
	"identity.status":      true,
	"identity.validate":    true,
	"identity.verify":      true,
	"citadel.read":         true,
	"citadel.search":       true,
	"citadel.count":        true,
	"citadel.read_room":    true,
}

// IsWriteTool returns true if the given RPC method modifies state
// and requires client-side approval.
func IsWriteTool(method string) bool {
	return WriteTool[method]
}

// IsReadTool returns true if the given RPC method is read-only.
func IsReadTool(method string) bool {
	return ReadTool[method]
}

// GateResult describes the outcome of a Cerber gate check.
type GateResult struct {
	Passed  bool   `json:"passed"`
	Reason  string `json:"reason,omitempty"`  // why it failed
	Head    string `json:"head,omitempty"`    // which head blocked (token/write/acl)
	Agent   string `json:"agent,omitempty"`   // which agent tried
	Method  string `json:"method,omitempty"`  // which RPC was attempted
}
