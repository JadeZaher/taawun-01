# Conductor composition workflow

Conductor coordinates existing trust boundaries; it does not execute generated
code, start containers, create per-artifact databases, provision relays, or
perform financial work. Inputs are declarative curated-template selections and
escaped configuration values accepted by `pkg/artifacts`.

The durable SQLite track store records optimistic aggregate versions and a
hash-linked append-only event history. A signed preview bundle is not a
publication. Publication is requested separately, then activated through the
verified `pkg/domains` service under a fresh WorkspaceService publish check.

Compliance evidence always exposes its madhhab, reference ID, disposition, and
qualified-review status. `pending-qualified-review` must never be rendered as a
fatwa, scholar approval, or final jurisprudential ruling.

Production construction requires injected workspace authorization, subject
resolution, curated validation, compliance auditing, a stable-signer artifact
builder, lifecycle-aware preview-origin authorization, and domain publication.
The legacy constructor is deliberately unconfigured and fails closed.

Requested preview origins are preflighted and normalized before a durable draft
or idempotency key is consumed. Conductor re-authorizes them again while
advancing a staged track; a denial at that race boundary leaves the track
resumable and never proceeds to validation or signing.

Explicit component documents are validated and canonicalized before origin
authorization, durable track creation, or idempotency consumption. Omitted
components stay omitted in the legacy composition hash; default documents are
resolved into the durable build request. Track cloning always deep-copies raw
documents, and signed-artifact acceptance compares the exact request documents
to the complete v2 manifest/file binding.

Workspace build discovery is a bounded recovery index, not artifact evidence.
Track summaries are authorized with the View capability, ordered by
`(updated_at DESC, id DESC)`, and paged with an opaque keyset cursor. They expose
only status/presence metadata. Callers must use the existing verified-preview
reload to reopen and verify the immutable artifact. Resume keeps its stricter
Build, original-creator, expected-version, and status checks; authorization and
creator checks happen before version comparison so track IDs cannot be used as
version oracles.
