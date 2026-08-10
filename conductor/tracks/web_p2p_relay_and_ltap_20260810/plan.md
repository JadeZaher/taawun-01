# Implementation Plan: Web P2P Relay & LTAP Storage

**Track ID**: `web_p2p_relay_and_ltap_20260810`  
**Status**: Completed  

---

## Phase 1: P2P Relay & Signaling Hub
- [x] Task 1.1: Define `P2PMessage` and `Client` structs.
- [x] Task 1.2: Implement `P2PRelayHub` with artifact-scoped peer map and concurrency mutexes.
- [x] Task 1.3: Implement WebSocket read and write pumps (`p2p.go`).

## Phase 2: LTAP Data Storage Engine
- [x] Task 2.1: Implement `LTAPStorageService` with per-artifact SQLite database pooling.
- [x] Task 2.2: Implement `CreateCollection` schema initializer (`ltap.go`).
