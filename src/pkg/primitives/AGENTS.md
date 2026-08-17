# Relay primitives

`p2p.go` handles opaque WebRTC signaling and encrypted fallback frames only. It must never persist, inspect, or transform application payloads beyond JSON framing; community data belongs in the browser runtime.

Relay sessions are HMAC-bound to one artifact and one peer. Production relay processes take their stable `RELAY_SHARED_SECRET` and browser-origin allowlist from environment variables. The in-process constructor uses an ephemeral secret solely for tests and local wiring. A control plane must authenticate its own caller before issuing a relay session; this package intentionally exposes no public token-issuance HTTP endpoint.

The `taawun-relay` binary is a standalone signaling process, not a TURN server or a data store. TLS termination, a valid origin allowlist, and short-lived sessions are deployment requirements. See the P0 private-beta track for the verified product boundary.
