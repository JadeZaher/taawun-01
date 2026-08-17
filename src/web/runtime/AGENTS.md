# Generated-app browser runtime

This directory owns the readable convergent-data boundary for generated Taawun
apps. The runtime is browser code, not a control-plane database and not a
Datastar signal store.

## State and merge contract

`IndexedDBOperationStore` persists an append-only log scoped by workspace and
artifact. An operation is a versioned `set` or tombstone for one approved
collection/key pair. Current values are projections of that log. The winner is
the operation with the highest tuple `(lamport, actorId, opId)`, compared by
code-unit order, so replay order does not affect the result. Deletions remain in
the log as tombstones and therefore cannot be resurrected by an older write.

The browser owns this state. Do not copy operation values, snapshots, member
records, announcements, or registration content into Datastar signals, request
parameters, logs, analytics, or control-plane responses. `_`-prefixed Datastar
signals are suitable for presentation state only; they are not a second state
store.

## Key and transport boundary

The 32-byte workspace key is injected as raw material or a non-extractable
AES-GCM `CryptoKey`, imported into memory, and discarded on `close()`. It is
never placed in IndexedDB, cookies, local/session storage, a URL, a relay frame,
or a Datastar signal. A host is responsible for obtaining the key through an
approved workspace key ceremony.

BroadcastChannel synchronizes same-origin tabs and may carry plaintext because
it remains within the browser-origin boundary. Cross-device batches are always
AES-256-GCM envelopes whose authenticated data binds the artifact and sending
peer. WebRTC uses the relay only for `join`, `offer`, `answer`, and
`ice-candidate` signaling. Until a data channel is open, `crdt-sync` WebSocket
frames carry the same encrypted envelope. The relay sees routing identifiers,
sizes, timing, and ciphertext; it must never receive a workspace key or readable
operation value.

Relay session tickets are short-lived, one-time routing authorization, not
workspace keys. The runtime sends one only as the second offered WebSocket
subprotocol after `taawun-relay-v1`; it never appears in the relay URL or an
`Authorization` header. A host must obtain a new runtime configuration with a
fresh ticket before reconnecting, and must not log the protocol header at the
TLS proxy.

## Authorization boundary

Configuration is explicit and fail-closed: artifact, workspace, peer, session,
relay, ICE servers, approved collections/keys, local actor policy, and remote
actor policies are validated before startup. Viewer writes are denied.
Maintainers require an exact configured collection scope. Architects may write
configured convergent collections only.

The runtime does not mint or cryptographically verify Shura capability tokens.
The host must verify signed capabilities first, then inject the resulting
`actor` and `peerPolicies`. Remote policies are keyed by relay peer ID and bind
that peer to one actor ID and role; each persisted operation carries both IDs,
and relay/WebRTC ingestion rejects an outer peer that does not match them. Use
the asynchronous `authorizeWrite` hook for
additional signature, revocation, or per-operation checks. An operation from an
actor with no injected policy is rejected even when its transport is valid.

Financial state does not belong here. AZOA intents, statuses, balances, and
settlement records remain transactional server state and must use a separate
adapter. A module adapter created by this runtime exposes only its explicitly
declared convergent collections.

## Delivery contract

The generated product is a workspace-signed Datastar card bound to approved
HTTPS domains. Cards can be rendered as stable isolated modules or composed by
the same signed monolithic shell. MCP remains the hosted LLM-facing builder and
control plane; this runtime does not generate `ui://`, AppBridge, or per-user MCP
App outputs.

Events and adapters are versioned (`taawun.runtime.*.v1` and
`taawun.module-adapter/v1`). Breaking payload or merge changes require a new
contract version and migration path rather than an in-place semantic change.

## Verification

Run focused pure tests from this directory with `node --test runtime.test.mjs`.
Use `demo.html` in two same-origin tabs to verify IndexedDB replay and
BroadcastChannel propagation. WebRTC and relay fallback require two valid,
peer-bound relay sessions and a shared in-memory workspace key; inspect relay
frames to confirm `crdt-sync.payload` contains only the version, algorithm,
scope, IV, and ciphertext.
