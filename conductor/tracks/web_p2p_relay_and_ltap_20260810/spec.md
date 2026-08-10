# Spec: Local-First Web P2P Relay & LTAP Storage Primitives

**Track ID**: `web_p2p_relay_and_ltap_20260810`  
**Status**: Completed  
**Authoritative Location**: `pkg/primitives/p2p.go`, `pkg/primitives/ltap.go`

---

## 1. Background
Addresses the problem highlighted in `ch01-a-million-apps.md`: millions of AI-generated browser artifacts have nowhere to live or waste cloud DB rows. Taawun provides a local-first peer network primitive where user devices hold the data, using Taawun as a signaling hub and relay fallback.

---

## 2. Acceptance Criteria
- [x] WebSocket handler at `/api/p2p/stream` manages client registration per `artifactId`.
- [x] Relays targeted or broadcast CRDT sync messages between connected browser instances.
- [x] `LTAPStorageService` dynamically provisions isolated SQLite database files per artifact.
