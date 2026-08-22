# Taawun continuous live QA

QA role: independent live-site acceptance lane for the private-beta MVP.
Started: 2026-08-17 MDT / 2026-08-18 UTC.
Scope: browser customer journeys, live API contracts and adversarial boundaries,
Railway health/log/metric correlation, regression verification, and synthetic-data
cleanup. This lane does not implement or deploy source fixes.

## Acceptance state

**BLOCKED — application `5c5018c` is deployed and its component-document HTTP
contract is independently live-green, including exact signed documents and the
pre-deploy Unicode/parser P1 fixes. Final acceptance is blocked by the Codex
Browser plugin trusted-path failure, so no fresh live UI, responsive, keyboard,
race or Shura-to-Finance cockpit credit is claimed.**
Deployment `68914f4c-a916-43f7-b0bf-fbd578c25f43` fixed the raw-JavaScript
preview, invalid composition, Railway-source throttle, publication state, account
lifecycle, and ethics boundaries. Deployment
`fe372625-e35f-4490-a8de-d8647826e7d3` made dashboard evidence truthful,
preserved mobile value/custody copy, corrected theme contrast, and exposed signed
receipt details. Deployment `1bfa3008-319c-49ba-bd52-090da3dbfa5e` now binds the
complete manifest and rejects mismatched or elapsed authorization evidence; that
trust gate is accepted. Deployment `58d87a23-6037-4cc5-8151-09b37167cef5`
correctly maps and safely correlates unauthorized requested preview origins.
Application `09cd897` / Railway deployment `c2374acf-7102-4471-91fd-e568f7bdb5ce`
is the last-known-good rollback baseline for component-document QA. Acceptance
remains blocked until the component checkpoint passes a fresh live browser pass.
A roughly 30-second all-route 502 interval at
05:10Z was correlated as a transient Railway edge/routing interruption rather
than a confirmed application crash; independent and implementation-lane recovery
soaks passed without redeployment. Any recurrence remains an immediate P0, but
the incident is not holding unrelated MVP testing. No MVP sign-off is possible
until all remaining P1 market findings are closed and a final comprehensive
browser pass is green.

After technical closure, acceptance also requires a credible market journey:
first-session comprehension, fast and recoverable preview/publish work, clear
value and trust boundaries, and usable organizer, member, approver, buyer, and
operator paths. Proposed fixes must compose existing capabilities rather than add
enterprise abstractions, fake activity, or misleading authority claims.

## Deployment ledger

| Deployment | Source | Status | QA result |
|:---|:---|:---|:---|
| `04f32db0-72e9-4364-82c6-bee0796ab161` | Railway did not expose a commit hash; deployment time aligns with remote PR head `68372da` | `SUCCESS` | P0 preview renderer failure reproduced; login, workspace, artifact build, signed file serving, and card content otherwise reached |
| `db2bf81c-c68d-4ecf-a949-f3fa240c69af` | `a484bfe` | Failed before health promotion | Container could not execute `/usr/local/bin/docker-entrypoint.sh`; deploy logs repeatedly reported `No such file or directory`, consistent with CRLF shebang packaging. Prior `SUCCESS` deployment remained active; no live regression was run. |
| `68914f4c-a916-43f7-b0bf-fbd578c25f43` | `9748666` | `SUCCESS` | P0 preview and prioritized contract fixes live-green. Full-site Railway 502 window at 05:10Z recovered without QA intervention and was classified as a transient edge/routing interruption after a 120/120 five-minute soak. Acceptance remains blocked on role/product viability. |
| `fe372625-e35f-4490-a8de-d8647826e7d3` | `da5e136` (`a39e246` is docs-only PR head) | `SUCCESS` | Dashboard, mobile/auth semantics, contrast, and visible receipt detail retests are live-green. P1 receipt-attestation scope gap remains; discoverability batch has not started. |
| `1bfa3008-319c-49ba-bd52-090da3dbfa5e` | `57d9fd1` (`f282aac` is docs-only PR head) | `SUCCESS` | Exact-manifest attestation trust gate accepted live. Complete digest/manifest binding, scalar and internal relationships, lifecycle/expiry, four origin categories, missing/corrupt evidence, sandbox/runtime, interaction, and signed-file boundaries are green. Separate P1: unauthorized requested preview origins return a generic application 500 instead of a deterministic client-error envelope. |
| `58d87a23-6037-4cc5-8151-09b37167cef5` | `e269126` (`42a6a30` is docs-only PR head) | `SUCCESS` | Preview-origin boundary accepted live: four categories and identical replay return deterministic 422/no track, corrected same-key request creates a signed preview, safe Railway telemetry correlates all six denials, spoofed request ID is ignored, and immutable receipt/sandbox/signed-file boundaries remain green. |
| `c2374acf-7102-4471-91fd-e568f7bdb5ce` | `09cd897` (`8b83d90` docs head; independent QA ledger `e8345ad`) | `SUCCESS` | Rollback baseline: real catalog, People/invitations, Viewer and Maintainer roles, Shura, finance and honest empty Bazaar surfaces are live. P1 continuity defect: an approved Shura decision is not exposed/carried into Finance, and the session proposal list remains stale. Component-document editing is not part of this release. |
| `12cbe8e9-67fa-45dc-97e8-057ea4781de9` | `5c5018c` (`27d3e7f` docs head) | `SUCCESS` | Component-document HTTP gate independently live-green: 3 templates/11 modules, v2 exact multi-component/custom/emoji manifest and files, verified reload, safe parser bounds, same-key correction, signed-file auth, and supported cleanup. Implementation's 13/13 Node/real-Chromium and live role smoke are green. Independent live UI acceptance is externally blocked by the Browser plugin trusted-path defect. |

The prior accepted checkpoint `80feecc7-e5fe-49cc-8cfa-858a64e86cee`
(`6a4d3a0`) is now `REMOVED` and is retained only as historical evidence in
`docs/deployment-live-qa.md`.

## Baseline journey — deployment `04f32db0`

Tested in the Codex in-app browser against
<https://taawun-production.up.railway.app> with a fresh synthetic principal.

| Step | Result | Evidence |
|:---|:---|:---|
| Cockpit | Green | `GET /` 200; login UI and authenticated Swiss cockpit rendered |
| Registration | Green for one bounded attempt | `POST /api/register` 201 at 04:17:50Z; response did not include `X-Request-ID` |
| Login | Green | `POST /api/login` 200; authenticated cockpit rendered |
| Workspace | Green | `POST /api/workspaces` 201; workspace `10` selected after a fresh login |
| Default catalog/composition | Green in cockpit | `community-iftar`; registration, announcements, and donation modules selected |
| Signed preview API | Green | `POST /api/artifacts/preview` 201 at 04:23:28.093Z in 42 ms |
| Signed preview files | Green | track `track_FmQqBlt2gkWKjJ2Q3ikQ8T3n`; index/theme/app files and the pinned Datastar asset all returned 200 |
| Visual iframe | **P0 fail** | Raw Datastar source appears before the styled card; the styled card and controls follow it |
| Browser console | No error emitted | The failure is HTML parsing corruption, not an ordinary runtime exception |
| Railway resource health | Green during repro | CPU average 0.0005 (max 0.0076); memory average 0.0198 GB (current/max 0.0312 GB) over one hour |

### P0-1 — Datastar runtime is emitted as visible iframe text

Expected: the sandboxed staging iframe begins with the signed community card.

Actual: the card is preceded by most of the Datastar runtime as visible text.
The customer content still renders after the leaked source.

Structural evidence from the live iframe `srcdoc`:

- The intended inline runtime starts at byte/index 5,822 and should close at
  40,062.
- A second external runtime tag is injected into the JavaScript at 12,907 and
  contributes an early `</script>` at 13,030.
- The injection occurs at Datastar source containing JavaScript replacement token
  `$&`. The cockpit uses `String.replace` with the entire runtime as the
  replacement string, so replacement-token expansion substitutes the matched
  external script tag inside the runtime.
- Railway confirms every supporting request returned success, proving the fault is
  client assembly rather than artifact serving.

The implementation task received the P0 fix batch with exact repro, deployment,
track/artifact identifiers, evidence-supported code area, and required
real-browser regression. Railway's plugin HTTP-log renderer did not expose request
IDs, and the application response did not supply one; timestamps, deployment,
method/path, track, and artifact IDs are recorded instead.

### P1 contract findings queued behind the P0 batch

These were rechecked on the same deployment while the P0 fix was in progress.
They will be sent as the next coherent implementation batch after P0 deployment
handoff/retest.

1. **Invalid catalog module is still a server error.** A valid authenticated
   preview request for workspace `10` with the only module set to
   `not-an-approved-module` returned `500` and
   `composition_operation_failed` at 04:27:44.821Z. Expected is a bounded
   `422 invalid_composition`-class response. The response now has `no-store` and
   `nosniff`, but no request ID.
2. **Registration throttle still does not group one live source.** Seven
   sequential strict-JSON failures from one QA client at 04:28:20–04:28:21Z all
   returned `400`; attempt six never returned `429` despite the configured
   five-attempt window. Railway recorded all seven requests but its available log
   renderer did not expose `srcIp` or request IDs.
3. **Ethics audit is public, permissive, and unbounded.** Anonymous valid,
   unknown-field, and 20,000-character prompt requests all returned `200` at
   04:29:00–04:29:01Z. Unknown fields and the oversized body were accepted; the
   responses had neither `no-store`, `nosniff`, nor request IDs.
4. **Domain-only self-delete is green, narrowing the lifecycle defect.** User
   `14` created pending claim `6v0CZbKbikcZOZijX3ZG0Xiu` and then successfully
   self-deleted with `204`. The remaining known `500` therefore needs the earlier
   OAuth-referenced user fixture or a fresh OAuth lineage; it is not reproduced by
   a pending domain claim alone.

## Post-fix live matrix — deployment `68914f4c`

Fresh browser principal `15`, workspace `11`, track
`track_pMM8J-Yj0Th6tApeCR83Ipzo`, artifact
`art_056d91d6aa5c93af3190f5c5d635ee73`.

| Area | Result | Live evidence |
|:---|:---|:---|
| Signed preview | **Green** | Iframe begins with the styled signed card; no Datastar source text; registration control interacted successfully. `srcdoc` contains one inline and zero external runtimes; exact sandbox remains `allow-scripts allow-forms`. |
| Signed files | **Green** | Anonymous preview-file GET is 401; bearer GET is 200 `text/html`, `no-store, private`, and `nosniff`. |
| Invalid module | **Green** | `zakat` against `community-iftar` returns 422 nested `invalid_composition`, not 500. |
| Register throttle | **Green** | Same Railway-observed source: attempts 2–5 strict-body 400, attempt 6 429 with `Retry-After`. A later attempt from a client-supplied different IP remained 429, confirming the edge does not permit easy source spoofing. |
| Ethics audit | **Green for P1 boundary** | Anonymous 401; authenticated wrong content type 415, unknown/trailing JSON 400, 8,193-byte body 413, 4,097-character prompt 400, valid request 200; attempt 31 per principal 429. |
| Pending publication | **Green** | Pending claim `31c1OWd8CnazactpjaREF0QA` returns 409 nested `publication_claim_unavailable`; track version remains 6 with no stale transition. Activation after revoke remains external-fixture gated. |
| Catalog | **Green** | Authenticated live catalog returns exactly three templates and eleven modules. |
| Relay binding | **Green** | Exact origin/workspace/artifact/hash request returns 201 with `taawun-relay-v1`; wrong origin, hash, and workspace all return indistinguishable 404. Last green Railway request ID before the later outage: `XjG7SH_6RMCM8pIx9o6EoQ`. |
| OAuth/MCP discovery | **Green with consistency gap** | Metadata endpoints return 200/no-store. Anonymous `/mcp` challenge returns 401 with the pinned protected-resource metadata URL and `taawun:read`; envelope is still plain text. |
| Exact CORS | **Green for origin isolation** | Configured production origin receives exact ACAO; `https://evil.example` receives none. No credentialed wildcard observed. |
| Invitation and Viewer boundary | **Green** | User `16` was denied workspace, track, and file access before invitation. After accepting Viewer invitation `invite_mCghWgajW5yXG42aH1lNh-OI`, workspace, signed file, and decided proposal reads returned 200 while preview/build and proposal creation returned 403. |
| Shura lifecycle | **Green** | Architect capability, proposal `proposal_JAZ6eTts7XkIcM4PuZD-7L6s`, vote, and APPROVED decision `decision_VuWnH1Ubwd0dkg2kfsSdv1d3` completed; proposal reached DECIDED version 3. Decision request `6wo5-Eu_Sy6u8NZCY53eZw`. |
| Financial sandbox | **Green for bounded sandbox lifecycle** | Seven flows advertised. Client-supplied actor was rejected 400. Synthetic donation quest `quest_ARbpv-_FLgOwBYwgG7C0t41l` progressed CREATED -> APPROVED -> EXECUTING -> RECONCILIATION_PENDING; all five expected events formed a valid hash chain. No real settlement or custody was implied. Request `A6laXQepR8eyxw6P9o6EoQ`. |
| Password/session invalidation | **Green** | Password update returned 200; the old JWT and old password immediately returned 401, and the new password logged in. Response hardening remains incomplete because the successful update lacked `no-store` and `nosniff`. |
| OAuth/MCP authorization | **Green** | Public PKCE client registration, authorize/consent redirect, token exchange, and MCP initialize succeeded. The authorization cookie was Secure, HttpOnly, and SameSite=Lax. Token request `EdV63M0rSzqL7_nbWUN5dQ`. |
| OAuth/domain-referenced self-delete | **Green** | User `15` deleted with 204 after signed artifact, pending domain, OAuth, Shura, finance, and invitation activity. JWT and OAuth access returned 401, refresh returned `invalid_grant`, and the old authorization cookie returned to login. Audit events remained readable with actor `15`; no raw FK/database error appeared. Delete request `bxxaUcbNRwa8GNnWnPRhug`. |
| Dashboard activity | **P1 fail** | Live dashboard returns hard-coded user counts and a fabricated `Logged in` event dated `2024-01-01T00:00:00Z`. A private-beta operator cannot distinguish real evidence from placeholder data. |

### Availability incident requiring Railway correlation

- At 05:10:13Z, `/favicon.ico`, the documentation probe, and the OpenAPI probe
  began returning Railway edge 502.
- At 05:10:38–05:10:43Z, both `/api/health` and `/` returned the same edge 502,
  proving a whole-service interval rather than missing-route behavior.
- The site had served a bound relay request at 05:08:49Z and returned to health/root
  200 at approximately 05:11Z without QA intervention.
- The implementation task received the exact interval, deployment, and last green
  Railway request ID for crash/restart/log correlation. The same instance remained
  RUNNING with zero application-observed restarts and no panic, exit, OOM,
  shutdown, second container start, new deployment, or matching application HTTP
  >=500 record. Railway does not expose a true internal restart counter, so this
  is not presented as a platform-level zero.
- Independent and control-room recovery gates produced fourteen consecutive
  health/root 200 responses over overlapping 40-second soaks. The implementation
  lane then ran 120/120 successful health/root probes every five seconds from
  05:13:17Z to 05:18:42Z: zero failures, p95 337.5 ms, max 519.3 ms.
  Thirty-minute Railway metrics stayed low (CPU max 0.00396 vCPU, memory max
  31.12 MB, p95 8 ms), with two aggregate 5xx among 140 requests and zero dropped
  network-flow records. Classification: transient Railway edge/routing event,
  not a confirmed application crash.
- A final low-rate health probe after synthetic cleanup returned 200 at
  05:34:01Z (Railway request `lXHQyFd8RUSfgFnN9fVATg`); no edge recurrence was
  observed during the resumed matrix.

## Synthetic data and cleanup

| Principal/workspace | Purpose | Cleanup state |
|:---|:---|:---|
| User `14`, workspace `10`, artifact `art_f0291aef6bf1fb2d7bc745dfebb5ed05` | Fresh P0 browser repro and domain-only self-delete narrowing | Cleanup completed through `DELETE /api/users/14` (`204`); retained identifiers are evidence only |
| User `15`, workspace `11`, track `track_pMM8J-Yj0Th6tApeCR83Ipzo`, artifact `art_056d91d6aa5c93af3190f5c5d635ee73`, pending claim `31c1OWd8CnazactpjaREF0QA` | Post-fix browser, relay, publication, OAuth/account-lifecycle, Shura, and financial QA | Cleanup completed through `DELETE /api/users/15` (`204`; request `bxxaUcbNRwa8GNnWnPRhug`). Old JWT, OAuth access/refresh authority, and authorization cookie were invalidated. Required audit attribution remains retained. |
| User `16`, workspace `12`, Viewer membership in workspace `11`, invitation `invite_mCghWgajW5yXG42aH1lNh-OI` | Independent principal, invitation acceptance, actor/workspace isolation, and Viewer denial | Workspace `12` deleted with `204` (`l-dQl7ODSOG2QPTA9I3ezw`), then user `16` deleted with `204` (`HiuWSuhXQ3O-dKlc-_9nXA`); old token returned 401. |
| OAuth public client `taawun_client_2dEPrJWKeYFmghlk-CV9xS3EOkoxJw4D3Ew5oCyCFm8` | PKCE authorization and MCP challenge/invalidation | Retained synthetic identifier because the public-client API has no supported deletion route. No secret, credential, access token, refresh token, or authorization code is recorded. |
| User `17`, workspace `13`, track `track_kSAC9qY_o8hFnb1iFhT5Jv1x`, pending claim `Rg7llNYN3kCziQqYVLYIyzPIM` | Trust-batch browser/API, responsive, signed receipt, contrast, publication, invitation, Shura, and financial regression | Workspace `13` deleted with `204` (`bB-5QnKRQHKK41_anpoFkQ`), then user `17` deleted with `204` (`d0uDN88V6TEK-w087WUN5dQ`); old token returned 401. Workspace deletion made the retained track inaccessible. |
| User `18`, Viewer membership in workspace `13`, invitation `invite_O3Z8DzHIEVM_lC3uXUs1_mk5` | Honest empty dashboard, pre/post-invite isolation, and Viewer denial | User deleted with `204` (`B-rcSchXTMGpEN_q2h0iww`); old token returned 401. |
| User `19`, workspace `14`, artifact `art_b62908753cc1548a6329b118c49bb5e6` | Fresh real-browser exact-manifest receipt, sandbox, and interaction retest | Pending supported operator cleanup. The in-app browser retains the authenticated session but exposes no supported account-delete UI or response-body/session bridge; QA will not extract credentials or use raw database deletion. |
| User `22`, workspace `17`, track `track_k-SFjjcMc1kJ8mX8m6Iy0UUA`, artifact `art_e68a7fb7d695e37e6ed01529c9f044b2` | Corrected live API digest, manifest, mutation, signed-file, and expiry matrix | Workspace and user both deleted with `204`; the old token returned `401`. |
| User `23`, workspace `18`, track `track_KL3HSuls6Y7ezZtt63YXwvZd`, artifact `art_9d38ec588ae3f2a586016c76f9947ee9` | Four-category preview-origin denial and corrected same-key signed-preview retest | Workspace and user both deleted through supported routes with `204`; the old token returned `401`. |
| User `24`, workspace `19`, track `track_j9ACIFlbwFHbL7INEe_kxwu1`, artifact `art_9ae54568d60a1da71df8db748b16dd42` | Identical denial replay and corrected same-key signed-preview retest | Workspace and user both deleted through supported routes with `204`; the old token returned `401`. |
| Users `26` (Architect), `27` (Viewer), and `29` (Maintainer); workspace `22`; artifact `art_4a115ebd40f8c784139e3e5032746c6f`; proposal `proposal_a17beB3U_DYPbQvz0HpdZGb-` | Live rollback-baseline organizer → Viewer → Maintainer catalog, signed-preview, People, Shura, finance and Bazaar journey | Workspace `22` deleted with `204`; all three users deleted through supported self-lifecycle routes with `204`; every old token returned `401`. No quest or Bazaar listing was created. |
| Inference-only user `30` | Implementation's registration-only component smoke stopped before login/workspace | Pending supported application-admin read-back/cleanup. Allocation is inferred only; no credential, workspace, track or artifact exists in the evidence. No raw deletion. |
| User `34`, workspace `26`, tracks `track_WwRfj3BvKx7vBvrMWC2xGln5` / `track_58qfrQdVG9rjBCJGDDCMrOMx` | Independent component exactness/adversarial HTTP matrix | Workspace/user deleted `204/204`; old token `401`. Signed artifacts remain immutable evidence. |
| Users `35/36/37`, workspace `27` | Role harness with rejected username-shaped invitee | All three users and workspace deleted through supported routes with `204`; all old tokens `401`. Downstream role statuses were discarded as harness-invalid. |
| Users `38` (Architect) / `39` (Viewer→Maintainer), workspace `28`, tracks `track_yr1lbtRKjfwGn_5Mg-eLwGz8` / `track_KA71mFCbqIvAJLAQwjB7H43i` | Corrected second-template, role, Shura and finance continuity matrix | Member/workspace/Architect deleted `204/204/204`; both old tokens `401`. Quest `quest_OoW7kRyOH8Uu7a_cSm8sTdIo` cancel endpoint returned `200`; Shura/financial audit identifiers retained as evidence. |
| One bounded People API principal/workspace | Authorized/cross-workspace People contract, fields and response-hardening check | Workspace and user both deleted with `204`; old token returned `401`. The synthetic credentials/token were never recorded. |
| Two intermediate in-memory API principals/workspaces | Unauthorized-origin 500 and relative-preview-URL harness correction | Supported workspace/user deletes ran from `finally`; operator read-back is requested because the deliberate harness exceptions suppressed their cleanup IDs/status output. No credential or token was persisted. |
| User `8` | Earlier domain/OAuth self-delete repro | Pre-existing cleanup blocker; pending supported operator cleanup only. QA will not attempt credential recovery or raw database deletion. |

No credentials or bearer tokens are recorded. Synthetic records will be removed
only through supported lifecycle routes. Users `8` and `19` cannot be removed by
this lane without their credentials or an authorized admin principal; the
implementation or control-room lane must perform that supported cleanup and
confirm the two exception-path API fixtures are absent.

## Remaining coverage for the current deployment family

- Cleanup of users `8` and `19`, plus read-back of the two exception-path API
  fixtures, by a supported authorized path.
- Activation-after-revoke remains DNS-fixture-limited. Viewer and Maintainer
  isolation, Shura authority, finance actor boundaries, OAuth/MCP, relay,
  session/password invalidation, exact CORS, and workspace isolation are
  live-green on the rollback baseline.
- The component-document server contract, role boundaries and Shura-to-Finance
  decision continuity are live-green on deployment `12cbe8e9`; the promoted
  Chromium suite covers the new editor. This lane still requires a fresh live
  browser pass for editor recovery, visible decision carry, stale-response races,
  keyboard/AT semantics, 320/400 px and effective 200% reflow. Request-ID
  coverage beyond the accepted origin-denial path, consistent errors, OpenAPI,
  and favicon remain P2.
- Controlled DNS verification/publication/public serving remains fixture-gated;
  it must not be weakened or claimed complete without an external DNS fixture.

Railway domain inventory is conclusive for this service: the only configured
domain is the active Railway service hostname
`taawun-production.up.railway.app` (`c9143ccc-bb40-434b-bff9-b1411a1ee739`).
There is no existing custom domain. Exact DNS verification, activation, and
Host-bound serving are therefore externally blocked until the control room
supplies a controlled hostname.

The control room has authorized one clearly labelled, fully synthetic Bazaar
SANDBOX listing/test-drive/purchase fixture after the current technical-fix
deployment is healthy. It must use test data only, remain isolated from real
settlement, preserve compliance and custody disclosures, record every ID, and be
cleaned up unless explicitly retained as a documented private-beta demo.

Source and live precondition review show that a valid Bazaar draft has no supported
delete route, review transitions require a real admin reviewer, and publication,
test-drive, and purchase all require an active verified publication matching the
listing content hash. The live public catalog returns 200 with a real empty list.
Creating an unpublishable orphan draft would violate the cleanup requirement and
would not exercise the authorized journey. The fixture is therefore externally
blocked by controlled DNS plus an authorized reviewer, with the missing cleanup
route recorded as a product constraint; no proof, reviewer, or settlement boundary
was bypassed and no orphan synthetic listing was created.

## Ranked market-viability backlog

This backlog is intentionally provisional until each item is exercised live on
the implementation deployment. Priorities describe commercial acceptance, not a
request for new platform primitives.

| Priority | Persona / job | Current evidence | Minimal acceptance criterion |
|:---|:---|:---|:---|
| P1 | Community organizer: understand value and reach a trustworthy preview | Fresh registration, workspace creation, concrete signed/local-first/sandbox copy, and an intact interactive signed preview are live-green at desktop, 400px, and 320px | Measure unsupported first-session time-to-preview and recovery friction in the next persona pass |
| P1 | Organizer/operator: find the next action after preview | Domain controls exist, but broader publication state and recovery need a complete live pass | Clear exact next step, failure recovery, and honest external-DNS requirement; no dead-end or authority overclaim |
| Closed | Invited Viewer/Maintainer and governance approver: find their work | Deployment `c2374acf` exposes real People/invitation and Shura surfaces. Viewer pre-invite isolation/read-only and Maintainer build/propose/vote boundaries passed live; session state cleared on principal switch. | Preserve role/workspace generation guards and safe capability explanations |
| Closed for empty-fixture scope | Marketplace buyer/test-driver: evaluate before purchase | Deployment `c2374acf` queries the real published-only Bazaar catalog and presents the truthful empty/external-DNS+reviewer state; no listing was fabricated | Preserve published-only truthfulness; full purchase stays externally fixture-blocked |
| P1 | Organizer: carry an approved Shura decision into sandbox finance | Live proposal reached `DECIDED · version 3 · 1 vote · decision APPROVED`, but no decision ID is displayed or carried into Finance's required blank field. The session proposal list remains stale at `OPEN · version 1`. | Show/copy and workspace-safely carry the durable approved decision ID; refresh session record after load/vote/decision; stale workspace/principal results must not carry it |
| Gate | Organizer/Maintainer: build from exact component documents | The new checkpoint has no deployed live evidence yet. `09cd897` is rollback-only and has no component-document editor. | Multiple live templates/components; exact JSON document round-trip and signing; adversarial bounds; Viewer read-only; recovery without draft/verified-preview loss |
| P1 | Private-beta operator: understand platform health and cleanup | Health exists, but request IDs/activity/support surfaces are incomplete | Correlatable safe errors, real activity or explicit empty state, and supported cleanup/recovery without database access |
| Closed | Private-beta operator: trust displayed activity | Empty account returns zero scoped counts plus explicit empty messages; populated account returns one accessible workspace/user and one real `workspace_created` activity, with no fixed 2024 row | Preserve deterministic accessible-workspace scope and honest empty states |
| Closed | Generated-card user: read buttons and trust the default theme | Live default/light/mid/dark accents all select readable foregrounds; measured minima are 6.07:1 default, 13.17:1 light, 6.45:1 mid, 11.95:1 dark, and 17.58:1 baseline badge | Preserve >=4.5:1 across arbitrary accepted accents |
| Closed | Organizer: rely on the signed receipt's exact scope | Live app `57d9fd1` returns exact `manifestJson` plus SHA-256 digest; the browser and independent mutation matrix bind complete manifest equality, scalar/internal relationships, lifecycle/expiry, and every origin category. Elapsed authorization is attested-expired, never active Verified; missing/corrupt evidence fails closed. | Preserve the exact binding, honest evidence wording, and real-Chromium negative matrix |
| Closed | Mobile first-session visitor: understand value and custody | Concrete signed/local-first/sandbox and accurate central-retention/Amanah/zero-custody/no-settlement copy remain visible at 400/320px with no overflow; authenticated boundary also remains visible | Preserve compact copy and true 200% reflow regression coverage |
| P2 | All personas: keyboard, zoom, mobile, and trust comprehension | Semantic landmarks and labels are present in the baseline snapshot; full focus/contrast/reflow testing remains | WCAG 2.1 AA keyboard path, visible focus, 200% zoom/reflow, >=44px targets, and no misleading compliance/custody copy |
| Closed | API integrator/operator: diagnose rejected preview origins safely | Deployment `58d87a23` returned nested `422 invalid_composition` for surface, embedder, connection, and resource denials plus identical replay; no denied response contained a track or raw origin/error, and the same key created a corrected signed preview. All six Railway/application records correlated by trusted edge ID with only `outcome=denied reason=origin_not_verified status=422`; a spoofed client ID was absent. | Preserve strict authority, pre-persistence denial, reusable idempotency, safe trusted request correlation, and non-sensitive telemetry |
| P2 | API integrator/operator: diagnose and integrate safely | Application responses do not emit their own request ID; sensitive profile/password/dashboard responses inconsistently omit `no-store`/`nosniff`; `/openapi.json` and `/favicon.ico` are real 404s | Consistent safe envelopes and request correlation, sensitive-response hardening, accurate live OpenAPI, and a real favicon |

### Pre-fix first-session and accessibility evidence — deployment `68914f4c`

- At 320 and 400 CSS pixels the logged-out page has no horizontal overflow and
  input/button controls remain at least 44.5 px high. The two auth-tab buttons are
  38 px high.
- The only concrete pre-login product explanation is hidden at both 320 and 400
  px. The visible mobile copy is reduced to the Taawun name, “Build for your
  community,” Qur'an 5:2, private-beta/architect access, and session-storage copy;
  it does not explain signed artifacts, local-first data, or the sandbox boundary.
- The auth tabs advertise `role=tab`, but both lack roving `tabindex`, there are no
  `role=tabpanel` elements, and ArrowRight leaves focus/selection on Sign in.
- The page's first skip link is “Skip to builder,” but its `#mainContent` target is
  hidden while authentication is shown.
- Registration password guidance is not connected with `aria-describedby`.
- At desktop width, keyboard Tab returned cleanly from the sandboxed preview to the
  surrounding page and the focused summary showed a visible 2 px gold outline.
- The generated default card uses `#57A68E` with white foreground, approximately
  2.90:1 for normal text. This affects primary trust/comprehension content and
  remains a P1 visual-accessibility blocker.
- Browser screenshot capture repeatedly timed out, so structural DOM, computed
  style, focus, and interaction evidence is recorded rather than claiming a saved
  screenshot.

Public product polish probes after the recovery gate returned a real 404 for both
`/favicon.ico` (Railway request `Kt4K3dX7R4WElrv-9I3ezw`) and `/openapi.json`
(`xalCfet1R6KVDrj6WUN5dQ`). The repository's only Swagger file describes obsolete
`/api/v1` cookie/CSRF routes and omits the live conductor, domain, Shura, finance,
Bazaar, OAuth/MCP, relay, and ethics contracts.

## Trust-batch retest — deployment `fe372625`

Application commit `da5e136`; Railway deployment
`fe372625-e35f-4490-a8de-d8647826e7d3` terminal `SUCCESS`. Implementation gate:
all Go packages, 12/12 Node tests including real Chromium, clean diff check, and
independent approval. Live cleanup health request `XpyNwFAFT-CjP0ImWUN5dQ`
returned 200; no edge 502 recurrence was observed.

| Area | Result | Evidence |
|:---|:---|:---|
| Honest dashboard | **Green** | User `18` with no workspace received scoped zero counts, `evidence_state: empty`, “No workspace evidence yet…” and “No recorded workspace activity yet.” Requests `7KVEB5t7TX-bUnZUpHNmDw` and `1XFX-eQNS5yfOOV4MwUFZXw`. User `17` received only one accessible workspace/user and one real `workspace_created` activity; no fixed 2024 timestamp or `Logged in` row. |
| Logged-out value/custody | **Green at live desktop/400/320** | Concrete workspace-signed/local-first/sandbox text and accurate central-retention, Amanah, zero-custody, and no-settlement boundaries remain visible with no horizontal overflow. |
| Auth semantics | **Green with one tooling limitation** | Tabs have roving `tabindex`, paired tabpanels, and ArrowRight/Home/End change both focus and selection. Registration password uses `aria-describedby=registerPasswordHelp`. Skip link changes to `#authPanel` while logged out and activates the visible account panel. The in-app browser key API would not synthesize Tab traversal, so actual skip-link keyboard activation is not overclaimed. |
| 200% reflow | **Green in verified release suite; partial live evidence** | The app's integrated real-Chromium test passes effective 160/200px layout widths. The in-app browser viewport capability floors requests below 240px, where the live app also has no overflow; exact 160/200px could not be independently forced live. |
| Default accent | **Green** | `#57A68E`: hero/buttons 6.07:1, category badge 6.45:1, opaque baseline badge 17.58:1. |
| Representative accents | **Green** | `#E8DEC9` 13.17:1, `#C7942C` 6.45:1, `#173D36` 11.95:1 for hero/buttons; badges remain 6.45:1 and baseline 17.58:1. Synthetic artifacts were removed with workspace `13`. |
| Positive signed receipt | **Green** | Browser artifact `art_009b6cbd16d2c30b28806fef909c10f0` visibly matched its raw manifest for artifact/hash/workspace, Ed25519 key `railway-artifact-v1`, preview expiry, exact surface origin, and both seed references marked “Reference-only; not scholar approval.” API track `track_kSAC9qY_o8hFnb1iFhT5Jv1x` returned a server `verified:true` attestation for artifact `art_66d33aff3aacaef9fb761dc2d026935b` (request `hwgbkEATT3aM-rsjWUN5dQ`). |
| Receipt fail-closed scope | **P1 fail** | In deployed `index.html`, `verifiedManifestReceipt` compares artifact ID, content hash, workspace, signature algorithm/key/value, but neither recomputes a canonical manifest digest nor checks lifecycle, expiry, or allowed origins. `manifestLifecycle` merely appends `expired`. Therefore altered signed authorization fields or an expired receipt can still retain “Verified” if the shallow attestation fields match. Existing regression covers signature mismatch and missing fields only. |
| Preview runtime | **Green** | Exact sandbox `allow-scripts allow-forms`, no `src`, one inline/zero external runtimes, no visible raw-JavaScript pattern; clicking Compose changed the live notice. |
| Surrounding boundaries | **Green** | Signed file anonymous 401/bearer 200 no-store; Viewer pre-invite 403/post-invite read 200/build 403; Architect and Viewer-read Shura capability 201/Viewer-propose 403; seven financial flows; pending claim publication 409 `publication_claim_unavailable` with track version 6 unchanged; supported workspace/user cleanup 204 and revoked tokens 401. |

## Exact-manifest trust retest — deployment `1bfa3008`

Application commit `57d9fd1`; Railway deployment
`1bfa3008-319c-49ba-bd52-090da3dbfa5e` terminal `SUCCESS`; image
`sha256:30bfd3f9eac2037ed991d9194630b38a95aabcc0de74e2b5387e0a7318a9b79b`.
The implementation gate passed every Go package, 12/12 Node tests including the
required real-Chromium receipt matrix, clean diff check, and independent review.
QA accepts the signed-receipt trust gate on this release.

| Area | Result | Evidence |
|:---|:---|:---|
| Fresh browser receipt | **Green** | User `19` / workspace `14` built browser artifact `art_b62908753cc1548a6329b118c49bb5e6`. The styled card appeared first and the visible receipt exactly matched raw artifact/hash/workspace, Ed25519 key `railway-artifact-v1`, preview lifecycle/future expiry, the configured surface, and both Hanafi seed references marked reference-only/no scholar approval. |
| Exact server attestation | **Green** | A separate corrected live API build returned `201`, track `track_k-SFjjcMc1kJ8mX8m6Iy0UUA`, artifact `art_e68a7fb7d695e37e6ed01529c9f044b2`. Independently recomputed SHA-256 of the exact returned `manifestJson` equalled `manifestDigest`; parsing that string produced complete equality with the returned manifest. The positive state was attested, active, and not expired. |
| Scalar and relationship negatives | **Green** | Artifact ID, content hash, workspace, signature value, algorithm, and key mismatches all withheld active verification. Re-attested source variants with mismatched authorization subject workspace or authorization signer key also withheld it. |
| Complete authorization negatives | **Green** | Altered lifecycle, changed future expiry, and altered surface/embedder/connection/resource categories all withheld active verification. A consistent elapsed-expiry variant remained attested-expired but not active. Missing attestation, missing digest, corrupt digest, and missing receipt fields all failed closed. |
| Origin-fixture limit | **Explicit** | Live positive evidence covers the sole configured platform preview surface; there is no verified custom-domain fixture for positive embedder/connection/resource values. The deployed real-Chromium suite supplies positive four-category display plus altered-category rendering coverage. QA does not claim a live verified four-category origin fixture. |
| Preview runtime and controls | **Green** | Browser iframe sandbox is exactly `allow-scripts allow-forms`; `src` is absent; `srcdoc` is 42,407 bytes with one inline and zero external scripts. The first visible content is the signed card, no raw Datastar appears, Compose announcement reports an approved-host binding, and browser warning/error count is zero. |
| Signed files | **Green** | Anonymous preview-document GET returned `401`; bearer GET returned `200 text/html`. |
| Corrected API cleanup | **Green** | Workspace `17` and user `22` both deleted through supported routes with `204`; the old token returned `401`. |

### Closed P1 — preview-origin error mapping

An authenticated otherwise valid preview request supplied syntactically valid
HTTPS `.example.invalid` values in all four requested-origin categories. The
authority correctly denied them because preview composition permits only the
configured platform origin plus currently verified workspace origins. The
response mapping is defective: it returned generic
`500 composition_operation_failed` rather than a deterministic client-error
envelope.

Railway correlation: request `jOSPCuB7RquAXedUnPRhug`,
`2026-08-18T07:00:15.097285737Z`, `POST /api/artifacts/preview`, HTTP 500,
4 ms total/upstream, deployment `1bfa3008-319c-49ba-bd52-090da3dbfa5e`,
instance `e882c02b-11f8-4edd-8d33-290171740787`, edge `us-west2`, no upstream
error. It reached the Go service at port 8080; no edge failure, panic, restart,
or deploy anomaly occurred. Bounded runtime logs had no request-correlated
composition reason. Source correlation supports the path:
`AuthorizeOriginsForLifecycle` returns `ErrOriginNotVerified`, conductor records
`preview_origin_denied`, and `writeCompositionServiceError` lacks a mapping, so
the default 500 is selected.

Deployment `58d87a23-6037-4cc5-8151-09b37167cef5` closes the defect. Fresh live
surface, embedder, connection, and resource denials returned nested `422
invalid_composition`, no raw origin/error and no track. An identical denial
replay returned the same result without conflict; correcting that same
idempotency key created a `201` signed preview, proving denial did not consume
durable idempotency. The corrected previews retained exact digest/manifest
attestation, future active authorization, anonymous file `401`, and bearer file
`200`.

Railway/application correlation covered six denial request IDs:
`ksBVqyneRiKSaNIF2h0iww`, `ZsqIokUqTjyW7WV22h0iww`,
`oucc7CmTRWyM6rukYqVb7A`, `GNmwx0NxSt6IUbAvYqVb7A`,
`qP_sHpd1Qm-KUBG7xtoGcA`, and `zAtxCfb2Rky1F6C50_TJvA`. Each edge response was
`422` in 3–6 ms without upstream error; the matching application event contained
only the trusted request ID plus `outcome=denied`,
`reason=origin_not_verified`, and `status=422`. No origin, principal, email,
token, payload, database cause, track, artifact, or client-spoofed request ID was
logged. This technical gate is accepted.

## Discoverability rollback baseline — deployment `c2374acf`

Application commit `09cd897`; Railway deployment
`c2374acf-7102-4471-91fd-e568f7bdb5ce` terminal `SUCCESS`; image
`sha256:825e94375a0920bcb88a25406d59552ee269717b18cc57b4b0a8174a26fc00a4`.
The implementation gate passed every Go package/command, 13/13 Node tests with
the required real-Chromium journeys, a clean diff check, and independent review.
This is the rollback baseline for the subsequent component-document checkpoint.

Fresh live browser evidence:

- Registration, workspace creation and signed preview took approximately 12.2
  seconds of active network/action time. The UI exposed all three real templates
  and eleven catalog modules; the Bazaar template displayed eight approved
  modules, and switching among all templates reconciled the visible composition.
- Browser artifact `art_4a115ebd40f8c784139e3e5032746c6f` rendered the styled
  card first with an active exact receipt. The iframe retained sandbox
  `allow-scripts allow-forms`, no `src`, 42,411-byte `srcdoc`, one inline and zero
  external scripts, no visible Datastar source, and an interactive announcement
  control.
- People showed real selected-workspace usernames and roles only. An invitation
  token was explicitly session-only and disappeared on sign-out. The Viewer saw
  no workspace before acceptance, then read both members and the decided proposal
  while build/propose/vote/decide/quest-mutation controls remained unavailable.
  Principal switches exposed no prior invitation, proposal or quest state.
- The Maintainer could build, propose and vote, but could not invite, decide,
  publish or advance a quest. Workspace-tab ArrowRight, Home, End and ArrowDown
  navigation selected the correct tab/panel and moved focus into the panel.
- Finance advertised seven real vetted flows and consistently described durable
  sandbox orchestration rather than balances, transactions, custody or settlement.
  Bazaar returned the real published-only empty state and explicitly named the
  controlled-DNS, reviewer and cleanup-safe-fixture dependencies.
- The live People API returned `200`, `Cache-Control: no-store`, `nosniff`, one
  member with only `joined_at`, `role`, `user_id` and `username`, and no email or
  password. A cross-workspace request returned nested `403 workspace_forbidden`
  with the same hardening. Its synthetic workspace/user cleaned with `204/204`;
  the old token returned `401`.

### Closed implementation P1 — Shura-to-Finance continuity (live UI retest pending)

The Architect created proposal `proposal_a17beB3U_DYPbQvz0HpdZGb-`, recorded one
approval vote and finalized it approved. The authoritative card correctly showed
`DECIDED · version 3 · 1 vote(s) · decision APPROVED`, and all mutation buttons
disabled. However, the UI exposed no durable decision ID and Finance's required
“Approved Shura decision ID” field remained blank, so a real organizer could not
continue without out-of-band API knowledge. The session proposal list also
remained stale at `OPEN · version 1`.

Minimal acceptance: display a copyable durable decision ID and carry it into
Finance only when it is final approved and bound to the selected workspace;
refresh the session proposal record after every load/vote/decision; clear it on
workspace/principal generation change. Viewer remains read-only and Maintainer
remains unable to decide or advance Architect-only quest states.

Application `5c5018c` implements the durable decision display/carry and stale
response guards, with promoted Chromium coverage. Independent live API on
deployment `12cbe8e9` confirmed the server seam: Viewer read `200` and vote
`403`; Maintainer vote `201` and decision `403`; Architect decision `201`;
authoritative reload returned `DECIDED`, version `3`, one vote, `APPROVED`, the
same decision ID and proposal version `3`. Finance rejected a wrong decision
with `409 shura_decision_not_approved`, rejected a missing decision with `422
shura_decision_required`, accepted the exact approved decision with `201`, and
the synthetic quest's cancel route returned `200`. The cockpit's visible copy
and prefill remain part of the externally blocked fresh browser gate.

Responsive limitation: the in-app browser's advertised viewport override did not
change the document layout viewport in this run (`innerWidth` remained 1280), so
independent live 400/320/200%-reflow evidence is not claimed for this release.
The promoted 13/13 real-Chromium suite covers those breakpoints. The temporary
override was reset or invalidated when the browser-control session restarted.

## Component-document checkpoint acceptance gate

Deployment `12cbe8e9-67fa-45dc-97e8-057ea4781de9` / application `5c5018c` is
under independent acceptance. Final approval still requires
a fresh organizer/Maintainer/Viewer browser journey over multiple live templates
and components; exact request → durable track/reload → render → manifest/digest/
signature equality for declared and user-added JSON fields; adversarial JSON,
reserved/prototype-like key, depth/size/key-count, Unicode/control-text and
authority-injection bounds; fail-closed tamper/missing/stale/expired evidence;
last-valid-draft and last-verified-preview recovery; and preservation of every
accepted auth, receipt, origin, signed-file, role, Shura, finance, cleanup and
service-health boundary. Custom DNS and Bazaar publication remain out of scope
and must not be weakened.

Browser-tooling limitation at `2026-08-18T13:31Z` and again after a full Codex
app restart: the installed Browser plugin had upgraded from bundle
`26.810.52044` to `26.814.41407`, but the secure browser bridge rejected the new
`browser-service.mjs` cache path as outside its configured trusted code paths.
The live Taawun tab itself was open in the in-app browser; initialization failed
before any Taawun navigation or interaction. This is classified as an external
QA-tooling blocker, not an application regression. API/source/deployment checks
may continue, but no component-document release may receive the required
real-browser acceptance credit until the bridge is repaired and a fresh journey
is completed.

Closed pre-deploy P1 source finding: Advanced JSON could accept an unpaired Unicode
surrogate such as `{"title":"Unicode","summary":"\ud800"}`. The browser's
control-character check does not reject surrogate code units and its encoder
substitutes U+FFFD, while Go's JSON token decoder also replaces the unpaired
escape before `utf8.ValidString` runs. The server can therefore canonicalize and
sign data different from the organizer's draft, and the receipt path verifies
the returned manifest internally without binding its component documents back
to the exact draft fingerprint. Commit `5c5018c` added fail-fast browser and
HTTP/MCP rejection before durable state, safe bounded `422`, one canonical raw
JSON parser, receipt-to-submitted-document binding, retention of the last valid
draft/preview, and positive exact round-trip coverage for valid surrogate
pairs/emoji. The independent HTTP evidence below confirms the server seam.

The same exactness batch includes the custom object/array value editor: unlike
the Advanced JSON editor, it decoded raw JSON before duplicate-key and
canonical-number checks. Inputs such as `{"a":1,"a":2}` or
`{"amount":1e2}` could be silently collapsed/rewritten and then accepted as
the mutated JavaScript value. Every raw JSON entry surface must apply one policy
before decoding, including nested object/array duplicates, exponent, negative
zero, trailing fractional zero, invalid scalar and trailing-token cases.

### Independent live HTTP evidence — deployment `12cbe8e9`

At `2026-08-18T14:05:41Z–14:05:53Z`, a fresh synthetic organizer (`user 34`)
and workspace (`26`) exercised the promoted component contract. The authenticated
catalog returned three templates, eleven modules and policy
`taawun.artifact/v2`. A two-component `community-iftar` request containing
declared fields plus scalar, array, nested-object and emoji data created `201`
track `track_WwRfj3BvKx7vBvrMWC2xGln5` and artifact
`art_b9ce40591575adb4f1c2a83f48333507`. The exact manifest JSON SHA-256 matched
its receipt, both documents matched the submitted data, verified track reload
returned the same manifest digest, and the manifest-listed component and
aggregate file digests matched their authenticated bytes. Anonymous component
file access returned `401`; bearer access returned `200`, `no-store, private`
and `nosniff`. Preview request ID: `dpdxs39aQFOjdL3wljLL4A`.

Adversarial results were deterministic nested `422 invalid_composition`:
unpaired surrogate `invalid_unicode_scalar`, explicit empty components
`components_required`, duplicate object key `duplicate_key`, reserved authority
key `reserved_key`, exponent number `non_canonical_number`, disallowed control
text `control_character`, and a restrained 9,003-byte request
`document_too_large`. No duplicate or reserved raw value appeared in its error.
The same surrogate denial replayed `422` under request IDs
`mRa2gmO7Q_600q_F9fVATg` and `_UfZ3eDwS9eZ2as5npoFkQ`; correcting that same
idempotency key created `201` track `track_58qfrQdVG9rjBCJGDDCMrOMx` and artifact
`art_df234ad60ddd7d1642f78dd7496a1644`, proving denial did not consume durable
idempotency.

Supported cleanup completed: workspace `26` `204`, user `34` `204`, and the old
token then returned `401`. A later Shura-role harness correctly received the
existing production registration throttle `429` before any account or workspace
was created; QA did not evade the boundary. Implementation separately reports
its live organizer/Viewer smoke cleaned workspace `25` and users `32/33` with
`204/204/204`. Its earlier registration-only allocation is inference-only user
`30`, with no workspace/artifact or recoverable credentials; retain it for
supported application-admin cleanup only. Existing users `8/19` and other
inferred exception fixtures remain under the same authority blocker.

At `14:19:46Z–14:20:08Z`, a corrected two-principal role journey used the
declared invitee email contract. Architect `38` created workspace `28` and a
verified two-component `bazaar-cooperative` preview
(`track_yr1lbtRKjfwGn_5Mg-eLwGz8`,
`art_ae4bbd3b101e154f28772a4adfcf4cd8`) with exact custom emoji data. Member
`39` received `403` for People and verified-track access before invitation,
accepted Viewer membership (`201/200`), then received People/track/component
file `200` with `no-store`/`nosniff` while preview mutation returned `403`.
After supported membership removal `204`, the same principal accepted a
Maintainer invitation (`201/200`) and created a separately verified exact edit
(`track_KA71mFCbqIvAJLAQwjB7H43i`,
`art_9454e9e705c60ed27faf209bf6709d9e`). The Shura/Finance results are recorded
in the closed continuity section above. Synthetic proposal
`proposal_K_OeeNUrSPHAYe9JZT2pdq69`, decision
`decision_39C2qJUa50e6LqMuhBJsHzfG`, and quest
`quest_OoW7kRyOH8Uu7a_cSm8sTdIo` remain as audit evidence; the quest cancel
endpoint returned `200`. Supported cleanup completed for member `39`, workspace
`28`, and Architect `38` with `204/204/204`; both old tokens then returned
`401`.

An earlier harness used username rather than the documented invitee email/user
ID and therefore received invitation `400`; no downstream role conclusion is
drawn from that run. Its users `35/36/37` and workspace `27` all cleaned via
supported routes (`204` each), and all three old tokens returned `401`.

The component release integrated gate passed all Go packages/commands, 13/13
Node tests including two promoted real-Chromium journeys, both production binary
builds, independent backend/UI review and an 8/8 health plus 8/8 root soak. This
is valuable interim evidence, but it is not a substitute for this lane's live
browser gate.

## Market-viability activation and continuity — frozen deployment `12cbe8e9`

The control-room charter changed after the component checkpoint: the external
Codex Browser trust-path failure is no longer an application acceptance blocker.
This pass therefore uses low-rate live HTTP, Railway correlation, current source
contracts and the already-promoted real-Chromium journeys. It claims no new
independent browser, mobile, keyboard or screen-reader evidence.

At `2026-08-19T05:27:06Z–05:27:19Z`, synthetic Architect `43` created workspace
`31`. The live catalog returned all three templates (`bazaar-cooperative`,
`community-iftar`, `community-workspace`) and eleven modules. A customized
two-component `community-iftar` request created verified track
`track_vlF2554GG4nEPjwOKKkNmkiK`; Railway request
`HmEXE6JJRo2RbOZn-_9nXA` was `201` in 133 ms. After discarding that bearer and
signing in afresh, workspace recovery remained `200`, but the dashboard contained
only real `workspace_created` activity and both the natural workspace history
queries `GET /api/conductor/tracks?workspaceId=31&limit=20` and
`GET /api/artifacts?workspaceId=31&limit=20` returned `404`. Supplying the already
known opaque track ID returned the exact verified preview `200`, and its append-only
history returned six events `200` with `no-store` and `nosniff`.

The exact signed documents were then edited and rebuilt into distinct verified
track `track_UloPHy_EDSHZx5OHxQzIuSgE`; Railway request
`Q5WgoNN7RCamvUI6jq4OvQ` was `201` in 57 ms. This proves durable exact reload and
immutable edit work, while also proving there is no discoverable relationship or
collection through which a customer can find either build. Source confirms the
cockpit stores the successful track only in `state.track`, never renders or copies
its random ID, and exposes recovery only through a manual `track_…` input. Local
drafts use principal/workspace/template-scoped `sessionStorage`, so device or
session loss also removes the only automatic draft path.

The role boundary is green. Viewer `44` received known-track `403` before invite,
then invitation `201`, acceptance `200`, known-track inspection `200`, collection
`404`, and build `403`. A role handoff is therefore safe when the opaque ID is
delivered out of band, but it is not a usable product journey. The highest-value
P1 batch sent to implementation is a bounded, deterministic, workspace-authorized
track-summary collection plus honest Build History: visible/copyable ID,
loading/empty/error/retry, verified inspection, build-capable exact draft recovery
into a new actor-bound track, Viewer inspect-only, safe event/failure status and
optimistic resume only where already supported. Summaries must exclude documents,
actor identity, signatures, raw failures and cross-workspace evidence.

Domain evidence has the same recovery gap. Synthetic claim
`TXPXXvmLuGJ2CUmk6zokVB4M` was created `201`, rediscovered through the existing
workspace claim list `200`, and correctly failed verification `422` because QA did
not alter external DNS. Source shows the cockpit never calls the existing list or
publication-history APIs and clears its in-memory claim on reload. The second P1
slice is to rediscover real claims and render pending/verified/revoked/expired,
signed-origin inclusion and exact next action without weakening DNS, review or
Bazaar publication gates.

Railway remained healthy throughout: the post-pass 88-request sample contained
59 `2xx`, 29 intentional/adversarial `4xx`, zero `5xx`, p95 62 ms and p99 133 ms;
no runtime error line appeared. All fixtures from the exploratory and corrected
runs cleaned through supported HTTP: workspace/user `29/40` `204/204`; claim
`8BzlD0ExdTFg95ojzGBdhUVo` `200`, workspace `30` `204`, users `42/41` `204/204`;
and claim `TXPXXvmLuGJ2CUmk6zokVB4M` `200`, member `44` `204`, workspace `31` `204`,
users `44/43` `204/204`. Every retained bearer read back `401`. The workspace-30
membership delete returned `400` because its intentionally invalid invitation had
created no membership; workspace and both users still cleaned successfully.

### Ranked remaining market-viability backlog

1. **Closed P1 — Build discovery and continuity:** deployment `d446f826` adds and
   independently passes the bounded authorized history and safe exact-draft
   recovery described above.
2. **Closed P1 — Domain readiness after reload:** deployment `d446f826`
   rediscovers real claim/publication evidence and passes the independent pending
   claim recovery seam without bypassing external DNS or review.
3. **P2 — Honest activity navigation:** `dashboard/recent` reports only workspace
   creation. After history exists, link to real build/domain activity or retain an
   explicit empty state; never synthesize activity.
4. **P2 / operator-authority dependency — cleanup visibility:** normal Architects
   correctly receive `403` for `/api/admin/users` and `/api/admin/statistics`.
   Existing admin routes provide supported enumeration/status operations, but no
   production application-admin authority is configured for this lane and user
   deletion is not exposed in the admin subrouter. Keep users `8`, `19`, and
   inference-only `30` on the supported-operator ledger. A provisioning/runbook
   decision and an audited supported lifecycle path are required; raw database or
   fabricated JWT access remains prohibited.
5. **External only:** controlled DNS, authorized Bazaar review/purchase fixture,
   federation, TURN and E2EE remain explicit dependencies, not product defects to
   bypass.

### Independent post-deploy acceptance — application `cedacb0`

Railway deployment `d446f826-bb48-4005-afc4-98e7fac8e046` is terminal
`SUCCESS`; image
`sha256:a33cce1d52df957432841f4c071083c943a45f9cc06307be85d49c74b79f7343`.
The docs-only PR head is `cea8fb9` and was not redeployed. The integrated gate
passed every Go package/command test, both production builds, 13/13 Node tests
including two promoted real-Chromium journeys, diff check and independent review.
This lane claims no fresh Codex Browser evidence.

At `2026-08-19T06:44:18Z–06:44:34Z`, an independent synthetic Architect
(`47`), Viewer (`48`) and Maintainer (`49`) exercised workspace `33`. Three exact
two-component previews created verified tracks, in order:

- `track_02T2_UMxCgpYe_UOFufN8xA0`, request `qNxGNtzTRGiRpZdWCYBc-A`,
  Railway `201` / 40 ms;
- `track_36_FZjrgERWYpAx7Dl7QNd30`, request `36AwsIRmSEKEzOmw6WHkDg`,
  Railway `201` / 86 ms;
- `track_qFFA-IV4XJmU9n5im6LDeiLK`, request `9th82coJSDeqY63A6WHkDg`,
  Railway `201` / 56 ms.

After discarding the original bearer and signing in afresh, a two-item history
page returned the third and second tracks, its opaque cursor returned only the
first track, and the combined result had no duplicate or skip. The exact summary
schema contained only `id`, `templateId`, `status`, `version`, `updatedAt`,
`previewPresent`, `artifactPresent`, `publicationPresent`, and
`authorizationExpiresAt`; it contained no actor, documents, signature, raw
failure or other workspace evidence. `limit=500` was safely capped and returned
the three available records. Unknown, duplicate, zero-limit and malformed-cursor
queries each returned deterministic `400`. History responses were `no-store` and
`nosniff`.

The newest history record reopened `200` with active verified evidence; its six
append-only events returned `200`, signed index returned bearer `200`, and
anonymous access returned `401`. Viewer history/track access was `403/403` before
invitation and `200/200` after invitation acceptance; Viewer build and domain
claim listing remained `403/403`. The Maintainer listed and reopened the exact
track `200/200`, received identical generic `403` envelopes for correct and
guessed resume versions of the Architect-owned track, then edited the recovered
documents into new actor-bound track `track_OkGqv3z3oqMvOn7kZaWmdigk`. Preview
request `1326VEDxQwi8ZVQ1WUN5dQ` was Railway `201` / 44 ms, the new track was
distinct from all Architect tracks, and it appeared in the authorized workspace
history.

Pending domain claim `Xc1ssxQfnqlI5rNKT7lchy8r` was created `201`, rediscovered
after another fresh login through the real claim list `200`, and had an honest
empty publication history `200`. Verification without external DNS remained
`422`; QA did not create, alter or bypass DNS. Live activation and Bazaar purchase
remain external-fixture gaps. Promoted Chromium/source coverage supplies the
non-live UI evidence for honest loading/empty/error/retry, exact draft recovery,
stale workspace/principal/track suppression, activation-only retry, 320/400 and
effective 200% reflow, keyboard semantics, and the complete pre-commit binding of
track workspace/artifact/preview/origins/expiry/subject to the attested manifest.

Railway correlation after both implementation and independent smokes sampled 93
requests: 71 `2xx`, 22 intentional `4xx`, zero `5xx`, p95 66 ms and p99 88 ms.
CPU peaked at 0.011 vCPU and memory at 0.0516 GB; bounded runtime output showed
one clean start and accepted auth events, with no panic, exit or application error.
Supported cleanup completed: claim delete `200`, member deletes `48/49` `204/204`,
workspace `33` `204`, users `48/49/47` `204/204/204`, and all three old tokens
returned `401`. The earlier implementation smoke independently cleaned users
`45/46` and workspace `32` with `204/204/204` and both old tokens `401`.

**Verdict:** ACCEPT for application correctness and private-beta activation /
continuity on `cedacb0` / `d446f826`. No P0/P1 application finding remains from
this batch. P2 operator-authority cleanup visibility and the explicit external
DNS/Bazaar/federation/TURN/E2EE dependencies remain documented; users `8`, `19`
and inference-only `30` must still be handled only through supported authorized
operator lifecycle paths.

## Signed Starter Path — deployment `b2fa4543`

Application source `f3cc84d` (feature `4064cf9`) was deployed as Railway
`b2fa4543-1627-4d08-b68a-5416f1e0006f`, image
`sha256:92d647c6932534d774ae3baf9bba7e80799844b469b237a0a82b335e24f9d9c2`.
The docs-only PR head was `1e050d2`; rollback remained
`cedacb0` / `d446f826-bb48-4005-afc4-98e7fac8e046`. The release handoff reported
all Go packages/commands and both production builds green, 13/13 Node tests
including two promoted real-Chromium journeys, diff check and independent review.
This lane independently reran all Go package/command tests successfully. Its local
Chromium and Edge launches both aborted before `Page.enable` because the Windows
GPU cache process could not acquire its host files; this is a Codex/host tooling
failure, not an application assertion. No fresh browser, keyboard, reflow or
screen-reader credit is claimed; the promoted real-Chromium evidence remains the
only UI evidence in this checkpoint.

Source and promoted-test review confirmed the intended ephemeral starter model:
no-workspace, history-loading, history-error, authorized zero-history and
non-empty states are distinct; the starter appears only after a successful
authorized zero-row history response; all three templates and eleven modules stay
explicit; a meaningful non-default document edit is required; preview verification
and history confirmation are separate; history failure retains the trusted preview
with bounded retry; fresh login skips an inapplicable newer draft-only row and
reopens one applicable verified track without events or per-row requests; People,
history and domain failures are independent; and principal/workspace resets clear
ephemeral invitation and completion state. The promoted organizer path used seven
counted activations. The five-person comprehension study remains pending genuine
private-beta participants and is not represented as a technical gate.

At `2026-08-19T08:46:05Z–08:46:10Z`, independent Architect `63` created workspace
`39`, received the exact three-template/eleven-module catalog and submitted a
meaningfully customized two-component `community-iftar` document containing
Unicode scalar, array and object data. Railway request
`xp-pekGjSjavgR_F9fVATg` returned preview `201` in 57 ms, creating track
`track_zIgDbxDKc-sAjZ0826Ec1gQC` and artifact
`art_33e45b6be623fba9bfabe79c673fda44`. The exact `manifestJson` digest, full
manifest/track/workspace/actor/component/signature/key/origin/lifecycle/expiry
relationships and every bearer-protected manifest file byte/digest matched.
Anonymous signed-file access remained `401`, and the signed document was the
styled card rather than raw Datastar source.

That positive path exposed one release-blocking P1 trust defect. The signed
manifest and authenticated artifact runtime require SHA-256
`2837d87acf6ee0ba8e4e63765926c25a98d63883b02f88be194a86b81d3fd24a`
over 34,083 LF-normalized bytes. The cockpit instead fetched and executed public
`/assets/datastar-v1.0.2.js`, whose deployed raw response was 34,092 bytes with
SHA-256
`ad76a361fa0ba5dcda7478dc20925ee4cf397dca0619fae0ac848613f31ce662`
and nine CRLF pairs. LF normalization yields the declared signed digest, but the
cockpit does not perform that normalization or any WebCrypto comparison: its
`loadPreviewFrame` fetches the public URL, reads text and inlines it before
committing the iframe and verified receipt. The signed-bundle loader normalizes
only the authenticated artifact copy, while the public `web.FS` file server emits
the raw checkout bytes. Active **Verified** can therefore accompany executed
runtime bytes outside the exact signed file.

The coherent P1 batch sent to implementation requires the cockpit to load the
actor-authorized signed runtime for that track and verify its raw bytes against
the exact manifest/file digest before committing iframe or receipt. Missing,
mismatched, delayed or stale runtime must preserve the prior trusted preview,
show verification unavailable/stale and keep publication disabled. Required
regression coverage includes a corrupt/public-mismatch response, old
workspace/principal response suppression, exact one-inline-runtime assembly,
the `allow-scripts allow-forms` sandbox, control interaction and signed-file
anonymous `401` / bearer `200`. Live retest must prove the bytes actually executed
equal the declared `2837…d24a` digest.

Railway remained healthy during this bounded pass: both preview requests sampled
were `201` (`wh9CgkzITdq4HTy79fVATg` 58 ms and
`BT3XaiFkR3Gz1qkq2h0iww` 46 ms), the defect request above was `201` / 57 ms,
the 78-request sample contained 67 `2xx`, eleven intentional `4xx` and zero
`5xx`, and bounded runtime output showed one clean start with no panic or exit.
Exploratory fixtures were cleaned only through supported routes: users
`52–54`, `55–57`, `58–60`, `61–62` and workspaces `36–38` were deleted with
successful `204` lifecycle responses where allocated, and retained tokens read
back `401`. The final defect fixture workspace `39` and users `63/64` returned
`204/204/204`; both tokens returned `401`. No fixture from this pass remains.

**Verdict:** BLOCKED for Signed Starter Path release acceptance on `f3cc84d` /
`b2fa4543` by the single P1 signed-runtime execution mismatch. Testing stopped at
the trust boundary instead of generating further role fixtures. No other P0/P1
application finding is asserted from this incomplete matrix, and no fresh Browser
credit is claimed. Retest must start from a new verified deployment, close this
runtime binding, then complete organizer history confirmation/fresh-login
derivation, Viewer inspection/build denial, Maintainer actor-bound clone,
independent loader recovery, stale response suppression and non-empty-workspace
regression before acceptance.

## Signed runtime repair — deployment `431bc9b3`

Application commit `c1e4aa5` was deployed as Railway
`431bc9b3-21b2-4e12-a4e1-c2a4a1fbfc39`, image
`sha256:d79c8682c0ca46a3a834901f4fce4083ccf67a63854b689adaac6333c96797a1`.
The final docs-only PR head was `8755f19f6c5f988077cf18a5ec9cbcd213906984`
and was not redeployed. Rollback remained `cedacb0` /
`d446f826-bb48-4005-afc4-98e7fac8e046`. The implementation gate passed all Go
packages/commands, both production builds, 13/13 Node tests including two
promoted real-Chromium journeys, diff integrity and independent security/source
review. Promotion then passed 8/8 health and 8/8 root probes, with the cockpit
title present and the redundant public runtime import absent.

The repaired cockpit binds `index.html`, `theme.css`, `app.css` and the bundled
runtime to the exact selected track file route, fetches all four with the current
Bearer token and `cache: no-store`, then checks raw byte count, SHA-256 and fatal
UTF-8 decoding before committing either receipt or iframe. Manifest descriptors
must be unique, traversal-free and internally bind the runtime/theme; supplied
preview URLs must match the current track exactly and may not add query or
fragment data. The signed document must still name the portable public runtime
requirement, but the cockpit replaces that tag using only separately verified
track-scoped bytes. The parent cockpit's former incidental App-name behavior is
now explicit vanilla DOM synchronization, so it no longer needs the unsigned
public runtime.

Promoted real-Chromium coverage mirrors the live failure with a canonical LF
signed runtime and byte-different CRLF public asset, and proves the public asset
is never requested. It rejects missing runtime, duplicate/traversal descriptors,
runtime/manifest mismatch, wrong-track and query substitution, signed-runtime
`404`, same-length SHA mutations of document/theme/styles/runtime, length drift
and delayed draft/workspace/principal responses. Every failure retains the prior
trusted iframe, receipt and raw manifest, marks it stale, disables publication and
recovers only through a fresh exact reopen. The same promoted gate preserves one
intact inline runtime, `allow-scripts allow-forms`, control interaction, seven
counted starter activations, explicit three-template choice, history-confirmation
retry, fresh-login one-track derivation, independent People/history/domain
failure recovery, Viewer read-only behavior, Maintainer actor-bound clone,
44-pixel targets and 320/400/effective-200% reflow. This lane claims no fresh
Browser, keyboard, mobile or screen-reader evidence.

The final independent live pass ran at
`2026-08-19T09:42:46Z–09:42:51Z`. Architect `70` and collaborator `71` created
workspace `44`. The initial workspace collection was empty; authorized history
returned a distinct zero-row `200` with `no-store` and `nosniff`; and the real
catalog returned all three templates and eleven modules. A meaningfully changed
two-component `community-iftar` request included custom Unicode scalar, array and
object data. Railway request `5t2zoqv0SoGdI7gzwUFZXw` returned `201` in 68 ms,
creating track `track_RtmPnvC7WfQO8M5L8wee6P3J` and artifact
`art_1de810d14ad154cc7ea45a7126b0c178`.

The exact `manifestJson` digest, full manifest equality, workspace/actor,
component documents, signature/key, origins, preview lifecycle and future expiry
all matched. Raw bytes and descriptors matched for all four executed files. The
authenticated signed runtime was exactly 34,083 bytes with SHA-256
`2837d87acf6ee0ba8e4e63765926c25a98d63883b02f88be194a86b81d3fd24a`
(`bNbVbX4iRTmxRfGWwUFZXw`, `200`), while anonymous access was `401`. The legacy
public runtime remained 34,092 CRLF bytes with SHA-256
`ad76a361fa0ba5dcda7478dc20925ee4cf397dca0619fae0ac848613f31ce662`,
but live root source contained no public runtime import and did contain the exact
track-scoped signed-file/digest/no-store path.

History then confirmed exactly one preview track. A fresh login listed history
once and reopened that one track with `includeVerifiedPreview=true`; the manifest
digest and exact documents were unchanged. Collaborator history and known-track
reads were generic `403/403` before invitation. After an Architect-created,
session-delivered Viewer invitation was accepted, Viewer history, exact track and
signed runtime reads returned `200`, while build remained `403`. The Viewer was
removed through the supported membership route, reinvited as Maintainer and
accepted. Maintainer inspected the source track, copied only curated draft fields
without track/creator/signer/lifecycle/idempotency authority, changed a component
document and created distinct actor-bound track
`track_kMLrOUYy1AyOfGO1R5AO9pk1`; Railway request
`p2ujKQv4QVCgMQwbjq4OvQ` returned `201` in 61 ms. Workspace history then contained
both real tracks.

One earlier independent pass on users `68/69` and workspace `43` replayed the
source request's persisted idempotency key and correctly received deterministic
`409 composition_conflict`. Source confirmed the cockpit's curated draft clone
does not copy that field, so this is harness-only evidence, not an application
finding. That pass still completed every runtime/Viewer assertion, then cleaned
membership, workspace and both users with `204`; both tokens returned `401`.
The corrected pass likewise deleted membership `71`, workspace `44`, users
`71/70` with `204`, and both final tokens returned `401`. No independent fixture
from either pass remains.

Railway correlation found both exact preview requests above on deployment
`431bc9b3`; the final 143-request bounded sample contained 124 `2xx`, nineteen
intentional `4xx`, zero `5xx`, p95 61 ms, p99 82 ms and max 92 ms. CPU averaged
0.0004 vCPU (max 0.0072) and memory averaged 0.0241 GB (max 0.0410). Bounded
runtime searches contained one clean start and no panic, fatal or error record.

**Verdict:** ACCEPT for the Signed Starter Path technical checkpoint on `c1e4aa5`
/ `431bc9b3`. The runtime trust P1 is closed; no P0/P1 application finding remains
from the completed API/source/promoted-Chromium matrix. Five-person comprehension
testing remains pending genuine private-beta participants and is not fabricated
or treated as a deployment gate. Fresh Browser/accessibility credit remains
explicitly unclaimed. External DNS/Bazaar/reviewer fixtures,
federation/TURN/E2EE, and supported application-admin cleanup for users `8`, `19`
and inference-only `30` remain the previously documented dependencies; no raw
database or fabricated authority was used.

## Acceptance rule

Under the current control-room charter, sign-off requires zero open P0/P1 findings,
green deployment and promoted real-Chromium tests, independent low-rate live API /
Railway verification, a final comprehensive pass with no new failures, supported
cleanup evidence, and explicit limitation of remaining gaps to authorized operator
provisioning, external DNS/Bazaar fixtures or the documented federation/TURN/E2EE
deferrals. Fresh independent browser evidence remains valuable but is not claimed
and is not a release blocker while the Codex trust-path defect persists.

## Fresh in-app Browser acceptance and cockpit gap audit — in progress

On `2026-08-21`, the independent lane resumed with the healthy bundled in-app
Browser `26.818.21641` against accepted app `c1e4aa5` / Railway deployment
`431bc9b3-21b2-4e12-a4e1-c2a4a1fbfc39`. This is fresh live visual and
interaction evidence rather than promoted-test credit. The public root showed the
concrete workspace-signed/local-first value statement and the accurate central
identity/artifact/control-record, encrypted-relay, zero-custody and no-settlement
boundaries. A synthetic first principal received an honest no-workspace state,
then a successful authorized zero-history state with all three real templates and
eleven catalog modules.

That pass found one live P1. After workspace A `45` explicitly selected
`community-iftar`, creating workspace B `46` through the cockpit selected B but
left A's template milestone visible: B had honest zero history while the Signed
Starter Path falsely reported `1 of 5` and “Community iftar selected explicitly.”
Railway recorded B's workspace creation as `201` at
`2026-08-21T04:32:57.165129725Z` in 28 ms. No component document or server
authorization leak was observed. The coherent state-isolation batch was sent to
implementation and the cross-workspace starter area remains paused pending a new
deployment; unrelated evidence below does not accept that gate.

Unrelated live paths remained green. Viewer `73` had no pre-invite workspace,
accepted an Architect-issued session-only invitation, saw only authorized real
People data with no emails, and remained read-only across build, Shura and Finance.
Maintainer `74` likewise had no pre-invite workspace, then received build/propose/
vote capability without invite/decide/publish authority. Maintainer created and
approved a synthetic proposal; Architect `72` recorded final decision
`decision_RPUXPNfm5GNyp05YKta9E4pV`, which safely prefilled Finance. Synthetic
quest `quest_vr4LDIfc4pLRkrXPCNjl7Veo` advanced only through valid sandbox states
and ended `CANCELLED`; every visible state said no funds, custody, balance or
settlement. Bazaar showed a truthful published-only empty state and named the
controlled-DNS/reviewer fixture dependencies without fabricating a listing.

A pending synthetic domain claim was rediscovered after sign-out/fresh login,
did not reconstruct its TXT proof, kept publication disabled, and returned the
actionable missing-TXT recovery message. Exact production-origin CORS preflight
returned `200` with the exact allow-origin header
(`_XIicTjaQU6qrUB_9fVATg`); a disallowed-origin preflight returned `200` without
an allow-origin header (`_kGg_Xo7Sii6dPVqnpoFkQ`). OAuth authorization/resource
metadata returned `200`; unauthenticated `/mcp` returned a pinned `401` Bearer
challenge with protected-resource metadata and `nosniff`; unauthenticated ethics
audit returned `401`. The bounded Railway `>=500` query remained empty.

The baseline fixture was then removed through supported routes only. Viewer and
Maintainer membership removal returned `204/204`; pending claim
`E96p6TYJP0bGEMk7KawVJch5` revoke returned `200`; workspace deletes for `45/46`
returned `204/204`; self-deletes for users `73/74/72` returned `204/204/204`; and
all three retained JWTs read back `401`. The cancelled quest remains immutable
audit evidence only. No DNS, Bazaar, reviewer, infrastructure, database or admin
authority was changed.

A later isolated gap-audit fixture exposed a separate P1 during cleanup. User
`75` created sole workspace `47`, then self-delete returned `204` before any
workspace delete reached the service; the old token returned `401`. Railway's
bounded DELETE log contains the user deletion at
`2026-08-21T05:09:34.454576854Z` and no `DELETE /api/workspaces/47`. A separate
self-cleaning outsider `76` received `403`, not `404`, from
`GET /api/workspaces/47`, proving that the workspace still exists while remaining
isolated. User `76` then self-deleted `204` and its old token returned `401`.
Because user `75` was tombstoned and no application admin is configured,
workspace `47` is now explicit operator-authority-blocked residue; no raw DB or
fabricated JWT cleanup will be attempted.

The expanded cockpit audit records the following current gaps separately from the
active P1:

| Rank | Planned/current/evidence | Impact | Existing-primitive fit |
| --- | --- | --- | --- |
| **P1 confirmed** | `DELETE /api/users/75` returned `204` while the account still owned workspace `47`; the only owner token became `401`, no workspace DELETE occurred, and an outsider read proved the workspace remains (`403`, not `404`). | Self-service account deletion can strand an owned workspace with no active supported cleanup authority. | Fail closed with a deterministic `409 owned_workspaces_remaining` until supported workspace cleanup, or implement one documented atomic workspace disposition that preserves required audit records; prove no partial tombstone. |
| P1 candidate | Approved-domain revocation is promised and the authenticated `DELETE /api/workspaces/{id}/domains/{claim_id}` route exists, but the live panel exposes claim/rotate/verify/reload only. | An Architect cannot visibly terminate a compromised or obsolete delivery origin. | Add confirmed Architect-only revoke, conflict-safe readback, retained history and fail-closed workspace/principal guards. |
| P1 candidate | Live People exposes real members plus create/accept invitation, while mounted membership-remove and invitation-revoke routes are unreachable. Pending invitation records intentionally disappear after reload. | An Architect cannot offboard a member or revoke even a current-session grant from the cockpit. | Add Architect-only member removal and current-session invitation revoke; keep the after-reload limitation honest unless a bounded existing-record list is approved. |
| P1 market-continuity candidate | Shura says “Paste a real proposal ID” and Finance requires an “Existing quest ID”; neither durable subsystem has a bounded workspace list route or UI. | Fresh-login/device loss makes governance and sandbox work commercially undiscoverable and weakens Shura-to-Finance continuity. | Add bounded redacted workspace summaries and role-safe inspect/recovery, mirroring accepted Build History without exposing capability tokens or actor identity. |
| P1 support/privacy candidate | The authenticated cockpit exposes Sign out only although supported self password-rotation/tombstoning and workspace deletion routes are mounted. | Customers cannot rotate all sessions or perform supported account/workspace cleanup without API/operator help. | Add a deliberately separated account/workspace lifecycle surface with confirmation, immediate invalidation and old-token `401` readback. |
| P2 | The core hosted MCP/OAuth control plane is invisible in the live cockpit: no MCP/OAuth text or link exists although metadata and `/mcp` are mounted. | A private-beta operator cannot discover how the LLM-facing differentiator is used. | Add honest read-only connection guidance using existing metadata; keep external-client proof explicitly pending. |
| P2 accessibility | Live target measurements found logout 27 px, auth tabs 38 px, tertiary/domain buttons 28.8 px, details summaries 33.2 px and the color control 42 px despite the explicit 44 px acceptance criterion. | Keyboard/touch usability and the stated accessibility gate are inconsistent with production CSS. | Raise the existing target tokens/hit areas without changing the Swiss visual hierarchy; retest desktop and reflow. |
| P2 accessibility | Applying a valid advanced component document rebuilt the editor and left `*:focus` empty; Add/Remove custom field use the same rerender path. Invalid trailing JSON and reserved `workspaceId` correctly failed closed and retained the last valid document. | A keyboard user loses their place after a successful edit even though data safety is correct. | Restore focus to the equivalent surviving control, or an announced component heading when the control is removed; cover Apply/Add/Remove and template-switch Apply/Cancel. |

Publication-lifetime acceptance remains fixture-free until the dedicated
deployment handoff. The approved contract deliberately separates an ordinary
source-linked publication-history row from the one currently selected active
artifact. Only that active, exact row may trigger full artifact verification and
the computed `manifestDigest`; ordinary source-linked rows remain lightweight,
while an inactive row is exact-selected only when an operator explicitly asks to
verify its artifact. This prevents list-time N+1 artifact reads and avoids
presenting historical linkage as fresh cryptographic verification.

Duplicate source links are valid publication history, not corruption. Reverse
publication-to-track binding is therefore populated only when exactly one
candidate survives authorization and exact validation; zero or multiple
candidates return `null` without disclosing whether an inaccessible candidate
exists. The ambiguous cockpit state must offer no one-click replacement and must
direct an authorized operator to exact verified Build History or a new signed
preview. Acceptance includes duplicate candidates, authorized ambiguity,
cross-workspace/no-oracle reads and the crash window between immutable replacement
creation and active-selection reconciliation.

The expiry boundary also separates recorded activation from current serving
truth. At and after the signed authorization expiry, the API retains the audit
fact `active=true` but must return `servingState=expired`; Host must not serve the
old artifact, and the cockpit must never describe that row or receipt as Active
or Verified. Controlled-clock acceptance will test expiry minus one millisecond,
the exact boundary and plus one millisecond, along with successor activation,
revoked-domain denial, concurrency/idempotency, cache/CSP headers and fail-closed
browser reconciliation. No TTL, DNS, Bazaar or reviewer fixture is authorized
before the dedicated deployment and control-room retest handoff.

The authoritative completeness audit preserving both the self-delete lifecycle
serialization and publication-lifetime charters, including duplicate-publication
ambiguity and null/no-oracle handling, is SHA-256
`C3C60EAF4DA7CD92A1D73BE748C36CFB719F990449F57ADB193E4F93FBBBF95A`.
All subsequent deployment and ledger handoffs must cite that value; earlier
completeness-audit hashes are superseded.

### Workspace-scoped cockpit state deployment retest — ACCEPT

The bounded workspace-state repair is live at application commit
`6c7a973d0619243784488bee0d9ee388a39c3c05`, Railway deployment
`bb2b3f31-7ebe-43f2-9b21-db553bc4e581` (`SUCCESS`), image
`sha256:c9961684da740845e1d0202fdc083c54eb81956fe1f0b727e621b67d453c9fd5`
and docs-only PR head `de3f96dd4e07f1b3e19b5a83364f24d3b43fc9b3`.
The integrated handoff reports all Go packages and production builds green,
13/13 promoted real-Chromium journeys, independent UI/security approval, a
16/16 health/root soak and zero bounded `5xx`.

Fresh in-app Browser `26.818.21641` evidence used a new synthetic organizer and
workspaces A `48` and B `49`. A explicitly selected `community-iftar`, selected
two allowed components and changed the Iftar summary to exact Arabic/emoji/plain
Unicode. Creating B through the cockpit selected B with no inherited template,
component, document, track, receipt, invitation or publication state; the guide
correctly showed `0 of 5`. Returning to A restored only A's scoped document but
still showed `0 of 5` and required three intentional confirmations for the
restored template, components and customization. A rapid A→B→A→B sequence ended
on a still-blank B with no stale A document or trusted receipt.

After explicit reconfirmation, A created track
`track_06we_ONN7uhO0pfdcbIyYNx9` and artifact
`art_9c7b1f10ce0766800efabf0b6191ab64`. Railway recorded the preview `201`
at `2026-08-21T05:47:55.342980015Z` in 48 ms. The live iframe began with the
styled signed card and exact Unicode content, not runtime source; its sandbox was
exactly `allow-scripts allow-forms`. The visible receipt said the Taawun build
service verified an active signature, bound workspace `48`, exact origin,
component-document digests and future preview expiry, and separate Build History
confirmed exactly one real track while stating it was not published.

With B selected, supported API deletion returned `204` under Railway request
`CNBS3v-nQ6WcCOten6XIxQ` at `2026-08-21T05:49:52.719727921Z` in 5 ms. A reload
correctly cleared the tab session; fresh login listed only A, automatically
reopened the one applicable verified track, retained the exact document and
receipt, and did not restore B. A separately registered principal received the
honest no-workspace state with no A/B name, document, track, receipt or milestone.
Promoted Chromium remains the evidence for artificially delayed create and
People/history/domain/track/invite/accept/publication response interleavings;
the live rapid-switch, deletion fallback and principal reset all failed closed.

A bounded diagnostic series then reached the documented login limiter and
returned `429` with safe copy. Further authentication requests were stopped; no
limiter bypass was attempted. After the full window, a single supported cleanup
pass reopened the exact track at `200` under Railway request
`loqiShjRSHORpR8InpoFkQ`, recomputed the exact `manifestJson` SHA-256 and matched
`manifestDigest`. The signed runtime descriptor resolved to
`assets/datastar-v1.0.2.js`; authenticated read returned `200`, 34,083 bytes,
`private, no-store` and `nosniff`, while anonymous read returned `401`.

Workspace A `48` deletion returned `204` under request
`O_z_XMaHQtKItOj-nPRhug` at `2026-08-21T06:04:50.597995278Z` in 3 ms and an
independent authorized principal then read it as `404`. Self-delete for organizer
`77` returned `204` under `2VQGEiLUR7W7IKhaLPU1MQ` in 76 ms; zero-workspace
principal `78` returned `204` under `zJPhRtudTV-jIbymLPU1MQ` in 48 ms; both old
tokens returned `401`. Workspace B `49` had already been deleted `204`. No fixture
from this retest remains. Railway correlation shows one clean start and no HTTP
`>=500` throughout the deployment window.

**Verdict:** ACCEPT the workspace-scoped cockpit-state P1 on `6c7a973d` /
`bb2b3f31`. The exact live A→B blank boundary, intentional A recovery, rapid
switch, deletion fallback, fresh-login signed-history recovery, new-principal
isolation, signed runtime/file boundary and supported cleanup are green. Promoted
Chromium supplies deterministic delayed-response coverage. No P0/P1 application
finding remains in this gate; the separately sequenced self-delete orphan P1 was
left open on that release and is evaluated immediately below. Workspace `47`
remains explicit operator-authority-blocked historical residue.

### Next gate: Decision-A self-delete serialization — fixture-free charter

The control room released the database-backed lifecycle-serialization lane only
after the workspace-state acceptance above. No production load race is
authorized. Repository acceptance must deterministically force both interleavings
using independent handles to the same SQLite file and an independent-process
`BEGIN IMMEDIATE` exercise: create-wins commits the owned workspace and makes
self-delete return `409 owned_workspaces_remaining` with the user and token still
valid; delete-wins commits the tombstone first and makes workspace creation fail
with no workspace or membership row while the old token reads `401`. Cancellation
and storage failure must roll back without partial lifecycle state.

After a verified deployment handoff, the live lane will run low-rate sequential
checks only. Fresh identity A will create one workspace, receive the nested safe
`409 owned_workspaces_remaining` envelope on self-delete, retain an unchanged
`200` profile, valid token and owned-workspace listing, then delete the workspace
`204`, self-delete `204`, and prove old profile and workspace-creation authority
both return `401`. Fresh zero-workspace identity B will self-delete `204` and
prove the same old-authority `401` boundaries. A distinct principal attempting to
delete either user must receive a generic `403` without an existence or ownership
oracle. A bounded domain/OAuth/reference case will confirm the precondition does
not corrupt audit/control references and that final supported cleanup revokes old
JWT/OAuth authority without raw database errors.

Every live mutation will retain only synthetic IDs, statuses, timestamps and
Railway request IDs, then correlate exact app/edge records and a bounded zero-5xx
readback. Workspace `47` remains a historical operator-authority-blocked residue;
no raw database, fabricated JWT, bootstrap-admin or production concurrency action
will be attempted. Publication-lifetime and UI-safety fixtures remain paused until
this gate closes.

### Decision-A self-delete deployment retest — ACCEPT

The exact application and PR commit
`b044365357664c3bb3d1654600f409ff74b8ef69` is live on the operationally current
Railway deployment `d60347f7-5dd6-4778-b3e2-bb3d088ff8e3` (`SUCCESS`), image
`sha256:6472195b6b2fe296473fd0129126b6b94c92dccb303eb0d208219b3eab2befd1`.
Control-room attribution confirmed that this was an app-identical second release
of `b044365`; it removed the earlier identical deployment `96e7fa37`. Direct
`/api/health` and root probes returned `200` before fixture creation, with health
request `O7FJsBI7Q62MidP0xtoGcA`. The integrated gate supplied the required
same-file independent-handle, independent-process `BEGIN IMMEDIATE`, forced
create-wins/delete-wins and rollback/cancellation/COMMIT-failure tests; no live
production race or load was attempted.

Fresh owner `79` created workspace `50` and a pending synthetic
`.example.invalid` domain claim without DNS verification or publication. The
claim returned `201` under `aVjeyKa1TUOIvyowWUN5dQ`, exercising the domain/audit
reference that had previously caused raw FK deletion failures. Distinct
zero-workspace principal `80` received generic `403` for both the owner-account
delete (`dNI-7eQTSpKV9-yMYqVb7A`, 3 ms) and workspace delete
(`L6nUtsPdSaOETcNZYqVb7A`, 3 ms), with no owned-workspace code, name, count or
lock oracle.

Owner self-delete returned `409` under `mX5n1NTSSkC85V-Ilt7tkg` in 49 ms. Its
129-byte response was exactly one nested `error` object with only `code` and
`message`; the code was `owned_workspaces_remaining`, the message was the bounded
deletion prerequisite, and the body contained no workspace name/ID/count, domain,
database cause or raw constraint text. Headers were
`application/json; charset=utf-8`, `Cache-Control: no-store`,
`X-Content-Type-Options: nosniff`, plus the Railway request ID. The unchanged JWT
then read the exact same profile `200` (`vW4tSM0BQB-R1-erYqVb7A`, 2 ms) and the
exact one-workspace collection `200` (`weUIa8-qRLSQqUbsYqVb7A`, 3 ms), proving
the conflict neither tombstoned nor rotated authority.

Supported workspace cleanup returned `204` under
`8daGPi5zRzykt0uBnPRhug` in 25 ms and the distinct principal then read workspace
`50` as `404` under `2OqM5SXDQaO6EitpYqVb7A`. Owner self-delete subsequently
returned bodyless `204`, `no-store` and `nosniff` under
`lt2CkUvlSEukwJ6Llt7tkg` in 50 ms. Its old JWT returned `401` for profile
(`zaXxCcQ5SpOSrZCMYqVb7A`) and workspace creation
(`Z5lS__HETrCchV_NwUFZXw`). Zero-owned principal `80` independently self-deleted
with the same bodyless/header contract under `6RPeEeq9TiaoS7CnYqVb7A` in 49 ms;
its old profile and workspace-create requests returned `401` under
`Tv83aGExSZ2l9qBWnPRhug` and `URYpEWQ1TFKwHxOsnPRhug`. OAuth metadata remained
`200`; the integrated lifecycle suite supplies OAuth-session/consent retention
coverage without creating a broader live OAuth authority fixture.

One harness summary flag initially printed `false` despite every named assertion
above being green. A local no-network reproduction proved PowerShell parsed the
comma-separated `@(expr, expr, ...)` aggregate as one chained comparison; this
was test arithmetic, not a second application run or defect. No repeat fixture
was created. Exact Railway readback confirmed every status/timing above and found
zero deployment-scoped HTTP `>=500` records.

**Verdict:** ACCEPT Decision-A lifecycle-safe self-delete on `b044365` /
`d60347f7`. Both fresh identities and workspace `50` were removed exclusively
through supported routes, both old authorities are `401`, the pending domain
reference caused no FK/raw-error regression, and no QA residue remains.
Workspace `47` remains the explicitly preserved pre-fix operator-authority
cleanup blocker. Publication-lifetime and UI-safety fixtures were not started.

### Publication-lifetime deployment retest — ACCEPT

The exact application and PR commit
`928c8f274460f00b4ac2059a0e7c43fb221d5791` is live on Railway deployment
`43f11880-806a-4719-84b3-d6587403ec2c` (`SUCCESS`). Railway returned one clean
application start at `2026-08-21T15:16:14Z`, and the final deployment list still
showed this release as the sole current `SUCCESS`. The connector did not expose
the runtime image digest, so build digest
`sha256:2719c97dc903ceafe5c568a55a8125f71f4fd8875e74d42bdf1b462b109c5c9a`
is recorded only as a build digest and is not represented as the running image.
The accepted rollback source remains `b044365357664c3bb3d1654600f409ff74b8ef69`;
its operational deployment `d60347f7-5dd6-4778-b3e2-bb3d088ff8e3` was removed
by promotion. The authoritative completeness audit is unchanged at SHA-256
`C3C60EAF4DA7CD92A1D73BE748C36CFB719F990449F57ADB193E4F93FBBBF95A`.

The integrated release handoff reported every Go package and command, both
production binaries, diff check, independent backend/security/UI reviews and
14/14 Node tests green, including the two promoted real-Chromium journeys.
Source readback on the exact commit confirmed controlled-clock tests for the
injected server clock, exact stored manifest bytes, expiry minus one millisecond,
the exact expiry boundary and plus one millisecond; Host `GET`/`HEAD`; revoked
claim and artifact failure; atomic successor activation; idempotency/concurrency;
crash-window recovery; non-unique exact publication binding; immutable
replacement/rollback lineage; and one-set-query bounded publication context.
The promoted browser test additionally proves the monotonic expiry deadline
removes all Active/Verified/healthy/live wording and interactivity before an
authoritative exact re-read, source-linked replacement and rollback, null or
ambiguous binding guidance, stale claim/principal responses, trusted keyboard
activation, at-least-44-pixel publication action targets and 160/200/320/400-pixel
reflow. These controlled tests are the temporal and activated-publication proof;
no live DNS, reviewer, Bazaar or shortened-TTL fixture was fabricated.

Fresh live in-app Browser evidence used synthetic Architect `82` and workspace
`51`. The honest zero-workspace path preceded supported workspace creation. The
Architect explicitly selected `community-iftar`, chose two allowed components,
changed the Iftar title, and created signed track
`track_ae3I_veiZ1kNT52Vq6K5jzVu` / artifact
`art_f1c167a686c1d81e054f68ad5f1477f3`. Railway recorded the preview `201` at
`2026-08-21T15:28:42.404381798Z` in 91 ms. The cockpit independently confirmed
the track in Build History, rendered the styled card rather than runtime source,
showed the exact workspace/component/origin/lifecycle/signature receipt, and
kept publication disabled. The iframe sandbox was exactly
`allow-scripts allow-forms`. The one executed inline module was 34,083 bytes and
its SHA-256 was exactly
`2837d87acf6ee0ba8e4e63765926c25a98d63883b02f88be194a86b81d3fd24a`,
matching the signed manifest runtime descriptor and the authenticated immutable
file bytes. Authenticated runtime and `index.html` reads returned `200`,
`private, no-store`, `nosniff` and exact manifest file digests; anonymous reads
returned `401`.

Claim `jP3HKAWG6_P0iqyBzHpRvNDJ` used one exact synthetic
`.example.invalid` origin. Its DNS proof value was displayed only in-session and
was never copied into this ledger or a report. The claim remained honestly
`pending`; the cockpit said external DNS verification was required, synthesized
no proof, loaded zero immutable activation records, hid exact-record,
replacement and rollback controls, and kept Publish disabled. A direct domain
publication attempt returned `409 origin_not_verified`. Conductor publication
request returned nested `409 publication_claim_unavailable`, `no-store` and
`nosniff`; before and after reads stayed exactly `PREVIEW_READY`, version `6`,
so denial persisted no stale transition. Railway correlated the request at
`2026-08-21T15:31:04.200996760Z` in 5 ms.

The live publication-context API returned `200` with `no-store`, `nosniff`, an
empty `publications` collection and server time for both default limit `20` and
explicit limit `50`. Exact selection of a missing authorized publication
returned the same bounded empty collection. Limit `51`, duplicate `limit`, and
mixed exact-selection/paging returned deterministic `400
invalid_publication_query`. Distinct outsider `83` received the same generic
`403 workspace_forbidden` for actual and missing exact-publication selectors and
for the claim list, preventing a claim/publication/version oracle; it then
self-deleted through the supported route and its old profile returned `401`.
Railway recorded the complete history series as `200`/`400`/`403` in 2–26 ms and
no `5xx`.

Invited Viewer `84` proved pre-invite `403`, accepted Viewer membership, listed
and reopened the exact signed history and preview, but had disabled composer and
domain controls, hidden replacement/rollback actions, no publication-context
authority and a disabled Publish action. Invited Maintainer `85` had build but
not publish authority, restored the exact documents into a new local draft,
changed one title and created actor-bound successor track
`track_cB_X7DUbiHWdzFsDRBgQGStg` / artifact
`art_3f271b34d2316f8b21f644d9bc8fe045`. Railway recorded its preview `201` at
`2026-08-21T15:37:50.500017677Z` in 105 ms. Both roles received `403` from the
publication-context API, and neither cockpit exposed replacement, rollback or
publication mutation.

A candidate principal-isolation concern was tested to completion and was not a
defect. The Architect entered distinctive unbuilt text `OWNER PRIVATE UNSAVED
LEAK SENTINEL`. An initial sign-out click occurred while automatic signed-track
recovery was still settling and did not cross the authentication boundary. Once
recovery settled, a clean sign-out cleared the sentinel, workspace and selected
components before another principal authenticated. Viewer login then showed no
sentinel, no checked components and only the server-derived authorized signed
preview. A later reload after supported account deletion showed the logged-out
state, empty workspace, waiting preview and no retained publication record or
action. The deterministic switch therefore failed closed; no P1 was sent.

The live browser had no console warning/error, keyboard roving moved account and
workspace tabs with Arrow/Home/End while retaining focus, and no horizontal
overflow appeared at the stabilized 385-pixel and 305-pixel live CSS viewports.
The promoted Chromium journey, rather than this in-app viewport adapter, remains
the exact 160/200/320/400-pixel reflow evidence. Rapid
authentication probes correctly reached one bounded `429` at
`2026-08-21T15:41:12.785030687Z` in 5 ms; further login attempts stopped until
the limiter window recovered. One non-blocking P2 accessibility/hygiene item
remains for a later coherent UI batch: pending-claim utility controls measured
29 px, Sign out 27 px and logged-out auth tabs 38 px, below the product's 44 px
target. The promoted active/expired publication action controls themselves are
covered at at least 44 px. Anonymous signed-file `401` responses also remain
without `Cache-Control`, while successful signed-file responses are
`private, no-store`; no sensitive bytes were returned.

Supported cleanup deleted workspace `51` at
`2026-08-21T15:48:55.234525080Z` (`204`, 4 ms), which removed both tracks,
artifacts, memberships and the pending claim. Viewer `84`, Maintainer `85` and
Architect `82` then self-deleted at `15:48:56Z`, each with `204`; all old profile
tokens returned `401`, and old track/runtime reads returned `401`. Outsider `83`
had already self-deleted `204`. No fixture from this gate remains. Workspace
`47` remains the explicitly preserved historical operator-authority blocker and
was not touched.

Final Railway readback found zero HTTP `500..599` records for deployment
`43f11880`, continuous low resource use (one-hour CPU maximum approximately
`0.0051` vCPU and memory maximum `0.0348` GB), exact `201` preview, `409`
pending-publication, `429` limiter and `204` cleanup correlations, and no restart
or deployment anomaly.

**Verdict:** ACCEPT the publication-lifetime P0 on application commit
`928c8f274460f00b4ac2059a0e7c43fb221d5791` / Railway deployment
`43f11880-806a-4719-84b3-d6587403ec2c`. Live Browser, API, signed-byte, role,
stale-principal, accessibility and cleanup boundaries are green. Exact temporal
expiry/Host serving, direct/null/ambiguous publication binding and immutable
replacement/rollback are accepted from the promoted controlled-clock, Host and
real-Chromium suites because producing those live states requires the forbidden
external DNS/reviewer fixture. No P0/P1 application finding remains. The later
UI-safety batch may now begin only after control-room handoff.

The five-person study remains `not-started`; no participant, consent, demand or
willingness-to-pay evidence is claimed. Continuous acceptance remains in progress
through the separately authorized UI-safety sequence and final one-time Fable
adversarial review; neither is claimed by this publication-lifetime gate.

### UI-safety and landing deployment retest — ACCEPT

The exact application and PR commit
`4a9b1c68044e0324105fddccfaf72810d502626c` is live on Railway deployment
`f8e86558-b90a-43a1-8599-245a8c30684d` (`SUCCESS`) with runtime image
`sha256:85726c4a1411319f10be0420db3b1f96237f3082012d9ab98accc9bef13a1eb8`.
The accepted parent and rollback source is
`928c8f274460f00b4ac2059a0e7c43fb221d5791`. The pre-release ledger was
independently read back at SHA-256
`52630F4C56CF0CDAFBD43C5874C32FD59ADB658DC7D528496244E567BA64F25F`
before this section was appended. Source attribution showed that the candidate
changed the accepted web client and its tests/documentation only; no backend
contract changed. The integrated handoff reported every Go package, both
production binaries, 15/15 Node tests including promoted real Chromium, and
separate security and UI/accessibility reviews green.

Fresh in-app Browser evidence started logged out. The landing page exposed one
visible H1, `Build a clear home for your community.`, a working skip target,
unambiguous Create account and Sign in actions, and plain-language statements
about preview-before-publication, local-first community records, encrypted relay,
central identity/signed-artifact/audit retention, purpose-based retention after
account deletion, and zero custody/settlement. Account tabs responded to keyboard
Arrow and Home navigation. At 400 px and 320 px the live document had no horizontal
overflow and retained the value, trust and custody copy. The same was true for the
authenticated cockpit at 640/400/320 px. All ordinary interactive controls sampled
at the required paths were at least 44 px. The focused skip link computed to 43 px
because of live pixel rounding; this is recorded as a non-blocking P2 measurement,
not a comprehension or completion failure, and the promoted accessibility gate is
green.

Synthetic Architect `86`, Viewer `87`, Maintainer `88`, zero-workspace principal
`89`, and workspace `52` exercised the authenticated matrix. People resolved the
Architect role before mutation controls enabled. Viewer saw only inspect controls:
template/build, invite, member removal, domain revocation and grant controls were
disabled or absent. Maintainer could build but could not invite, remove members,
revoke a domain claim, publish or perform Architect-only workspace cleanup. Direct
API probes agreed: authorized People returned `200`, `no-store` and `nosniff`;
Viewer member-removal and domain-revocation attempts returned generic `403` with
no count, version or authority oracle.

The Architect created a current-session Viewer invitation. Its result explicitly
said the secret existed only in that session. The revoke confirmation was a named
alert dialog; Cancel returned focus to the invoking control, and Confirm produced
an exact revoked-version readback, removed the displayed secret and revoke action,
and returned focus to the invite section. A separate invitation was accepted and
then revoked at its stale expected version; the server returned deterministic
`409` without disclosing the token. No invitation secret entered the ledger,
screenshots, logs or another principal's state.

Member removal used the same confirmation/focus discipline. Cancel returned focus
to the exact Viewer removal control. Confirm removed Viewer `87`, refreshed the
People count to three and focused the members heading. The removed principal's JWT
still read its own profile but immediately lost workspace `52`: its workspace list
no longer contained the workspace and its People request returned `403`.

A single synthetic `.example.invalid` domain claim was created and left pending;
no DNS proof was altered or verified. A fresh authoritative cockpit reload
rediscovered it as pending. The revoke alert dialog had correct accessible naming,
Cancel restored focus, and Confirm returned the exact revoked option, hid and
cleared proof, disabled verify/publish/replace/rollback, retained the staging-only
preview warning, and focused the claim selector. Live deletion returned `200` for
the Architect and generic `403` for the Viewer. Verified-active Host shutoff,
expiry and immutable activation history continue to rely on the accepted
controlled-clock/service/Host and promoted-browser evidence; no external DNS,
reviewer or Bazaar authority was fabricated.

Maintainer `88` selected `community-iftar` from the real three-template catalog
and selected `iftar-registration` plus `announcements`. Invalid advanced JSON
showed a bounded error and retained the last valid document and Apply-button focus.
Switching to an incompatible template opened a confirmation; Cancel returned focus
to the template control and kept the document. The final exact document changed the
title to `Open Iftar Registration — QA 🌙` and added a synthetic custom scalar,
array and object. Preview creation returned `201` at
`2026-08-21T19:36:30.232Z` in 64 ms and created
`track_wGtGqkai8WkP652_bezYAaU9` /
`art_05254ddc803010d61a151c8835397e42`.

The browser showed `Verified staging ready` only after the complete signed receipt
and separately confirmed one Build History track. The styled frame started with
the customized Unicode card, not raw JavaScript; sandbox was exactly
`allow-scripts allow-forms`, with one intact inline runtime and no external script.
The registration control was interactive and reported a tab-local preview save.
Receipt facts matched the exact raw manifest: Ed25519 key
`railway-artifact-v1`, workspace `52`, template v2, both modules, component-file
digests, configured production surface, lifecycle/expiry and reference-only review
values. The exact manifest included the custom scalar/array/object and recomputed to
SHA-256 `7e1264cecb989e599f9647ec5c9b78775d8e1e2ae92a6b6d62b4f445a860853c`.
Its authenticated immutable Datastar runtime was 34,083 bytes and recomputed to
`2837d87acf6ee0ba8e4e63765926c25a98d63883b02f88be194a86b81d3fd24a`,
exactly matching the signed descriptor. Authenticated runtime and index reads were
`200`; anonymous reads were `401`. Publication stayed disabled.

The account surface truthfully stated the 12-character password minimum, session
invalidation behavior, purpose-based retention, and supported workspace-first
account deletion path. Inputs carried `minlength=12`, `autocomplete=new-password`
and associated guidance/live-status semantics. Browser policy reserved final
password submission for human handoff, so the authorized synthetic rotation used
the same supported HTTP route: it returned `200`; the old password and old token
returned `401`, and the new password authenticated. A browser request made with the
now-stale session immediately returned the logged-out account view with `Your
session expired. Please sign in again.` and cleared workspace, preview and invite
state.

Owned-workspace self-delete returned bounded, non-destructive `409`; the same
Architect token still read its profile and workspace. The account and workspace
delete confirmations were correctly named alert dialogs and Cancel returned focus
to their invokers. Browser policy similarly reserved the final destructive clicks,
so the authorized synthetic cleanup used the supported lifecycle endpoints and
then reloaded the browser for authoritative recovery: workspace `52` returned
`204`; a fresh owner login showed an empty workspace selector, Waiting preview,
hidden build action, no invite grant and history guidance to select a workspace.
Zero-owned principal `89` self-deleted `204`, and its old profile and workspace
creation returned `401`. Viewer `87`, Maintainer `88` and Architect `86` then
self-deleted `204`. All current, pre-rotation and owner tokens returned `401`.
The final in-app Browser reload showed the logged-out landing, and all synthetic
credentials and cached manifest/runtime values were cleared from the browser-control
kernel.

Final direct probes returned `/api/health` `200`, root `200 text/html`, and the
landing body. Railway showed deployment `f8e86558` as the sole current `SUCCESS`,
zero deployment-scoped HTTP `500..599`, one-hour CPU average `0.0004` vCPU/max
`0.0069`, and memory average `0.0268` GB/max `0.0482`. Exact request correlation
included the preview `201` above, owner self-delete `409`, password rotation `200`,
domain deletion `200`, Viewer domain denial `403`, workspace cleanup `204`, and all
supported user cleanup. No panic, restart, upstream failure or unexplained `5xx`
was observed.

**Verdict:** ACCEPT application commit
`4a9b1c68044e0324105fddccfaf72810d502626c` / Railway deployment
`f8e86558-b90a-43a1-8599-245a8c30684d`. Fresh live Browser, low-rate API,
signed-byte, role, confirmation/focus, stale-session and cleanup boundaries are
green, and no P0/P1 application finding remains. No fixture from this gate
remains: workspace `52` and users `86`–`89` were removed only through supported
routes and all old authorities are `401`. Historical workspace `47` remains the
explicit operator-authority cleanup blocker and was not touched. External
DNS/reviewer/Bazaar proof and the five-person human study remain separately
blocked/not-started and are not claimed by this gate.

## Truthful SEO landing-page checkpoint — 2026-08-21

Application commit `ba95c5d3d5dc3d61c1dcbf70559456a4064862c3` replaced the
logged-out product-first screen with a comprehensive plain-language landing page
for mosques, Muslim charities, student groups and grassroots organizers. The
page describes the three real starting templates, eleven customizable building
blocks, team roles, decision and no-money practice flows, the protected preview
journey, and the current private-beta boundaries. It does not claim public custom
domain availability, live payments or settlement, scholar approval, marketplace
acceptance, customer adoption, testimonials, pricing, or a completed human study.

Search-facing evidence is aligned with the visible page: the live title is
`Community Page Builder for Muslim Organizations | Taawun`; the description is
private-beta scoped; canonical, Open Graph and Twitter metadata use the current
Railway service URL; minimal `WebSite` JSON-LD is present; and `/robots.txt` plus
`/sitemap.xml` both return `200`. The public landing has one visible H1, semantic
section headings, ten visible questions and answers, and real anchor targets. The
account and authenticated application surfaces are marked `data-nosnippet`; no
review, rating, offer, price, FAQ or application schema is fabricated.

The exact source passed the full Go package sweep, both production builds, and
all 13 non-browser Node/runtime checks. Independent code, accessibility and
truth/SEO review each returned APPROVE after the final corrections. The in-app
Browser verified the exact source at desktop, 400 px, 320 px and the tool's 240 px
floor for a requested 200 px width: there was no horizontal overflow, one visible
H1, controls were at least 44 px, and account shortcuts focused the correct input.
A fresh live Browser pass on the deployed page reconfirmed the desktop landmarks,
ten-question FAQ, 44 px minimum controls, working account shortcuts and zero
console warnings/errors. A separate standalone Chromium launch was unavailable
because the local browser process failed before page attachment; no additional
credit is claimed from that runner.

Railway deployment `c8f17826-25d6-44b3-a6ed-dcf66a06255b` reached terminal
`SUCCESS` with image
`sha256:dc006cbf8229ed0f80de3dd5874849c438f59cd054409ef8dd0cf1767de9d1c1`.
Fresh `/api/health`, `/`, `/robots.txt` and `/sitemap.xml` probes all returned
`200`. A bounded 30-minute Railway snapshot showed ten requests, nine `2xx`, one
non-server `4xx`, zero `5xx`, p95 41 ms, CPU average/max
`0.00007`/`0.00216` vCPU, and memory average/max `30.3`/`33.9` MB. Runtime logs
show one volume mount, one database initialization and one server start; Railway
classifies the two application startup lines as error-stream records, but neither
contains an error and there is no panic, exit or restart signal.

**Verdict:** ACCEPT the truthful SEO landing page on application commit
`ba95c5d3d5dc3d61c1dcbf70559456a4064862c3` / Railway deployment
`c8f17826-25d6-44b3-a6ed-dcf66a06255b`. This gate created no user, workspace,
domain, publication, Bazaar, finance or reviewer fixture. Workspace `47`,
controlled DNS/reviewer/Bazaar proof, the genuine five-person study, and deferred
federation/TURN/E2EE remain unchanged and unclaimed.

## Scroll-lit geometric landing checkpoint — 2026-08-22

Application commit `0b13e505c444a406c336300b4aa27f4ed8b9dd96` moved the
account cockpit to `/account` and made `/` a dedicated public story. The root
contains no login, registration, application state or bearer-token code. The
account document is `noindex,nofollow,noarchive`, remains the same document after
login, and selects its register or login tab from the ordinary
`/account#register` and `/account#login` links. Published customer Hosts do not
inherit the control-site account route; the exact Host boundary is covered by a
404 regression.

The landing retains the Swiss dark-emerald system and adds a progressively
enhanced Islamic geometric tessellation. A real in-app Browser at 1,280 px
observed `renderer=webgpu` and `data-enhanced=true`, five scroll states, a state
`0` to `1` transition, reversible decorative motion and no console warnings or
errors. The renderer requests a low-power non-fallback adapter, caps resolution
and frame rate, redraws only for scroll/resize/resume events, and tears down on
device loss or sustained frame cost. Reduced motion, forced colors, increased
contrast, reduced transparency, data-saving, small-screen and modest-device
gates keep the CSS tessellation instead of requesting WebGPU. At 320 px the live
source used that static fallback with one H1, no horizontal overflow and sampled
account controls at or above 44 px.

The full Arabic text of Sūrat an-Nūr 24:35 is static, marked `lang=ar` and
`dir=rtl`, and attributed in visible text. Its adjacent note limits the
relationship to visual inspiration from light through glass and explicitly says
the verse is not a product claim, certification or ruling. The page makes no
claim of scholar approval or religious authority.

The exact commit passed all Go package and command tests and both production
builds. A source/runtime sweep passed 14/14. An unrestricted promoted
Node/Chromium sweep passed 18/19; its sole late domain-revocation wait then passed
on one focused rerun, classifying it as an isolated timing failure rather than a
product finding. The new real-WebGPU lifecycle, route-continuity, responsive,
keyboard and existing Viewer/Maintainer journeys were green. Independent code
and accessibility reviewers returned APPROVE with no P0/P1.

Railway deployment `05d22d21-e442-4676-be51-0936c72fbb49` reached terminal
`SUCCESS` with runtime image
`sha256:4973e15b240407a67293a87a12f71d679ae3715d958163815f62fc85f4260c28`.
Fresh `/api/health`, `/`, `/account` and `/robots.txt` checks returned `200`;
root exposed the aligned search title and WebGPU loader without an auth form,
while `/account` exposed the noindexed account document and a 12-character
password minimum. The deployed in-app Browser again observed real WebGPU,
five scroll states, the attributed Arabic verse, no overflow and zero console
warnings or errors. Three final low-rate health/root pairs were 6/6 `200`.

The first bounded production snapshot recorded eight `2xx`, zero `4xx`, zero
`5xx`, p95 28 ms, CPU average/max 0.000108/0.00243 vCPU, and memory average/max
24.23/25.28 MB. Runtime logs show one volume mount, one database initialization
and one server start, with no panic, exit or restart. Railway categorizes the two
Go standard-logger startup lines as error-stream records; their text is normal
startup information and deployment-scoped HTTP `500..599` is empty.

**Verdict:** ACCEPT application commit
`0b13e505c444a406c336300b4aa27f4ed8b9dd96` / Railway deployment
`05d22d21-e442-4676-be51-0936c72fbb49`. This release created no user,
workspace, domain, publication, Bazaar, finance, reviewer or participant fixture.
It does not add evidence for controlled DNS/public Host serving, Bazaar review or
purchase, or the genuine five-person study. Workspace `47` remains the supported
application-admin cleanup backlog; federation, TURN and complete E2EE key
lifecycle remain deferred.

## Full-surface geometric landing and navigation checkpoint — 2026-08-22

Application commit `6499c872a7e5d41f0bc08a27ba78013985fa2ae7` removes the
framed decorative viewport and lets one transparent WebGPU or CSS tessellation
occupy the landing surface behind the story. Six authored scroll states follow
right, right, left, right, right, left, while desktop content uses the inverse
side. The renderer interpolates before each section center, strengthens the
depth/refraction transition, uses premultiplied transparency, and preserves the
low-power adapter, 1.5-million-pixel, event-driven frame, sustained-cost and
device-loss bounds. Small screens use restrained off-screen edge peeks instead
of WebGPU.

The former detached checkbox is now a subtle 44 px navigation toggle with a
stable `Decorative motion` name and `aria-pressed` state. It controls the static
mobile scene as well as WebGPU and remains available on fallback-adapter,
device-loss and reduced-data paths. At mobile widths it sits directly beside a
44 px disclosure button. Opening that menu focuses the first revealed link;
Escape and link activation return focus to the visible menu control. The brand
compacts below 240 px, and promoted Chromium proves no brand/control overlap or
horizontal overflow at 160, 200, 320 and 400 px.

The landing now presents only the requested excerpt from Sūrat al-Māʾidah 5:2,
marked `lang=ar`, `dir=rtl`, and visibly attributed as an excerpt. The former
visual-inspiration explanation and the Sūrat an-Nūr 24:35 block are absent. The
verse remains selectable semantic content outside the decorative renderer and
is not used as a product, certification, scholar-approval or religious-ruling
claim.

The exact source passed `go test ./src/web ./src/pkg/... ./src/cmd/...`, both
production binary builds, and 19/19 Node/runtime tests with promoted real
Chromium. The browser suite covers WebGPU pause/resume, null-adapter and device
loss fallbacks, reduced motion, forced colors, reduced data, menu focus,
narrow-width reflow and the existing authenticated product journeys.
Independent code and accessibility reviewers returned APPROVE after the final
shader, fallback-control, accessible-name, focus-order and 160 px corrections.

Railway deployment `1ba31e17-8b40-49e0-a576-71164e80cba8` reached terminal
`SUCCESS` with runtime image
`sha256:13bdca9e46932afc86aa36b8b303e1bc4d64bf6d59ba0fc6d9b7c57909c599f9`.
Railway's first `/api/health` check succeeded. Independent public GETs returned
`200` for `/api/health` in 0.296 seconds and `/` in 0.511 seconds with
`text/html; charset=utf-8`. Deployment-scoped HTTP `500..599` was empty; the
bounded runtime readback contains one database initialization and one server
start with no panic, exit or restart. The one-hour service snapshot reported CPU
average/max `0.0001`/`0.0012` vCPU and memory average/max `0.0203`/`0.0376` GB.

A fresh deployed in-app Browser at 1,440 px observed the exact six-side sequence,
`renderer=webgpu`, `data-enhanced=true`, reversible motion and the exact 5:2
excerpt with zero console errors. At 390 px it observed the static edge treatment,
adjacent 44 px motion/menu controls, no horizontal overflow, first-link focus on
open and menu-button focus after Escape. No account, user, workspace, domain,
publication, Bazaar, finance, reviewer or participant fixture was created.

**Verdict:** ACCEPT application commit
`6499c872a7e5d41f0bc08a27ba78013985fa2ae7` / Railway deployment
`1ba31e17-8b40-49e0-a576-71164e80cba8`. Rollback remains application
`0b13e505c444a406c336300b4aa27f4ed8b9dd96` / Railway deployment
`05d22d21-e442-4676-be51-0936c72fbb49`. Controlled DNS/reviewer/Bazaar proof,
the genuine five-person study, workspace `47`, federation, TURN and complete
E2EE key lifecycle remain unchanged and unclaimed.

## Sharper tessellation and settled-motion checkpoint — 2026-08-22

Application commit `f1c427a8732b19f9f6b48698c6e2f7d8abc4aea4` replaces the
soft sinusoidal star boundary with a repeated straight-segment eight-point star,
rotated inner rosette and interlocking diagonal diamond straps. The base lattice
is no longer globally warped. Mirrored depth and localized emerald, gold and rust
refraction remain as displaced layers, so the construction stays legible at rest
while motion retains its glass-like dimensional character. Moving frames use
slightly broader antialiasing and the exact settled frame resolves to the sharper
line treatment.

Scroll input now changes only a normalized target. A fixed 60 Hz underdamped
spring with capped elapsed time and velocity approaches that target independently
of wheel or swipe rate, allows a gentle bounded overshoot, snaps to the exact
destination at finite displacement and velocity thresholds, and then stops
scheduling frames. The still mobile/CSS treatment uses one fixed-duration gentle
overshoot curve. User pause, reduced motion and the existing capability fallbacks
remain nonanimated.

The Sūrat al-Māʾidah 5:2 excerpt now includes the visible English translation
“Cooperate with one another in goodness and righteousness, and do not cooperate
in sin and transgression.” directly beneath the Arabic, attributed to Dr. Mustafa
Khattab, The Clear Quran. Both languages remain selectable semantic content
outside the decorative renderer; no new religious-authority, certification or
visual-inspiration claim was added.

The exact source passed all Go package and command tests, both production binary
builds, 19/19 combined Node/runtime tests, and a focused 12/12 landing/customer
Chromium run after correcting a test-only `innerText` versus `textContent`
expectation for CSS-transformed attribution text. The promoted WebGPU regression
proves that one scroll event produces continued bounded submissions, a real
overshoot, a sharper final uniform, settlement in fewer than 120 frames and no
later submissions. Independent code and accessibility reviews returned APPROVE
with no P0, P1 or P2 finding.

Local and deployed in-app Browser inspection at 1,440 px observed the crisp
star/rosette/diamond construction, mirrored refracted layers, visibly softer
transition and sharper settled state. The deployed page reported
`renderer=webgpu`, `data-spring=settled`, the exact H1 and translation. At 390 px
it used the intended static fallback with the motion toggle beside the collapsed
menu, exact translation and no horizontal overflow.

Railway deployment `b946bcc0-3eef-4c78-989e-3a5d932a37b0` reached terminal
`SUCCESS` with runtime image
`sha256:7c900c781922d18829b543d8372263ba4041d0fe40735e94a209662721a914f2`.
Independent public GETs returned `200` for `/api/health` in 0.261 seconds and `/`
in 0.402 seconds with `text/html; charset=utf-8`. Deployment-scoped HTTP
`500..599` was empty. The bounded snapshot reported p95 25 ms over the first two
requests, CPU average/max `0.0001`/`0.0013` vCPU and memory average/max
`0.0207`/`0.0376` GB. Runtime output contains one database initialization and one
server start with no panic, exit or restart.

**Verdict:** ACCEPT application commit
`f1c427a8732b19f9f6b48698c6e2f7d8abc4aea4` / Railway deployment
`b946bcc0-3eef-4c78-989e-3a5d932a37b0`. Rollback remains application
`6499c872a7e5d41f0bc08a27ba78013985fa2ae7` / Railway deployment
`1ba31e17-8b40-49e0-a576-71164e80cba8`. No account, user, workspace, domain,
publication, Bazaar, finance, reviewer or participant fixture was created.
Controlled DNS/reviewer/Bazaar proof, the genuine five-person study, workspace
`47`, federation, TURN and complete E2EE key lifecycle remain unchanged and
unclaimed.

## Interlocked tessellation and two-sway landing checkpoint — 2026-08-22

Application commit `023c667f73bb2bc7c6c54aa4a58b835b2c8645eb` sharpens the
landing geometry into repeated eight-point stars, nested rosettes, interlocking
diamond straps and shared octagonal junctions. Paired bridge rails now begin at
the real star-arm boundary and meet the octagonal outline without crossing the
negative-space interiors. The emerald and rust refraction samples remain tightly
stacked beside the exact base path in cell space, with a smaller perpendicular
mirrored rail instead of a displaced duplicate lattice.

The six desktop scenes now group geometry right, right, left, left, right,
right, so the full landing performs exactly two lateral crossings and readable
content uses the inverse placement. Same-side section changes send zero travel
transition and retain one invariant resting lattice orientation. A critically
damped fixed-step response and explicit target-crossing guard remove reverse
swing and overshoot. Only the two real side crossings receive the restrained
travel rotation, 0.002-cell shimmer and inverse 3.5% outer-star / 4.5%
inner-rosette kaleidoscope overlap; all three return to their exact resting
geometry on settlement. The CSS fallback uses the same side grouping with a
non-overshooting 1.2-second curve and a 3.5% maximum scale pulse.

The exact source passed all Go packages and both production binary builds.
The elevated combined Node/runtime sweep passed 18/19; its sole failure was a
new shader-source probe that searched for a non-literal signature fragment. A
focused promoted-Chromium rerun then exposed that the new same-side assertion's
scroll target actually landed on the first real side crossing. After correcting
those two test-only probes, the focused real-Chromium WebGPU lifecycle passed.
Taken together, every promoted test is green on the exact application source.
Independent code review closed both the original bridge-interior finding and the
hidden per-section whole-field rotation finding. Accessibility review returned
APPROVE with no P0/P1/P2 regression.

Railway deployment `20818f11-623c-46d7-9848-d9ea91334261` reached terminal
`SUCCESS` with runtime image
`sha256:b1ecbaac5343d7dcce49de1c5672be7a308c038dd09197921f9aa9996784a7a0`.
Independent public GETs returned `200` for `/api/health`, `/` and
`/geometric-renderer.js`. The served shader contained the exact octagonal
connector, corrected bridge gate, tight rail, invariant-rest and side-gated
kaleidoscope contracts. Deployment-scoped HTTP `500..599` was empty. Railway's
bounded 15-minute snapshot reported CPU average/max
`0.0001072`/`0.0023663` vCPU and memory average/max `12.998`/`14.902` MB.
Runtime output contains one database initialization and one server start; the
platform labels those normal Go standard-logger startup lines as error-stream
records, with no panic, exit, restart or application error.

A fresh deployed in-app Browser at 1,440 px observed `renderer=webgpu`,
`data-enhanced=true`, settled motion, the exact six-state sequence with two side
changes, one H1, the separate account route and no horizontal overflow. At
390 px it observed the intended static fallback, adjacent 44 px motion/menu
controls, no navigation overlap and no horizontal overflow. No account, user,
workspace, domain, publication, Bazaar, finance, reviewer or participant fixture
was created.

**Verdict:** ACCEPT application commit
`023c667f73bb2bc7c6c54aa4a58b835b2c8645eb` / Railway deployment
`20818f11-623c-46d7-9848-d9ea91334261`. Rollback remains application
`f1c427a8732b19f9f6b48698c6e2f7d8abc4aea4` / Railway deployment
`b946bcc0-3eef-4c78-989e-3a5d932a37b0`. Controlled DNS/reviewer/Bazaar proof,
the genuine five-person study, workspace `47`, federation, TURN and complete
E2EE key lifecycle remain unchanged and unclaimed.

## Continuous same-side geometric motion checkpoint — 2026-08-22

Application commit `e1826d1cbd818fe1525d879a7dffcf7d87272221` keeps the
landing tessellation visibly alive throughout an unsettled scroll without adding
another whole-field sway. A fixed-step internal phase advances only while the
critically damped scroll spring is moving and freezes when it settles. Same-side
scenes retain zero lateral travel and the invariant resting orientation while
the nested star, rosette, tight refracted rails and mirrored depth breathe within
strict sub-three-percent bounds. The two real side crossings retain the full
approved rotation and kaleidoscope amplitude, so the six-state path remains
right, right, left, left, right, right.

The exact source passed every Go package, both production builds and all 12 Node
contracts with Chromium serialized on Windows. Production binary SHA-256 values
were `B9A549852330F3B82199698CEA848C089158D2A70F053E614EB771B6A1D5B970`
and `4E59164B86924898C8BB5150484674A45514DF29BDCDFA3D87D69B5113E82943`.
The promoted WebGPU test sampled phase changes during a right-to-right scroll,
proved lateral transition stayed zero, bounded every fixed-step phase delta, and
proved animation frames stop after settlement. Independent review returned
APPROVE with no finding.

Railway deployment `d82c182d-2fa9-46df-819a-9f9b1fb2ad8c` reached terminal
`SUCCESS` with runtime image
`sha256:a10b24cc1d84c7e8ba34e9d77d16d04cea05a57e085839bd319198545cdad990`.
Independent GETs returned `200` for `/api/health`, `/` and
`/geometric-renderer.js`; Railway request IDs were
`v0SJtyN4RAm6mFXmxtoGcA`, `1shYxKzBRay603Xs0_TJvA` and
`PGCCgwgcRKG_vO5e0_TJvA`, with total durations 28 ms, 29 ms and 3 ms.
Deployment-scoped HTTP `500..599` was empty. The bounded 15-minute snapshot
reported CPU average/max `0.000115`/`0.002608` vCPU and memory average/max
`23.964`/`25.501` MB. Runtime output contains one volume mount, one database
initialization, one server start and one container start with no panic, exit,
restart or application error; Railway labels the two normal Go standard-logger
startup lines as error-stream records.

A fresh deployed in-app Browser at 1,280 px observed the expected title and H1,
one WebGPU canvas and `data-enhanced=true`. The first scroll advanced state 0 to
state 1 while remaining on the right with no transition flag. Continued and
reverse scrolling observed the left state 2/3 grouping and right state 4/5
grouping, then settled with the transition flag cleared and the canvas intact.
No account, user, workspace, domain, publication, Bazaar, finance, reviewer or
participant fixture was created.

**Verdict:** ACCEPT application commit
`e1826d1cbd818fe1525d879a7dffcf7d87272221` / Railway deployment
`d82c182d-2fa9-46df-819a-9f9b1fb2ad8c`. Rollback remains application
`023c667f73bb2bc7c6c54aa4a58b835b2c8645eb` / Railway deployment
`20818f11-623c-46d7-9848-d9ea91334261`. Controlled DNS/reviewer/Bazaar proof,
the genuine five-person study, workspace `47`, federation, TURN and complete
E2EE key lifecycle remain unchanged and unclaimed.
