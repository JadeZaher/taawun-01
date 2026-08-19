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

## Acceptance rule

Under the current control-room charter, sign-off requires zero open P0/P1 findings,
green deployment and promoted real-Chromium tests, independent low-rate live API /
Railway verification, a final comprehensive pass with no new failures, supported
cleanup evidence, and explicit limitation of remaining gaps to authorized operator
provisioning, external DNS/Bazaar fixtures or the documented federation/TURN/E2EE
deferrals. Fresh independent browser evidence remains valuable but is not claimed
and is not a release blocker while the Codex trust-path defect persists.
