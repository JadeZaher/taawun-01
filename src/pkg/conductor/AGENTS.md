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
