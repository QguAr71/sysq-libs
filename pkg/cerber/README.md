# cerber

Agent Identity Gate — three-headed gatekeeper for AI agent memory access.

Implements the Cerber Protocol v1.0.

## Heads

1. **Token auth** — validate bearer token (future)
2. **Write gate** — `IsWriteTool()` returns true for state-modifying operations
3. **ACL** — superwing/wing ownership check (future)

## Current

Head 2 (write gate) — extracted from Lazarus mcp/auth.go. Lists all MCP tools that modify state and require client approval.

## Tests

1 test — write gate validation.
