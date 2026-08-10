# Conductor Workflow: Taawun Platform

## Development Methodology

### 1. Test-Driven Development (TDD Cycle)
Every feature, service, or driver added to Taawun follows the strict TDD cycle:
1. **Red**: Write a failing unit test in `pkg/<package>/<package>_test.go`.
2. **Green**: Write the minimal Go code required to make the test pass.
3. **Refactor**: Optimize performance, readability, and Go ergonomics while maintaining 100% green tests.

### 2. Quality & Security Gates
Before marking any phase or track complete:
- **Taqwa Audit Gate**: Zero bypasses in `pkg/ethics` (Riba, gambling, and prohibited domain models must be blocked).
- **Anti-Gharar Gate**: All cloud deployment manifests must include explicit SLA, CPU, memory, and preview URLs.
- **Go Test Suite**: `go test -v ./...` must pass with zero failures.
- **Build Verification**: `go build ./...` must compile cleanly without warnings.

---

## Track Structure & Format

Each platform feature or architectural initiative is documented as a Conductor Track in:
```
conductor/tracks/<track_id>/
  ├── metadata.json  # Authoritative track status, phases, tasks, dependencies
  ├── spec.md        # Technical requirements, user stories, acceptance criteria
  ├── plan.md        # Phase-by-phase TDD implementation checklist
  └── DECISIONS.md   # Architectural rulings, trade-offs, and design rationale
```

Track ID format: `<shortname>_YYYYMMDD` (e.g. `go_iac_control_plane_20260810`)

---

## Commit Strategy & Git Hygiene

### Commit Message Format
```
feat(<scope>): <short description>

Track: <track_id>
Phase: <phase_number>
Tasks completed: <list>
```

### Feature Branch Naming
`feature/<track_shortname>` (e.g., `feature/conductor-track-and-iac-engine`).
