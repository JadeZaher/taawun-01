# Implementation Plan: Shura Workspace & Granular RBAC

**Track ID**: `shura_rbac_governance_20260810`  
**Status**: Planned  

---

## Phase 1: Schema & Role Enums
- [ ] Task 1.1: Add `Architect`, `Maintainer`, `Viewer` role enums in `pkg/models`.
- [ ] Task 1.2: Implement RBAC permission check middleware in `pkg/shura`.

## Phase 2: Shura Workspace API
- [ ] Task 2.1: Implement team invitation & role update endpoints.
- [ ] Task 2.2: Add unit tests for permission enforcement.
