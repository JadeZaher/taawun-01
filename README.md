# Attention Ahmad Zaher

For testing purposes if approved
[99 Dollars An App](https://nabimuhammad.com/99-dollars-an-app/)
[Taawun Github Repository](https://github.com/99-dollars-an-app/taawun)
Email: [99-dollars-an-app@nabimuhammad.com](mailto:99-dollars-an-app@nabimuhammad.com)

---

# Taawun

Taawun is a collaborative platform where community teams create, customize, and deploy applications in shared workspaces.

This repository represents **Phase 0 (Foundation)**, delivering a secure, role-based backend, real-time collaboration infrastructure, and a clean, minimal frontend.

## Core Features

- **Secure Authentication**: JWT-based sessions with bcrypt password hashing and double-submit cookie CSRF protection. Includes token revocation on logout.
- **Granular RBAC**: Two-tier Role-Based Access Control.
  - *Platform Roles*: `admin`, `architect`, `maintainer`, `viewer`.
  - *Workspace Roles*: Assigned per-project to allow flexible team collaboration.
- **Community Workspaces**: Shared environments where users can invite registered members, manage permissions, and collaborate.
- **Real-Time Collaboration**: Server-Sent Events (SSE) broadcast workspace and membership changes instantly to all connected clients without manual refreshing.
- **Minimalist UI**: Swiss-style, Bootstrap 5 frontend focused on clarity and performance.

## Tech Stack

- **Backend**: Go 1.21+, Gorilla Mux, `golang-jwt`
- **Database**: SQLite (via `modernc.org/sqlite`)
- **Frontend**: HTML Templates, Bootstrap 5, Vanilla JS
- **Infrastructure**: Docker, Docker Compose

## Quick Start (Local Development)

### 1. Prerequisites
- Go 1.21 or higher

### 2. Environment Setup
Create a `.env` file in the root directory. You need a secure random string for the JWT secret.

**PowerShell:**
```powershell
$secret = -join ((48..57)+(65..90)+(97..122) | Get-Random -Count 48 | ForEach-Object {[char]$_})
"JWT_SECRET=$secret`nPORT=8080`nDB_PATH=./taawun.db`nCOOKIE_SECURE=false" | Out-File -FilePath .env -Encoding UTF8
```

**Bash/Linux/macOS:**
```bash
echo "JWT_SECRET=$(openssl rand -hex 32)" > .env
echo "PORT=8080" >> .env
echo "DB_PATH=./taawun.db" >> .env
echo "COOKIE_SECURE=false" >> .env
```

### 3. Run the Server
```bash
go run ./cmd/server
```
The application will automatically create the SQLite database, run migrations, and seed the default admin account.

## Quick Start (Docker)

If you prefer containerized development, simply run:

```bash
docker-compose up --build
```
This will build the Go binary, start the server on port `8080`, and persist the SQLite database in a local `./data` volume.

## Default Credentials

Upon first boot, an administrative account is automatically generated:

- **URL**: [http://localhost:8080](http://localhost:8080)
- **Email**: `admin@taawun.com`
- **Password**: `admin123`

*(Note: It is highly recommended to change this password or create a new admin user in a production environment).*

## API Documentation

The full REST API contract, including RBAC enforcement rules and CSRF requirements, is documented in:
- OpenAPI Spec: `docs/swagger.yaml`
- JSON Artifacts & Fixtures: `all_32_json/`

## Project Structure

```text
taawun/
├── cmd/server/       # Go backend source code (split by domain)
├── internal/web/     # HTML templates and static assets
├── migrations/       # SQL schema definitions
├── docs/             # OpenAPI/Swagger documentation
├── all_32_json/         # API contracts, fixtures, and testing artifacts
├── Dockerfile        # Multi-stage production build
└── docker-compose.yml
```