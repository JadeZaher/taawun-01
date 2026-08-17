# Taawun sellable-MVP checkpoint passoff

Checkpoint date: 2026-08-17

This is a substantial, compiling foundation wave, not a finished or deployed
product. The active product goal remains: ship a lean, sellable Taawun MVP whose
hosted MCP lets Taawun's or a customer's LLM compose reviewed primitives into
signed Datastar cards for verified domains.

## Product boundary now implemented

Taawun hosts one OAuth-protected remote MCP control plane. The MCP discovers and
inspects curated templates/primitives, audits a composition, and creates an
immutable Ed25519-signed card bundle bound to the issuing user, workspace,
lifecycle, expiry, and exact origins. MCP Apps/AppBridge and per-user MCP servers
are deliberately out of scope.

Generated output supports a card, isolated iframe/embed, or monolithic surface
from the same signed manifest. Convergent community records live in the browser
runtime; identity, governance decisions, marketplace publication, and financial
orchestration remain server-owned. No generated app contains a ledger.

## Implemented in this checkpoint

- Official MCP Go SDK Streamable HTTP server with typed discovery, inspection,
  compliance, composition, manifest, and card tools.
- OAuth remote MCP boundary: RFC 9728/RFC 8414 discovery, CIMD with SSRF
  controls, DCR fallback, PKCE S256, RFC 8707 resource binding, scoped workspace
  consent, hashed opaque tokens, refresh rotation/replay revocation, and RFC 7009
  revocation.
- Immutable Ed25519 artifact builder with three templates, eleven primitives,
  Swiss geometric default design, per-file digests, CSP, and card/embed/monolith
  output.
- Exact verified-domain claims via DNS TXT proof, hashed challenges, revocation,
  explicit publication activation/rollback history, and Host-bound public serving
  with signature/expiry/workspace/origin/file verification.
- IndexedDB append-only local-first runtime with deterministic LWW replay,
  tombstones, BroadcastChannel, WebRTC, AES-GCM encrypted relay fallback, and
  role/policy hooks. Financial records are prohibited from browser collections.
- Durable sandbox financial orchestration for donation, marketplace escrow,
  revenue split, Zakat, Qard Hasan, stipend, and multi-party approval with integer
  minor units, idempotency, explicit lifecycle, optimistic versions,
  reconciliation, and hash-linked audit. It does not claim live settlement.
- Ed25519 Shura capabilities plus durable invitations, revocation, proposals,
  deliberation, votes, quorum, required approvers, manual decisions, JWKS, and
  hash-linked audit.
- Durable Bazaar core with immutable revisions, anti-gharar disclosures,
  compliance/Shura references, published-only discovery, idempotent sandbox
  escrow purchase, reconciled entitlement, and revision-pinned installs.
- Durable Conductor core replacing Docker/LTAP execution with declarative
  validation, compliance evidence, signed preview, retry/resume, and separate
  publication request/activation stages.
- Standalone authenticated relay binary and signed AZOA federation envelopes.
- Canonical Go 1.25 non-root/read-only container, persistent `/data`, corrected
  `/api/health` checks, and hardened SQLite WAL/foreign-key/busy-timeout settings.

## Verification evidence

The final integrated checkpoint sweep passed:

```text
go test ./src/pkg/... ./src/cmd/...
node --test src/web/runtime/runtime.test.mjs   # 6 passed, 0 failed
go build -trimpath -o <temp>/taawun.exe ./src/cmd
go build -trimpath -o <temp>/taawun-relay.exe ./src/cmd/relay
git diff --check
```

Docker/Compose execution was not available because Docker is not installed in
this desktop environment. The Compose health path was corrected by inspection.

## Screenshots

- `docs/screenshots/taawun-login.png`
- `docs/screenshots/builder-cockpit.png`
- `docs/screenshots/local-first-runtime.png`
- `docs/screenshots/signed-community-cards.png`
- `docs/screenshots/ethical-finance-cards.png`

The signed-card screenshots were produced by the real artifact builder with all
eleven catalog primitives. The temporary screenshot helper and data are beneath
ignored `src/data/` and are not part of the PR.

## Do not claim these are finished

These are the next-session P1/product-wiring items:

1. `main.go` does not yet construct or mount the new Conductor, Bazaar, Shura, or
   financial services. The legacy Conductor route intentionally fails closed.
   The central builder's `/api/templates` and artifact-preview routes are also not
   wired, so its structured cockpit is present but not yet an end-to-end build UI.
2. Relay WebSocket tokens still use the URL query and are not yet bound to the
   current user/device/workspace membership. Move credentials to an allowed
   WebSocket subprotocol or one-time ticket and bind the signed session claims.
3. Password updates need the same policy as registration plus session-version
   revocation. Public login/register/OAuth registration need bounded bodies,
   throttling, quotas, and cleanup.
4. Shura invitation acceptance records the Shura invitation but does not yet add
   the user through WorkspaceService, so accepted invitees cannot receive usable
   capabilities until the membership flow is joined.
5. Financial service methods take actor IDs as domain inputs. Only expose them
   through an authenticated adapter that injects the principal and verifies a
   Shura decision/approval reference.
6. Customer card origins are enforced for signing/CSP/public Host delivery, but
   the central API still uses one process-wide CORS list. Add per-artifact/domain
   Datastar application adapters without globally granting customer origins API
   access.
7. CIMD metadata should be pinned to an issued grant/token family so client-host
   outages or `Cache-Control: no-store` cannot turn token validation into a live
   outbound dependency.
8. The runtime accepts an in-memory workspace key, but member/device E2EE key
   enrollment and rotation are intentionally deferred. Follow
   `conductor/e2ee-workspace-keys.md`; do not build a larger key-management product.
9. Durable relay federation inbox/outbox, configured STUN/TURN credentials, the
   fuller declarative composition schema, central UI adapters, documentation,
   and Railway deployment remain next-session work.

## Recommended continuation order

1. Build one composition root in `main.go` and mount the existing services and
   router helpers. Make register → workspace → catalog → signed preview work in
   the central cockpit before adding new primitives.
2. Close review findings 2–7 above and add integration tests at the authenticated
   adapter boundaries.
3. Connect generated card events to local-first/Shura/financial/Bazaar/compliance
   adapters; keep server-owned state out of IndexedDB.
4. Update the cockpit to render the full dynamic catalog, verified domains,
   review evidence, publish/rollback, Bazaar test drive, and MCP connection guide.
5. Add the minimal device-key envelope lifecycle, relay/ICE interoperability, and
   durable federation inbox/outbox.
6. Rewrite the stale root/source READMEs and `.env.example`, run browser/security
   verification, then deploy to the existing Railway Hadith Ontology project.

## Environment and repository notes

Production startup currently requires at least:

- `TAWUN_JWT_SECRET`
- `TAWUN_PUBLIC_BASE_URL`
- `TAWUN_ALLOWED_ORIGINS`
- `TAWUN_MCP_ALLOWED_HOSTS`
- `TAWUN_ARTIFACT_SIGNING_KEY_ID`
- `TAWUN_ARTIFACT_SIGNING_PRIVATE_KEY`
- `RELAY_SHARED_SECRET`
- `RELAY_ALLOWED_ORIGINS`
- `APP_DB_PATH`
- `TAWUN_ARTIFACT_ROOT`

The tracked `deliverable/Taawun.exe` was already locally modified and is
intentionally excluded from this PR. Do not overwrite or discard it without the
user's explicit direction. No live AZOA credentials, qualified scholar approval,
customer TLS/domain routing, or production LLM provider credentials are present.
