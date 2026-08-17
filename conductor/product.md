# Product Guide: Taawun Platform Blueprint v0.2

> *"And cooperate in righteousness and piety"* — Surah Al-Ma'idah 5:2

## Vision & One-Paragraph Thesis
Taawun lets communities — mosques, charities, student associations, Muslim founders — describe an application in natural language and get a working, deployable product. Unlike Lovable and its peers, generated apps are not React SPAs tethered to a centralized backend. Each app is a **static microfrontend** composed over a shared primitive layer (Appwrite-style ergonomics, but Taawun-hosted only where essential), with **local-first data owned by the community itself**, synchronized peer-to-peer via RxDB over WebRTC. Anything touching money runs on **AZOA**, a federated financial orchestration engine (first node self-hosted by us). Compliance with Islamic jurisprudence is a scholar-authored guardrail corpus enforced through RAG at generation time, review time, and runtime.

---

## The Core Architectural Stance (Amanah & Zero Data Custody)
**Taawun does not hold your data. It holds your identity and your highway.**

Deploying an app in Taawun v1 means publishing a static microfrontend bundle. The app's data lives on the devices of the community using it, replicated locally-first through RxDB and synchronized peer-to-peer over WebRTC. What Taawun hosts centrally is minimal: an SSO-style identity service that works across every deployed app, and a relay layer (signaling, STUN/TURN, and optional encrypted sync super-peer relays) that makes the P2P mesh performant and resilient when peers are offline or behind NATs. The relays move encrypted bytes; they do not become a database.

This stance is the direct technical implementation of **amanah**: user data as a sacred trust.

### The State Split: Convergent vs. Transactional
1. **Convergent State**: Content, schedules, member lists, page copy, UI themes. Lives in CRDT-backed RxDB collections. Conflicts merge automatically; offline edits are first-class; eventual consistency applies.
2. **Transactional State**: Balances, payments, escrows, splits, pledges. Must NEVER live in a CRDT. Money requires exactly-once semantics, fail-closed behavior, and real-world reconciliation. Delegated to **AZOA quest graphs**.

---

## 5 System Layers

1. **Layer 1 — The Vibecoding Engine**: LLM orchestrated through MCP against a sandboxed build environment. Generates microfrontend bundles composing Taawun primitive widgets. Augmented by Compliance RAG.
2. **Layer 2 — Shared Primitives**: Taawun SSO (cross-app identity), Relay infrastructure (WebRTC signaling, TURN, encrypted sync super-peers), static bundle hosting, and notification fan-out.
3. **Layer 3 — The App Runtime**: In-browser microfrontend shell, RxDB WebRTC replication plugin, IndexedDB persistence, and Taawun SDK.
4. **Layer 4 — Financial Orchestration (AZOA)**: Powers platform marketplace (escrow quests, creator revenue splits) and app-level financial primitives (Zakat calculator, donation drives, Qard Hasan trackers, volunteer stipends) using AZOA STAR primitives.
5. **Layer 5 — Compliance Infrastructure**: Scholar-authored RAG corpus enforcing fiqh guardrails at generation, publish (fiqh-linting), and runtime.

---

## Target Users & Roles (Signed Capability RBAC)

| Role / Segment | Responsibilities | Capability Tokens Issued |
|:---|:---|:---|
| **Architect** | Vibecodes new apps/templates, manages workspace AZOA relationships & billing | Full CRDT write + AZOA quest execution & template publish |
| **Maintainer** | Edits convergent state (donation page copy, event schedules) | Write access to convergent CRDT collections |
| **Viewer** | End-user, congregant, donor | Read-only access & public app interaction |

---

## Marketplace & Anti-Gharar Deployment Lifecycle
- **Live Staging Preview**: Primary mitigation of *gharar*. Buyers test live staging preview before payment.
- **AZOA Escrow Quests**: Escrow settles to creator and platform only after buyer confirmation. Failed or ambiguous deployments fail closed and refund.
- **Riba-Free Flat Pricing**: Flat fees for static bundle hosting, SSO seats, relay bandwidth tier, and AZOA capacity. No interest-bearing late penalties.

---

## Design System (Blueprint v0.2 & AZOA Style Reference)
- **Typography**: `Fraunces` (Serif titles & Arabic callouts), `IBM Plex Sans` (Body), `IBM Plex Mono` (Code & Eyebrows).
- **Color Palette**:
  - Ink Backgrounds: `#11161a` (base), `#171e24` (containers)
  - Parchment Text: `#eee7d8` (dim: `#c9c0ac`)
  - Emerald Accents: `#3c7263` / `#57a68e`
  - Gold Highlights: `#c7a24a`
  - Terracotta/Rust Accent (AZOA): `#c8501e`
- **Pattern**: Geometric Girih star SVG background (`.girih-bg`).

