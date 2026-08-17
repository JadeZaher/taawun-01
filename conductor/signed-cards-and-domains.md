---
type: security-architecture
title: Signed Cards and Approved Domains
status: accepted
---

# Signed cards and approved domains

## Trust statement

A card is an immutable Taawun application module whose manifest is signed by
the platform and bound to an authenticated subject, workspace, artifact hash,
and approved hostname set. A valid signature proves provenance and binding; it
does not grant a browser more authority than the current Shura capability.

## Signing contract

- Use Ed25519 with a server-held private key and a public key ID.
- Canonicalize a versioned payload containing the artifact content hash, card
  IDs and versions, user subject, workspace ID, approved hosts, issued time,
  optional expiry, and manifest contract version.
- Sort set-like fields before canonical encoding. Reject unknown contract
  versions and duplicate hosts.
- Store the signature and key ID in the manifest. Expose public verification
  keys from a cacheable read-only endpoint.
- Rotate by adding a new key ID; retain prior public keys while unexpired cards
  can still verify. Never place private signing material in an artifact.

## Domain approval lifecycle

1. A workspace owner requests a normalized hostname.
2. Taawun returns a random, single-purpose verification value for a DNS TXT
   record under `_taawun.<hostname>`.
3. Verification resolves the TXT record through an injected resolver, compares
   exact values, and records `verified_at`. A platform administrator may perform
   a separately audited manual verification when DNS is not practical.
4. Card signing resolves approved hosts from durable workspace state. Tool or
   HTTP input cannot add an unverified host.
5. Revocation prevents new serving and publication immediately. Existing
   artifacts remain auditable but are not served on the revoked host.

Hostname matching is exact unless a separately verified wildcard claim exists.
Production claims reject loopback, IP literals, credentials, ports, paths,
queries, and fragments.

## Serving boundary

- Resolve the request `Host` after trusted-proxy normalization and compare it
  to the signed manifest and current non-revoked domain claim.
- Use exact CORS and frame-ancestor policies derived from the card contract.
- A modular embed uses a sandboxed iframe by default. Its host handshake checks
  the parent origin against the signed allowlist.
- A monolithic composition verifies every included card and refuses mixed
  workspace or disjoint host bindings.
- Datastar control actions re-authorize the current user/capability; a signed
  HTML card is not an authorization token.

## LLM access

Users connect their chosen MCP-capable LLM through the standard HTTP MCP
authorization flow: protected-resource metadata, an OAuth 2.1 authorization
code grant with PKCE, explicit consent, and access tokens audience-bound to the
canonical Taawun MCP resource. Optional named, expiring, revocable tokens may
support developer automation; Taawun stores only their hash and prefix and
reveals plaintext once. Both token forms bind scopes to the authenticated user
and permitted workspaces. MCP arguments may select among those workspaces but
cannot assert a different user identity, and inbound tokens are never passed
through to downstream services.
