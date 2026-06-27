// Package access implements the Access Controller (formerly Cerber).
// It is the security gate for all operations in Lazarus.
//
// Two paths:
//   - External: MCP Bridge → AuthorizeExternal (HMAC + nonce + token)
//   - Internal: Kernel.dispatch → AuthorizeInternal (ACL only, no HMAC)
//
// Account storage is in ~/.lazarus/accounts/<name>.yaml (outside Citadel, III-36).
package access
