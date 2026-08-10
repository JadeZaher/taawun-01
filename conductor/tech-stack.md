# Tech Stack Specification: Taawun Platform

## Architecture Overview

Taawun is structured as a modular **Go Control Plane & Infrastructure Engine** running an embedded Model Context Protocol (MCP) server, Web P2P signaling relay, Appwrite-inspired primitives, and Taqwa ethics scanner, paired with an embedded frontend GUI.

---

## Core Components & Technology Stack

| Layer / Subsystem | Technology | Version | Purpose & Rationale |
|:---|:---|:---|:---|
| **Core Language** | Go | 1.21+ | High concurrency, low latency, memory safety, binary embeddability |
| **HTTP Router** | Gorilla Mux | 1.8.1 | High-performance RESTful routing and subrouter middleware |
| **CORS Handler** | Gorilla Handlers | 1.5.2 | Cross-Origin Resource Sharing for browser sandbox integration |
| **WebSocket Engine** | Gorilla Websocket | 1.5.3 | Real-time Web P2P signaling and WebSocket fallback stream relay |
| **Container Engine** | Docker Engine API | v25+ | Sandboxed app compilation, staging previews, and VPS deployments |
| **Embedded Storage** | SQLite3 (go-sqlite3) | 1.14.x | CGO-compiled LTAP (Hybrid Transactional/Analytical) storage |
| **Security & Auth** | golang-jwt / bcrypt | v5 / v0.54 | Token verification, password hashing, and API key management |
| **Agent Interface** | MCP JSON-RPC 2.0 | 2024-11-05 | Standardized AI agent tooling for local and cloud execution |
| **Web UI** | Embedded Go FS (`web.FS`) | HTML5/JS | Geometric Swiss-style UI embedded directly inside compiled binary |

---

## Infrastructure & Deployment Drivers

| Environment | Driver / Provider | Details |
|:---|:---|:---|
| **Local Staging Sandbox** | Docker API Driver (`pkg/iac`) | Spawns isolated container sandboxes with explicit RAM/CPU limits |
| **Peer-to-Peer Artifacts** | Web P2P Relay (`pkg/primitives`) | WebRTC signaling + WebSocket fallback relay for browser tools |
| **Cloud VPS (Planned)** | Hetzner / Fly.io / Caddy | Automated TLS certification, domain routing, and flat-rate hosting |

---

## Testing & Quality Assurance

| Test Category | Tool | Scope |
|:---|:---|:---|
| **Unit & Integration Tests** | Go `testing` package | `pkg/conductor`, `pkg/ethics`, `pkg/mcp`, `pkg/primitives`, `pkg/iac` |
| **Taqwa Audit Benchmarks** | Go test suites | Validating Riba detection regex and AST pattern matchers |
| **MCP Tool Tracing** | `httptest` recorder | JSON-RPC 2.0 request/response verification |
