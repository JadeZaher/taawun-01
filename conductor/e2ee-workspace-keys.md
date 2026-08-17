---
type: architecture
title: Local-first workspace key lifecycle
status: accepted
---

# Local-first workspace key lifecycle

Encrypted relay payloads require a key-distribution story that does not give the
platform plaintext access to community data. A raw workspace key in configuration
is a runtime seam, not a shippable lifecycle.

## Device and key model

- Each authenticated browser creates a non-extractable Web Crypto ECDH device
  private key in IndexedDB and registers only its public key and device metadata.
- An Architect's browser creates the random AES-256 workspace data key. It wraps
  that key independently to each authorized device using an ephemeral ECDH shared
  secret, HKDF, and AES-GCM with workspace, generation, recipient device, and
  sender identity as additional authenticated data.
- Taawun stores public device keys and opaque wrapped-key envelopes. It cannot
  decrypt the workspace data key or relay payloads.
- The browser unwraps the key into a non-extractable in-memory AES key for the
  card runtime. Plaintext keys never enter URLs, logs, localStorage, server JSON,
  or SQLite.

## Membership changes

Accepting an invitation enrolls the member, not an encryption key. An authorized
existing device must wrap the current generation to the new member's device.
Until then the new device reports `key_pending` and cannot read or write encrypted
collections.

Removing a member or revoking a device starts a new workspace-key generation.
Remaining authorized devices receive new envelopes and future operations use the
new generation. Rotation cannot make a previously disclosed old key unknowable;
the UI and audit trail must state that historical ciphertext may remain readable
to a formerly authorized device that retained it.

## Trust boundaries

- Device registration requires current Taawun identity and workspace membership.
- Envelope uploads require Architect capability and exact recipient membership.
- The server validates algorithms, public-key shape, envelope size, generation,
  and uniqueness but never accepts or returns a plaintext workspace key.
- Relay peers bind authenticated actor and device IDs to the Shura capability.
- Financial intents, balances, decisions, publication state, and compliance
  approvals remain server-owned records and are never placed in these encrypted
  convergent collections.
