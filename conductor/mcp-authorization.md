---
type: security-architecture
title: Remote MCP Authorization
status: accepted
---

# Remote MCP authorization

Taawun's hosted `/mcp` endpoint is an OAuth-protected resource so our LLM and
customer-selected MCP clients can act for a Taawun user without sharing the
user's password or dashboard session token.

## Minimal interoperable flow

- Publish RFC 9728 protected-resource metadata for the canonical HTTPS MCP URI.
- Publish OAuth authorization-server metadata and expose authorization, token,
  revocation, and supported client-registration metadata.
- Prefer HTTPS Client ID Metadata Documents for clients with no prior
  relationship. Fetch them with public-IP-only DNS resolution, no redirects,
  bounded response size/time, exact `client_id` matching, and cache controls.
  Keep dynamic public-client registration as an interoperability fallback.
- Use authorization code with PKCE `S256`; codes are random, single-use,
  short-lived, client/resource/redirect-bound, and stored hashed.
- Require the RFC 8707 `resource` value and issue access tokens whose audience
  is exactly the canonical Taawun MCP resource.
- Rotate refresh tokens on every use and revoke the token family on detected
  reuse. Do not advertise `offline_access` as a protected-resource requirement.
- Return standards-shaped `WWW-Authenticate` challenges and never accept tokens
  in a query string.

## Scopes and consent

The initial scope set stays small:

- `taawun:read` — inspect template, primitive, compliance, workspace, and card metadata.
- `taawun:build` — compose and create a signed staging card for an authorized workspace.
- `taawun:publish` — publish or revoke a reviewed card on a verified workspace domain.

Consent identifies the requesting MCP client, selected workspaces, and requested
scopes. The server stores consent per user and client. A tool call still checks
workspace membership and Shura capability; OAuth scope is necessary but never
sufficient on its own.

## Token boundary

- Access tokens are short-lived opaque credentials stored only as SHA-256
  hashes. Each request resolves the token server-side and validates its exact
  resource audience, expiry, client, active consent, user, scopes, and selected
  workspace grants.
- The MCP resource never forwards its inbound token to AZOA, Railway, a relay,
  or another API. Downstream credentials are separate server-held credentials.
- Logs may contain token prefixes or IDs, never authorization codes, access
  tokens, refresh tokens, PKCE verifiers, or user passwords.
- Revoked users, clients, consents, workspaces, or Shura capabilities fail closed
  even if a cryptographic token has not yet expired.

The first-party dashboard JWT is a separate credential and is never accepted at
the OAuth-protected MCP resource. Local development may use an HTTP loopback
issuer and redirect URI; it is not the production customer connection path.
