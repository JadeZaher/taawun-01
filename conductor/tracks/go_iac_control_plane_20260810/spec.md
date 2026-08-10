# Spec: Infrastructure-as-Code Control Plane & Docker Provider Engine

**Track ID**: `go_iac_control_plane_20260810`  
**Status**: Completed  
**Authoritative Location**: `pkg/iac/docker_provider.go`

---

## 1. Background & Rationale
Vibecoded community applications (such as standup trackers, Halal business directories, or mosque donation counters) require automated, instant container provisioning without requiring manual cloud infrastructure setup.

---

## 2. Requirements

### Functional Requirements
- **FR-1**: Expose a unified `AppManifest` struct containing `ArtifactID`, `Name`, `Image`, `Port`, `HostPort`, `MemoryMB`, `CPULimit`, and `Env`.
- **FR-2**: Provide a `DockerProvider` implementing `Deploy(ctx, manifest)` and `Teardown(ctx, artifactID)`.
- **FR-3**: Support dry-run fallback if local Docker daemon is offline during testing.

### Non-Functional Requirements
- **NFR-1**: Deployment execution time under 1.5 seconds.
- **NFR-2**: Memory limit enforcement via Docker `--memory` flag.

---

## 3. Acceptance Criteria
- [x] `AppManifest` correctly compiles host-to-container port mappings.
- [x] `DockerProvider.Deploy` returns a valid `ContainerStatus` object with staging URL.
- [x] `DockerProvider.Teardown` cleanly removes containers.
