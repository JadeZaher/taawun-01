---
type: architecture
---

# Tech Stack Specification: Taawun Platform Blueprint v0.2

## Architecture Overview

Taawun is structured as a 5-layer local-first vibecoding ecosystem with zero readable community data custody. The platform comprises a Go Control Plane, MCP Vibecoding Engine, RxDB WebRTC P2P Sync Relay Layer, AZOA Financial Orchestration Engine, and Scholar-authored Compliance RAG.

---

## 5-Layer Technology Stack

| Layer | Technology | Key Components | Purpose & Rationale |
|:---|:---|:---|:---|
| **Layer 1: Vibecoding Engine** | Go 1.25, official MCP Go SDK, Streamable HTTP, curated artifact builder | `pkg/mcp`, `pkg/artifacts` | Generates immutable modular bundles with embedded Compliance RAG and no arbitrary-code execution path |
| **Layer 2: Shared Primitives** | Taawun SSO, WebRTC Signaling, STUN/TURN | `pkg/primitives` | Cross-app identity, WebRTC signaling & encrypted sync super-peers |
| **Layer 3: App Runtime** | Datastar 1.0.2, signed card manifests, IndexedDB/CRDT runtime, WebRTC/opaque relay | `web`, generated bundles | Local-first convergent state plus server-owned control/transactional projections across approved-domain card, embed, and monolithic surfaces |
| **Layer 4: Financial Engine** | AZOA Quest Graphs, STAR Primitives | `pkg/financial` | Fail-closed, exactly-once settlement for escrows, Zakat, Qard Hasan |
| **Layer 5: Compliance** | Scholar-Authored RAG Corpus, Fiqh-Linter | `pkg/ethics` | Generation, publish (Bazaar), and runtime fiqh compliance audit |

---

## Security & RBAC Capabilities

| Role | Capabilities | Verification |
|:---|:---|:---|
| **Architect** | Full CRDT write, AZOA quest trigger, template publish | Signed JWT Capability Token (`pkg/shura`) |
| **Maintainer** | Write access to convergent state collections (events, copy) | Signed JWT Capability Token (`pkg/shura`) |
| **Viewer** | Read-only interaction & public features | Public / Anonymous Session |

---

## Design System Specs (Blueprint v0.2 & AZOA Reference)

- **Typography**: `Fraunces` (Serif titles & Arabic callouts), `IBM Plex Sans` (Body), `IBM Plex Mono` (Code & Eyebrows).
- **Color Palette**:
  - Ink Backgrounds: `#11161a` (base), `#171e24` (containers)
  - Parchment Text: `#eee7d8` (dim: `#c9c0ac`)
  - Emerald Accents: `#3c7263` / `#57a68e`
  - Gold Highlights: `#c7a24a`
  - Terracotta Accent: `#c8501e`
- **Pattern**: Geometric Girih star SVG background (`.girih-bg`).

---

## Testing & Quality Assurance

| Test Category | Tool | Scope |
|:---|:---|:---|
| **Unit & Integration Tests** | Go `testing` package | `pkg/conductor`, `pkg/ethics`, `pkg/mcp`, `pkg/primitives`, `pkg/financial`, `pkg/shura` |
| **Taqwa Audit Benchmarks** | Go test suites | Validating Riba detection, Fiqh-linting, and Anti-Gharar staging previews |
| **AZOA Quest Tests** | Go `testing` | Validating fail-closed escrow settlement and zero-interest Zakat quests |

