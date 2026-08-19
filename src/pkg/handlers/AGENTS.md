# HTTP handlers

Handlers translate strict HTTP contracts into package/service calls. Keep domain
logic, persistence, and authentication outside this directory.

`ArtifactHTTPHandler` must be mounted behind the application's authentication
and workspace-authorization middleware. It accepts a resolver for the trusted
subject, workspace, and verified-origin policy established upstream; it does
not parse credentials or mint sessions. Client authority fields are replaced
or rejected, and cross-authority reads are reported as not found.

Artifact downloads are content-addressed and immutable. Every read goes through
the artifact package's manifest and file-digest verification before bytes are
served. Never join a URL path directly to the artifact output root.

Preview responses expose a positive receipt-verification state only after the
artifact store reopens and verifies the complete signed bundle. The receipt
attests to the exact serialized manifest with a SHA-256 digest; the browser
recomputes the digest and compares the complete payload before checking expiry.
Signature-shaped client data alone is never evidence of active verification.

Composition outcome logs contain only a bounded correlation ID, outcome,
reason class, and HTTP status. Railway's request ID is accepted only from the
same trusted direct-proxy boundary as client attribution; otherwise the handler
generates its own ID. Never log the actor, origin, token, email, or request body.

Workspace People responses are private workspace evidence. They use `no-store`
and `nosniff`, keep forbidden/not-found outcomes bounded, and translate unknown
repository failures to a generic 500 envelope rather than exposing database text.

Component validation failures use a 422 envelope whose only details are the
bounded component ID, key path, and reason class. Never echo a component value or
raw document. Verified track reload is opt-in and must reopen the immutable
artifact before returning the same manifest digest/signature receipt used by the
initial preview; persisted track shape alone is not verification.

Workspace track listing is a recovery/discovery contract, not a global artifact
registry. Require one positive `workspaceId`, default the limit to 20, cap it at
50, and accept only the opaque cursor emitted by Conductor. Responses use
`no-store`/`nosniff` and contain only redacted track summaries; component
documents, actors, hashes, signatures, claims, publication details, raw failures,
tokens, and event detail do not belong in the collection response.

## Railway client attribution

Public throttles may accept Railway's documented `X-Real-IP` only when the
runtime carries `RAILWAY_ENVIRONMENT_ID` and the immediate socket peer is
loopback, RFC-private, or Railway's internal `100.0.0.0/8` proxy range. Never
accept `X-Forwarded-For`, a comma-separated client chain, or `X-Real-IP` from a
direct public peer. The socket peer remains the fallback when the header is
missing or invalid.
