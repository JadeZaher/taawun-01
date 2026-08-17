---
type: product-spec
title: P0 Sellable MVP — Complete Primitive Loop
status: in-progress
---

# P0 Sellable MVP — Complete Primitive Loop

## Problem and outcome

Mosque and charity operators need a simple way to launch a community-operations app without surrendering readable community data or rebuilding payment flows. The first sellable release lets an Architect create an approved community app, preview it, deploy it, invite collaborators, publish or reuse a template, and prove both the data and financial boundaries.

The MVP is successful when a three-device community can use one app offline for convergent content, govern it through Shura capabilities, exchange a reusable template through the Bazaar lifecycle, and create fail-closed AZOA sandbox intents with citable compliance results. Every primitive must work end to end before the product is marketed; integrations that move real money remain visibly gated until production credentials and reconciliation are verified.

## P0 capabilities

- An Architect selects or prompts a community-ops template and composes the registration, scheduling, donation, governance, compliance, and collaboration modules.
- The build produces an immutable surface bundle with an explicit manifest and a live Datastar staging preview.
- The same approved signed cards render as a monolithic app, dedicated-domain surfaces, and isolated cross-origin embeds without forking business logic.
- The bundle stores only convergent content locally and synchronizes encrypted bytes through an authenticated relay.
- An Architect can invite Maintainers and Viewers; Shura capabilities are signed, scoped, revocable, and enforced at the runtime boundary.
- The template creates AZOA sandbox quest intents for donations, escrow, Zakat, Qard Hasan, stipends, and revenue splits. It never maintains a local ledger and never represents a sandbox quest as settled money.
- Every generation and publish decision contains a versioned, citable compliance result. Seed policy data is explicitly marked as pending qualified scholarly review.
- The Bazaar supports template submission, live staging, compliance review, approval, versioned publication, installation, and sandbox purchase/escrow outcomes.
- Communities can run the standalone relay binary and opt into signed cross-node federation; federation failure is isolated and never silently changes local state.
- Workspaces choose a Hanafi, Shafi'i, Maliki, Hanbali, or explicitly neutral compliance baseline, with the selected baseline visible in results.

## Acceptance criteria

- [ ] Three browser instances can create and synchronize a non-financial schedule or registration change while offline-first behavior is observable.
- [ ] A Maintainer can edit only approved convergent collections; a Viewer cannot write; a capability signature or scope mismatch is rejected.
- [ ] The relay rejects unsigned, expired, oversized, rate-limited, and unapproved-origin connections.
- [ ] The generated bundle has a staging URL and an immutable manifest describing render modes, module contracts, theme tokens, origins, relay tier, data boundary, and limits before any checkout action.
- [ ] A signed card can run on its approved dedicated domain, inside the monolithic shell, and as an isolated embed; the hosted MCP tools return useful structured/text output to our LLM or a customer's LLM.
- [ ] The MCP control plane exposes typed discovery, audit, build, inspect, and publish tools over Streamable HTTP; every call is bound to the authenticated user and authorized workspace.
- [ ] Datastar actions transmit only allowlisted control or transactional fields and never readable convergent community content.
- [ ] Donation actions use a contract-tested AZOA sandbox adapter with idempotency, durable status, reconciliation, and fail-closed outcomes.
- [ ] Compliance results name the source record, version, baseline, and review status; unreviewed policy never masquerades as a fatwa or scholar approval.
- [ ] Shura invitation, capability issuance, revocation, and role enforcement can be completed from the product UI and MCP tools.
- [ ] Bazaar templates can be submitted, staged, audited, approved, versioned, installed, and exercised through sandbox escrow without an arbitrary-code execution path.
- [ ] The relay binary has a documented one-command deployment path; two configured nodes can exchange signed, replay-protected federation envelopes while an unconfigured node remains safely standalone.
- [ ] Each supported madhhab baseline can be selected and retrieved through the same citable compliance contract; neutral mode clearly preserves multi-baseline differences.

## Production gates, not removed features

- Bazaar discovery, creator payouts, revenue splits, and escrow are fully represented in the product workflow; external money movement remains sandboxed until a real AZOA adapter and reconciliation contract pass certification.
- Federation is implemented as an opt-in signed protocol and self-hostable relay topology. Cross-settlement remains sandboxed until participating operators configure trusted nodes and production financial adapters.
- Generated apps compose reviewed server actions and immutable browser modules. Arbitrary generated server code and arbitrary container execution are excluded because they are an unsafe implementation technique, not a user-facing feature.
- All four madhhab tags and neutral comparison mode are supported as citable corpus metadata. The platform presents sources and review status and does not market machine output as a fatwa or scholar approval.

## Measures

- A new workspace reaches a deployable preview in under 15 minutes.
- Three-device convergent sync completes in a local-network demonstration with no readable payload retained by the relay.
- One operator completes the full build, Shura, Bazaar, compliance, sandbox-AZOA, embed, MCP, and deployment loop without editing code.
- All P0 acceptance tests are green, and the product clearly labels external effects that are sandboxed or awaiting operator configuration.
