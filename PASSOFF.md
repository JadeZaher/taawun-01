# Taawun sellable-MVP checkpoint passoff

Checkpoint date: 2026-08-17 (deployed 2026-08-18 UTC)

This is the private-beta product checkpoint for deployment, not a claim that
every future federation or key-management capability is finished. The hosted
MCP and central cockpit now compose reviewed primitives into signed Datastar
cards for verified domains through the real service composition root.

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
- One authenticated composition root: the cockpit now calls the real catalog,
  Conductor preview/track/publication APIs, exact-domain claims, signed preview
  files, Bazaar, Shura, financial sandbox, and relay-ticket adapter. The
  fail-closed legacy Conductor route is no longer the product path.
- Browser journey exercised locally: register, sign in, create/select a
  workspace, inspect the catalog, compose a signed preview, render its
  sandboxed frame, and create a DNS TXT claim. A verified customer hostname is
  required before the final publish button becomes available.
- P1 boundary work: one-use relay tickets bind principal, workspace, artifact,
  peer/device identifier, exact origin, and expiry in a WebSocket subprotocol;
  password changes rotate first-party and OAuth sessions; bounded strict JSON,
  public throttles, OAuth quotas, and expiry cleanup protect public endpoints.
- Shura invitation acceptance joins the invited principal to WorkspaceService
  with a bounded role mapping and safe replay behavior. Financial HTTP derives
  its actor from the authenticated principal and requires a final Shura
  decision; it never accepts an actor ID from JSON.
- CIMD grants pin a validated client-metadata snapshot for consent, token,
  refresh, validation, and revocation. Customer origins remain outside the
  global API CORS allowlist: published cards are Host-bound signed files, not
  cross-origin central API clients.
- Standalone authenticated relay binary and signed AZOA federation envelopes.
- Canonical Go 1.25 non-root/read-only container, persistent `/data`, corrected
  `/api/health` checks, and hardened SQLite WAL/foreign-key/busy-timeout settings.
- Railway production service `taawun` is live at
  `https://taawun-production.up.railway.app`; deployment
  `80feecc7-e5fe-49cc-8cfa-858a64e86cee` passed the Railway `/api/health` gate.
  Live registration, login, workspace creation, catalog inspection, signed
  preview composition/serving, domain claim, and missing-proof behavior are
  recorded in `docs/deployment-live-qa.md`.

## Verification evidence

The final integrated checkpoint sweep passed:

```text
go test ./src/pkg/... ./src/cmd/...
node --test src/web/index.test.mjs src/web/runtime/runtime.test.mjs
go build -trimpath -o <temp>/taawun.exe ./src/cmd
go build -trimpath -o <temp>/taawun-relay.exe ./src/cmd/relay
git diff --check
```

Docker/Compose execution was not available on the desktop because Docker is not
installed. Railway built the repository Dockerfile successfully and promoted
the same source checkpoint after its service-level health check passed.

## Screenshots

- `docs/screenshots/taawun-login.png`
- `docs/screenshots/builder-cockpit.png`
- `docs/screenshots/local-first-runtime.png`
- `docs/screenshots/signed-community-cards.png`
- `docs/screenshots/ethical-finance-cards.png`

The signed-card screenshots were produced by the real artifact builder with all
eleven catalog primitives. Existing screenshots are retained; the local browser
QA above exercises the materially changed signed-preview flow without adding a
synthetic replacement screenshot.

## Do not claim these are finished

These remain intentionally deferred or operator-owned:

1. Member/device E2EE key enrollment, rotation, and recovery. The runtime keeps
   an in-memory workspace key only; follow `conductor/e2ee-workspace-keys.md`
   rather than building a separate key-management product.
2. Durable relay federation inbox/outbox, multi-node trust, and configured
   short-lived STUN/TURN credentials. The current relay is one process with
   process-local one-use-ticket replay tracking, so deployments must use sticky
   routing until a shared replay store is designed deliberately.
3. A customer-controlled DNS/TLS hostname is needed to complete a real external
   verify-and-publish demonstration. The product correctly refuses to publish a
   preview whose signed origin set omits the verified hostname.
4. Qualified scholar approval, live settlement-provider credentials, and a real
   external MCP-client authorization demonstration remain operator work. Sandbox
   financial flows and compliance evidence must not be described as those
   authorities.

## Recommended continuation order

1. Complete one controlled customer-DNS claim, fresh signed preview, activation,
   and public-host smoke test before onboarding a customer.
2. Add the deliberately deferred device-key envelopes, relay/ICE
   interoperability, and durable federation only when their operator model is
   chosen.

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
