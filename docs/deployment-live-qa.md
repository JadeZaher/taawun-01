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
`git diff --check`. Railway deployment and paired live retest are the next gate;
their identifiers will be appended without replacing the original baseline.

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
