---
type: architecture-decision
title: Hosted MCP control plane and signed Datastar cards
status: accepted
---

# Hosted MCP control plane and signed Datastar cards

## Decision

Taawun hosts one authenticated remote MCP server that our LLM or a customer's
LLM uses as the vibecoding control plane. Its typed tools discover templates and
primitives, audit a composition, build signed card artifacts, inspect manifests,
and publish approved versions.

The resulting product surfaces are Datastar web outputs delivered through three
composition modes:

1. **Card** — a signed module document served by Taawun on an approved customer domain.
2. **Embed** — an isolated card exposed through a small Web Component/iframe contract for an existing site.
3. **Monolith** — multiple approved cards composed by the same renderer into one application shell.

MCP tools return typed structured data plus concise text for any compatible LLM
client. They are not the end-user application surface. MCP Apps and `ui://`
resources are a possible later integration, not an MVP dependency.

## State boundary

Datastar is authoritative for control-plane state and confirmed transactional outcomes. Convergent community content remains browser-owned and encrypted. It is never placed in ordinary server-bound Datastar signals.

- `_`-prefixed Datastar signals are local presentation state.
- Server actions explicitly filter the small set of permitted control or transactional fields.
- Each module manifest declares its data classification and allowed server signal names.
- AZOA references remain server-confirmed transactional state; the Taawun server does not invent settlement.

This is a deliberate, documented exception to Datastar's usual backend-source-of-truth model.

## Composition boundary

- Every signed card owns one stable DOM root and never mutates another card's subtree.
- CSS is scoped to module roots and uses namespaced `--taawun-*` tokens.
- Monolithic pages compose module templates directly.
- Cross-origin embeds default to sandboxed iframe isolation. A Web Component only supplies lifecycle, sizing, theme input, and versioned `postMessage` events.
- Datastar runs only on Taawun-controlled or explicitly approved customer origins. Its documented `unsafe-eval` CSP requirement is never silently broadened to unrelated hosts.
- Direct card network access is allowlisted by the artifact contract and exact CORS origin handling.

## Signed card contract

- The immutable manifest binds the artifact hash, card IDs and versions,
  workspace, issuing user/subject, approved origins/domains, signer key ID,
  issuance time, optional expiry, and signature algorithm.
- A card is served only when its manifest signature verifies and the request host
  is one of the approved domains. Host/origin approval is enforced server-side;
  a browser-provided field is never sufficient.
- Modular embeds and monolithic compositions reference the same signed card
  versions. Composition does not fork business logic or weaken the card's data,
  compliance, or capability boundary.
- Signing keys remain server-side. Published artifacts expose only the key ID,
  verification material, and signature needed to prove provenance.

## MCP control-plane contract

- Remote transport is official MCP Streamable HTTP.
- Authentication resolves a Taawun user before tool execution. Workspace and
  subject identity come from the authenticated context, not trusted tool input.
- Read tools expose template, primitive, manifest, and compliance contracts.
  Mutating tools require an Architect or appropriately scoped Maintainer.
- Build and publish tools produce or promote only curated card templates; they
  cannot execute arbitrary generated code, Docker images, or shell commands.
- Tool results use strong schemas, explicit side-effect annotations, and concise
  text fallbacks so both Taawun's LLM and customer-selected MCP clients work.

## Why no micro-frontend framework yet

Independent repositories, teams, dependency graphs, and deployment trains do not exist in the MVP. Introducing module federation or a route orchestrator now would add a second runtime without improving the customer outcome. Immutable module bundles and a public manifest preserve a future migration path.

## Production constraints

- Exact origin allowlists with `Vary: Origin`; never credentialed wildcard CORS.
- Content Security Policy is restrictive by default.
- Streamable HTTP is the remote MCP transport. Datastar SSE endpoints are card/application endpoints, not the legacy MCP HTTP+SSE transport.
- Assets are content-addressed and immutable.
- Theme overrides are limited to validated tokens.
- Embed messages are schema-versioned and reject unknown origins or versions.

See `.omc/research/taawun-mcp-microfrontends-2026-08-16.md` for source notes.
