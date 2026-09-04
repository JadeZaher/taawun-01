---
type: product-status
title: Sellable MVP Feature Parity Matrix
status: in-progress
---

# Sellable MVP feature parity

## Mosque-alpha scope decision

The customer-facing alpha is governed by `tracks/mosque_alpha_20260902/`, not by
completion of every capability in this broader MVP matrix. Its required vertical
slice is Guided Mosque Essentials, a Taawun-hosted public URL, durable event
registration, safe small-team collaboration, replacement/rollback/unpublish,
and production operations. Bazaar, live finance, default Shura, federation,
relay, E2EE provisioning, and external MCP stay hidden or clearly sandboxed and
do not block that alpha. Custom domains are optional after the hosted URL works;
if exposed, genuine DNS and trusted TLS remain mandatory.

This matrix is the scope guardrail for the MVP. A capability may be sandboxed,
operator-configured, or reference-only when real-world authority is unavailable,
but it is not silently removed from the product workflow.

| Product capability | MVP experience that must ship | Current implementation state | Ship gate |
|:---|:---|:---|:---|
| Vibecoding builder | A non-technical organizer is guided from intent through composition, customization, signed preview, protected review, and public launch; Advanced controls remain available | The authenticated cockpit is wired to real catalog, Conductor, review, domain, and publication APIs, but exposes too many product/system choices at once; deterministic and AI-assisted guidance are specified but not implemented | Ship the persistent deterministic guided shell, bounded typed proposals when enabled, accessible ordering, two-profile review proof, and Taawun-hosted public delivery |
| Signed Starter Path | An empty-workspace organizer explicitly chooses a real template, customizes a component, verifies the exact signed preview and reaches a role-valid review handoff | Deployed on `b2fa4543`: zero-history is a successful authorized empty history; progress is derived ephemeral state, verification and history confirmation are separate, and invitation tokens remain session-only. The promoted keyboard path is seven controls and non-empty workspaces retain the existing builder/history experience. | Run the pending manual five-person comprehension study with real private-beta participants; the study does not block technical deployment and no result may be fabricated |
| Declarative composition | LLM configures pages, ordered slots, primitive instances, safe content, and theme tokens without arbitrary server code | Catalog and fixed compositions implemented | Signed composition spec must round-trip through MCP, preview, card, embed, and monolith outputs |
| Immutable artifacts | Versioned manifest, content-addressed assets, staging preview, tamper detection | Signed builder, preview-file adapter, bundled pinned Datastar runtime, and HTTP/MCP verification implemented | Mosque alpha: verify the complete Taawun-hosted `/s/{slug}` lifecycle. Broader MVP/custom domain: separately verify a customer-controlled hostname with genuine DNS and trusted TLS |
| Modular delivery | Signed cards served on approved domains, isolated embeds, and monolithic composition | Card/embed/monolith outputs and Host-bound public serving implemented | Connect every primitive to its runtime/server adapter |
| Theming | Swiss geometric default with validated organization tokens | Builder manifest and central UI in progress | Accessible validated tokens across all adapters |
| Approved domains | Workspace owner claims a hostname, proves control, routes it to the service, receives trusted TLS, and can revoke it; cards are host-bound | Taawun DNS proof, revocation, activation history, rollback, and Host-bound delivery are implemented; external edge routing and TLS remain operator work | Execute the controlled-hostname section of the live QA runbook with genuine DNS and browser-trusted TLS |
| Taawun identity | Registration, login, workspace membership, secure sessions | Registration/login/workspaces work in the cockpit and on the live Railway service; strict bounded public JSON, throttles, session versions, and password invalidation are implemented | Define private-beta operator account and retention policy |
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
| Conductor | Observable build/audit/preview/review/publish stages | Real declarative Conductor service plus bounded workspace Build History expose authorized durable status, exact verified reopen, protected workspace-member review locators, safe draft recovery, and publication retry without leaking event detail | Mosque alpha: complete protected review plus hosted `/s/{slug}` activation/replacement/rollback/unpublish. Broader MVP: add explicit review grants and custom-domain activation |
| MCP control plane | Hosted typed discovery/audit/build/inspect/publish tools for our LLM or a customer's LLM over Streamable HTTP | Official SDK tools, OAuth scopes, workspace binding, preview origins, and signed artifact inspection implemented | External-client smoke test and richer declarative composition schema |
| Relay federation | Independent nodes exchange authenticated opaque relay coordination | Local signed relay sessions implemented | Configured node trust and failure isolation |
| AZOA federation | Signed cross-node settlement intents and receipts | Signed envelope/replay boundary implemented | Durable inbox/outbox and sandbox cross-node demonstration |
| Notifications | Workspace-scoped operational activity | Existing service under authorization hardening | Builder/Shura/Bazaar event integration |
| Deployability | Central Railway service, persistent data, separate liveness/readiness, documented env, monitoring, support, backup and restore | Railway production service is live with persistent `/data`, exact-origin variables, sealed secrets, and a shallow `/api/health`; repeatable QA has a dedicated runbook | Add dependency-aware readiness, reproducible tagged promotion, alerts/support, consistent encrypted off-platform backup, and an isolated restore drill |

## Honest boundary

Taawun can ship the complete workflow before it has authority to move live money
or issue religious rulings. In those two areas the MVP ships production-shaped
contracts, durable sandbox behavior, sources, review state, and explicit operator
configuration. It must never substitute fabricated settlement or fabricated
scholarly approval for the external authority that is still pending.
