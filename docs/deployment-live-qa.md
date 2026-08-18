# Railway private-beta deployment and live QA

QA date: 2026-08-17 MDT / 2026-08-18 UTC

## Deployment

| Item | Value |
|:---|:---|
| Railway project | `hadith-ontology` (`43973172-fd19-4c41-8097-369cf805cbe6`) |
| Environment | `production` |
| Service | `taawun` (`3a254bda-f83c-4b47-aaa6-03a045546e88`) |
| Public route | <https://taawun-production.up.railway.app> |
| Verified deployment | `80feecc7-e5fe-49cc-8cfa-858a64e86cee` (`SUCCESS`) |
| Source checkpoint | `6a4d3a0` |
| Image digest | `sha256:f3f6977b3e61ad08b0c1369db824d295483d337f34dc5cdb1aa1c25e90c31466` |
| Persistent volume | `taawun-volume` (`ba6e345a-6a57-4c43-841c-804136160526`) at `/data` |
| Railway health gate | `GET /api/health`, 120-second timeout |

The service has exact production values for the public base URL, application
origin, MCP host, and relay origin. JWT, relay, artifact-signing, and
Shura-signing secrets were generated for this deployment and stored as sealed
Railway variables. Their values are not recorded here. SQLite control and AZOA
sandbox databases and immutable artifacts use the mounted `/data` volume.

The first image start exposed a Railway-volume ownership mismatch: the image
was marked deployed while SQLite could not create `/data/taawun.db`. The image
entrypoint now initializes the mounted directories as root and immediately
drops to the unprivileged `taawun` user. Railway's service-level health gate was
also added so the same restart-loop cannot be reported as a successful future
deployment.

## Live QA evidence

All requests below targeted the public Railway route. Synthetic credentials and
tokens were generated in memory and were not written to the repository or this
document.

| Journey step | Result |
|:---|:---|
| Health | `GET /api/health` returned `200` with `status: ok`, active Conductor, MCP, and P2P |
| Cockpit | `GET /` returned `200` and the Taawun builder document |
| Registration | `POST /api/register` returned `201` |
| Authentication | `POST /api/login` returned `200` and a usable bearer token |
| Workspace | Authenticated `POST /api/workspaces` returned `201` |
| Catalog | Authenticated `GET /api/templates` returned `200` and three curated templates |
| Signed preview | Authenticated `POST /api/artifacts/preview` returned `201` |
| Manifest trust | 64-character content hash; non-empty Ed25519 signature from `railway-artifact-v1` |
| Preview serving | Authenticated signed preview document returned `200` and contained the configured app name |
| Domain claim | Authenticated exact-origin claim returned `201` in `pending` state with a TXT challenge |
| Missing DNS proof | Verification without the TXT record returned actionable `422 dns_proof_missing` |

The live signed-preview run created Conductor track
`track_JfWC4G-oafIDRYZBRS1TvQC-` and artifact
`art_19542a23dafe6a823e029d54c0e7cef4`. These identifiers are evidence only;
the synthetic QA workspace is not a customer tenant.

## Verification sweep

The final source checkpoint passed:

```text
go test -count=1 ./src/pkg/... ./src/cmd/...
node --test src/web/index.test.mjs src/web/runtime/runtime.test.mjs
git diff --check
```

The Go sweep passed every package; the browser tests passed 8/8. Railway then
built the same checkpoint through the repository Dockerfile and promoted it
only after `/api/health` passed.

## 2026-08-18 private-beta remediation batch

The paired live-QA task reproduced staging iframe corruption on successful
Railway deployment `04f32db0-72e9-4364-82c6-bee0796ab161`. Track
`track_FmQqBlt2gkWKjJ2Q3ikQ8T3n` and artifact
`art_f0291aef6bf1fb2d7bc745dfebb5ed05` were served successfully, but the
cockpit's `srcdoc` assembly treated Datastar's literal `$&` as a JavaScript
replacement token and exposed runtime source before the signed card.

The first remediation batch now:

- inserts preview CSS and the pinned Datastar runtime through replacement
  callbacks, with a real-Chromium regression covering intact runtime parsing,
  first-visible card content, and an interactive card control;
- classifies invalid curated compositions as `422 invalid_composition`;
- requires a currently verified domain claim before publication request or
  activation and returns `409 publication_claim_unavailable` after revocation;
- attributes Railway registration attempts through the documented
  `X-Real-IP` header only from an environment-marked trusted proxy peer,
  including Railway's internal `100.0.0.0/8` range;
- anonymizes account identity, revokes first-party and OAuth authority, removes
  non-audit grants, and preserves referenced governance/domain records; and
- moves the ethics audit behind private-beta authentication with strict bounded
  JSON, per-principal throttling, hardened response headers, and safe errors.

Integrated source verification passed 11/11 Node tests (including headless
Chrome), all Go package/command tests, both production binary builds, and
`git diff --check`.

The functional fixes were committed as `a484bfe`. The first upload,
`db2bf81c-c68d-4ecf-a949-f3fa240c69af`, failed before application startup
because the Windows deployment archive materialized the container entrypoint
with a CRLF shebang. Commit `9748666` pins `docker-entrypoint.sh` to LF and
makes the Docker build reject a wrong shebang or any CR byte. Deployment
`68914f4c-a916-43f7-b0bf-fbd578c25f43` then reached Railway `SUCCESS`; live
health and cockpit promotion checks both returned `200`.

Paired QA confirmed the signed-preview renderer, invalid-composition response,
Railway-aware registration throttle, authenticated ethics audit, pending-claim
publication response, and relay origin/identity binding live-green on that
deployment. The broader lifecycle/role and synthetic Bazaar matrix remains in
progress, so this is not a final private-beta sign-off.

### Transient Railway edge incident

Between `2026-08-18T05:10:13Z` and `05:10:43Z`, Railway's public edge returned
two aggregate `502` responses for both the constant in-memory health endpoint
and the embedded cockpit root. The same deployment recovered without a new
release by approximately `05:11Z`.

The bounded investigation found no application-level `5xx`, panic, exit, OOM,
second container start, or health-handler dependency. The same instance remained
`RUNNING`; CPU and memory were continuous and flat, no dropped network flow was
recorded, and service ingress/egress was zero during the edge window before
resuming. The evidence therefore supports a transient Railway edge/routing
interruption, with **zero application-observed restarts**. Railway's internal
restart counter was not exposed by the available interface.

The release was deliberately not redeployed. Independent recovery checks passed
8/8, followed by a five-minute low-rate soak of `/api/health` and `/`: 120/120
responses were `200`, with `337.5 ms` p95 and `519.3 ms` maximum latency. A
future recurrence must capture the Railway edge upstream reason and instance
event before attributing it to application code.

## 2026-08-18 first-session trust handoff

Commit `da5e136` removes fabricated dashboard evidence, keeps the signed/local-
first and custody boundaries visible through 320/400-pixel and 200% reflow
layouts, selects signed-card foregrounds through measured WCAG contrast, and
shows a server-verified manifest receipt with signing key, workspace, lifecycle,
expiry, exact origins, and reference-only review sources. Missing or mismatched
verification evidence never renders a positive verification claim.

The integrated gate passed every Go package/command test and 12/12 Node tests,
including the required real-Chromium cockpit, opaque preview sandbox, signed-file
boundary, interaction, receipt-tamper, and responsive-reflow regressions.
Railway deployment `fe372625-e35f-4490-a8de-d8647826e7d3` reached terminal
`SUCCESS`; its Docker healthcheck passed, and independent public probes returned
`200` for `/api/health` (`status=ok`) and `/` with the Taawun cockpit. This is an
implementation handoff awaiting the paired QA task's fresh browser acceptance,
not a self-issued private-beta sign-off.

## Acceptance boundary

No customer-controlled DNS zone was provided for this QA session. The live run
therefore stops honestly at TXT challenge issuance and verifies that missing
proof fails closed. A private-beta customer or operator must add the challenge
to a controlled public hostname, verify it, refresh the signed preview with that
exact origin, activate publication, and smoke-test Host-bound delivery before
describing the external custom-domain journey as demonstrated.

The deployment remains a single Taawun instance, matching the relay's
process-local one-use-ticket replay store. Durable relay federation, shared
replay state, TURN provisioning, device-key enrollment/E2EE recovery, live
financial settlement, and qualified scholarly authority remain explicitly
deferred.

## 2026-08-18 exact-manifest attestation handoff

Commit `57d9fd1` binds the browser receipt to the exact manifest serialized by
the trusted server after artifact signature verification. The client recomputes
the SHA-256 digest, compares the complete manifest, checks the displayed scalar
identity/signature relationships, and requires a future authorization expiry
before showing the active verified state. Cryptographically attested but expired
evidence is labelled signed-and-expired; missing, changed, inconsistent, or
expired authorization never renders the active `Verified` claim.

The integrated release gate passed all Go package/command tests and 12/12 Node
tests, including the required real-Chromium positive and tamper matrix. Railway
deployment `1bfa3008-319c-49ba-bd52-090da3dbfa5e` reached terminal `SUCCESS`
with image digest
`sha256:30bfd3f9eac2037ed991d9194630b38a95aabcc0de74e2b5387e0a7318a9b79b`.
Independent public probes returned `200` for `/api/health` with `status=ok` and
`200` for `/` with the Taawun cockpit. This remains an implementation handoff
pending paired live-browser acceptance.

## 2026-08-18 preview-origin denial boundary handoff

Commit `e269126` keeps strict preview-origin authority while making expected
denials deterministic at the HTTP boundary. Requested origins are authorized
and normalized before a draft or idempotency key is persisted, then checked
again before validation and signing. An unverified surface, embedder,
connection, or resource returns nested `422 invalid_composition`; dependency,
workspace, cancellation, and deadline failures retain their operational error
classes. A revocation race leaves a staged track resumable and cannot produce a
build request, artifact, or stale success.

Outcome telemetry contains only a trusted/generated bounded request ID, fixed
outcome and reason classes, and status. Railway request IDs are trusted only
from the existing marked direct-proxy boundary; origins, principals, tokens,
emails, bodies, and database errors are never logged.

The integrated gate passed every Go package/command and 12/12 Node tests with
required real Chromium; the verifier-requested focused Conductor/handler
assertions also passed and independent review approved the final diff. Railway
deployment `58d87a23-6037-4cc5-8151-09b37167cef5` reached terminal `SUCCESS`
with image digest
`sha256:9f6670b46c32637fa2052a58d7e09d05478b3922f0640dc72ff566822df145b3`.
Public `/api/health` and `/` probes both returned `200`.

Synthetic operator cleanup remains separately blocked on authority, not code:
the supported admin routes exist, but the service has no configured bootstrap
admin environment and this lane has no authenticated admin principal. The
attempt stopped before reading or mutating fixture data. No database access,
token minting, or inferred deletion was used; the control room was asked to
supply an authorized admin session or explicitly authorize deliberate use of
the existing bootstrap-admin provisioning primitive.

## 2026-08-18 role-aware workspace discoverability handoff

Application commit `09cd897` adds one compact Workspace surface over existing
private-beta primitives. It renders the real approved template/module catalog,
workspace-scoped usernames and roles, invitation create/accept, Shura
create/recover/vote/decide, the seven durable financial sandbox flows and quest
lifecycle, and the public published-only Bazaar catalog. Where the server has no
list API, the cockpit labels browser-session records and supports durable recovery
by ID instead of inventing a feed. Empty/error/retry states remain explicit, and
finance/Bazaar copy continues to disclaim custody, balances, settlement,
certification, and scholarly authority.

The People response contains only active usernames, roles, and join times after
workspace authorization; email and password fields are absent. Delayed workspace
or prior-principal responses are rejected before updating the cockpit, and
workspace-bound invitation, Shura, quest, and Bazaar results cannot write across
selection or sign-out boundaries. Viewer controls stay read-only, Maintainers may
build/propose/vote, and Architects retain invitation, decision, publication, and
state-valid sandbox controls.

The integrated gate passed every Go package/command and 13/13 Node tests. Required
real Chromium covered organizer → invited Viewer → Maintainer, rapid workspace
and sign-out response races, keyboard tab/panel navigation, 320/400-pixel and
effective 200% reflow, Shura vote/decision, donation quest
PENDING → APPROVED → EXECUTING → SETTLED, and Bazaar
error → empty → published test-drive/purchase/refresh/install. The accepted
signed-preview/receipt/sandbox regression also remained green. Independent review
approved the corrected diff.

QA ledger commit `e8345ad` preserves the independent task's supplied evidence
verbatim. Railway deployment `c2374acf-7102-4471-91fd-e568f7bdb5ce` reached
terminal `SUCCESS` with image
`sha256:825e94375a0920bcb88a25406d59552ee269717b18cc57b4b0a8174a26fc00a4`.
Public `/api/health` returned `200` with `status=ok`; `/` returned `200` and the
served document contained the Workspace tools, People, financial sandbox, and
published Bazaar surfaces. Persona acceptance remains with the paired live-QA
task; this handoff does not claim a published Bazaar fixture or controlled-DNS
journey.

## 2026-08-18 signed component-document checkpoint handoff

Application commit `5c5018c` is the coherent signed component-document
checkpoint. The independently reviewed implementation comprises the
Shura-to-Finance decision handoff (`1b07df0`), the v2 durable component contract
and legacy-v1 compatibility (`c5ff806`), raw Unicode-scalar rejection
(`86dc1a3`), and the role-aware browser editor (`f78a191`). The data contract,
bounds, recovery behavior, and rollback boundary are documented in
`docs/component-document-contract.md`.

The browser now requires an explicit real-catalog template choice, keeps one
stable component instance per allowed module, edits declared and custom JSON
data, and reconciles incompatible template switches without discarding the old
draft. HTTP, MCP, Conductor, build requests, rendered files, manifests, digests,
signatures, and verified track reloads bind the same canonical documents.
Duplicate keys, non-canonical or precision-losing numbers, unpaired Unicode
surrogates, reserved security fields, and bounded-depth/size violations fail
before durable track or idempotency state. Viewer access remains read-only;
Maintainer and Architect writes still require the server's workspace
capability.

The single integrated release gate passed every Go package/command test, 13/13
Node tests including both promoted real-Chromium journeys, both production
binary builds, and `git diff --check`. Chromium covered two signed templates,
custom field add/edit/remove, exact Unicode round-trip, adversarial JSON,
explicit template reconciliation, delayed build/file/runtime isolation,
verified-track reload, receipt tampering, Viewer isolation, Maintainer editing,
responsive reflow, and the approved Shura decision-to-finance handoff.

Railway deployment `12cbe8e9-67fa-45dc-97e8-057ea4781de9` reached terminal
`SUCCESS` with image digest
`sha256:a1eddb5082114ea962f3932b15baec942ac4d42012d27b00b4a49918c35f0afc`.
Deploy logs show one container start, database initialization, and the server
binding `:8080` without an application error. Eight low-rate public probe pairs
over approximately forty seconds returned 8/8 `200` for `/api/health`
(`status=ok`) and 8/8 `200` for `/` (`Taawun Builder`).

A bounded live API smoke used the real three-template/eleven-module catalog and
created an exact Unicode component preview (`201`) on track
`track_9a5s93wgCRQPpKsLubsmeMvc`. The signed document was anonymous `401` and
owner `200`; a fresh Viewer was denied People access before invitation (`403`),
accepted the invitation (`200`), then read People, the track, and signed file
(`200`) while preview mutation remained denied (`403`). Workspace `25` and
users `32`/`33` were deleted through supported routes with `204/204/204`.

One earlier smoke harness stopped after registration because registration and
login are intentionally separate. It created no workspace or artifact, but its
random credential was not retained after the process exited. Allocation order
suggests user `30`; that ID is an inference, not a read-back. It remains a
supported-admin cleanup item and was not deleted or inspected through raw
database access. The rollback evidence remains application `09cd897` /
deployment `c2374acf-7102-4471-91fd-e568f7bdb5ce`; use Railway's supported
redeploy path only for an application-caused regression, not for a transient
edge event.

This is an implementation/deployment handoff, not independent live-browser
acceptance. QA's Codex Browser bridge is temporarily blocked by an external
trusted-plugin-path mismatch and will run the required fresh browser matrix
after that tooling trust state is refreshed.
