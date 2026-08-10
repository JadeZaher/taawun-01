# Spec: Platform Gap Analysis Remediation

**Track ID**: `gap_analysis_remediation_20260810`  
**Status**: Planned  
**Scope**: `pkg/primitives`, `pkg/iac`, `pkg/ethics`, `pkg/shura`

---

## 1. Requirements

### Functional Requirements
- **FR-1**: Integrate DuckDB analytical query layer (`pkg/primitives/ltap_olap.go`) for cross-app metrics.
- **FR-2**: Implement Caddy dynamic reverse proxy driver (`pkg/iac/caddy.go`) for automated subdomain routing and HTTPS TLS.
- **FR-3**: Extend Web P2P relay (`pkg/primitives/p2p.go`) with Yjs CRDT binary sync protocol.
- **FR-4**: Implement Riba-free fee split engine (`pkg/ethics/revenue_split.go`) ensuring zero late fee interest accumulation.
- **FR-5**: Implement hash-chained audit logging (`pkg/audit/logger.go`) for immutable Shura decisions.

---

## 2. Acceptance Criteria
- [ ] DuckDB executes cross-artifact analytical aggregation queries in under 50ms.
- [ ] Caddy provider dynamically registers route `https://<appName>.taawun.community` on container startup.
- [ ] Yjs CRDT binary updates stream between peer clients without state corruption.
