# Taawun Platform & Conductor Track - Source Code

Taawun ("Cooperate in righteousness and piety" - Surah Al-Ma'idah 5:2) is an Infrastructure-as-Code (IaC) control plane, embedded Model Context Protocol (MCP) server, and multi-tenant orchestration platform built for ethical AI-assisted application generation ("vibecoding").

---

## Architecture Overview

```
src/
├── cmd/               # Application entry point & router configuration
├── pkg/
│   ├── conductor/     # Conductor Track (Lifecycle pipeline coordinator)
│   ├── mcp/           # Embedded Go MCP Server (JSON-RPC 2.0 interface)
│   ├── iac/           # Infrastructure-as-Code Engine (Docker API driver)
│   ├── primitives/    # Appwrite-style Ergonomics (LTAP Storage & Web P2P Relay)
│   ├── ethics/        # Taqwa Compliance & Anti-Gharar Guardrails
│   ├── database/      # SQLite database initialization
│   ├── handlers/      # REST API HTTP handlers
│   ├── models/        # Data models & workspace structs
│   ├── repositories/  # Database persistence repositories
│   └── services/      # Business logic services
└── web/               # Embedded frontend GUI
```

---

## Key Developer Ergonomics & Spec API

### 1. Platform Specification Endpoint (`GET /api/conductor/spec`)
Provides real-time OpenAPI metadata and platform capability documentation.

### 2. Conductor Track Orchestration (`POST /api/conductor/track`)
Executes an end-to-end vibecoding pipeline across 6 step stages:
1. `STAGED_PROMPT` - Input validation & metadata staging.
2. `ETHICS_APPROVED` - Taqwa compliance & Anti-Gharar audit (`pkg/ethics`).
3. `MANIFEST_COMPILED` - Infrastructure-as-Code manifest compilation (`pkg/iac`).
4. `P2P_PROVISIONED` - Web P2P signaling relay & LTAP SQLite database creation (`pkg/primitives`).
5. `CONTAINER_STAGED` - Docker sandbox container staging preview.
6. `DEPLOYED_COMPLETE` - Output metrics, live endpoints, and trace history.

### 3. Embedded MCP Server (`POST /api/mcp`)
Exposes tools over Model Context Protocol JSON-RPC 2.0:
- `run_ethics_check`: Prompt & AST compliance scanner.
- `deploy_container`: Infrastructure provisioning driver.
- `init_p2p_relay`: Local-first Web P2P signaling initialization.

### 4. Web P2P Signaling & WebSocket Relay (`GET /api/p2p/stream`)
WebRTC signaling hub and WebSocket fallback stream for browser-sandbox artifacts running local-first networks.

---

## Requirements

- **Go**: 1.21 or higher
- **GCC**: Required for SQLite CGO compilation
- **Docker**: Optional for containerized staging deployment

---

## How to Run (Development)

### Windows PowerShell:
```powershell
go run ./cmd/main.go
```

### Mac / Linux:
```bash
make run
```

---

## Running Unit Tests

```bash
go test -v ./...
```
