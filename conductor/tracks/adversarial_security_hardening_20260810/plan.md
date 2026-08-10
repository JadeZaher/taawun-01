# Implementation Plan: Adversarial Security & Infrastructure Hardening

**Track ID**: `adversarial_security_hardening_20260810`  
**Status**: Planned  

---

## Phase 1: Taqwa AST Semantic Auditor (`pkg/ethics`)
- [ ] Task 1.1: Write failing unit test `TestHaramCheckEngine_ObfuscatedRiba` with homoglyphs and unicode tricks.
- [ ] Task 1.2: Implement AST normalization and symbolic expression parser in `haram_check.go`.
- [ ] Task 1.3: Verify all ethics tests pass.

## Phase 2: Signed P2P Relay Tokens & Rate Limiting (`pkg/primitives`)
- [ ] Task 2.1: Write failing unit test for unsigned WebSocket connection attempts.
- [ ] Task 2.2: Implement HMAC-SHA256 token verification in `p2p.go`.
- [ ] Task 2.3: Implement 64KB frame size limit and per-IP rate limiter.

## Phase 3: Docker Go SDK & Sandbox Constraints (`pkg/iac`)
- [ ] Task 3.1: Replace `exec.Command` in `docker_provider.go` with official `github.com/docker/docker/client`.
- [ ] Task 3.2: Configure container security host config (`ReadonlyRootfs`, `CapDrop: ["ALL"]`, `User: "1000:1000"`).
- [ ] Task 3.3: Implement base image whitelist validator.

## Phase 4: WSS Secure WebSockets & Sandbox DX
- [ ] Task 4.1: Add TLS configuration options for WebSocket relay endpoints.
- [ ] Task 4.2: Draft `@taawun/sdk` client package.

## Phase 5: Verification & Security Audit
- [ ] Task 5.1: Run red-team penetration test sweep.
- [ ] Task 5.2: Execute full test suite `go test -v ./...`.
