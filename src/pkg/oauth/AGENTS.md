# Taawun OAuth boundary

This package is the authorization server and resource-server boundary for the remote `/mcp` endpoint. It implements the MCP 2025-11-25 HTTP authorization profile with OAuth Authorization Code + PKCE S256, RFC 8707 resource binding, RFC 9728 protected-resource discovery, RFC 8414 authorization-server discovery, dynamic public-client registration, token rotation, and revocation.

All bearer credentials are high-entropy opaque values. SQLite stores only SHA-256 hashes. Authorization codes and refresh tokens are one-time credentials; replay of a rotated refresh token revokes its entire token family. Access tokens are short-lived and are revalidated against the current user, client, consent, resource, expiry, and revocation state on every HTTP request.

The authorization UI uses a short-lived Secure, HttpOnly, SameSite=Lax session cookie and a separate CSRF value. Platform JWTs are neither exposed to the OAuth browser flow nor accepted by the MCP resource server. First-party `/api` JWT authentication remains a separate boundary.

Consent names the requesting client, exact MCP resource, requested scopes, and selected workspaces. OAuth scope and workspace grant are necessary but not sufficient: every MCP operation rechecks the user's current persisted workspace role. `taawun:read`, `taawun:build`, and `taawun:publish` map to view/audit, build, and publish capabilities respectively.

Redirect URIs are registered and compared as exact strings. Wildcards, fragments, and credentials are rejected. Remote redirects require HTTPS; HTTP is accepted only for loopback desktop clients. The same limited loopback exception is available for local development issuer/resource URLs; production deployments must use HTTPS.
