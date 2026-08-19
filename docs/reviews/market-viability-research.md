---
type: research
title: Taawun private-beta market viability and next-slice evidence
status: approved-for-implementation
date: 2026-08-19
source_checkpoint: cedacb0
deployment: d446f826-bb48-4005-afc4-98e7fac8e046
---

# Taawun private-beta market viability and next-slice evidence

## Decision summary

Taawun now has a credible technical differentiator: a user can compose bounded
component documents, receive a workspace-bound signed preview, inspect the exact
manifest and receipt, recover the build from authorized history, and hand it to
another role without transferring creator or signer authority. The accepted live
release is healthy and the independent continuity matrix is green.

The remaining sellability gap is activation continuity. A new Organizer sees a
capable cockpit, but the product does not yet turn the existing template,
component, signed-preview, history, and invitation primitives into one obvious
route from an empty workspace to a reviewable signed result. Current alternatives
set the buyer expectation that templates, reuse, collaboration, recovery, and
sharing are connected parts of the first-use path.

**Approved Next slice:** a server-evidence-driven **Signed Starter Path** that
guides an empty workspace through explicit template choice, one meaningful
component customization, signed preview, durable-history confirmation, and a
Viewer-review handoff. The control room approved this bounded slice on
2026-08-19 with the implementation and acceptance refinements below.

## Method and limitations

- Current Taawun evidence is the accepted application checkpoint `cedacb0` on
  Railway deployment `d446f826-bb48-4005-afc4-98e7fac8e046`, the independent live
  API/Railway matrix, source review, and 13/13 promoted real-Chromium tests. No
  fresh Codex Browser credit is claimed.
- Competitor evidence uses current first-party product documentation accessed on
  2026-08-19. It describes marketed and documented capability, not independent
  usability benchmarks.
- This is desk research, not customer discovery. It does not prove willingness to
  pay, conversion lift, or demand. The recommended slice therefore includes a
  falsifiable private-beta comprehension test.
- No reviewed first-party alternative documentation surfaced a comparable
  per-build cryptographic manifest/receipt bound to exact documents, workspace,
  origins, signer, lifecycle, and expiry. That is an inference from the reviewed
  material, not a claim that no competitor has any signing feature.
- External DNS, qualified-reviewer and Bazaar purchase fixtures remain unavailable.
  They are not included in the recommended implementation slice. Federation,
  TURN, E2EE and enterprise abstractions remain out of scope.

## Buyer-relevant comparison

| Capability | Taawun today | Bubble | Glide | Softr | Airtable |
| --- | --- | --- | --- | --- | --- |
| First useful result | Three real curated templates and 11 modules feed an exact signed composer, but a new workspace encounters the full toolbox rather than one guided success path. | Templates/AI or scratch lead into an editor; publishing a web app is presented as a one-button action once issues are clear. | A 540-item gallery, including nonprofit and operations categories, explicitly promises a project jump-start. | Nearly 100 templates duplicate both interface and database into the workspace for immediate customization. | Operational reuse is record- and interface-centric; record templates create configured parent/sub-records or apply values to existing records. |
| Reusable structure | Exact component documents can be recovered from a verified track into a new actor-bound draft. There is no named, workspace-level component-document preset/library. | Reusable elements centralize elements, workflows and states; edits update all instances. | Linked Glide Tables reuse centrally managed data across apps; templates can also be copied into a team. | Templates, native blocks and data sources accelerate repeat app construction; recent Vibe Coding blocks have version restore/duplicate. | Record templates provide repeatable structured records and defined merge behavior. |
| Collaboration and roles | Architect/Maintainer/Viewer capabilities are enforced server-side. Invitation acceptance is safe, but the cockpit exposes a copy-once/session token handoff rather than a durable delivery/status flow. | Owners invite registered collaborators and choose view/edit, data and log access; multi-editor presence is visible. | Team invitations are email-delivered; app roles can limit views/actions, subject to documented data-source constraints. | The Users menu tracks invited/activated status, can resend invitations, and user groups constrain pages, blocks, actions and records. | Workspace/base/interface permissions, interface-only collaborators, link invitations, and access requests are first-class. |
| Trust and custody evidence | Differentiated: exact files, manifest digest, signature/key, workspace, origins, lifecycle, expiry and references are visibly attested; tamper/missing/expired evidence fails closed. | Security guidance, privacy rules, development/live separation and version controls establish platform trust, but the reviewed docs do not expose a comparable signed per-build receipt. | Roles, row ownership and hosted sharing establish access controls; the reviewed docs do not expose a comparable signed per-build receipt. | Server-side group/data restrictions establish access control; the reviewed docs do not expose a comparable signed per-build receipt. | Layered workspace/base/interface/field permissions and revision history establish platform controls; the reviewed docs do not expose a comparable signed per-build receipt. |
| Recovery and history | Authorized, bounded build history; immutable status/events; exact verified reopen; draft recovery creates a new track and preserves actor authority. | Branches, savepoints, restore and a who/when/what changelog are prominent. | Centralized team data and secure support links help continuity/support; the reviewed docs do not describe full-app branch history comparable to Bubble. | Vibe-coded blocks expose version history, restore and duplicate; this is narrower than Taawun's exact signed build record. | Record/automation revisions exist, while published interface recovery is more limited and permission-dependent. |
| Publication | Preview and publication are deliberately distinct. Domain proof, verified origins and review gates are strict, but the complete custom-domain journey remains externally unproven. | Development-to-Live deployment is described as effectively one click after issue resolution. | Every published app gets a `glide.page` URL; paid plans add a guided custom-domain flow. | Template apps publish from the studio to a subdomain or configured custom domain. | Interfaces can be published and shared to permissioned collaborators; some public sharing is plan-dependent. |
| Support and operations | Service health, request IDs, redacted history and supported lifecycle routes exist. Customers cannot yet create a concise support-safe diagnostic packet, and this lane has no configured application-admin authority for orphan cleanup. | Collaboration/log access and version history give maintainers a familiar diagnosis path. | A time-limited read-only Support Link gives support app visibility without full access. | Users expose status, last seen on eligible plans, invitation resend, magic-link lifecycle and deletion/deactivation controls. | Admin and permission surfaces enumerate users, access and interface ownership; enterprise admin operations are explicitly separated. |

## Primary-source evidence

### Bubble

- [Reusable Elements](https://manual.bubble.io/help-guides/design/elements/reusable-elements)
  documents reusable elements/workflows/states, centralized updates and
  consistency.
- [Collaborators](https://manual.bubble.io/help-guides/maintaining-an-application/collaboration)
  documents invitation, per-collaborator access and multi-user editing.
- [Version control](https://manual.bubble.io/help-guides/maintaining-an-application/version-control)
  documents Main/Live branches, custom branches, savepoints, restoration and a
  who/when/what changelog.
- [Publishing a web app](https://manual.bubble.io/help-guides/publishing-your-app/web-app)
  documents the issue-checker gate and one-button Development-to-Live deploy.

### Glide

- [Template gallery](https://www.glideapps.com/templates?category=Non-profit)
  currently presents 540 templates, including nonprofit, operations and portal
  categories, as a way to jump-start a project.
- [Multiple roles](https://help.glideapps.com/en/articles/9953555-does-glide-support-multiple-roles)
  documents Admin/Reader-style access and the relevant data-source limits.
- [Inviting team members](https://help.glideapps.com/en/articles/10658980-inviting-new-members-to-your-team)
  documents email invitation and acceptance.
- [Glide glossary](https://help.glideapps.com/en/articles/11025531-the-glide-glossary)
  documents Linked Glide Tables and read-only Support Links that expire after 30
  days.
- [Custom domains](https://help.glideapps.com/en/articles/9421250-troubleshooting-custom-domains)
  documents the default `glide.page` address and paid custom-domain wizard.

### Softr

- [Create an App From a Template](https://docs.softr.io/start-here/create-an-app-from-a-template)
  documents nearly 100 templates and one-step duplication of interface plus
  database for immediate customization.
- [User Groups](https://docs.softr.io/user-groups-and-permissions/user-groups)
  documents page/block/action controls and server-side conditional filtering.
- [Users Menu](https://docs.softr.io/add-and-manage-users/users-menu) and
  [automated invitations](https://docs.softr.io/add-and-manage-users/automated-user-invitations)
  document invited/activated state, resend, magic-link handling and automated
  invitation delivery.
- [Workflows](https://docs.softr.io/workflows/workflows) documents native triggers,
  actions, templates and permission-aware app/database integration.
- [Vibe Coding](https://docs.softr.io/vibe-coding) documents block-level version
  history, restore and duplicate.

### Airtable

- [Permissions overview](https://support.airtable.com/docs/airtable-permissions-overview)
  and [base permissions](https://support.airtable.com/docs/base-permissions)
  document workspace/base/interface roles and read-only through creator access.
- [Managing and sharing interfaces](https://support.airtable.com/docs/managing-and-sharing-interfaces)
  documents collaborator invitations, invitation links, access requests and
  permission management.
- [Using record templates](https://support.airtable.com/docs/using-record-templates-in-airtable)
  documents repeatable parent/sub-record creation and field-aware application to
  existing records.
- [Admin user access](https://support.airtable.com/docs/managing-user-access-to-workspaces-and-bases)
  documents permission changes, removal and deactivation as distinct operations.

## Ranked candidate slices

ICE uses 1–10 Impact, Confidence and Ease values; displayed score is
`Impact × Confidence × Ease / 10` (maximum 100). Scores are directional and
must be replaced by customer evidence after private-beta trials.

| Rank | Candidate slice | Impact | Confidence | Ease | ICE | Value / effort | Existing dependencies |
| ---: | --- | ---: | ---: | ---: | ---: | --- | --- |
| 1 | Signed Starter Path: one obvious empty-workspace route from real template to exact signed preview, history confirmation and Viewer-review action | 9 | 9 | 8 | 64.8 | Very high / low | Catalog, component editor, preview, receipt, history, People/invitations |
| 2 | Honest workspace next actions: derive recent builds, pending claims and role-valid next actions from real server state | 7 | 9 | 8 | 50.4 | High / low | History summaries, domain claims, current role/capabilities, dashboard surface |
| 3 | Review handoff packet: copyable track reference plus Viewer invitation/recovery guidance, without public or bearer artifact links | 8 | 8 | 7 | 44.8 | High / low-medium | Invitations, People, track reopen, signed-file authorization |
| 4 | Customer-generated support snapshot: redacted workspace/track/request/version/status evidence that a customer can copy for support | 7 | 8 | 6 | 33.6 | Medium-high / medium | Redacted history, events/status, request IDs; no admin authority required |
| 5 | Named reusable component-document starters: save, name and reapply bounded documents within a workspace | 8 | 7 | 5 | 28.0 | High / medium-high | New bounded persistence/list/authz contract; exact canonical documents |

### Why the other slices are not first

- Named starters address a real reuse gap, but require a new persistence and
  authorization contract. Verified-track draft recovery already supplies safe
  reuse for the first private-beta cohort.
- A full review packet becomes more valuable after the first signed result is
  reliably reached. The Signed Starter Path can expose the existing Viewer invite
  action now without inventing public links or email delivery.
- A customer support snapshot is commercially useful and safer than granting
  operator access, but it does not solve first-session comprehension.
- Dashboard expansion should follow the same evidence model as the Starter Path;
  it must never synthesize activity or imply publication readiness.

## Recommended Next slice: Signed Starter Path

### Job to be done

When a community Organizer creates an empty workspace, they need to turn one
real use case into a signed, recoverable artifact and hand it to a read-only
reviewer quickly enough to understand why Taawun is different.

### Smallest source-fit implementation

Use only existing server primitives. Present an empty-workspace, role-aware path:

1. **Choose a real template** from the current three-item catalog; select nothing
   implicitly and preserve catalog loading/error/retry behavior.
2. **Customize one allowed component** using the existing declared/custom field
   editor and exact bounded JSON validation.
3. **Create and explain the signed preview** using the existing preview and
   complete receipt verifier. Explain in plain language that it is bound to the
   selected workspace and exact documents, and that preview is not publication.
4. **Confirm continuity and offer review** by linking to the newly created real
   Build History entry and offering the role-valid next action. Architects may
   invite a Viewer; Maintainers may share the track reference with an existing
   member or ask an Architect; Viewers remain inspect-only. Keep invitation tokens
   session-only and require a trusted delivery channel.

Do not add analytics, sample activity, public artifact links, automatic template
selection, email infrastructure, publication bypasses, or new authority types.

### Source-fit constraints before authorization

Implementation's read-only dependency audit confirmed that this slice needs no
new backend contract, migration, persistence layer or authority type, provided
the authorization brief fixes these boundaries:

- “Empty workspace” means a selected, authorized workspace whose history request
  completed successfully with zero rows. It does not mean no workspace exists,
  history is loading, or history failed.
- Invitation activity and tokens are deliberately session-only. The path may
  create/copy an invitation, but must not restore or claim durable “invited”
  progress after reload. Durable invitation history is outside this slice.
- Current workspace orchestration starts history/domain recovery only after
  People succeeds. These reads must become independently guarded so a People
  outage does not hide available history; role-sensitive actions remain disabled
  until membership resolves.
- A history summary is discovery evidence, not artifact verification.
  `previewPresent` must never complete the signed step. After reload, reopen only
  the selected/newest applicable track with verified-preview inclusion and run
  the complete existing verifier. Do not create an N+1 reopen/events flow across
  the history list.
- Preview verification and history confirmation are separate outcomes. If preview
  succeeds but history refresh fails, report “signed preview verified; history
  confirmation pending” with a retry. Do not mark both complete.
- Demonstrating meaningful customization after reload requires exact verified
  track documents compared with current catalog defaults; redacted summaries do
  not contain sufficient evidence.

### Control-room authorization refinements

- Starter progress is **derived ephemeral UI state only**. Do not persist a
  completion flag, introduce analytics state, or add a new backend record. On
  reload/fresh login, derive signed progress only by reopening one applicable
  authorized track through the complete existing verifier.
- Existing behavior for a workspace with one or more history records is unchanged.
  The starter path is an empty-workspace activation aid, not a replacement for
  Build History, direct composition, domain recovery or the five workspace tools.
- The five-person comprehension study is a post-deploy private-beta validation
  activity. It must use real participants, must not be fabricated, and does not
  block the technical deployment gate.
- Fresh Codex Browser evidence is not required and must not be claimed. Promoted
  real Chromium plus independent live API, Railway log/metric and source evidence
  remain the technical acceptance gate.

### Falsifiable acceptance criteria

1. A fresh Architect in a selected, authorized, successfully loaded zero-history
   workspace sees one primary activation path and can reach an actively verified
   preview in at most eight intentional control activations after workspace
   selection, excluding text entry and network wait. The counted path is recorded
   explicitly in the promoted-browser assertion.
2. All three live templates and their allowed real modules remain available; no
   template or component is selected without an explicit user action.
3. The preview request contains the exact edited documents. Manifest/file digests,
   workspace, signature, lifecycle, expiry and configured preview origin pass the
   existing verifier before the path reports success.
4. Success copy names the concrete value (“exact workspace-bound signed preview”),
   distinguishes preview from publication, and exposes two real next actions:
   reopen the new history record and perform the role-valid review handoff. It
   never calls session-only invitation state durable.
5. Reload and fresh login derive signed progress only by verifying one applicable
   authorized track from workspace history, not from a summary flag or local
   completion state. A different workspace or principal cannot inherit the prior
   path state, draft, receipt, track or invitation token.
6. Viewer sees the exact authorized build/receipt but cannot edit or build;
   Maintainer can recover the exact draft only into a new actor-bound track.
7. Catalog, validation, preview, history, signed-file and invitation failures keep
   the last valid draft and trusted preview, label stale/error state honestly, and
   offer a bounded retry. A stale preview never enables publication.
8. Keyboard order, live announcements, 320/400 px layouts and effective 200%
   reflow remain green in promoted real Chromium; no target under 44 px is added.
9. After technical deployment acceptance, run a real five-person private-beta
   comprehension check. At least four participants should complete a customized
   signed preview within five minutes and accurately answer: what was signed,
   what workspace it belongs to, whether it is published, and what the next
   reviewer action is. Record failures without coaching; do not fabricate results
   or use this market-validation study to block the technical deployment gate.
10. People, history and domain recovery are independently guarded. A People error
    cannot conceal successfully loaded history, while capability-sensitive actions
    stay disabled until membership resolves. Only the selected/newest applicable
    track is reopened for verification.

### Technical QA charter

- Fresh Architect: register/login/workspace, each of the three templates across
  cases, declared/custom document edit, exact signed preview, receipt comparison,
  history reopen and Viewer invitation.
- Negative/recovery: catalog empty/incomplete/late, invalid JSON/Unicode/bounds,
  preview 422/502/offline, history retry, signed-file 401/expiry/tamper, invitation
  rejection and copy failure. Separately fail People and history to prove their
  recovery paths do not mask each other. Preserve last valid draft and trusted
  preview.
- Roles/isolation: pre-invite Viewer 403, post-invite inspect-only, Maintainer
  actor-bound clone, workspace A→B delayed responses, logout→new principal, no
  token or path-completion carry.
- Trust: exact manifest/document/file digest/signature/origin/lifecycle binding;
  stale or altered evidence cannot say Verified or enable publication.
- Accessibility/responsive: keyboard-only completion, focus recovery, announced
  step/error/success changes, 320/400 px and effective 200% reflow.
- Live acceptance: low-rate API probes and Railway correlation on the promoted
  deployment, zero unexplained 5xx, promoted real-Chromium pass, supported
  synthetic cleanup, and explicit DNS/Bazaar fixture limitations.
- Count the fresh empty-workspace path from workspace selection to active Verified
  output. It must require at most eight intentional control activations, excluding
  text entry and network wait. Record the exact counted actions.
- Regress an existing non-empty workspace: its current composition, history,
  verified reopen, role controls and domain readiness behavior must remain
  unchanged and must not be replaced by starter completion UI.

## Operational monitoring snapshot

At the research-cycle readback on 2026-08-19, Railway still reported deployment
`d446f826-bb48-4005-afc4-98e7fac8e046` as `SUCCESS`. The bounded 93-request sample
contained 71 `2xx`, 22 intentional `4xx`, and zero `5xx`; p95 was 66 ms and p99
88 ms. Over the sampled hour CPU peaked at 0.0034 vCPU and memory at 0.0516 GB.
No new synthetic fixture was created.
