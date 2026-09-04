# Conductor composition workflow

Conductor coordinates existing trust boundaries; it does not execute generated
code, start containers, create per-artifact databases, provision relays, or
perform financial work. Inputs are declarative curated-template selections and
escaped configuration values accepted by `pkg/artifacts`.

The durable SQLite track store records optimistic aggregate versions and a
hash-linked append-only event history. A signed preview bundle is not a
publication. Publication is requested separately, then activated through the
verified `pkg/domains` service under a fresh WorkspaceService publish check.
Published tracks also persist the exact domain `publication_id` in a normal,
non-unique indexed column. Shared publication links are valid. Legacy JSON is
backfilled only when its publication/workspace/claim/artifact/hash linkage
matches the exact domain row; no artifact, actor, time, or history heuristic is
permitted.

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

Preview reissue never mutates or extends the source track. It requires a fresh
Build capability check, an exact source version, and a complete signed preview
whose durable curated request still matches its build request and manifest.
Only that curated request is cloned; the current actor subject, preview-origin
authorization, expiry, compliance evidence, signature, artifact, and track are
created afresh through the normal composition workflow.

Publication retry uses the domain authority's exact indexed active-row lookup
and accepts only an exact workspace/claim/artifact/hash match. Publication
context reverse lookup is one bounded indexed set query and returns a binding
only when exactly one fully validated track remains; zero, multiple, malformed,
and mismatched candidates all remain unbound.

Activation does not write a pre-publication intent event. The domain transaction
is idempotent for an already-active exact artifact, and only the final published
track transition uses optimistic compare-and-swap; concurrent tracks may safely
share the one committed publication.
