---
type: product-status
title: Sellable MVP Feature Parity Matrix
status: in-progress
---

# Sellable MVP feature parity

This matrix is the scope guardrail for the MVP. A capability may be sandboxed,
operator-configured, or reference-only when real-world authority is unavailable,
but it is not silently removed from the product workflow.

| Product capability | MVP experience that must ship | Current implementation state | Ship gate |
|:---|:---|:---|:---|
| Vibecoding builder | Prompt/select, compose modules, theme, audit, preview, publish, and deploy | Authenticated cockpit is wired to the real catalog, Conductor preview, domain, and publication APIs; local browser QA passes through signed preview | Complete controlled DNS verification and public-host publish smoke test |
| Declarative composition | LLM configures pages, ordered slots, primitive instances, safe content, and theme tokens without arbitrary server code | Catalog and fixed compositions implemented | Signed composition spec must round-trip through MCP, preview, card, embed, and monolith outputs |
| Immutable artifacts | Versioned manifest, content-addressed assets, staging preview, tamper detection | Signed builder, preview-file adapter, bundled pinned Datastar runtime, and HTTP/MCP verification implemented | Hosted lifecycle verification against a customer-controlled hostname |
| Modular delivery | Signed cards served on approved domains, isolated embeds, and monolithic composition | Card/embed/monolith outputs and Host-bound public serving implemented | Connect every primitive to its runtime/server adapter |
| Theming | Swiss geometric default with validated organization tokens | Builder manifest and central UI in progress | Accessible validated tokens across all adapters |
| Approved domains | Workspace owner claims a hostname, proves control, and can revoke it; cards are host-bound | DNS proof, revocation, activation history, rollback, and verified public delivery implemented | External TLS/custom-host routing runbook and integrated test |
| Taawun identity | Registration, login, workspace membership, secure sessions | Registration/login/workspaces work in the cockpit; strict bounded public JSON, throttles, session versions, and password invalidation are implemented | Deploy with production secrets and operator account policy |
| LLM access | User authorizes our LLM or a third-party MCP client through OAuth 2.1/PKCE; optional named tokens support developer automation | OAuth discovery, CIMD/DCR, consent, token rotation/revocation, and MCP scope gates implemented | Real external MCP-client authorization smoke test |
| Shura governance | Architect/Maintainer/Viewer invitations, signed capabilities, revocation, audit trail | Signed capabilities, invitations, quorum, votes, decisions, JWKS, audit, and invite-to-workspace membership are composed into the service root | Add customer-facing governance UI/card workflows |
| Local-first state | IndexedDB-backed convergent collections, offline edits, deterministic merge | Runtime, replay, merge, BroadcastChannel, WebRTC, and encrypted relay fallback implemented | Live two-device signed relay demonstration |
| E2EE workspace keys | Devices exchange a workspace key through member-authorized wrapped key envelopes; relay and platform cannot decrypt application data | Runtime accepts an in-memory key; provisioning lifecycle pending | Owner/member device enrollment, revoke/rekey, and no plaintext key persistence |
| Relay infrastructure | Authenticated signaling, opaque encrypted payload transit, origin/rate/size controls | Authenticated control-plane issuer creates short-lived one-use identity/workspace/artifact/origin-bound tickets; browser sends them only as a WebSocket subprotocol | Single-instance/sticky deployment until a shared replay store is intentionally added |
| NAT traversal | Runtime consumes operator-configured STUN/TURN credentials without embedding long-lived secrets in artifacts | Signaling exists; ICE provisioning pending | Short-lived ICE configuration works on separate networks and fails closed when unavailable |
| Self-hosted relay | Zero-dependency `taawun-relay` deployment | Binary implemented | Cross-platform builds and one-command operator path |
| Template Bazaar | Submit, stage, audit, approve, version, discover, install, and purchase | Durable Bazaar is constructed in the composition root with workspace, compliance, Shura-decision, and financial adapters | Customer-facing Bazaar flow and sandbox purchase smoke test |
| Donation and escrow | Sandbox intent, idempotency, approval, fail-closed settlement/refund, reconciliation | Durable vetted flows/provider boundary/audit are mounted behind an authenticated actor-injecting adapter with final Shura-decision checks | UI/card adapter and integrated reconciliation test |
| Revenue split | Creator/platform split represented in AZOA quest graph | Exact 10,000-bps vetted flow and durable reconciliation implemented | Bazaar purchase integration |
| Zakat | Calculator/input contract plus citable policy and sandbox quest | Durable vetted quest plus reference-only card estimate implemented | Validated calculation inputs and UI/server adapter |
| Qard Hasan | Interest-free loan workflow and sandbox quest | Durable vetted state machine implemented | Terms/compliance UI and Shura approval integration |
| Volunteer stipends | Approved stipend workflow and sandbox quest | Durable vetted state machine implemented | Terms UI and Shura approval integration |
| Compliance corpus | Hanafi, Shafi'i, Maliki, Hanbali, and neutral retrieval with citations/status | Tagged seed corpus implemented | UI/MCP selection, publish gate, and review-status visibility |
| Conductor | Observable build/audit/preview/publish stages | Real declarative Conductor service replaces the legacy fail-closed path in the composition root | Hosted verified-domain activation smoke test |
| MCP control plane | Hosted typed discovery/audit/build/inspect/publish tools for our LLM or a customer's LLM over Streamable HTTP | Official SDK tools, OAuth scopes, workspace binding, preview origins, and signed artifact inspection implemented | External-client smoke test and richer declarative composition schema |
| Relay federation | Independent nodes exchange authenticated opaque relay coordination | Local signed relay sessions implemented | Configured node trust and failure isolation |
| AZOA federation | Signed cross-node settlement intents and receipts | Signed envelope/replay boundary implemented | Durable inbox/outbox and sandbox cross-node demonstration |
| Notifications | Workspace-scoped operational activity | Existing service under authorization hardening | Builder/Shura/Bazaar event integration |
| Deployability | Central Railway service, persistent data, health/readiness, documented env | Canonical non-root/read-only Go 1.25 container and verified local builds/browser journey are ready | Verified Railway deployment and live QA documentation |

## Honest boundary

Taawun can ship the complete workflow before it has authority to move live money
or issue religious rulings. In those two areas the MVP ships production-shaped
contracts, durable sandbox behavior, sources, review state, and explicit operator
configuration. It must never substitute fabricated settlement or fabricated
scholarly approval for the external authority that is still pending.
