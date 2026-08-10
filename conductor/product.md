# Product Guide: Taawun Platform

> *"And cooperate in righteousness and piety"* — Surah Al-Ma'idah 5:2

## Vision
Taawun is a multi-tenant Infrastructure-as-Code (IaC) control plane, Model Context Protocol (MCP) vibecoding engine, and local-first application ecosystem designed to remove technical barriers for community organizations, non-profits, student groups, and ethical entrepreneurs. It bridges AI-assisted natural language software generation ("vibecoding") with rigorous Islamic ethics, data sovereignty (Amanah), and peer-to-peer data ownership.

---

## Target Users & Constituencies

| User Segment | Description | Primary Use Case | Priority |
|:---|:---|:---|:---|
| **Community Architects** | Technical volunteers, developers, and platform builders | Vibecoding templates, managing cloud manifests & infrastructure specs | Core (Active) |
| **Organization Maintainers** | Mosque administrators, non-profit directors, team leads | Customizing deployed app content, donation thresholds, and volunteer rosters | Core (Active) |
| **Community Members & Viewers** | General public, congregants, volunteers, donors | End-user interaction with deployed apps (Standup trackers, Zakat calculators, Halal directories) | Core (Active) |
| **Ethical Entrepreneurs** | Creators of Halal applications & tools | Publishing templates to The Bazaar, earning Riba-free commissions | Core (Active) |

---

## Problems Solved

1. **The Sandbox Paradox ("A Million Apps, Nowhere to Live")**: AI code generators produce transient browser artifacts isolated from real data. Taawun provides **Local-First Web P2P networks** (`ch01-a-million-apps.md`) with relay fallbacks, giving generated tools a permanent home without wasteful cloud tenant bloat.
2. **Contractual Uncertainty (Gharar)**: Buying unpredictable AI-generated tools with hidden uptime or resource limits violates Islamic jurisprudence. Taawun mandates zero-cost live staging previews with explicit SLA and capacity specs before payment checkout.
3. **Interest & Exploitative Billing (Riba)**: Traditional cloud PaaS platforms enforce interest-bearing late penalties. Taawun enforces flat infrastructure pricing and Riba-free fee structures.
4. **Data Bleed & Privacy (Amanah)**: Donor lists and mosque records are a sacred trust. Taawun enforces cryptographic tenant isolation and peer-to-peer data storage where client devices own their records.
5. **Content Guardrails**: Automated Haram-check AST & prompt scanners prevent the generation or hosting of non-compliant platforms (Riba loan calculators, gambling, prohibited content).

---

## Core Features & System Boundaries

### Implemented
- **Go IaC Control Plane (`pkg/iac`)**: Docker Engine API provider for automated container provisioning & manifest generation.
- **Embedded Go MCP Engine (`pkg/mcp`)**: JSON-RPC 2.0 interface connecting LLM vibecoding agents directly to local and sandboxed environments.
- **Local-First Web P2P & LTAP Storage (`pkg/primitives`)**: WebRTC signaling + WebSocket relay hub for browser artifacts, paired with isolated SQLite analytical/transactional storage.
- **Taqwa Ethics & Anti-Gharar Engine (`pkg/ethics`)**: Automated AST/prompt scanner for Islamic compliance and pre-payment staging preview validation.
- **Platform Specification API (`/api/conductor/spec`)**: OpenAPI-compliant specification metadata endpoint.

### Planned (Wave 2)
- **The Shura Workspace & Granular RBAC (`pkg/shura`)**: Formalized role-based access for `Architect`, `Maintainer`, and `Viewer`.
- **The Bazaar Marketplace**: Public showcase directory of vetted app templates with 1-click test drives.
- **Multi-Cloud IaC Extensions**: Hetzner Cloud, Fly.io, and Caddy automated TLS/domain routing.

---

## Success Metrics

1. **Vibecoding Execution Latency**: Under 2 seconds from prompt submission to staged container preview.
2. **Ethics Compliance Pass Rate**: 100% automated blockage of Riba, gambling, or non-compliant domain models.
3. **P2P Relay Connection Success**: 99.5% WebRTC/WebSocket connection success for sandboxed browser artifacts.
4. **Infrastructure Cost Efficiency**: Zero un-refrigerated empty tenant databases through local-first P2P data sync.
