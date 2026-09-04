---
type: product-spec
title: Mosque Alpha — Guided Site, Events, and Registration
status: in-progress
---

# Mosque Alpha — Guided Site, Events, and Registration

## Release statement

The mosque alpha is a narrow, deployable product for one invited mosque. A
non-technical organizer can create a useful public site, publish an event,
collect bounded registrations, collaborate with a small team, update the live
site, and withdraw it. Taawun's broader governance, marketplace, finance,
federation, and developer surfaces remain available only behind Advanced or
development boundaries and do not block this release.

This track is the only customer-alpha ship gate. The broader
`p0_private_beta_20260816` track remains the platform/MVP program; foundation
completion does not imply mosque-alpha readiness.

## Users and jobs

- **Architect:** establishes the mosque workspace, controls access, publishes,
  rolls back, unpublishes, and owns registration-data policy.
- **Maintainer:** edits approved site and event content but cannot change
  workspace ownership or access registration responses in this alpha.
- **Viewer:** reviews an exact signed build and cannot mutate content, people,
  publications, domains, or submissions.
- **Visitor:** anonymously reads the public page and can submit a deliberately
  small event-registration form.
- **Operator:** deploys, monitors, backs up, restores, supports, and rolls back
  the service without reading customer secrets or unnecessary submission data.

## Core outcomes

1. **Launch a mosque site.** The organizer supplies identity, location/contact,
   prayer and Jumu'ah times, announcements, events, accessibility notes, and an
   optional external donation-provider link. A recommended Mosque Essentials
   structure produces a real preview and a stable Taawun-hosted public URL.
2. **Publish an event.** The organizer supplies date/time, place, capacity,
   description, contact, and allowlisted registration questions. A visitor can
   submit; authorized organizers can read, export, close, and delete responses.
3. **Keep it current.** Draft changes do not disturb the active site. The
   organizer reviews an immutable successor, activates it, rolls back once, or
   unpublishes while retaining authorized audit history.
4. **Collaborate safely.** Architect, Maintainer, and Viewer permissions hold in
   both UI and server. Invitation acceptance is atomic or safely reconcilable;
   removal immediately denies old sessions and protected review locators.
5. **Publish and withdraw clearly.** The UI distinguishes Draft, Private
   preview, Team review, and Public. Custom domains are optional; if offered,
   their DNS/TLS proof must be genuine.

## Guided co-builder contract

The default experience uses six outcome-oriented stages:

1. Tell us about your mosque.
2. Choose today's goal: launch the site, add an event, or update the live site.
3. Review the recommended Mosque Essentials structure and why it was chosen.
4. Add details one section at a time with Back, Skip, Undo, Save and exit, and a
   visible saved state.
5. Review the rendered page and plain-language public/data-custody boundaries.
6. Publish to a Taawun-hosted URL or ask an existing member for protected review.

New and empty workspaces enter Guided mode. Advanced mode operates on the same
versioned draft and switching modes is lossless. The core path never requires
raw JSON or the terms module, track, manifest, origin, Shura, Bazaar, or MCP.

The optional assistant converts ordinary-language intent into a versioned,
typed proposal over approved fields. Every proposal includes what changed, why,
limitations, and Apply/Edit/Reject. Unknown fields, executable content, stale
draft versions, and invalid component documents are rejected before mutation.
Provider failure returns to deterministic guidance without losing work. The
assistant cannot publish, invite, remove, revoke, delete, claim a domain, move
money, create routes, or receive credentials, tokens, capabilities, signing
material, registration bodies, or unrelated workspace data.

## Must / Should / Won't

### Must ship

- Persistent, resumable, versioned shared page draft and user-specific guided
  progress, with optimistic conflict handling.
- Guided default experience, one primary action per stage, lossless Advanced
  mode, and a deterministic provider-offline path.
- Mosque Essentials declarative template: identity, location/contact, prayer
  schedule, announcements, event details/registration, external donation link,
  accessibility notes, and footer; all visible values come from typed data.
- Accessible section ordering preserved through draft, signed build, review,
  replacement, rollback, and public serving.
- Stable anonymous `/s/{slug}` publication on a Taawun-controlled origin,
  signature/digest verification, history, replacement, rollback, and unpublish.
- Real registration storage bound to the exact site/publication/form, with
  validation, idempotency, origin/Fetch-Metadata and abuse controls, capacity and
  close state, consent, retention, organizer read/export/delete, and strict
  staging/production isolation.
- Architect/Maintainer/Viewer invitation, acceptance, denial, removal, and
  anti-replay proof; no accepted invitation may exist without recoverable
  membership state.
- Dependency-aware readiness, reproducible promotion, backup/restore proof,
  monitoring/alerts, rollback, support/incident ownership, and supported fixture
  cleanup.
- Real-server browser coverage plus keyboard, screen-reader, zoom, narrow-mobile,
  reduced-motion, contrast, and automated WCAG checks for the critical journey.

### Should ship or follow immediately

- Bounded typed AI proposals behind a feature flag; deterministic guidance is
  always sufficient and a live model is not a release dependency.
- Arabic content entry, RTL rendering, and bilingual labels.
- Guided custom-domain setup after the hosted site is live.
- Prayer-time import from an operator-selected source with provenance and manual
  override, seasonal/event duplication, SEO/social/print metadata, and optional
  registration notification through an approved provider.
- Expiring, explicitly revocable review grants and plain-language activity history.

### Won't ship in this alpha

- Live payments, custody, settlement, escrow, Zakat collection, Qard Hasan,
  stipends, revenue splits, or marketplace purchases.
- Bazaar publishing, default Shura voting, relay/federation, cross-node AZOA,
  multi-device E2EE provisioning, TURN, or external MCP-client authorization as
  customer launch gates.
- Arbitrary HTML, JavaScript, routes, server code, containers, or plugins from AI.
- Automated religious determinations or claims of fatwa, certification, or
  scholar approval.
- Custom-domain success demonstrated with Host overrides, fake DNS, or fake TLS.

## Data and trust boundaries

- Draft, assistant proposal, signed build, review locator, active publication,
  and registration submission are distinct records with distinct authority.
- A public slug is routing, never permission. Every public read verifies the
  exact signed artifact, workspace binding, file digest, lifecycle, and expiry.
- Registration is centrally stored customer data, not local-first state. The
  product states purpose, custody, retention, access, export, and deletion before
  enabling a form. The alpha excludes health, immigration, financial, and other
  sensitive registration questions.
- Registration responses are Architect-only in this alpha. Maintainer, Viewer,
  and other workspaces receive the same bounded non-enumerating denial.
- Workspace/site timezone is a required IANA timezone. Prayer schedules carry an
  effective local-date range; events carry local date/time, zone, and resolved
  offset. Public rendering always labels the mosque timezone and applies IANA DST
  rules without guessing religious times.

## Hosted publication lifetime

The slug remains stable across replacement and rollback. Each active publication
authorization lasts at most 90 days and the service attempts renewal 14 days
before expiry only while the workspace, artifact, and owner policy remain valid.
The organizer receives dashboard warnings at 14, 7, and 1 day; renewal failure
alerts the operator. After expiry the public route returns a plain `410 Gone`
without page or workspace detail while authorized history remains available.
All deadlines use server time and replacement preserves the slug.
- No secret, invitation token, private component body, or submission body enters
  logs, evidence, analytics, AI prompts, artifacts, or the repository.

## Product acceptance

With five representative non-technical mosque operators:

- At least 4 of 5 publish a basic site without facilitator intervention or
  opening Advanced mode.
- Median registration-to-public-site time is at most 15 minutes and median first
  meaningful preview time is at most 10 minutes.
- At least 4 of 5 identify the next action within five seconds at each stage and
  distinguish draft, private preview, team review, and public site.
- No participant has more than one unrecovered wrong turn; hesitation,
  terminology confusion, and recovery failures become tracked remedies.

The deployed technical journey must create a fresh organizer and workspace,
build/publish Mosque Essentials, serve an anonymous mobile visitor, submit and
manage a registration, prove role boundaries and member removal, edit and
replace the public build, roll back, unpublish, clean up through supported APIs,
and restore the backed-up state in isolation.

## Release boundary

The release is **NO-GO** while any P0/P1 acceptance failure remains, registration
is tab-local, the public site requires customer DNS, invitation acceptance can be
consumed without membership, the real server is not exercised end-to-end, or a
consistent off-platform backup has not been restored successfully. A promoted
Railway container and fixture-based browser tests are necessary evidence, not
sufficient evidence.
