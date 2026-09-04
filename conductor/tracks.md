---
type: track-index
title: Conductor Tracks — Taawun Platform
lastReconciled: 2026-09-02
---

# Conductor Master Tracks Index

**Status is authoritative in each track's `metadata.json`.** This index summarizes the architectural roadmap of the Taawun Platform, reconciled on 2026-09-02.

Total Tracks: 12 | 4 Foundation Tracks Completed · 4 In Progress · 4 Planned

## Program — Mosque-facing alpha integration (Wave 5)

| Wave | Track ID | Title | Status | Scope |
|:---|:---|:---|:---|:---|
| 5 | `mosque_alpha_20260902` | Mosque Alpha — Guided Site, Events, and Registration | **In Progress** | The only customer-alpha ship gate: guided Mosque Essentials, hosted public URL, real registration, collaboration, operations, accessibility, and deployed E2E |

Wave 5 deliberately narrows the customer outcome. It consumes the applicable
builder-review and guided-co-builder work without requiring every platform
primitive in Wave 4. See its `spec.md`, `plan.md`, and `release-gates.md` for the
authoritative scope and NO-GO criteria.

## Program — Sellable MVP integration (Wave 4)

| Wave | Track ID | Title | Status | Scope |
|:---|:---|:---|:---|:---|
| 4 | `p0_private_beta_20260816` | Sellable MVP — Complete Primitive Loop | **In Progress** | Broader platform/MVP program: hosted MCP, signed cards, Datastar surfaces, local-first runtime, Shura, Bazaar, AZOA, compliance, federation, deployment; not the mosque-alpha gate |
| 4 | `builder_review_delivery_20260902` | Builder Review Links and Live Delivery QA | **In Progress** | Protected preview handoff, browser E2E, verified-domain publication, revocation, Railway promotion runbook |
| 4 | `guided_ai_cobuilder_20260902` | Guided AI Co-builder for Non-technical Organizers | **In Progress** | One-question-at-a-time builder, progressive disclosure, typed assistant proposals, safe Advanced mode |

The earlier “Completed” labels record foundation-track completion, not current
sellable-product readiness. Wave 5 owns the mosque-facing integration gate.

---

## Program — Core Platform & IaC Runtime (Wave 1)

| Wave | Track ID | Title | Status | Scope |
|:---|:---|:---|:---|:---|
| 1 | `go_iac_control_plane_20260810` | Infrastructure-as-Code Control Plane & Docker Provider | **✅ Completed** | `pkg/iac`, `cmd` |
| 1 | `mcp_vibecoding_engine_20260810` | Embedded Model Context Protocol (MCP) Server | **✅ Completed** | `pkg/mcp` |
| 1 | `web_p2p_relay_and_ltap_20260810` | Web P2P Relay Hub & LTAP Storage Primitives (`ch01-a-million-apps.md`) | **✅ Completed** | `pkg/primitives` |
| 1 | `taqwa_ethics_guardrails_20260810` | Taqwa Ethics Audit & Anti-Gharar Engine | **✅ Completed** | `pkg/ethics` |

---

## Program — Security Hardening & Gap Remediation (Wave 2)

| Wave | Track ID | Title | Status | Scope |
|:---|:---|:---|:---|:---|
| 2 | `adversarial_security_hardening_20260810` | Adversarial Security & Infrastructure Hardening | **Planned** | `pkg/ethics`, `pkg/primitives`, `pkg/iac` |
| 2 | `gap_analysis_remediation_20260810` | Platform Gap Analysis Remediation (DuckDB, Caddy, CRDTs) | **Planned** | `pkg/primitives`, `pkg/iac`, `pkg/shura` |

---

## Program — Shura Governance & Community Marketplace (Wave 3)

| Wave | Track ID | Title | Status | Scope |
|:---|:---|:---|:---|:---|
| 3 | `shura_rbac_governance_20260810` | Shura Workspace Dynamics & Granular RBAC (Architect, Maintainer, Viewer) | **Planned** | `pkg/shura` |
| 3 | `template_bazaar_marketplace_20260810` | The Bazaar — Community Template Directory & 1-Click Customization | **Planned** | `pkg/bazaar` |

---

## Track Index Details

Per-track `spec.md` / `plan.md` / `metadata.json` / `DECISIONS.md` live at `conductor/tracks/<track_id>/`.
