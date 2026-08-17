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

The public handler maps an uncredentialed request by exact `Host`, not by a path
or browser-supplied workspace ID. Every request joins the active publication to
an unexpired verified claim, reopens the artifact, rechecks manifest bindings,
and reads a manifest-listed digest-verified file. Revoked domains, expired
grants, expired manifests, unknown hosts, and unlisted files fail closed.
Control-plane hosts are a separate configured exact set and alone may fall back
to the authenticated builder application.
