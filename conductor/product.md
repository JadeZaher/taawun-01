---
type: product-guide
---

# Product Guide: Taawun Platform Blueprint v0.2

> *"And cooperate in righteousness and piety"* — Surah Al-Ma'idah 5:2

## Vision & One-Paragraph Thesis
Taawun lets communities — mosques, charities, student associations, Muslim founders — describe an application through our LLM or their preferred MCP-capable LLM and get a working, deployable product. Unlike Lovable and its peers, generated apps are not React SPAs tethered to a centralized data backend. Taawun's hosted MCP control plane composes curated primitives into **signed cards bound to a user, workspace, and approved domain**. Cards run as Datastar surfaces and can be embedded modularly or composed into a monolithic application, with **local-first data owned by the community itself**, synchronized peer-to-peer through an IndexedDB/CRDT runtime over WebRTC or an authenticated opaque relay. Anything touching money runs on **AZOA**, a federated financial orchestration engine (first node self-hosted by us). Compliance with Islamic jurisprudence is a scholar-authored guardrail corpus enforced through RAG at generation time, review time, and runtime.

---

## The Core Architectural Stance (Amanah & Zero Data Custody)
**Taawun does not hold your data. It holds your identity and your highway.**

Deploying an app in Taawun v1 means publishing an immutable manifest plus browser assets and server-rendered adapters. The app's convergent data lives on the devices of the community using it, persisted in IndexedDB and synchronized peer-to-peer over WebRTC or through an authenticated encrypted-byte relay. What Taawun hosts centrally is minimal: identity, immutable artifacts, explicitly transactional control records, and the relay highway. The relays move encrypted bytes; they do not become a readable community database.

This stance is the direct technical implementation of **amanah**: user data as a sacred trust.

### The State Split: Convergent vs. Transactional
1. **Convergent State**: Content, schedules, member lists, page copy, UI themes. Lives in CRDT-backed RxDB collections. Conflicts merge automatically; offline edits are first-class; eventual consistency applies.
2. **Transactional State**: Balances, payments, escrows, splits, pledges. Must NEVER live in a CRDT. Money requires exactly-once semantics, fail-closed behavior, and real-world reconciliation. Delegated to **AZOA quest graphs**.

---

## 5 System Layers

1. **Layer 1 — The Vibecoding Engine**: LLM orchestrated through typed MCP tools against a curated artifact builder. Generates immutable modular bundles composing Taawun primitives. Augmented by Compliance RAG; arbitrary generated server code or container execution is not required.
2. **Layer 2 — Shared Primitives**: Taawun SSO (cross-app identity), Relay infrastructure (WebRTC signaling, TURN, encrypted sync super-peers), static bundle hosting, and notification fan-out.
3. **Layer 3 — The App Runtime**: User/workspace-signed Datastar cards on approved domains, modular embeds and monolithic composition, IndexedDB/CRDT persistence, WebRTC/relay synchronization, and the Taawun browser SDK.
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

