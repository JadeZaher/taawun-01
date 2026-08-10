# Spec: Adversarial Security & Infrastructure Hardening

**Track ID**: `adversarial_security_hardening_20260810`  
**Status**: Planned  
**Scope**: `pkg/ethics`, `pkg/primitives`, `pkg/iac`, `cmd`

---

## 1. Background & Threat Model
Following the red-team adversarial review of the Taawun platform, 5 key security and economic vulnerability vectors were identified across the ethics scanner, Web P2P relay hub, Docker container provider, browser sandbox CSPs, and bandwidth allocation.

---

## 2. Technical Requirements

### 1. Ethics Engine Hardening (`pkg/ethics`)
- **FR-1.1**: Replace string regex matching with AST symbolic parsing to detect obfuscated Riba calculations, homoglyph substitutions (`in\u200Bterest`), and unicode tricks.
- **FR-1.2**: Implement LLM semantic auditor callback for financial domain code blocks.

### 2. P2P Relay Authentication & Anti-Spoofing (`pkg/primitives`)
- **FR-2.1**: Require HMAC-SHA256 signed session tokens for `artifactId` and `peerId` during WebSocket upgrade.
- **FR-2.2**: Enforce connection rate limiting (max 10 connections per minute per IP) and frame payload cap (64KB max per frame).

### 3. Docker Container Sandboxing (`pkg/iac`)
- **FR-3.1**: Refactor `DockerProvider` to use the official Docker Go SDK (`github.com/docker/docker/client`).
- **FR-3.2**: Enforce container security profile: `--read-only` root filesystem, `--cap-drop=ALL`, non-root user execution (`1000:1000`), and mandatory base image whitelist validation.

### 4. Browser Sandbox & WSS TLS (`cmd`, `pkg/primitives`)
- **FR-4.1**: Enforce WSS (TLS) for WebSocket relays to prevent mixed-content blocking in browser sandboxes.
- **FR-4.2**: Ship `@taawun/sdk` npm wrapper package for client-side sandbox integration.

---

## 3. Acceptance Criteria
- [ ] Obfuscated Riba prompts (`іntеrеst`, unicode tricks) fail ethics audit cleanly.
- [ ] Unsigned WebSocket connection attempts to `/api/p2p/stream` are rejected with HTTP 401.
- [ ] Container deployment attempts with non-whitelisted Docker images are blocked.
- [ ] WebSocket relay drops frames exceeding 64KB.
