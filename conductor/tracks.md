---
type: track-index
title: Conductor Tracks — Taawun Platform
lastReconciled: 2026-08-10
---

# Conductor Master Tracks Index

**Status is authoritative in each track's `metadata.json`.** This index summarizes the architectural roadmap of the Taawun Platform, reconciled on 2026-08-10.

Total Tracks: 6 | 4 Completed · 0 InProgress · 2 Planned

---

## Program — Core Platform & IaC Runtime (Wave 1)

| Wave | Track ID | Title | Status | Scope |
|:---|:---|:---|:---|:---|
| 1 | `go_iac_control_plane_20260810` | Infrastructure-as-Code Control Plane & Docker Provider | **✅ Completed** | `pkg/iac`, `cmd` |
| 1 | `mcp_vibecoding_engine_20260810` | Embedded Model Context Protocol (MCP) Server | **✅ Completed** | `pkg/mcp` |
| 1 | `web_p2p_relay_and_ltap_20260810` | Web P2P Relay Hub & LTAP Storage Primitives (`ch01-a-million-apps.md`) | **✅ Completed** | `pkg/primitives` |
| 1 | `taqwa_ethics_guardrails_20260810` | Taqwa Ethics Audit & Anti-Gharar Engine | **✅ Completed** | `pkg/ethics` |

---

## Program — Shura Governance & Community Marketplace (Wave 2)

| Wave | Track ID | Title | Status | Scope |
|:---|:---|:---|:---|:---|
| 2 | `shura_rbac_governance_20260810` | Shura Workspace Dynamics & Granular RBAC (Architect, Maintainer, Viewer) | **Planned** | `pkg/shura` |
| 2 | `template_bazaar_marketplace_20260810` | The Bazaar — Community Template Directory & 1-Click Customization | **Planned** | `pkg/bazaar` |

---

## Track Index Details

Per-track `spec.md` / `plan.md` / `metadata.json` / `DECISIONS.md` live at `conductor/tracks/<track_id>/`.
