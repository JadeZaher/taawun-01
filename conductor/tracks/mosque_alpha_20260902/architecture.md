---
type: technical-design
title: Mosque Alpha Architecture
status: proposed
---

# Mosque Alpha architecture

## System shape

```text
Organizer browser -- authenticated control plane
  |-- workspace membership and secure session
  |-- shared versioned page draft + personal guided progress
  |-- deterministic guide + optional typed AI proposal
  |-- immutable signed build and protected member review
  `-- publication history + registration inbox

Visitor browser -- /s/{mosque-slug}
  |-- verified active signed artifact
  |-- mosque information, prayer schedule, announcements and events
  |-- external donation-provider handoff
  `-- bounded registration submission

Existing verified-domain adapter
  `-- optional second public route after genuine DNS proof
```

SQLite and one Railway instance are acceptable for the first invited mosque if
the single-instance constraint is explicit and backup/restore is proven. A
database migration does not solve the current product gaps and is not an alpha
prerequisite.

## Records and invariants

### Draft and progress

Add `workspace_page_drafts`, unique by workspace, containing template, brief,
organization, ordered typed components, theme, monotonically increasing version,
actor, and timestamps. Add `workspace_builder_progress`, unique by workspace and
user, containing mode, current guided stage, and small help state.

- `GET/PUT /api/workspaces/{id}/page-draft`; writes require `expectedVersion`
  and return bounded `409` metadata on conflict.
- `GET/PUT /api/workspaces/{id}/builder-progress`.
- Existing session draft migrates once after successful server read and is then
  cleared only after acknowledged persistence.
- Architect and Maintainer share the page; each user keeps their own place in
  the guide. Guided and Advanced use the same draft schema.

### Typed proposal

`POST /api/workspaces/{id}/builder/proposals` accepts current stage, brief, and
draft version. Its response uses `taawun.builder-proposal/v1`, an enumerated set
of operations over allowlisted paths, summary, reason, and limitations. The
server validates request scope, redacts forbidden context, validates provider
output, applies the proposal to a copy, and runs catalog/component validation.
Applying remains a separate version-checked draft mutation. Consequential APIs
are never tools exposed to the proposal provider.

### Hosted publication

Add `workspace_site_publications`: immutable activation ID, workspace, normalized
unique slug, artifact/content hash, predecessor/rollback relation, actor,
activation and deactivation timestamps. Suggested routes:

- `POST/GET /api/workspaces/{id}/site-publications`
- `POST /api/workspaces/{id}/site-publications/{publicationID}/activate`
- `DELETE /api/workspaces/{id}/site-publications/current`
- `GET /s/{slug}` and signed asset subpaths

Route `/s/` before the control-host catch-all. Never weaken the verified-domain
adapter. Both adapters resolve one active publication and run the same signature,
digest, workspace, lifecycle, expiry, CSP, content-type, and cache checks.
The slug is durable; renewable activation authority is not. Activations expire
after at most 90 days, renew from server time beginning 14 days before expiry,
and produce organizer warnings at 14, 7, and 1 day. Renewal failure leaves the
current version active only until expiry, then public reads return `410 Gone`.
Replacement and rollback preserve the slug.

### Registration

Add `site_registration_forms` bound to workspace, publication, and component,
with schema version, consent/retention copy, open/closed state, capacity, and
deadline. Add `site_registration_submissions` with opaque ID, exact bindings,
normalized allowlisted values, idempotency key/digest, status, timestamps, and
deletion/anonymization state; append audit events without submitted values.

- `POST /s/{slug}/_taawun/registrations`
- `POST /_taawun/registrations` on a verified custom host
- `GET /api/workspaces/{id}/registrations?cursor=&limit=`
- `GET /api/workspaces/{id}/registrations/export.csv`
- `PUT/DELETE /api/workspaces/{id}/registrations/{submissionID}`

Public controls include exact active-form binding, strict content type/body/
field/count limits, same-origin and Fetch-Metadata enforcement, bounded per-site
and per-source rate limits, honeypot, idempotency, capacity transaction, and
non-leaking errors. CORS alone is insufficient. Organizer reads are paginated;
CSV cells are escaped against formula injection. Deletion and retention jobs are
audited without retaining deleted content.

Each logical event has a stable form-series ID. Every publication creates an
immutable form revision; only the revision on the active publication accepts new
submissions. Prior responses remain visible in the same Architect inbox, tagged
with their revision. Capacity, deadline, and explicit open/closed state live on
the series and do not reset on replacement or rollback. Rollback changes the
active schema revision but cannot reopen a closed series or resurrect capacity.
An intentionally new event receives a new series. Inactive revisions reject new
submissions while authorized historical read/export remains available.

### Time and schedule data

The workspace requires a validated IANA timezone. Prayer schedules store local
wall-clock values plus effective local-date range; events store local date/time,
zone, and the resolved offset used by the signed build. Public output labels the
zone. Rendering and tests use the IANA database for DST transitions, including
ambiguous/nonexistent local times, and never infer prayer times.

### Auth and collaboration

Invitation acceptance and membership creation share one database transaction,
or use an explicit recoverable state with a proved reconciler. No terminal
`ACCEPTED` state may precede membership. Test every commit boundary, retries,
revoke races, and removed-member replay. Registration response access is
Architect-only in this alpha.

The invited alpha uses a supported operator-assisted recovery flow: an
authorized support operator verifies the pre-recorded mosque contact through a
second channel, issues a single-use 15-minute reset locator whose digest is
stored, and never sees the new password. Successful reset rotates the account
session version, invalidates all sessions and outstanding reset locators, and
appends a value-free audit event. Issuance and consumption are rate limited;
operators cannot change membership or bypass the reset. The support runbook
records verification method, actor, request ID, and outcome without the locator.
Self-service recovery and verified email remain follow-on auth hardening.

## Readiness and recovery

Keep shallow liveness separate from readiness. Readiness checks database read,
artifact-root read/write, required signing configuration, and ability to verify a
known canary artifact. It fails closed and never returns secrets.

A consistent recovery set includes the primary SQLite database and WAL state,
financial database even when sandboxed, artifact files, schema/migration version,
and separately inventoried signing/configuration secrets. Backups are encrypted
off-platform. Alpha objectives are RPO <= 1 hour, RTO <= 4 hours, and 30-day
retention. Logical restore evidence includes `PRAGMA integrity_check`, entity
counts, authentication, registration read/export, publication bindings,
signatures, files, and artifact digests. It does not claim public serving from a
different origin. Public disaster recovery is a separate genuine route/DNS
cutover and failback exercise using a predeclared recovery hostname; Host
overrides or weakened origin checks never count.

Release configuration fixes replica/autoscaling count at one, proves the volume
is mounted at the expected path, and prevents deploy or rollback overlap from
creating two concurrent SQLite writers. Readiness fails if the writer lease or
expected volume is unavailable.

## Architectural decisions

- Add a platform-publication adapter; do not make customer DNS prerequisite to
  first value and do not weaken DNS proof for optional custom domains.
- Persist drafts on the server; session storage cannot satisfy Save and exit,
  collaboration, or recovery.
- Keep registration outside immutable artifacts; it is transactional customer
  data with a distinct privacy lifecycle.
- Extend declarative typed components; exclude arbitrary executable generation.
- Preserve existing platform foundations behind Advanced or feature flags rather
  than deleting them.
- Avoid simultaneous edits to the monolithic `src/web/index.html`; Lane A owns it
  until the builder surface is safely modularized.
