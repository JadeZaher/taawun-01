# Taawun continuous live QA

QA role: independent live-site acceptance lane for the private-beta MVP.
Started: 2026-08-17 MDT / 2026-08-18 UTC.
Scope: browser customer journeys, live API contracts and adversarial boundaries,
Railway health/log/metric correlation, regression verification, and synthetic-data
cleanup. This lane does not implement or deploy source fixes.

## Acceptance state

**BLOCKED — the technical baseline and first-session presentation are live-green,
but signed-receipt trust binding and persona discoverability still have open P1s.**
Deployment `68914f4c-a916-43f7-b0bf-fbd578c25f43` fixed the raw-JavaScript
preview, invalid composition, Railway-source throttle, publication state, account
lifecycle, and ethics boundaries. Deployment
`fe372625-e35f-4490-a8de-d8647826e7d3` made dashboard evidence truthful,
preserved mobile value/custody copy, corrected theme contrast, and exposed signed
receipt details. Its client verification attestation still does not bind every
rendered signed authorization field or reject expiry, so acceptance remains
blocked. A roughly 30-second all-route 502 interval at
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

The prior accepted checkpoint `80feecc7-e5fe-49cc-8cfa-858a64e86cee`
(`6a4d3a0`) is now `REMOVED` and is retained only as historical evidence in
`docs/deployment-live-qa.md`.

## Baseline journey — deployment `04f32db0`

Tested in the Codex in-app browser against
&lt;https://taawun-production.up.railway.app&gt; with a fresh synthetic principal.

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
  contributes an early `&lt;/script&gt;` at 13,030.
- The injection occurs at Datastar source containing JavaScript replacement token
  `$&amp;`. The cockpit uses `String.replace` with the entire runtime as the
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
| Financial sandbox | **Green for bounded sandbox lifecycle** | Seven flows advertised. Client-supplied actor was rejected 400. Synthetic donation quest `quest_ARbpv-_FLgOwBYwgG7C0t41l` progressed CREATED -&gt; APPROVED -&gt; EXECUTING -&gt; RECONCILIATION_PENDING; all five expected events formed a valid hash chain. No real settlement or custody was implied. Request `A6laXQepR8eyxw6P9o6EoQ`. |
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
  &gt;=500 record. Railway does not expose a true internal restart counter, so this
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
| User `8` | Earlier domain/OAuth self-delete repro | Pre-existing cleanup blocker; pending supported operator cleanup only. QA will not attempt credential recovery or raw database deletion. |

No credentials or bearer tokens are recorded. Synthetic records will be removed
only through supported lifecycle routes. User `8` cannot be removed by this lane
without its credentials or an authorized admin principal; the implementation or
control-room lane must perform that final supported cleanup.

## Remaining coverage for the current deployment family

- Cleanup of pre-existing user `8` by a supported authorized path.
- Maintainer-specific invitation permissions and activation-after-revoke remain
  fixture-limited; Viewer isolation, Shura, financial sandbox, OAuth/MCP, relay,
  session/password invalidation, exact CORS, and actor/workspace isolation are
  live-green.
- Signed-receipt attestation binding, discoverability/time-to-value, Maintainer
  persona coverage, no-store/nosniff, request-ID logging, consistent errors,
  OpenAPI, and favicon. Activity truthfulness, responsive layout, default theme
  contrast, and auth-tab semantics are live-green.
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
| P1 | Invited Viewer/Maintainer and governance approver: find their work | Existing invitation/Shura APIs are not discoverable in the current cockpit snapshot | Small, role-aware entry points to accept/join, review, vote/decide, with Viewer denial explained safely |
| P1 | Marketplace buyer/test-driver: evaluate before purchase | Bazaar capability is wired but absent from the observed authenticated cockpit | A published fixture can be found, test-driven before payment, and purchased only through disclosed sandbox escrow |
| P1 | Private-beta operator: understand platform health and cleanup | Health exists, but request IDs/activity/support surfaces are incomplete | Correlatable safe errors, real activity or explicit empty state, and supported cleanup/recovery without database access |
| Closed | Private-beta operator: trust displayed activity | Empty account returns zero scoped counts plus explicit empty messages; populated account returns one accessible workspace/user and one real `workspace_created` activity, with no fixed 2024 row | Preserve deterministic accessible-workspace scope and honest empty states |
| Closed | Generated-card user: read buttons and trust the default theme | Live default/light/mid/dark accents all select readable foregrounds; measured minima are 6.07:1 default, 13.17:1 light, 6.45:1 mid, 11.95:1 dark, and 17.58:1 baseline badge | Preserve &gt;=4.5:1 across arbitrary accepted accents |
| P1 | Organizer: rely on the signed receipt's exact scope | Visible signature/key, workspace, lifecycle/expiry, exact origins, and reference-only review facts match the positive manifest, but the client verifier compares only artifact/hash/workspace/signature fields; it does not recompute/bind the full manifest or reject expiration | A server attestation or client verification step must bind every displayed signed field, including lifecycle/expiry and all origin categories; missing, altered, mismatched, or expired evidence must never render `Verified` |
| Closed | Mobile first-session visitor: understand value and custody | Concrete signed/local-first/sandbox and accurate central-retention/Amanah/zero-custody/no-settlement copy remain visible at 400/320px with no overflow; authenticated boundary also remains visible | Preserve compact copy and true 200% reflow regression coverage |
| P2 | All personas: keyboard, zoom, mobile, and trust comprehension | Semantic landmarks and labels are present in the baseline snapshot; full focus/contrast/reflow testing remains | WCAG 2.1 AA keyboard path, visible focus, 200% zoom/reflow, &gt;=44px targets, and no misleading compliance/custody copy |
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

## Acceptance rule

Sign-off requires zero open P0/P1 findings, a real-browser sellable journey, green
deployment/tests, a final comprehensive regression pass with no new failures, and
explicit limitation of remaining gaps to external DNS or the documented
federation/TURN/E2EE deferrals.

## Implementation handoff awaiting independent retest

Application commit `57d9fd1` addresses the receipt-attestation scope finding by
binding the trusted server's exact serialized manifest to a SHA-256 digest and
requiring the browser to recompute and compare the complete rendered manifest.
The active verified state additionally requires matching artifact, content,
workspace, signature algorithm/key/value, internal subject/signer relationships,
and a future authorization expiry. Expired authentic evidence is labelled
signed-and-expired; missing, altered, inconsistent, or expired authorization does
not display the active `Verified` claim.

The implementation gate passed all Go package/command tests, 12/12 Node tests
including required real Chromium, `git diff --check`, and independent source/test
review. Railway deployment `1bfa3008-319c-49ba-bd52-090da3dbfa5e` reached
terminal `SUCCESS`; direct public probes returned `200` for `/api/health`
(`status=ok`) and `/` with the Taawun cockpit title. QA acceptance and persona
discoverability work remain paused until the independent live retest completes.

## Origin-denial implementation handoff awaiting independent retest

QA accepted the exact-manifest trust gate on application commit `57d9fd1` and
Railway deployment `1bfa3008-319c-49ba-bd52-090da3dbfa5e`. The sole technical
P1 then became the generic `500` returned for a correctly rejected unverified
preview origin.

Application commit `e269126` authorizes and normalizes requested preview origins
before creating durable draft/idempotency state, rechecks authority before
validation/signing, and maps expected surface/embedder/connection/resource
denial to nested `422 invalid_composition`. A revocation race leaves a resumable
staged track with no build request or artifact. Logs contain only a bounded
trusted/generated request ID, fixed outcome/reason, and status; customer and
credential fields are excluded.

The implementation gate passed all Go packages/commands, 12/12 Node tests with
required real Chromium, verifier-requested focused assertions, `git diff
--check`, and independent approval. Railway deployment
`58d87a23-6037-4cc5-8151-09b37167cef5` reached terminal `SUCCESS`; public health
and cockpit probes returned `200`. Live denial/retry/telemetry acceptance remains
pending.

Supported cleanup has not been claimed: user `19`/workspace `14` is observed;
users/workspaces `20`/`15` and `21`/`16` remain inference-only pending readback;
user `22`/workspace `17` is observed clean. Existing admin lifecycle routes
require an authenticated admin principal, but the deployment exposes no
configured bootstrap-admin environment to this lane. No fixture data was read
or changed, and no raw database or token-minting path was used.
