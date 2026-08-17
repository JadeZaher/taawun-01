## `all_json/manifest.json`
```json
{
  "app": "taawun",
  "version": "2.1.0",
  "generated_on": "2026-08-15",
  "purpose": "Wall-to-wall JSON asset kit: configuration, API contracts, and testing artifacts",
  "counts": { "technical": 5, "functional": 19, "testing": 12, "manifest": 1, "total": 37 },
  "categories": {
    "technical": ["technical/config.json", "technical/config.development.json", "technical/config.production.json", "technical/seed_data.json", "technical/container_metadata.json"],
    "functional": ["functional/openapi.json", "functional/requests/*", "functional/responses/*"],
    "testing": ["testing/fixtures/*", "testing/postman/taawun_collection.json", "testing/e2e/e2e_scenarios.json"]
  }
}
```

---

## 1. TECHNICAL (Infrastructure & Configuration)

### `all_json/technical/config.json`
```json
{
  "app": { "name": "taawun", "service": "taawun", "version": "2.1.0" },
  "server": { "port_env": "PORT", "port_default": 8080 },
  "env_file": ".env",
  "env_vars": { "PORT": "8080", "JWT_SECRET": "REQUIRED_32_PLUS_CHARS", "DB_PATH": "./taawun.db", "COOKIE_SECURE": "false" },
  "database": { "driver": "sqlite", "driver_import": "modernc.org/sqlite", "dsn_pragma": "_pragma=foreign_keys(1)", "default_path": "./taawun.db" },
  "auth": {
    "jwt_algorithm": "HS256",
    "token_ttl_hours": 24,
    "bcrypt_cost": 10,
    "cookie": { "name": "token", "path": "/", "http_only": true, "secure": false, "same_site": "Lax", "max_age_seconds": 86400, "logout_max_age": -1 }
  },
  "security": {
    "csrf": { "cookie_name": "csrf_token", "header_name": "X-CSRF-Token", "exempt_paths": ["/api/v1/auth/login", "/api/v1/auth/register"] },
    "headers": { "X-Content-Type-Options": "nosniff", "X-Frame-Options": "DENY", "Referrer-Policy": "no-referrer" }
  },
  "rbac": {
    "platform_roles": ["admin", "architect", "maintainer", "viewer"],
    "workspace_roles": ["architect", "maintainer", "viewer"],
    "workspace_creation_requires": ["admin", "architect"]
  },
  "templates": { "dir": "internal/web/templates", "layout": "layout.html", "content_block": "content", "pages": ["index.html", "login.html", "register.html", "dashboard.html", "workspaces.html", "workspace_detail.html", "admin.html"] },
  "static": { "dir": "internal/web/static", "prefixes": ["/css/", "/js/"] },
  "sse": { "endpoint": "/_/sse", "heartbeat_seconds": 5, "signals": ["connected", "timestamp", "heartbeat", "workspace_event"] },
  "admin_seed": { "condition": "when_no_admin_exists", "id": "admin-001", "email": "admin@taawun.com", "name": "Admin User", "role": "admin" },
  "web_routes": ["/", "/login", "/register", "/dashboard", "/workspaces", "/workspaces/{id}", "/admin", "/logout"],
  "api_routes": ["/health", "/api/v1/health", "/api/v1/auth/register", "/api/v1/auth/login", "/api/v1/auth/logout", "/api/v1/profile", "/api/v1/workspaces", "/api/v1/workspaces/{id}", "/api/v1/workspaces/{id}/members", "/api/v1/workspaces/{id}/members/{userID}", "/api/v1/admin/users", "/api/v1/admin/users/{id}/role", "/_/sse"]
}
```

### `all_json/technical/config.development.json`
```json
{
  "environment": "development",
  "server": { "port": 8080 },
  "database": { "path": "./taawun.db" },
  "auth": { "cookie": { "secure": false }, "jwt_secret_source": ".env" },
  "logging": { "level": "debug", "admin_seed_log": true }
}
```

### `all_json/technical/config.production.json`
```json
{
  "environment": "production",
  "server": { "port": 8080 },
  "database": { "path": "/app/data/taawun.db" },
  "auth": { "cookie": { "secure": true, "same_site": "Lax" }, "jwt_secret_source": "ENV_ONLY", "admin_seed": "disable_after_first_run" },
  "logging": { "level": "info" }
}
```

### `all_json/technical/seed_data.json`
```json
{
  "_note": "Dev-only seed. Hash passwords with bcrypt (cost 10) before inserting.",
  "users": [
    { "id": "admin-001", "email": "admin@taawun.com", "name": "Admin User", "role": "admin", "password_plain_dev_only": "admin123" },
    { "id": "1755264000000000001", "email": "alice@example.com", "name": "Alice Architect", "role": "architect", "password_plain_dev_only": "secret123" },
    { "id": "1755264000000000002", "email": "bob@example.com", "name": "Bob Viewer", "role": "viewer", "password_plain_dev_only": "secret123" }
  ],
  "workspaces": [
    { "id": "1755264000000000100", "owner_id": "1755264000000000001", "name": "Design Team", "description": "Shared space for designers" }
  ],
  "workspace_members": [
    { "workspace_id": "1755264000000000100", "user_id": "1755264000000000001", "role": "architect" },
    { "workspace_id": "1755264000000000100", "user_id": "1755264000000000002", "role": "viewer" }
  ]
}
```

### `all_json/technical/container_metadata.json`
*(Unchanged from previous version)*

---

## 🔌 2. FUNCTIONAL (API Contracts & Payloads)

### `all_json/functional/openapi.json`
```json
{
  "openapi": "3.0.3",
  "info": { "title": "Taawun API", "description": "Workspace management platform API (v2.1 RBAC)", "version": "2.1.0" },
  "servers": [{ "url": "http://localhost:8080", "description": "Development server" }],
  "paths": {
    "/health": { "get": { "summary": "Health check", "responses": { "200": { "description": "OK", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/HealthResponse" } } } } } } },
    "/api/v1/auth/register": { "post": { "summary": "Register new user (sets token & csrf cookies)", "requestBody": { "required": true, "content": { "application/json": { "schema": { "$ref": "#/components/schemas/RegisterRequest" } } } }, "responses": { "201": { "$ref": "#/components/responses/AuthOK" }, "400": { "$ref": "#/components/responses/BadRequest" }, "409": { "$ref": "#/components/responses/Conflict" } } } },
    "/api/v1/auth/login": { "post": { "summary": "Login (sets token & csrf cookies)", "requestBody": { "required": true, "content": { "application/json": { "schema": { "$ref": "#/components/schemas/LoginRequest" } } } }, "responses": { "200": { "$ref": "#/components/responses/AuthOK" }, "401": { "$ref": "#/components/responses/Unauthorized" } } } },
    "/api/v1/auth/logout": { "post": { "summary": "Logout (blacklists token, clears cookies)", "security": [{ "bearerAuth": [] }, { "cookieAuth": [] }], "parameters": [{ "$ref": "#/components/parameters/CsrfHeader" }], "responses": { "200": { "$ref": "#/components/responses/MessageOK" } } } },
    "/api/v1/profile": { "get": { "summary": "Current user profile", "security": [{ "bearerAuth": [] }, { "cookieAuth": [] }], "responses": { "200": { "description": "User", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/User" } } } } } } },
    "/api/v1/workspaces": {
      "get": { "summary": "List own workspaces", "security": [{ "bearerAuth": [] }, { "cookieAuth": [] }], "responses": { "200": { "description": "List", "content": { "application/json": { "schema": { "type": "array", "items": { "$ref": "#/components/schemas/Workspace" } } } } } } },
      "post": { "summary": "Create workspace (Admin/Architect only)", "security": [{ "bearerAuth": [] }, { "cookieAuth": [] }], "parameters": [{ "$ref": "#/components/parameters/CsrfHeader" }], "requestBody": { "required": true, "content": { "application/json": { "schema": { "$ref": "#/components/schemas/WorkspaceRequest" } } } }, "responses": { "201": { "description": "Created", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/Workspace" } } } }, "403": { "$ref": "#/components/responses/Forbidden" } } }
    },
    "/api/v1/workspaces/{id}": {
      "parameters": [{ "name": "id", "in": "path", "required": true, "schema": { "type": "string" } }],
      "get": { "summary": "Get workspace", "security": [{ "bearerAuth": [] }, { "cookieAuth": [] }], "responses": { "200": { "description": "Workspace", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/Workspace" } } } } } },
      "put": { "summary": "Update workspace (Maintainer+)", "security": [{ "bearerAuth": [] }, { "cookieAuth": [] }], "parameters": [{ "$ref": "#/components/parameters/CsrfHeader" }], "requestBody": { "required": true, "content": { "application/json": { "schema": { "$ref": "#/components/schemas/WorkspaceRequest" } } } }, "responses": { "200": { "description": "Updated" }, "403": { "$ref": "#/components/responses/Forbidden" } } },
      "delete": { "summary": "Delete workspace (Architect+)", "security": [{ "bearerAuth": [] }, { "cookieAuth": [] }], "parameters": [{ "$ref": "#/components/parameters/CsrfHeader" }], "responses": { "200": { "$ref": "#/components/responses/MessageOK" }, "403": { "$ref": "#/components/responses/Forbidden" } } }
    },
    "/api/v1/workspaces/{id}/members": {
      "parameters": [{ "name": "id", "in": "path", "required": true, "schema": { "type": "string" } }],
      "get": { "summary": "List workspace members", "security": [{ "bearerAuth": [] }, { "cookieAuth": [] }], "responses": { "200": { "description": "Members", "content": { "application/json": { "schema": { "type": "array", "items": { "$ref": "#/components/schemas/WorkspaceMember" } } } } } } },
      "post": { "summary": "Add member (Architect+)", "security": [{ "bearerAuth": [] }, { "cookieAuth": [] }], "parameters": [{ "$ref": "#/components/parameters/CsrfHeader" }], "requestBody": { "required": true, "content": { "application/json": { "schema": { "$ref": "#/components/schemas/MemberRequest" } } } }, "responses": { "201": { "$ref": "#/components/responses/MessageOK" }, "403": { "$ref": "#/components/responses/Forbidden" } } }
    },
    "/api/v1/workspaces/{id}/members/{userID}": {
      "parameters": [{ "name": "id", "in": "path", "required": true, "schema": { "type": "string" } }, { "name": "userID", "in": "path", "required": true, "schema": { "type": "string" } }],
      "delete": { "summary": "Remove member (Architect+)", "security": [{ "bearerAuth": [] }, { "cookieAuth": [] }], "parameters": [{ "$ref": "#/components/parameters/CsrfHeader" }], "responses": { "200": { "$ref": "#/components/responses/MessageOK" }, "403": { "$ref": "#/components/responses/Forbidden" } } }
    },
    "/api/v1/admin/users": { "get": { "summary": "List all users (Admin only)", "security": [{ "bearerAuth": [] }, { "cookieAuth": [] }], "responses": { "200": { "description": "Users", "content": { "application/json": { "schema": { "type": "array", "items": { "$ref": "#/components/schemas/User" } } } } }, "403": { "$ref": "#/components/responses/Forbidden" } } } },
    "/api/v1/admin/users/{id}/role": {
      "parameters": [{ "name": "id", "in": "path", "required": true, "schema": { "type": "string" } }],
      "put": { "summary": "Update user platform role (Admin only)", "security": [{ "bearerAuth": [] }, { "cookieAuth": [] }], "parameters": [{ "$ref": "#/components/parameters/CsrfHeader" }], "requestBody": { "required": true, "content": { "application/json": { "schema": { "$ref": "#/components/schemas/RoleUpdateRequest" } } } }, "responses": { "200": { "$ref": "#/components/responses/MessageOK" }, "403": { "$ref": "#/components/responses/Forbidden" } } }
    },
    "/_/sse": { "get": { "summary": "Server-Sent Events stream", "security": [{ "bearerAuth": [] }, { "cookieAuth": [] }], "responses": { "200": { "description": "Event stream", "content": { "text/event-stream": { "schema": { "type": "string" } } } } } }
  },
  "components": {
    "parameters": {
      "CsrfHeader": { "name": "X-CSRF-Token", "in": "header", "required": true, "description": "Must match the csrf_token cookie", "schema": { "type": "string" } }
    },
    "securitySchemes": {
      "bearerAuth": { "type": "http", "scheme": "bearer", "bearerFormat": "JWT" },
      "cookieAuth": { "type": "apiKey", "in": "cookie", "name": "token" }
    },
    "responses": {
      "AuthOK": { "description": "Auth success", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/LoginResponse" } } } },
      "MessageOK": { "description": "OK message", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/MessageResponse" } } } },
      "BadRequest": { "description": "Bad request", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/APIError" } } } },
      "Unauthorized": { "description": "Invalid credentials", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/APIError" } } } },
      "Forbidden": { "description": "Insufficient role or CSRF mismatch", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/APIError" } } } },
      "NotFound": { "description": "Not found", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/APIError" } } } },
      "Conflict": { "description": "Already exists", "content": { "application/json": { "schema": { "$ref": "#/components/schemas/APIError" } } } }
    },
    "schemas": {
      "User": { "type": "object", "properties": { "id": { "type": "string" }, "email": { "type": "string" }, "name": { "type": "string" }, "role": { "type": "string", "enum": ["admin", "architect", "maintainer", "viewer"] }, "created_at": { "type": "string" } }, "required": ["id", "email", "name", "role"] },
      "Workspace": { "type": "object", "properties": { "id": { "type": "string" }, "owner_id": { "type": "string" }, "name": { "type": "string" }, "description": { "type": "string" }, "current_user_role": { "type": "string", "enum": ["architect", "maintainer", "viewer", "admin"] }, "created_at": { "type": "string" }, "updated_at": { "type": "string" } }, "required": ["id", "owner_id", "name"] },
      "WorkspaceMember": { "type": "object", "properties": { "user_id": { "type": "string" }, "email": { "type": "string" }, "name": { "type": "string" }, "role": { "type": "string" }, "created_at": { "type": "string" } } },
      "RegisterRequest": { "type": "object", "properties": { "email": { "type": "string" }, "password": { "type": "string" }, "name": { "type": "string" } }, "required": ["email", "password", "name"] },
      "LoginRequest": { "type": "object", "properties": { "email": { "type": "string" }, "password": { "type": "string" } }, "required": ["email", "password"] },
      "LoginResponse": { "type": "object", "properties": { "token": { "type": "string" }, "user": { "$ref": "#/components/schemas/User" } }, "required": ["token", "user"] },
      "WorkspaceRequest": { "type": "object", "properties": { "name": { "type": "string" }, "description": { "type": "string" } }, "required": ["name"] },
      "MemberRequest": { "type": "object", "properties": { "email": { "type": "string" }, "role": { "type": "string", "enum": ["architect", "maintainer", "viewer"] } }, "required": ["email", "role"] },
      "RoleUpdateRequest": { "type": "object", "properties": { "role": { "type": "string", "enum": ["admin", "architect", "maintainer", "viewer"] } }, "required": ["role"] },
      "MessageResponse": { "type": "object", "properties": { "message": { "type": "string" } } },
      "APIError": { "type": "object", "properties": { "error": { "type": "string" } }, "required": ["error"] },
      "HealthResponse": { "type": "object", "properties": { "status": { "type": "string" }, "service": { "type": "string" }, "version": { "type": "string" } } }
    }
  }
}
```

### Requests
**`all_json/functional/requests/register_request.json`**
```json
{ "name": "Jane Doe", "email": "jane@example.com", "password": "secret123" }
```
**`all_json/functional/requests/login_request.json`**
```json
{ "email": "admin@taawun.com", "password": "admin123" }
```
**`all_json/functional/requests/workspace_create_request.json`**
```json
{ "name": "Design Team", "description": "Shared space for designers" }
```
**`all_json/functional/requests/workspace_update_request.json`**
```json
{ "name": "Design Team v2", "description": "Updated description" }
```
**`all_json/functional/requests/member_add_request.json`** *(New)*
```json
{ "email": "bob@example.com", "role": "viewer" }
```
**`all_json/functional/requests/admin_role_update_request.json`** *(New)*
```json
{ "role": "maintainer" }
```

### Responses
**`all_json/functional/responses/health_response.json`**
```json
{ "status": "healthy", "service": "taawun", "version": "2.1.0" }
```
**`all_json/functional/responses/register_response.json`**
```json
{ "token": "eyJhbGciOi...", "user": { "id": "1755264000000000001", "email": "jane@example.com", "name": "Jane Doe", "role": "architect", "created_at": "2026-08-15 10:00:00" } }
```
**`all_json/functional/responses/login_response.json`**
```json
{ "token": "eyJhbGciOi...", "user": { "id": "admin-001", "email": "admin@taawun.com", "name": "Admin User", "role": "admin", "created_at": "2026-08-15 09:00:00" } }
```
**`all_json/functional/responses/workspace_response.json`**
```json
{ "id": "1755264000000000100", "owner_id": "1755264000000000001", "name": "Design Team", "description": "Shared space", "current_user_role": "architect", "created_at": "2026-08-15 10:05:00", "updated_at": "2026-08-15 10:05:00" }
```
**`all_json/functional/responses/workspaces_list_response.json`**
```json
[
  { "id": "1755264000000000100", "owner_id": "1755264000000000001", "name": "Design Team", "description": "Shared space", "current_user_role": "architect", "created_at": "2026-08-15 10:05:00", "updated_at": "2026-08-15 10:05:00" }
]
```
**`all_json/functional/responses/members_list_response.json`** *(New)*
```json
[
  { "user_id": "1755264000000000001", "email": "alice@example.com", "name": "Alice Architect", "role": "architect", "created_at": "2026-08-15 10:05:00" },
  { "user_id": "1755264000000000002", "email": "bob@example.com", "name": "Bob Viewer", "role": "viewer", "created_at": "2026-08-15 10:06:00" }
]
```
**`all_json/functional/responses/admin_users_response.json`**
```json
[
  { "id": "admin-001", "email": "admin@taawun.com", "name": "Admin User", "role": "admin", "created_at": "2026-08-15 09:00:00" },
  { "id": "1755264000000000001", "email": "alice@example.com", "name": "Alice Architect", "role": "architect", "created_at": "2026-08-15 10:00:00" }
]
```
*(Keep `logout_response.json`, `delete_workspace_response.json`, `profile_response.json`, `error_response.json` as they were, just ensure roles in profile match the new enums).*

---

## 🧪 3. TESTING (Fixtures, Collections, E2E)

### Fixtures
**`all_json/testing/fixtures/valid_register.json`**
```json
{ "case": "register_success", "request": { "name": "Test User", "email": "test@example.com", "password": "secret123" }, "expected_status": 201, "expected_cookies": ["token", "csrf_token"], "expected_fields": ["token", "user.id", "user.role"], "expected_role": "architect" }
```
**`all_json/testing/fixtures/valid_workspace.json`**
```json
{ "case": "workspace_create", "auth": true, "csrf": true, "request": { "name": "QA Space", "description": "Fixture workspace" }, "expected_status": 201, "expected_fields": ["id", "owner_id", "name"] }
```
**`all_json/testing/fixtures/valid_member_add.json`** *(New)*
```json
{ "case": "member_add", "auth": true, "csrf": true, "precondition": "workspace exists, target user exists", "request": { "email": "bob@example.com", "role": "viewer" }, "expected_status": 201, "expected_message": "Member added" }
```
**`all_json/testing/fixtures/forbidden_viewer_edit.json`** *(New)*
```json
{ "case": "rbac_enforcement", "auth": true, "csrf": true, "precondition": "user is viewer in workspace", "request": { "name": "Hacked Name" }, "expected_status": 403, "expected_error": "Maintainer role or higher required" }
```
*(Update other fixtures to include `"csrf": true` for POST/PUT/DELETE requests).*

### Postman Collection
**`all_json/testing/postman/taawun_collection.json`**
```json
{
  "info": { "name": "Taawun API v2.1 Collection", "description": "CSRF Protection: Ensure the Pre-request Script is active to automatically inject the X-CSRF-Token header from cookies.", "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json" },
  "event": [
    {
      "listen": "prerequest",
      "script": {
        "type": "text/javascript",
        "exec": [
          "const csrfToken = pm.cookies.get('csrf_token');",
          "if (csrfToken && ['POST', 'PUT', 'DELETE'].includes(pm.request.method)) {",
          "    pm.request.headers.add({key: 'X-CSRF-Token', value: csrfToken});",
          "}"
        ]
      }
    }
  ],
  "variable": [{ "key": "base_url", "value": "http://localhost:8080" }, { "key": "token", "value": "" }],
  "item": [
    { "name": "Health", "request": { "method": "GET", "url": "{{base_url}}/health" } },
    { "name": "Register", "request": { "method": "POST", "url": "{{base_url}}/api/v1/auth/register", "header": [{ "key": "Content-Type", "value": "application/json" }], "body": { "mode": "raw", "raw": "{\"name\":\"Alice\",\"email\":\"alice@example.com\",\"password\":\"secret123\"}" } } },
    { "name": "Login", "request": { "method": "POST", "url": "{{base_url}}/api/v1/auth/login", "header": [{ "key": "Content-Type", "value": "application/json" }], "body": { "mode": "raw", "raw": "{\"email\":\"admin@taawun.com\",\"password\":\"admin123\"}" } } },
    { "name": "Profile", "request": { "method": "GET", "url": "{{base_url}}/api/v1/profile", "header": [{ "key": "Authorization", "value": "Bearer {{token}}" }] } },
    { "name": "List Workspaces", "request": { "method": "GET", "url": "{{base_url}}/api/v1/workspaces", "header": [{ "key": "Authorization", "value": "Bearer {{token}}" }] } },
    { "name": "Create Workspace", "request": { "method": "POST", "url": "{{base_url}}/api/v1/workspaces", "header": [{ "key": "Content-Type", "value": "application/json" }, { "key": "Authorization", "value": "Bearer {{token}}" }], "body": { "mode": "raw", "raw": "{\"name\":\"Design Team\",\"description\":\"Shared space\"}" } } },
    { "name": "Add Member", "request": { "method": "POST", "url": "{{base_url}}/api/v1/workspaces/:id/members", "header": [{ "key": "Content-Type", "value": "application/json" }, { "key": "Authorization", "value": "Bearer {{token}}" }], "body": { "mode": "raw", "raw": "{\"email\":\"bob@example.com\",\"role\":\"viewer\"}" } } },
    { "name": "List Members", "request": { "method": "GET", "url": "{{base_url}}/api/v1/workspaces/:id/members", "header": [{ "key": "Authorization", "value": "Bearer {{token}}" }] } },
    { "name": "Remove Member", "request": { "method": "DELETE", "url": "{{base_url}}/api/v1/workspaces/:id/members/:userID", "header": [{ "key": "Authorization", "value": "Bearer {{token}}" }] } },
    { "name": "Admin List Users", "request": { "method": "GET", "url": "{{base_url}}/api/v1/admin/users", "header": [{ "key": "Authorization", "value": "Bearer {{token}}" }] } },
    { "name": "Admin Update Role", "request": { "method": "PUT", "url": "{{base_url}}/api/v1/admin/users/:id/role", "header": [{ "key": "Content-Type", "value": "application/json" }, { "key": "Authorization", "value": "Bearer {{token}}" }], "body": { "mode": "raw", "raw": "{\"role\":\"maintainer\"}" } } },
    { "name": "Logout", "request": { "method": "POST", "url": "{{base_url}}/api/v1/auth/logout", "header": [{ "key": "Authorization", "value": "Bearer {{token}}" }] } }
  ]
}
```

### E2E Scenarios
**`all_json/testing/e2e/e2e_scenarios.json`**
```json
{
  "base_url": "http://localhost:8080",
  "global_requirements": { "csrf_header": "X-CSRF-Token", "csrf_cookie": "csrf_token" },
  "scenarios": [
    { "name": "register_and_stay_logged_in", "steps": [
      { "method": "POST", "path": "/api/v1/auth/register", "body_ref": "testing/fixtures/valid_register.json", "expect_status": 201, "expect_cookies": ["token", "csrf_token"] },
      { "method": "GET", "path": "/dashboard", "expect_status": 200, "expect_contains": "Dashboard" }
    ]},
    { "name": "workspace_crud_and_members", "steps": [
      { "method": "POST", "path": "/api/v1/workspaces", "body_ref": "testing/fixtures/valid_workspace.json", "expect_status": 201, "capture": "workspace_id", "require_csrf": true },
      { "method": "GET", "path": "/api/v1/workspaces/{workspace_id}", "expect_status": 200 },
      { "method": "POST", "path": "/api/v1/workspaces/{workspace_id}/members", "body_ref": "testing/fixtures/valid_member_add.json", "expect_status": 201, "require_csrf": true },
      { "method": "GET", "path": "/api/v1/workspaces/{workspace_id}/members", "expect_status": 200 },
      { "method": "DELETE", "path": "/api/v1/workspaces/{workspace_id}", "expect_status": 200, "require_csrf": true }
    ]},
    { "name": "rbac_enforcement", "steps": [
      { "method": "PUT", "path": "/api/v1/workspaces/{workspace_id}", "body": { "name": "Hacked" }, "expect_status": 403, "expect_error": "Maintainer role or higher required", "require_csrf": true, "precondition": "caller is viewer" }
    ]}
  ]
}
```