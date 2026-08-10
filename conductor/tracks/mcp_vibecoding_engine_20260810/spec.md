# Spec: Embedded Model Context Protocol (MCP) Vibecoding Engine

**Track ID**: `mcp_vibecoding_engine_20260810`  
**Status**: Completed  
**Authoritative Location**: `pkg/mcp/mcp_server.go`

---

## 1. Requirements

### Functional Requirements
- **FR-1**: JSON-RPC 2.0 handler accepting `initialize`, `tools/list`, and `tools/call` methods at `/api/mcp`.
- **FR-2**: Tool `run_ethics_check` — Calls `pkg/ethics` to audit prompts & ASTs for Taqwa compliance.
- **FR-3**: Tool `deploy_container` — Calls `pkg/iac` to provision container sandboxes.
- **FR-4**: Tool `init_p2p_relay` — Returns Web P2P signaling relay endpoints.

---

## 2. Acceptance Criteria
- [x] `initialize` method returns protocol version `2024-11-05` and server info `Taawun-Go-MCP-Server`.
- [x] `tools/list` returns descriptions and schema metadata for all 3 tools.
- [x] Unit tests in `pkg/mcp/mcp_test.go` verify JSON-RPC serialization.
