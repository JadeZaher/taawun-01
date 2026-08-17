# Relay primitives

`p2p.go` handles opaque WebRTC signaling and encrypted fallback frames only. It must never persist, inspect, or transform application payloads beyond JSON framing; community data belongs in the browser runtime.

Relay sessions are HMAC-signed, single-use tickets with a five-minute maximum
lifetime. They bind one artifact, workspace, authenticated principal, device,
peer, and exact browser origin. Production relay processes take their stable
`RELAY_SHARED_SECRET` and browser-origin allowlist from environment variables.
The in-process constructor uses an ephemeral secret solely for tests and local
wiring. A control plane must authenticate its own caller and recheck current
workspace membership and artifact scope before calling `IssueSession`; this
package intentionally exposes no public token-issuance HTTP endpoint.

WebSocket credentials travel only as the second offered subprotocol alongside
`taawun-relay-v1`; query-token and `Authorization` credentials are rejected.
Artifact and peer query parameters remain non-secret route identifiers. The
relay selects only the fixed protocol, so the credential is not echoed in the
response. The relay still sees no readable application payload.

The `taawun-relay` binary is a standalone signaling process, not a TURN server or a data store. TLS termination, a valid origin allowlist, and short-lived sessions are deployment requirements. See the P0 private-beta track for the verified product boundary.
