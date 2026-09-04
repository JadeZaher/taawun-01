---
type: execution-plan
title: Builder Review Links and Live Delivery QA Plan
status: in-progress
---

# Builder Review Links and Live Delivery QA Plan

1. Document the protected review-link contract and the full promotion runbook.
2. Harden link parsing, automatic signed-preview reopen, stale-state disabling,
   workspace switching, expiry handling, and role boundaries.
3. Add deterministic source/browser coverage for the new handoff and retain the
   existing signed-file, principal, workspace, and publication race tests.
4. Run one integrated local package, browser, build, and diff verification sweep.
5. Deploy the exact working tree to Railway and wait for terminal status.
6. Run the live browser journey from signup through protected review, editing,
   revocation, and controlled-domain publication where genuine DNS control is
   available.
7. Append deployment evidence and a de-identified pain-point ledger to the
   runbook; convert every accepted remedy into a task below.
8. Run supported fixture cleanup, verify old sessions are invalid, and update
   the track status only when every non-external acceptance gate passes.

## Tasks

- [x] Add the landing-page builder demonstration.
- [x] Add a protected review-link control to an active signed preview.
- [x] Make a review URL automatically inspect and reopen the signed preview.
- [x] Add an authorized-workspace selection hint and retry after invitation acceptance.
- [ ] Add deterministic review-link browser coverage.
- [x] Expose explicit preview lifetimes and a direct immutable re-sign/reissue
  path for expired protected previews; preserve the old track and URL as expired.
- [x] Complete the integrated local release sweep.
- [x] Deploy and capture terminal Railway evidence.
- [ ] Complete the live organizer and Viewer browser journey.
- [ ] Complete genuine controlled-domain publication and revocation, or record
  the exact external blocker without claiming the gate.
- [ ] Apply accepted pain-point remedies and rerun the affected journey once.
  Workspace creation and custom-field continuity are fixed and promoted;
  preview-expiry recovery is implemented and awaiting promotion; the larger
  guided co-builder remains in `../guided_ai_cobuilder_20260902/`.
- [ ] Complete supported cleanup and final evidence reconciliation.

## Follow-on product remedies discovered by the journey

- [ ] Add an opaque, expiring, explicitly revocable review-session grant with
  immutable audit history; keep track/file membership checks independent.
- [ ] Neutralize active preview/share presentation at the exact server-derived
  preview-authorization deadline without trusting wall-clock rollback.
- [ ] Make invitation acceptance and initial membership creation atomic or
  safely reconcilable, including dependency-failure, retry, revoke race, and
  removed-member anti-reinstatement tests.
- [ ] Add keyboard-accessible component ordering that is signed and preserved
  across restored drafts, previews, replacement builds, and public delivery.
- [ ] Add a real-Go-server browser lane with isolated persistent storage; retain
  fixture Chromium for deterministic edge cases.
- [ ] Turn registration into the first real deployed submission primitive with
  validation, origin/CSRF and rate controls, consent/retention rules, organizer
  readback/export, and staging/production isolation.
- [ ] Route first-time and empty-workspace usability remedies through
  `../guided_ai_cobuilder_20260902/`: one recommended next action, progressive
  disclosure, safe Advanced mode, and typed assistant proposals.
