# Taawun MVP Viability — Adversarial Review

Reviewer role: adversarial MVP-viability gate for a sellable private beta.
Review date: 2026-08-17. Source checkpoint: `6a4d3a0` (matches `docs/deployment-live-qa.md`).
Method: static reading of `PASSOFF.md`, `README.md`, `conductor/product.md`,
`conductor/feature-parity.md`, `docs/deployment-live-qa.md`, package `AGENTS.md`
files, and the Go/JS source and tests. No network, deploy, or source edits were
performed. Only this file was written.

---

## Executive verdict: CONDITIONAL GO

Taawun is a genuinely well-built control plane. The security-sensitive machinery —
Ed25519 artifact signing, DNS-TXT domain proof, OAuth 2.1 with PKCE/CIMD/SSRF
controls, one-use identity-bound relay tickets, fail-closed Host-bound serving,
session-version invalidation, strict bounded JSON — is implemented carefully and
covered by tests (97 `Test*` functions across 23 files). The documentation is
unusually honest about what is deferred. This is not a demo with hidden mocks;
the composition root wires real services (`src/cmd/main.go:93-195`).

It is **not yet sellable as written**, for one product-truth reason and a small
set of operational defects, none of which are architectural:

1. **The flagship buyer journey has never completed once, end to end.** The live
   QA stops at DNS TXT challenge issuance and confirms missing proof fails closed
   (`docs/deployment-live-qa.md:49-51,72-78`). No customer-controlled domain has
   been verified, published, and served by exact Host. The single most important
   sellable claim — "publish your community app to your own domain" — is built and
   fails closed, but is unproven in reality. Selling before proving it once is the
   real risk.

2. **Published cards self-expire.** The only UI publish path bakes a 24-hour
   authorization expiry into the artifact, and the hard maximum is 90 days. Serving
   fails closed on expiry, so a live customer site goes dark. This is an
   operational blocker for a "sellable" site, though it is a small code change.

The verdict is CONDITIONAL GO: the platform can enter a **vendor-operated** private
beta once the P0/P1 items below are closed and the one controlled-domain
publish-and-serve smoke test is recorded. It must not be described to customers as
a self-service domain-publishing product until that smoke test exists.

---

## What was verified as genuinely working (evidence)

These are load-bearing claims I confirmed in source, not took on faith:

- **Artifact signing is real and stable-keyed.** `NewSignedBuilder` requires an
  injected Ed25519 key; the Conductor service refuses to construct unless
  `builder.ProductionReady()` is true (`src/pkg/conductor/track.go:207`), which is
  false for ephemeral signers (`src/pkg/artifacts/build.go:96-98`). Manifest
  signature covers a fixed field list and the signer key must match the
  authorization signer (`src/pkg/artifacts/signing.go:110-123`).
- **Content addressing and tamper detection.** Content hash is a semantic hash
  excluding volatile fields; re-open re-verifies every file digest and byte count
  (`src/pkg/artifacts/build.go:217-257,259-270`). Path joins are escape-checked
  (`safeJoin`, `src/pkg/artifacts/build.go:320-334`).
- **DNS proof is exact and constant-time.** Challenge is a hashed
  `workspace|origin|challenge` digest; verification compares with
  `subtle.ConstantTimeCompare` and only accepts `_taawun.<host>` TXT
  (`src/pkg/domains/service.go:139-188,426-428`; `origin.go:61-63`). Not-found and
  permanent DNS errors fail closed (`service.go:157-174`).
- **Public serving fails closed on every axis.** Host is resolved to an active
  publication joined to a still-verified claim; the bundle is re-opened and
  re-validated for workspace, expiry, approved domain, and approved origin before
  any byte is served (`src/pkg/domains/public.go:55-101`;
  `src/pkg/domains/delivery.go:148-177`). Path traversal is rejected
  (`public.go:70-78`; `src/pkg/artifacts/read.go:105-111`).
- **OAuth 2.1 discipline.** PKCE S256 required, exact `resource` binding, redirect
  URIs exact-match, public-clients-only, refresh rotation with family revocation on
  replay, and session-version checks at issue/validate/refresh
  (`src/pkg/oauth/service.go:192-234,413-460,462-501,503-531`;
  `repository.go:825-831`). Tokens are stored hashed (`credentialHash`).
- **CIMD SSRF controls are strong.** Metadata fetch pins the resolved host, rejects
  private/loopback/link-local/CGNAT addresses, forbids redirects, bounds size and
  time, and requires JSON (`src/pkg/oauth/cimd.go:87-180`).
- **Relay tickets are identity/artifact/origin-bound and one-use.** Every claim is
  derived from the authenticated principal and a re-opened manifest, never from
  client JSON; tokens are HMAC-signed, ≤5 min, and reserved once
  (`src/pkg/handlers/relay_ticket_handler.go:65-110`;
  `src/pkg/primitives/p2p.go:147-186,228-260`).
- **Financial actor cannot be chosen by the client.** The HTTP layer injects the
  authenticated principal and requires a final Shura decision; the sandbox provider
  never reports settlement (`src/pkg/handlers/financial_handler.go:76-99,166-189`;
  `src/pkg/financial/azoa.go:199-211`).
- **Password change invalidates every session.** `session_version` increments on
  password update (`src/pkg/repositories/user_repository.go:121,141`) and is checked
  by both first-party JWT validation (`src/pkg/services/auth_service.go:107`) and
  OAuth (`src/pkg/oauth/service.go:284,434,481,524`).
- **First-party token is held in memory only.** The cockpit keeps the bearer in a
  JS `state.token` variable, not `localStorage` (`src/web/index.html:573`); no
  `localStorage`/`sessionStorage` token persistence exists. XSS surface for token
  theft is reduced, at the cost of logout-on-refresh (a UX note, below).

---

## P0 — Blockers (must fix before any customer touch)

### P0-1. The verified-domain publish-and-serve journey is unproven end to end
The product's headline is domain-bound signed cards, but no run has ever verified a
real DNS TXT record, activated a publication, and served the card by exact Host.
Live QA explicitly stops at challenge issuance and documents this as the acceptance
boundary (`docs/deployment-live-qa.md:49-51,72-78`; `PASSOFF.md:118-125`).
Everything downstream of verification (`Verify` → `Publish`/`Activate` →
`activePublicationForHost`) is therefore exercised only by unit tests, not against
a live hostname with real TLS routing.

Why it is a blocker: you cannot sell "publish to your domain" having never done it
once. Any first customer becomes the first integration test of TLS termination,
custom-host routing, and CNAME/proxy behavior that Railway does not yet perform for
non-Railway hosts.

Fix: complete one controlled-domain claim → TXT verify → preview refresh with the
verified origin → publication activate → `GET https://<host>/` smoke test, and
record it. This is already the #1 continuation item in `PASSOFF.md:131-137`.

### P0-2. Published customer sites expire (24h via the UI, 90d hard cap)
The cockpit's build payload never sets `ttlHours` (`src/web/index.html:880-897`),
so the handler defaults to 24 hours (`src/pkg/handlers/composition_handler.go:132-134`).
The Conductor stamps `ExpiresAt = now + TTLHours`
(`src/pkg/conductor/track.go:287`) and the same signed artifact is what gets
published. Serving calls `CheckManifestExpiry` on every request through
`validateArtifactForClaim` (`src/pkg/domains/delivery.go:148-177`;
`src/pkg/domains/public.go:65`), so **a published card returns HTTP 503 the moment
its authorization expires** — 24 hours after build on the default path. Even the
maximum permitted TTL is 90 days (`src/pkg/conductor/track.go:496`), so no card can
be published to live longer than 90 days without a re-publish.

Why it is a blocker: a "sellable" hosted site that goes dark in a day (or at best a
quarter) with no renewal automation is not sellable. This is the gap between "the
publish flow works in a test" and "a customer's site stays up."

Fix: (a) let the publish path use a production-appropriate lifetime and/or add an
automatic re-sign-and-re-activate on approaching expiry; (b) at minimum, surface
the expiry in the cockpit and document the renewal runbook. Treat the 90-day cap as
a deliberate product decision to revisit, not an accident.

---

## P1 — High severity (fix before onboarding real users)

### P1-1. Public auth/relay throttles key on `RemoteAddr` behind a proxy
Both throttlers derive the client key from `r.RemoteAddr`
(`src/pkg/handlers/auth_handler.go:231-240`;
`src/pkg/oauth/rate_limit.go:62-71`), and relay connection limiting does the same
(`src/pkg/primitives/p2p.go` `allowConnection`). On Railway the app sits behind an
edge proxy, so `RemoteAddr` is typically the proxy, not the end client. Two
consequences: (a) **one attacker can lock out all users** by exhausting the shared
login bucket (10 attempts / 15 min) attributed to the proxy IP; (b) per-client
throttling is effectively disabled. The limiter's own storage cap
(`maximumPublicAuthClients = 4096`) does not help because the key space collapses to
one source.

Fix: parse a trusted `X-Forwarded-For`/`X-Real-IP` from the known Railway edge (only
when the immediate peer is the trusted proxy) and key throttles on the real client.
Do not trust the header unconditionally.

### P1-2. `/api/ethics/audit` is public, unauthenticated, unbounded, unthrottled
Registered before the authenticated `/api` subrouter, it decodes the request body
with a bare `json.NewDecoder(r.Body)` — **no `MaxBytesReader`, no auth, no rate
limit** — then runs a compute-bound RAG audit (`src/cmd/main.go:262-277`). This is
the one endpoint that ignores the strict-bounding discipline applied everywhere else
(compare `src/pkg/handlers/auth_handler.go:157-185`). It is a trivial anonymous
CPU/bandwidth DoS surface and an odd information-exposure surface for a private beta.

Fix: move it behind `AuthMiddleware`, or at minimum bound the body with
`http.MaxBytesReader`, require `application/json`, and apply the public throttle.

### P1-3. Single-instance relay replay store forces sticky routing (scaling ceiling)
One-use ticket tracking is an in-process map (`usedSessions`,
`src/pkg/primitives/p2p.go:96,141`). This is correct for one instance but means
horizontal scaling silently breaks one-use semantics (a ticket replayed against a
second instance is accepted). It is documented as deferred
(`PASSOFF.md:120-122`; `conductor/feature-parity.md:26`), but it is a **hard cap of
one relay instance** for the beta and must be an explicit operational constraint,
not a footnote — a second instance is a security regression, not just a scaling
limit.

Fix (operational, not code): pin the deployment to a single relay instance / sticky
routing and add a deploy guardrail so nobody scales it out before a shared replay
store exists.

---

## P2 — Medium severity (track and disclose)

### P2-1. Platform admins bypass workspace membership everywhere
`WorkspaceService.authorize` returns any workspace for a global `RoleAdmin` without a
membership check (`src/pkg/services/workspace_service.go:282-284`), and
`GetWorkspaces` returns all workspaces for admins (`:87-93`). Because domain
publication and financial orchestration authorize through
`AuthorizeWorkspaceCapability`, a platform admin can claim/publish domains and drive
financial quests in any tenant. For a vendor-operated beta this is acceptable
operator power, but it means **tenant isolation depends entirely on the platform-admin
account's secrecy**. Disclose it, restrict the bootstrap admin, and keep it out of
any future self-service multi-tenant posture.

### P2-2. Published manifests carry `Lifecycle: preview`
The publish path serves the preview-lifecycle artifact; `validateArtifactForClaim`
checks domain/origin/expiry but not `Lifecycle == Published`
(`src/pkg/domains/delivery.go:148-177`), and the artifact built during the track is
always `BundleLifecyclePreview` (`src/pkg/conductor/track.go:287`). Serving is still
safe, but a customer inspecting a live card's manifest sees `preview`, which
undercuts the "immutable published bundle" story. Cosmetic/labeling, but visible.

### P2-3. Cockpit UX: in-memory token means refresh logs the user out
Storing the bearer only in `state.token` (`src/web/index.html:573`) is good for XSS
resistance but means a page refresh drops the session mid-journey (after composing a
preview, before publishing). For a private beta this is tolerable; note it so it is a
choice, not a surprise. Do not "fix" it by moving the token to `localStorage`.

### P2-4. Relay ticket capability floor is `View`
Ticket issuance requires only `WorkspaceCapabilityView`
(`src/pkg/handlers/relay_ticket_handler.go:76`), so a Viewer can obtain relay
transit for an artifact whose exact origin they present. Transit is opaque and E2EE
is deferred, so this is low-risk, but confirm it matches the intended role model
before governance UI ships.

---

## Attack and failure scenarios considered

- **Forged/expired card served publicly.** Blocked: serving re-opens the bundle and
  re-checks signature-derived fields, workspace, expiry, approved host, and approved
  origin (`public.go:55-101`; `delivery.go:148-177`). *Failure mode instead:* the
  card expires and 503s (P0-2).
- **Cross-workspace domain hijack.** Blocked: an origin already
  pending/verified elsewhere returns `ErrOriginClaimed`
  (`service.go:103-109`) and a partial unique index enforces it
  (`store.go:39-40`).
- **DNS spoof / partial match.** Blocked: constant-time digest compare over a
  workspace-salted challenge on an exact record name (`service.go:156-174`).
- **SSRF via CIMD client URL.** Blocked: host-pinned dialer rejecting non-public IPs
  and redirects (`cimd.go:146-180`).
- **OAuth code interception / refresh replay.** Blocked: PKCE S256, exact redirect
  and resource, refresh family revocation on reuse
  (`service.go:413-460,462-501`).
- **Relay ticket theft / replay / cross-origin use.** Bounded: HMAC-signed, ≤5 min,
  one-use, origin/artifact/peer-matched (`p2p.go:147-186,228-260`) — but one-use
  holds only on a single instance (P1-3).
- **Client-chosen financial actor.** Blocked: actor injected from auth, JSON actor
  fields ignored (`financial_handler.go:76-99`).
- **Anonymous DoS.** Partially open: `/api/ethics/audit` is unauthenticated and
  unbounded (P1-2); auth throttles collapse behind the proxy (P1-1).
- **Availability failure:** published sites dark after 24h/90d (P0-2); single relay
  instance is a correctness dependency (P1-3).

---

## Product and operational viability

**Product coverage is honest.** `conductor/feature-parity.md` marks money as durable
sandbox and compliance as reference-only, and the generated card contains no ledger
and only host-supplied server signals (`src/pkg/artifacts/render.go:211-246`). The
"zero data custody" thesis is actually reflected in the artifact (static bundle,
`connect-src 'self'` + explicitly allowed origins, `frame-ancestors 'none'` default;
`render.go:271-309`). Nothing material is silently faked.

**Operational readiness is close but not proven.** The container is non-root,
read-only except `/data`, drops privileges after fixing volume ownership
(`Dockerfile:13-32`; `docker-entrypoint.sh`), and SQLite uses WAL + busy timeout +
foreign keys + immediate txlock (`src/pkg/database/database.go:64-78`). Health gating
is in place. Missing for a real beta: a **backup/restore drill** for the three
SQLite stores and artifact tree on the single `/data` volume, and a **custom-domain
routing runbook** (both already flagged in `conductor/feature-parity.md:20,41`). A
single volume with no documented restore is a data-loss blocker the moment a paying
tenant exists.

---

## Private-beta acceptance gates (must all be green to sell)

1. One controlled customer-style domain fully verified, published, and served by
   exact Host over HTTPS, recorded with identifiers (closes P0-1).
2. Published-card lifetime is production-appropriate with a documented or automated
   renewal path; expiry is visible in the cockpit (closes P0-2).
3. Throttles key on the real client IP behind the Railway proxy (closes P1-1).
4. `/api/ethics/audit` is authenticated or bounded+throttled (closes P1-2).
5. Deploy guardrail documents/enforces single-instance relay (closes P1-3).
6. Backup and restore of `/data` (control DB, AZOA DB, artifacts) drilled once.
7. Bootstrap-admin account is created deliberately, with the platform-admin
   cross-tenant power (P2-1) written into the operator model.

---

## Seven-day owner checklist

- Day 1: Set a production artifact TTL for publishes and surface expiry in the
  cockpit; decide the 90-day-cap policy (P0-2).
- Day 1–2: Trust the Railway edge and key auth/OAuth/relay throttles on the real
  client IP (P1-1); bound + gate `/api/ethics/audit` (P1-2).
- Day 2–3: Run the full controlled-domain publish-and-serve smoke test against a
  hostname you control; capture claim/track/artifact/publication IDs and the served
  response (P0-1); append to `docs/deployment-live-qa.md`.
- Day 3: Add a single-instance guardrail/annotation for the relay (P1-3).
- Day 4: Perform and document a `/data` backup + restore drill.
- Day 5: Deliberately provision the bootstrap admin; write the platform-admin power
  and tenant-isolation caveat into the operator runbook (P2-1).
- Day 6: Fix the published-manifest `Lifecycle` label (P2-2); note the
  refresh-logs-out UX behavior for beta users (P2-3).
- Day 7: Re-run `go test ./src/pkg/... ./src/cmd/...` and the browser tests; confirm
  all gates green; tag the beta build.

---

## Acceptable deferrals vs blockers

**Acceptable to defer for a vendor-operated private beta** (documented and honest in
`PASSOFF.md:112-137` and `README.md:83-92`):
- Member/device E2EE workspace-key enrollment, rotation, recovery (in-memory key
  only today).
- Durable relay federation inbox/outbox, multi-node trust, shared replay store, and
  operator-provisioned short-lived STUN/TURN.
- Live AZOA settlement credentials; sandbox orchestration must not be described as
  settlement.
- Qualified scholar approval; compliance corpus is reference-only.
- External third-party MCP-client authorization demonstration.
- Federation TURN E2EE live settlement as an integrated whole — correctly out of
  scope for this checkpoint.

**Not acceptable to defer (blockers to selling):**
- Completing the verified-domain publish-and-serve journey at least once (P0-1).
- Published cards that expire out from under a customer with no renewal (P0-2).
- Proxy-collapsed throttles (P1-1) and the open compute endpoint (P1-2) in an
  internet-facing beta.
- An undrilled single-volume persistence story once real tenant data exists.

---

## Bottom line

The engineering is strong and the scope claims are honest, which is exactly why the
remaining gaps matter: they are the difference between "the mechanism is correct" and
"a customer's site is live and stays live." Close P0-1 and P0-2, fix the two P1
exposure/availability defects, drill backup/restore, and this is a defensible
vendor-operated private beta. Until the first real domain has been published and
served, do not market it as a self-service domain-publishing product.

---

## Remediation and viability backlog addendum — 2026-08-18

This is a cumulative implementation/live-QA addendum, not a second adversarial
review. The original checkpoint, method, and conditional verdict above remain
historical evidence.

Current remediation status:

- **Live-green on deployment `68914f4c-a916-43f7-b0bf-fbd578c25f43`:**
  iframe/runtime rendering, invalid-composition classification, Railway
  registration attribution, pending-claim publication state, private
  authenticated ethics-audit boundaries, and relay identity/origin binding.
- **Still under live QA:** account deletion lifecycle, remaining role contracts,
  responsive/accessibility details, and the synthetic Bazaar sandbox journey.
- **Still green and protected:** identity/workspace isolation, signed preview and
  event integrity, invitation membership, Shura decisions, financial actor
  injection defense, sandbox quest lifecycle, exact-origin claims, customer CORS,
  relay identity/origin binding, and pinned CIMD snapshots.
- **Fixture-dependent:** controlled DNS verification, public custom-host serving,
  and Bazaar purchase need an operator-provided live domain and suitable published
  listing/decision fixtures. These boundaries must not be weakened to manufacture
  a green result.

An all-route Railway edge `502` window occurred for roughly 30 seconds after
promotion and self-recovered without a new deployment. Runtime/resource evidence
showed zero application-observed restarts and no matching application `5xx`; an
independent 8/8 recovery gate and a 120/120 five-minute health/root soak passed.
It is tracked as a transient edge/routing incident unless new platform evidence
identifies an application cause.

Commit `da5e136` is the next implementation checkpoint. It replaces fabricated
dashboard evidence with deterministic accessible-workspace data and honest empty
states, preserves accurate value/custody copy through effective 200% reflow,
measures signed-card text contrast, and exposes only server-verified manifest
trust facts. Deployment `fe372625-e35f-4490-a8de-d8647826e7d3` reached Railway
`SUCCESS`; container and public health/cockpit gates passed. These remediations
are awaiting the paired QA task's independent live retest and are not recorded as
accepted solely from source tests.

Ranked next increments after baseline QA signs off:

1. **First-session differentiated journey:** surface the existing broader catalog,
   invitations/Shura, sandbox finance, and Bazaar/test-drive APIs through the
   smallest cohesive Swiss cockpit UI, with honest sandbox/review labels.
2. **Recovery and domain guidance:** resumable progress, actionable publication
   state, visible signed-manifest trust cues, expiry/renewal guidance, and clear
   empty/error states.
3. **HTTP trust polish:** consistent first-party error envelopes, no-store/nosniff,
   Railway request-ID-correlated safe 5xx logging, real OpenAPI, durable recent
   activity, and favicon/metadata polish.
4. **Operator proof:** controlled-domain publish/serve smoke test followed by a
   backup/restore drill and explicit single-instance relay guardrail.
5. **Measured private-beta checkpoint:** use only lightweight, boundary-safe funnel
   diagnostics already available from request/track events; do not add surveillance
   or a separate analytics platform.

The sole receipt-attestation P1 identified during the first trust retest is
addressed in commit `57d9fd1`. The response now binds the exact server-serialized
manifest to a digest after trusted artifact verification, while the cockpit
recomputes and compares the complete manifest and rejects expired or internally
inconsistent authorization. The promoted real-Chromium matrix covers altered
identity/signature fields, lifecycle, expiry, every allowed-origin category,
missing evidence, and elapsed expiry. Railway deployment
`1bfa3008-319c-49ba-bd52-090da3dbfa5e` reached terminal `SUCCESS`; public health
and cockpit probes both returned `200`. This is source/deployment evidence
awaiting independent QA acceptance, not a second review or a broadened trust
claim.

QA accepted the exact-manifest trust gate on application commit `57d9fd1` and
identified one separate boundary defect: correctly rejected unverified preview
origins were surfaced as a generic `500`. Commit `e269126` preflights authority
before durable idempotency state, preserves a second race check, maps expected
denial to nested `422 invalid_composition`, and adds bounded request-correlated
outcome telemetry without logging customer or credential data. All Go packages,
12/12 Node tests with real Chromium, focused operational-classification tests,
and independent review passed. Railway deployment
`58d87a23-6037-4cc5-8151-09b37167cef5` reached terminal `SUCCESS`; public health
and cockpit probes returned `200`. Live QA acceptance is pending, so this is not
a second adversarial review or final MVP sign-off.

The synthetic cleanup ledger is preserved with its evidence levels: user
`19`/workspace `14` is observed remaining; user `20`/workspace `15` and user
`21`/workspace `16` are sequence inferences whose exception handlers may already
have deleted them; user `22`/workspace `17` is observed clean. Supported admin
cleanup routes exist, but no authenticated admin principal or configured
bootstrap-admin environment was available. No raw database cleanup or authority
bypass was attempted.

The next ranked increment is implemented in application commit `09cd897`, without
re-running or reclassifying this review. The Swiss cockpit now exposes the real
catalog, workspace-scoped People/invitations, Shura, durable financial sandbox,
and published-only Bazaar through one role-aware Workspace surface. It explicitly
labels session-only records where no list primitive exists, offers ID-based
recovery, preserves honest error/empty states, and keeps the controlled-DNS and
authorized-reviewer Bazaar fixture dependency visible. Delayed workspace and
prior-principal responses fail closed before UI state changes.

Every Go package/command and 13/13 Node tests passed, including required real
Chromium organizer, invited Viewer, and Maintainer paths plus the previously
accepted signed-receipt matrix. Independent source/test review approved the final
diff. Railway deployment `c2374acf-7102-4471-91fd-e568f7bdb5ce` reached terminal
`SUCCESS`; public health and cockpit checks returned `200` and confirmed the new
surfaces are served. This is a deployment handoff awaiting independent persona
acceptance, not a second adversarial review or a claim that external DNS/Bazaar
fixtures are now available.

The next bounded checkpoint is implemented in application commit `5c5018c`,
again without re-running or reclassifying the original adversarial review. It
turns the real curated catalog into a component-document builder while retaining
one instance per allowed module and the existing signed-artifact primitives.
Exact canonical JSON now flows through HTTP/MCP, durable tracks, build requests,
safe rendering, per-component files, the v2 manifest/content hash/signature, and
verified reload. Legacy v1 artifacts retain a narrow component-free read path.
The editor fails closed on incomplete catalog contracts, duplicate or
non-canonical JSON, unpaired Unicode surrogates, reserved authority/network/code
keys, and bounded depth/size violations; delayed responses cannot replace a
newer principal, workspace, draft, iframe, or trusted receipt.

The Shura prerequisite is also closed in source: an exact final approved
decision ID is displayed and carried into Finance only for the same selected
workspace/proposal/version, while the finance server continues to revalidate
authority. Viewer state is read-only and Maintainer/Architect controls retain
their existing capability boundaries.

All Go packages/commands, 13/13 Node tests with promoted real Chromium, both
production builds, diff hygiene, backend security review, and UI release review
passed. Deployment `12cbe8e9-67fa-45dc-97e8-057ea4781de9` reached Railway
terminal `SUCCESS`; 8/8 health and 8/8 root probes were `200`. A live bounded
organizer-to-Viewer smoke passed real 3-template/11-module catalog, exact Unicode
preview, signed-file authorization, invitation isolation/acceptance, Viewer
read, Viewer build denial, and supported cleanup. Workspace `25` and users
`32`/`33` cleaned with `204`.

One registration-only harness principal has no workspace or artifact and is
inferred by allocation sequence as user `30`; no read-back or deletion is
claimed. It joins the supported-admin cleanup backlog. Independent live-browser
acceptance remains pending because the Codex Browser plugin currently fails
before browser selection on an external trusted-path mismatch; API-only evidence
does not replace that required gate.

Independent QA later accepted the application/API technical gate on deployment
`12cbe8e9-67fa-45dc-97e8-057ea4781de9`. The live matrix independently covered
exact v2 document/digest/file/reload binding, adversarial Unicode and JSON
denials with idempotency recovery, role/workspace isolation, Maintainer build,
Shura `DECIDED` version 3, approved-decision finance creation plus missing/wrong
rejection, signed-file boundaries, and supported cleanup. No Taawun P0/P1 fix
prompt was issued. The only remaining checkpoint blocker is the external Codex
Browser bundle trusted-path failure before browser selection, so no independent
live UI/mobile/keyboard/race credit is claimed yet and the candidate release is
held unchanged.
