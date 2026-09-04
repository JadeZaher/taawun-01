# Builder and user-site live QA runbook

For the mosque-facing release decision, this procedural runbook is subordinate
to `conductor/tracks/mosque_alpha_20260902/release-gates.md`. The mandatory alpha
public path is a real Taawun-hosted `/s/{slug}` URL. A custom domain is optional;
when tested, it still requires genuine DNS and browser-trusted TLS.

This runbook promotes one exact Taawun build and tests the complete customer
journey without bypassing authentication, workspace authority, signed-artifact
verification, DNS ownership, or supported cleanup.

## Evidence and safety rules

- Use fresh synthetic emails and ordinary product UI controls. Never record a
  password, bearer, invitation token, DNS challenge, or private component body.
- Record only deployment, request, user, workspace, track, artifact, claim, and
  publication identifiers needed for bounded correlation and cleanup.
- A track ID and review URL grant no access. A custom domain is publishable only
  after genuine external DNS proof; the hosted `/s/{slug}` adapter is governed
  by its separate signed-publication lifecycle.
- Stop on any ambiguous mutation result. Reconcile through the supported read
  route before retrying; never repeat a create, publication request, or revoke
  merely because the browser response was unclear.
- Run the complete local test/build sweep once after all implementation fixes.

## 1. Promotion gate

1. Follow `conductor/tracks/mosque_alpha_20260902/plan.md` G2: freeze and commit
   the candidate, require a clean tree, and record the exact commit and rollback
   deployment before testing or building.
2. Run the one final integrated package/build/browser/security sweep against that
   commit. Build from it and record binary, image, and SBOM digests; annotate or
   tag the already-tested commit. Preserve only bounded failure summaries.
3. Confirm Railway is fixed at one replica, the expected persistent volume is
   mounted, and deploy/rollback overlap cannot produce two SQLite writers.
4. Deploy the exact image with an auditable release message and wait for the
   newest exact-scoped deployment to reach terminal `SUCCESS`.
5. Confirm liveness and dependency-aware readiness, graceful restart, landing and
   hosted public routes, bounded logs/`5xx`, current backup age, alert delivery,
   and the required 24-hour low-volume soak. The exact performance profile and
   thresholds are authoritative in `release-gates.md`.

## 2. Fresh organizer journey

1. Open `/`, follow the primary builder call to action, and create a new account.
2. Confirm automatic sign-in, create a workspace, and record only the synthetic
   user and workspace IDs.
3. Tell the guide the mosque name, location, required IANA timezone, audience,
   and what visitors should be able to do.
4. Choose today's goal from launch the basic site, add an event, or update the
   live site. Review the recommended Mosque Essentials structure and its reason.
5. Add identity/contact, prayer and Jumu'ah times, announcement, event,
   accessibility, external donation-provider link, and footer one section at a
   time. Exercise Back, Skip, Undo, Save and exit, close/reopen, and resume.
6. Review the rendered page and plain-language Draft, Private preview, Team
   review, Public, data-custody, timezone, and publication-expiry explanations.
7. Create the signed preview. Confirm visible page content, active status,
   manifest/receipt, component digests, expiry, exact origin, and the protected
   review-link control.
8. Copy the review link. Confirm it uses
   `/account#preview=<track-id>&workspace=<workspace-id>` and contains no
   bearer, invitation secret, workspace role, or component content. The
   workspace value is only a selector.
9. Publish the exact signed build to a Taawun-hosted `/s/{slug}` URL and confirm
   an unauthenticated mobile context sees no control-plane information.
10. Submit one real event registration with a fresh idempotency key and retain it
    for the collaboration, authorization, backup, and deletion checks below.

### Guided-experience observations

For a new/empty workspace, separately record:

- How many simultaneously visible choices compete with the next required task.
- Whether the organizer can identify one recommended next action in five
  seconds without knowing “template,” “module,” “track,” “manifest,” or “origin.”
- Whether purpose/audience answers produce an understandable recommendation and
  whether rejecting it preserves work.
- Whether Back, Skip, Undo, Save and exit, and resume preserve the exact draft.
- Whether Guided → Advanced → Guided is lossless and does not repeat mutations.
- Whether assistant proposals state what changed and why, remain editable, and
  fail back to deterministic guidance when the provider is unavailable.
- Whether the organizer can correctly distinguish draft, verified preview,
  protected member review, and anonymous public serving.

### Advanced-mode regression (not part of first-use usability)

After the Guided journey is complete, a technical regression tester—not a
first-time study participant—switches to Advanced, tries every legacy template,
edits scalar/object custom fields, exercises invalid JSON and reserved-key
recovery, and returns to Guided. The exact draft and version must remain stable;
no action may repeat. This appendix protects existing capability without making
raw controls part of the mosque operator's path.

## 3. Maintainer and Viewer handoff and authorization

1. As Architect, create Maintainer and Viewer invitations and transfer each token
   through the designated ephemeral test channel. Record invitation IDs/versions
   only.
2. In separate browser contexts, create or sign in to both accounts and accept
   each invitation. Prove acceptance retry is idempotent.
3. As Viewer, open the review URL. Confirm sign-in routing, workspace handling,
   automatic exact-track inspection, signed-file verification, and iframe render.
4. Confirm every edit/build, People, domain, publication, registration-response,
   finance, and governance mutation unavailable to Viewer is disabled or denied.
5. As Maintainer, edit allowed site/event content and build a successor. Confirm
   People, domain, publication, and registration-response reads/mutations remain
   disabled or server-denied.
6. As both roles, attempt to enumerate/read the retained registration; confirm
   the same bounded denial. Repeat from another workspace.
7. Select a different authorized workspace if available; confirm the review fails
   closed there and succeeds after returning to the correct workspace.

## 4. Edit, rebuild, and expiry behavior

1. Return to Architect and inspect the Maintainer's successor. Confirm the prior
   iframe is marked stale while the review-link control disables immediately.
2. Activate/rebuild as required and confirm a new track/review URL appears. The old link must still
   identify only its immutable old build and must not follow the draft.
3. Exercise an expired preview fixture only through the supported TTL contract;
   confirm files are removed from active presentation and copying/publication are
   disabled. Do not change clocks or database rows.

## 5. Revocation behavior

1. Remove Maintainer and Viewer through the Architect UI and authoritative People
   readback.
2. Reload old review links in both contexts. Confirm exact-track, signed-file,
   edit/build, and registration reads are denied without leaking workspace detail.
3. If a pending invitation was never accepted, revoke it at the displayed exact
   version and clear its secret from the clipboard/channel.

## 6. Taawun-hosted user-site deployment

This section is mandatory for mosque-alpha public delivery.

1. Activate the exact signed preview at a unique, normalized `/s/{slug}` path.
2. In a clean anonymous mobile context, confirm trusted HTTPS, the exact mosque
   content and assets, safe headers, and no control-plane information.
3. Retry the retained registration's idempotency key and confirm the Architect
   sees one response and can export it safely.
4. Trigger or await a consistent backup while the site and response are active;
   record its safe ID, completion, source deployment/schema, and timestamp and
   verify it is inside the one-hour RPO.
5. Confirm a Maintainer, Viewer, and another workspace cannot enumerate or read
   responses.
6. Close registration, confirm further submissions fail without leaking state,
   reopen it, delete the response, and confirm retention/audit behavior.
7. Build an edited successor, activate it, exercise rollback once, and confirm
   review links remain bound to their immutable builds.
8. Unpublish. Confirm anonymous serving stops while authorized history remains.
9. Restore that exact backup in isolation and verify integrity, publication
   bindings/signatures, response count/digest, and artifact digests. Do not claim
   public serving from a different origin; destroy the recovery fixtures after.

## 7. Optional controlled-domain user-site deployment

Run this section only when custom domains are exposed in the alpha. It must pause
when no genuinely controlled test hostname is available.

1. Enter an exact public HTTPS origin owned for this test and issue DNS proof.
2. Install the returned TXT record outside Taawun, then use Verify. Never capture
   the TXT value in this repository.
3. Create a fresh preview whose allowed surface includes the verified origin.
4. Request and activate publication. Confirm the host serves the exact artifact,
   correct CSP/content type/no-store headers, and no control-plane page.
5. Build an edited successor and exercise replacement or rollback once. Confirm
   publication history identifies the active exact version.
6. Revoke the domain claim. Confirm public serving stops while immutable build
   and audit history remain inspectable to authorized members.

If step 1 cannot begin, record `externally blocked: controlled hostname not
available`; do not create a fake claim, override the resolver, or mark public
deployment accepted.

## 8. Pain-point ledger

For every hesitation, wrong turn, misleading label, inaccessible control, stale
state, or unclear recovery message, append one row:

| Severity | Journey step | Observed evidence | User impact | Remedy | Track task | Status |
|:---|:---|:---|:---|:---|:---|:---|
| P2 | First workspace | Empty state says “Create one below,” but the fields and button are inside a collapsed disclosure | A new organizer sees no immediate action and must discover the disclosure | Open workspace creation automatically for an empty account; make it the first Guided step | `guided_ai_cobuilder_20260902` Phase 1 | fixed in deployment `671210c5-1151-4af5-af29-cdf13ac92fa7`; live recheck pending |
| P1 | Component customization | Renaming the new custom field appeared to succeed during editing, but the signed preview rendered the key as `custom_1` | The organizer can sign content different from what they believe they entered | Commit key/value input without rerendering the row; preserve focus and cover exact draft state | `guided_ai_cobuilder_20260902` Phase 1 | fixed and Chromium-covered in deployment `671210c5-1151-4af5-af29-cdf13ac92fa7`; live recheck pending |
| P2 | First builder view | Build, People, Shura, Finance sandbox, Bazaar, Account, templates, raw component controls, trust evidence, and deployment concepts are exposed before first value | Non-technical users must understand the product architecture to find the next step | Default empty workspaces to the eight-step Guided journey; retain Advanced as an explicit escape hatch | `guided_ai_cobuilder_20260902` Phase 1 | accepted |

Severity is P0 for authority/data exposure, P1 for blocked completion, P2 for a
recoverable but material usability failure, and P3 for polish. Every accepted
P0–P2 remedy must map to `conductor/tracks/builder_review_delivery_20260902/plan.md`
or a named follow-on track.

## 9. Supported cleanup and closeout

1. Delete synthetic registrations and unpublish the Taawun-hosted site.
2. Revoke any pending invitations; otherwise remove both test memberships.
3. Revoke the optional test domain claim if one was created and confirm both
   public adapters are unavailable.
4. Delete the synthetic workspace, Maintainer, Viewer, and organizer accounts
   through supported routes/UI. Capture status and request IDs only.
5. Confirm the Architect, Maintainer, and Viewer old sessions all receive `401`
   and the deleted workspace/track is unavailable.
6. Recheck deployment status, health/root, bounded runtime output, and bounded
   HTTP `5xx` logs. Reconcile the track tasks and record the exact external DNS
   result separately from application acceptance.

## 10. Live run — 2026-09-02

### Promotion evidence

- Railway project/environment/service: `hadith-ontology` / `production` /
  `taawun`.
- Initial builder/review deployment:
  `3a1dc762-b333-4b9d-87df-79fcf594df15`, terminal `SUCCESS`.
- Guided-first fixes deployment:
  `671210c5-1151-4af5-af29-cdf13ac92fa7`, terminal `SUCCESS`, image
  `sha256:c12e3920c9d31756bc7d25beaeda179e6139c4d0bf1b80c810269fb8aff67b06`.
- `/api/health`: `status=ok`; Conductor active, MCP enabled, P2P enabled.
- Bounded runtime evidence: persistent `/data` mounted, database initialized,
  server started on `:8080`; no startup failure or restart appeared.
- Local gate: all Go packages passed, both production binaries built, all 22
  Chromium tests passed, and `git diff --check` passed.

### Completed live journey

- Created disposable Architect and Viewer accounts through the deployed UI.
- Created workspace `54` through the deployed UI.
- Composed Community Iftar from registration, announcements, and donation
  components; edited plain content, Unicode organization text, and a custom
  field.
- Created and verified signed track
  `track_Zr1li1BR6hzGf1zGqeoZrY8n`, artifact
  `art_5e38a3a35961deb418f4e7e7534cfddd`, bound to workspace `54` and the exact
  Railway origin.
- Confirmed receipt signature, component digests, preview expiry, iframe render,
  and an interactive tab-local registration outcome.
- Copied the exact protected review URL and confirmed a signed-in non-member saw
  no track, receipt, manifest, signed files, or iframe.

### Paused and external gates

- Viewer invitation/acceptance, automatic post-accept review reopen, role denial,
  membership removal, and fixture cleanup remain paused before the explicit
  permission-changing invitation action.
- Controlled-domain publication remains pending a genuinely controlled hostname
  and authorized DNS/TLS operation. No claim was fabricated and no Host override
  was counted as public delivery.
- The current signed preview expires at `2026-09-04T03:50:27.392Z`; this is
  evidence about the immutable pre-fix track, not the guided-first deployment.
