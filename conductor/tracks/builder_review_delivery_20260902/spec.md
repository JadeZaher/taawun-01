---
type: product-spec
title: Builder Review Links and Live Delivery QA
status: in-progress
---

# Builder Review Links and Live Delivery QA

## Problem and outcome

Organizers can compose and verify a page, but an artifact selector alone does not
feel like a usable review handoff. The builder must let an organizer send an
active, exact-build URL to an existing workspace member without turning the URL
into authority. The complete path must also be exercised in a real browser from
new account through verified-domain publication, revocation, and cleanup.

The outcome is a legible compose → preview → review-link → edit → replace or
revoke → publish lifecycle with evidence-backed recovery guidance at every
failure boundary.

## Functional requirements

- A currently active and verified preview exposes one copyable review URL of the
  form `/account#preview=<track-id>&workspace=<workspace-id>`; the workspace
  value is a selection hint and never authority.
- The review URL identifies an immutable track but never contains a bearer,
  invitation secret, workspace claim, or publication authority.
- Opening a review URL signs the recipient in when necessary, checks membership
  against the selected workspace, verifies the exact signed manifest and files,
  and opens the preview automatically.
- A wrong-workspace selection fails without leaking whether another workspace
  owns the track. Selecting the authorized workspace retries the same URL.
- Editing the local draft immediately marks the visible preview stale and
  disables copying until a fresh exact build succeeds.
- Viewer access can inspect a shared preview but cannot edit, build, invite,
  publish, revoke a domain, or gain authority from the track ID.
- Removing the Viewer or deleting the workspace makes subsequent review-link
  access fail through supported authorization checks.
- The current URL is an immutable-build locator, not a separately revocable
  grant. A future opaque review-session primitive must add explicit expiry,
  revocation, workspace/track binding, recipient policy, and audit without
  weakening signed-preview authorization.
- Component ordering becomes an explicit accessible composition field and must
  round-trip through draft restore, signature, preview, and publication.
- One deployed component graduates from tab-local demonstration to a bounded
  server-owned workflow; registration is the preferred first slice.
- Verified-domain publication remains a separate Architect action. Domain
  revocation must stop public serving without changing immutable build history.
- The public demo describes the real compose, preview, and protected-review loop
  and explicitly distinguishes it from anonymous public publication.

## Acceptance criteria

- [ ] Static and promoted-browser tests cover review-link enablement, copying,
  exact hash parsing, automatic reopen, wrong-workspace recovery, stale-disable,
  expiry, Viewer read-only behavior, and post-removal denial.
- [ ] A clean local integrated suite and production build pass once after all
  fixes are applied.
- [ ] A Railway deployment reaches terminal `SUCCESS`; health, root, bounded
  runtime logs, and HTTP `5xx` readback pass.
- [ ] A fresh live-browser fixture completes signup, workspace creation, template
  composition, component editing, signed preview, protected handoff, Viewer
  inspection, edit/rebuild, membership removal, and supported cleanup.
- [ ] A real controlled hostname completes claim, DNS proof, publication,
  replacement or rollback, public fetch, and revocation. If no controlled
  hostname is available, the release records this as `externally blocked` and
  does not simulate DNS or call the publication gate passed.
- [ ] Every observed usability pain point is recorded with evidence, severity,
  owner, remedy, and a track task or explicit deferral.
- [ ] Invitation acceptance cannot consume a token without durably creating the
  membership; injected failure remains safely retryable while removed members
  cannot replay an old accepted token to regain access.
- [ ] No credential, invitation token, bearer, private component payload, or DNS
  challenge value is retained in the repository or QA report.

## Measures

- A first-time organizer reaches a verified preview and copies its review link
  without needing a track-ID explanation.
- An invited Viewer reaches the exact preview from the link in one authenticated
  handoff and sees an explicit read-only boundary.
- Stale, expired, removed-member, revoked-domain, and ambiguous network states
  never present an active or publicly serving claim.
- Fixture cleanup uses supported lifecycle routes and both old sessions return
  `401` afterward.
