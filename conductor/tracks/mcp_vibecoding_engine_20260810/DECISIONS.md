# Architecture Decisions: MCP Vibecoding Engine

**Track ID**: `mcp_vibecoding_engine_20260810`

---

### Decision 1: Single HTTP Post Endpoint for MCP
- **Ruling**: Expose MCP at `/api/mcp` handling standard JSON-RPC 2.0 requests over HTTP POST.
- **Rationale**: Enables AI agents, web frontends, and IDE extensions to invoke tools with standard HTTP standard requests without requiring complicated SSE connection states.
