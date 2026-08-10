# Architecture Decisions: Web P2P Relay & LTAP Storage

**Track ID**: `web_p2p_relay_and_ltap_20260810`

---

### Decision 1: Scoped Relaying by Artifact ID
- **Ruling**: Peers register with `artifactId` query param. Messages are broadcast strictly within the same `artifactId` namespace, ensuring zero cross-tenant bleeding.
