# Taawun Platform Blueprint v0.2 — Handoff & Session Passoff

**Repository Target**: `https://github.com/99-dollars-an-app/taawun.git`  
**Branch**: `main` (Synchronized & Pushed)  
**Status**: All Conductor Tracks & Blueprint v0.2 Architecture Integrated & Operational

---

## Executive Summary & Blueprint v0.2 Architecture

Taawun is a vibecoding platform for the Ummah: Lovable's prompt-to-app experience, rebuilt on local-first peer-to-peer foundations, with Islamic compliance enforced structurally rather than cosmetically.

### 1. Core Architectural Stance: Zero Data Custody (Amanah)
- **Taawun does not hold community data. It holds identity and transit.**
- Microfrontends are published as static bundles. Data lives on community devices, replicated locally through RxDB and synchronized peer-to-peer over WebRTC.
- Centralized infrastructure is strictly limited to **Taawun SSO** (cross-app identity) and **Relay Infrastructure** (signaling, STUN/TURN, and encrypted sync super-peers).

### 2. State Split: Convergent vs. Transactional
- **Convergent State** (content, schedules, page copy, UI themes): Lives in CRDT-backed RxDB collections with WebRTC sync. Offline edits are first-class; eventual consistency applies.
- **Transactional State** (balances, payments, escrows, splits, pledges): Delegated to **AZOA quest graphs**. Fail-closed, exactly-once settlement with real-world reconciliation.

### 3. 5 System Layers
1. **Layer 1: Vibecoding Engine**: MCP JSON-RPC 2.0 orchestration against sandboxed build environments with embedded Compliance RAG.
2. **Layer 2: Shared Primitives**: Taawun SSO, WebRTC signaling, STUN/TURN, and encrypted sync super-peer relays.
3. **Layer 3: App Runtime**: Microfrontend shell, RxDB WebRTC replication, IndexedDB persistence, and Taawun SDK.
4. **Layer 4: Financial Orchestration (AZOA)**: Marketplace escrow quests, revenue splits, and app-level STAR primitives (Zakat, donation drives, Qard Hasan, volunteer stipends).
5. **Layer 5: Compliance Infrastructure**: Scholar-authored RAG guardrail corpus enforcing fiqh compliance at generation time, publish time (fiqh-linting), and runtime.

### 4. RBAC as Signed Capability Tokens
- **Architect**: Full CRDT write + AZOA quest execution & template publishing.
- **Maintainer**: Write access to convergent CRDT collections (schedules, text, UI).
- **Viewer**: Read-only access and public app interaction.

### 5. Design System Specs (AZOA Reference)
- **Typography**: `Fraunces` (Serif titles & Arabic callouts), `IBM Plex Sans` (Body), `IBM Plex Mono` (Code & Eyebrows).
- **Color Palette**:
  - Ink Backgrounds: `#11161a` (base), `#171e24` (containers)
  - Parchment Text: `#eee7d8` (dim: `#c9c0ac`)
  - Emerald Accents: `#3c7263` / `#57a68e`
  - Gold Highlights: `#c7a24a`
  - Terracotta Accent (AZOA): `#c8501e`
- **Pattern**: Geometric Girih star SVG background (`.girih-bg`).

---

## Code Base Map & Verification

- `src/pkg/conductor`: Conductor Track orchestration pipeline.
- `src/pkg/mcp`: Embedded MCP JSON-RPC 2.0 Vibecoding Engine.
- `src/pkg/primitives`: WebRTC P2P signaling relay & LTAP storage.
- `src/pkg/ethics`: Taqwa Ethics Guardrails & Compliance RAG scanner.
- `src/pkg/financial`: AZOA STAR primitives (Escrow quests, Zakat, Qard Hasan).
- `src/pkg/shura`: Capability token RBAC verifier (`Architect`, `Maintainer`, `Viewer`).
- `conductor/`: Full product guide, tech stack, setup state, and 8 track specifications.

---

## Next Action Items for Continuation
1. Expand scholar-authored RAG corpus in `pkg/ethics` with tagged madhhab rulings.
2. Extend RxDB WebRTC sync client adapters in `src/web/`.
3. Launch community-hosted relay nodes and AZOA federation bridges.
