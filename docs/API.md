# Taawun API v2.1 (Phase 0)

Auth: JWT via `token` cookie or `Authorization: Bearer`. All mutating calls require
`X-CSRF-Token` header equal to the `csrf_token` cookie.
Email verification and email notifications deferred to Phase 2 (no SMTP dependency).

| Endpoint | Method | Access |
|---|---|---|
| /health, /api/v1/health | GET | public |
| /api/v1/auth/register | POST | public |
| /api/v1/auth/login | POST | public |
| /api/v1/auth/logout | POST | authenticated |
| /api/v1/profile | GET | authenticated |
| /api/v1/workspaces | GET | authenticated (own memberships) |
| /api/v1/workspaces | POST | platform admin or architect |
| /api/v1/workspaces/{id} | GET | member or platform admin |
| /api/v1/workspaces/{id} | PUT | workspace maintainer+ |
| /api/v1/workspaces/{id} | DELETE | workspace architect |
| /api/v1/workspaces/{id}/members | GET | member or platform admin |
| /api/v1/workspaces/{id}/members | POST | workspace architect |
| /api/v1/workspaces/{id}/members/{userID} | DELETE | workspace architect |
| /api/v1/admin/users | GET | platform admin |
| /api/v1/admin/users/{id}/role | PUT | platform admin |
| /_/sse | GET | authenticated |

RBAC: platform roles admin > architect > maintainer > viewer; workspace roles are
assigned per membership; the owner is implicitly architect.
