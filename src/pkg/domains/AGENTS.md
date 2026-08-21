# Verified workspace domains

This package is the only authority for origins embedded in signed artifacts.
Browser requests and MCP tool arguments are proposals; neither establishes
ownership. `Service.AuthorizeOrigins` re-authorizes the actor, loads the
workspace's verified and unexpired rows from SQLite, and accepts only an exact
subset.

## Ownership proof

Claims normalize to one public `https://host` origin. Paths, non-default ports,
IP literals, single-label hosts, Unicode hosts, and wildcards are rejected.
Verification applies only to that exact origin: parent domains, child domains,
and sibling domains never inherit it.

The DNS proof is a random TXT value at `_taawun.<host>`. Plaintext is returned
only by the claim response. SQLite stores a SHA-256 digest bound to the
workspace and normalized origin, so a database read cannot recover an active
challenge. DNS lookup is injected for deterministic tests.

Pending challenges and verified grants expire. Expired and user-revoked rows
remain as `revoked` audit records with timestamps and a reason; they are never
deleted by the lifecycle API. Reverification creates a new proof.

## Authorization and persistence

Claim, list, inspect, verify, and revoke require the workspace publish
capability, which maps to owner or workspace administrator in
`WorkspaceService` (platform administrators retain their existing override).
Artifact composition requires build capability and an exact verified subset.

Authenticated staging composition may additionally use exact platform origins
injected at startup. That exception is lifecycle-aware and preview-only. It does
not create a domain claim, does not appear in verified-origin resolution, and
cannot satisfy public publication activation.

The service applies an idempotent schema to the application's existing SQLite
connection. The active-origin partial unique index prevents two workspaces from
holding the same normalized origin concurrently. Keep migrations additive and
preserve revoked rows for auditability.

The publication-context read is an Architect-authorized audit view with a
default page of 20, maximum 50, and an opaque `(activated_at,id)` keyset cursor.
Exact `publicationId` selection is mutually exclusive with paging. Ordinary
inactive rows never open artifacts and carry null proof fields. Only the sole
active or exact-selected row may open one artifact; its manifest digest hashes
the exact verified stored bytes. Reverse track linkage is nullable and never a
candidate-count oracle. The page includes the same captured `serverTime` used to
derive every serving state and expiry decision in that response.

HTTP routes must remain behind bearer authentication. Do not expose challenge
digests, accept client-provided verification status, seed claims from CORS or
environment allowlists, or treat a successful DNS proof as authority for any
other hostname.

## Public delivery

Verification does not publish a bundle. `Publish` creates an immutable
activation event only after reopening the content-addressed artifact and
checking its signature, expiry, workspace subject, and exact approved host.
Activating an older event creates another event linked to the prior one; it does
not rewrite history. Exactly one event can be active for an origin.

`active` records the immutable activation fact; it is not serving health.
Derived `servingState` separately reports `serving`, `inactive`, `expired`,
`claim_unavailable`, or `artifact_invalid` from one captured server instant.
Activation reloads the claim inside its write transaction, revalidates the
artifact, and links a normal replacement to its immediate predecessor while an
explicit rollback keeps the selected source lineage.
After those transaction-local rechecks, publishing an artifact that is already
the exact active workspace/claim/origin/host/artifact/hash returns that durable
row without deactivation or insertion.

The public handler maps an uncredentialed request by exact `Host`, not by a path
or browser-supplied workspace ID. Every request joins the active publication to
an unexpired verified claim, reopens the artifact, rechecks manifest bindings,
and reads a manifest-listed digest-verified file. Revoked domains, expired
grants, expired manifests, unknown hosts, and unlisted files fail closed.
Control-plane hosts are a separate configured exact set and alone may fall back
to the authenticated builder application.

Public delivery captures the server clock once, opens the artifact at most once,
and uses the same verified result for binding, exact-boundary expiry, CSP, and
file verification. GET and HEAD stop serving old bytes at expiry; a successor
appears only after its activation transaction commits.
