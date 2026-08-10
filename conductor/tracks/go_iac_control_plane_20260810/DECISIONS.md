# Architecture Decisions: Go IaC Control Plane

**Track ID**: `go_iac_control_plane_20260810`

---

### Decision 1: Direct Docker CLI & API Driver
- **Ruling**: Use direct Docker CLI execution (`docker run -d ...`) wrapped in Go `exec.CommandContext` with dry-run fallback.
- **Trade-off**: Requires Docker installed on host or VPS, but avoids heavy SDK version mismatches and complex socket authentication.

### Decision 2: Isolated Container Port Mapping
- **Ruling**: Dynamically assign unique host ports starting from 8081+ to prevent port collision between vibecoded apps.
