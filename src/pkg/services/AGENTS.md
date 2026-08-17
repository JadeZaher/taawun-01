# Service authorization invariants

- Authentication uses a runtime-provided HS256 secret of at least 32 bytes. Access tokens carry a typed user ID plus issuer, audience, subject, issued-at, not-before, and expiry claims. Token validation reloads the user so suspended accounts and role changes take effect immediately.
- That JWT is only the first-party control UI/API credential. Remote MCP uses `pkg/oauth` opaque access tokens; the two bearer domains must not accept each other's credentials. `AuthService.Authenticate` is the shared password-verification seam for the OAuth consent session and does not mint or expose a JWT.
- Workspace authorization belongs in `WorkspaceService`, not only in HTTP middleware. Platform administrators may operate across workspaces; ordinary users need a workspace role. All members may read, workspace owners and workspace administrators may manage membership and settings, and only owners may delete.
- The owner role is assigned only at workspace creation. Membership requests may assign administrator, member, or viewer roles, preventing a workspace administrator from creating another owner.
- Notification mutations include the authenticated user ID in the database predicate so notification IDs cannot be used across accounts.
