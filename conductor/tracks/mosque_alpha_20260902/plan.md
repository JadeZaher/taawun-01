---
type: execution-plan
title: Mosque Alpha Execution Plan
status: in-progress
---

# Mosque Alpha execution plan

## Critical-path rule

This plan is dependency ordered. Authoring lanes may run in parallel, but the
assembled-system test and independent approval begin only after their contracts
converge. Apply implementation fixes first, then run one integrated test/lint/
build/security sweep at the end.

Sequential effort for the smallest truthful slice is approximately 20–31
engineer-days. Three disciplined lanes can target 2–3 working weeks. Estimates
are planning ranges, not evidence of completion.

## G0 — Freeze the alpha boundary and ownership (0.5–1 day)

- [ ] Approve the Must/Should/Won't boundary in `spec.md` and hide excluded
  surfaces from the default path without deleting platform primitives.
- [ ] Name product/content owner, deployment operator, security approver,
  registration-data owner, support primary/backup, and optional DNS owner.
- [ ] Approve registration purpose, allowed questions, access, retention,
  export/delete, privacy copy, and incident-notification policy.
- [ ] Define alpha feature flags and honest public wording: guided mosque site
  and event registration, not arbitrary app generation or live finance.

**Pass:** every consequential surface and customer-data class has an owner;
excluded capabilities are hidden or unmistakably sandboxed.

## Lane C1 — Authority correctness (1–2 days; starts after G0)

- [ ] Make invitation acceptance and membership atomic or explicitly
  reconcilable; inject failure at every commit boundary.
- [ ] Prove idempotent retry, revoke race handling, role denial, session-version
  invalidation, removal, and anti-reinstatement.
- [ ] Complete the live Architect → Viewer invite/accept/review/remove path and
  supported cleanup with de-identified evidence.

**Pass:** no accepted-without-membership state; removed members cannot reuse an
accepted token, session, protected locator, open file, or relay path.

## Lane A1 — Persistent draft and guided product (5–8 days; starts after G0)

- [ ] Add workspace draft and user-progress storage/APIs with optimistic
  versions, bounded conflicts, and safe one-time session-draft migration.
- [ ] Implement the six-stage default Guided experience with one primary action,
  Back, Skip, Undo, Save and exit, resume, and lossless Advanced switching.
- [ ] Use plain-language state labels and progressively disclose raw component,
  receipt, domain, Shura, finance, Bazaar, MCP, and destructive controls.
- [ ] Add accessible signed section ordering and preserve it across every
  lifecycle adapter.
- [ ] Add deterministic recovery for refresh, offline/ambiguous responses,
  invalid data, concurrent edits, and provider outage.

**Pass:** a new organizer closes/reopens the browser and resumes the same valid
draft; no core task requires raw JSON or technical platform vocabulary.

## Lane B1 — Mosque Essentials (3–4 days; starts after draft schema)

- [ ] Add versioned typed fields/renderers for identity, address/contact,
  accessibility notes, prayer/Jumu'ah schedule, announcements, general event,
  registration, external donation-provider link, and footer.
- [ ] Remove fixed demo values from published output; retain old artifact
  readability through schema/renderer versioning.
- [ ] Validate links, dates/times, content limits, directionality, ordering,
  keyboard behavior, and mobile rendering.
- [ ] Require an IANA workspace timezone; sign effective local-date ranges,
  event offsets and zone labels; test DST gaps/folds and invalid zones.

**Pass:** every visible public value is organizer-owned typed data and the signed
round trip preserves the exact page without executable content.

## Lane B2 — Taawun-hosted publication (3–5 days; after B1)

- [ ] Add unique safe slugs, immutable activation history, replacement,
  rollback, unpublish, and explicit UI states.
- [ ] Serve `/s/{slug}` before control-host fallback through the same artifact
  verification pipeline as custom domains.
- [ ] Verify anonymous serving, asset paths, headers, caches, expiry, stale
  versions, collision/race behavior, and no control-plane information leakage.
- [ ] Preserve the slug across replacement/rollback; enforce 90-day maximum
  activation, renewal beginning 14 days before authorization expiry, 14/7/1-day warnings, operator
  failure alert, and bounded `410 Gone` after expiry.

**Pass:** a mosque receives a browser-reachable Taawun-controlled HTTPS URL
without DNS work; unpublish stops anonymous serving while authorized immutable
history remains.

## Lane B3 — Real event registration (5–7 days; after B1 and B2)

- [ ] Add form/submission storage, schema/version bindings, open/closed state,
  capacity/deadline, consent, retention, deletion/anonymization, and audit.
- [ ] Add public submit and authorized paginated list/export/status/delete APIs.
- [ ] Enforce exact active publication/form binding, content/field limits,
  origin/Fetch-Metadata, rate/abuse controls, idempotency, non-leaking tenant
  denial, CSV-injection defense, and staging/production isolation.
- [ ] Add organizer inbox/close/export/delete UI and plain-language custody copy.
- [ ] Preserve one form series across compatible successor/rollback revisions:
  only the active revision accepts, prior responses remain labeled/readable,
  capacity and close state never reset, and inactive revisions reject writes.

**Pass:** duplicate submission is idempotent; capacity is transactional; another
workspace and Viewer learn nothing; deleted values do not remain in logs/audit.

## Lane A2 — Bounded AI co-builder (3–5 days; after A1 and B1)

- [ ] Implement `taawun.builder-proposal/v1`, allowlisted operations, version
  binding, validation-on-copy, rationale/limitations, and Apply/Edit/Reject.
- [ ] Ship deterministic proposal fixtures/local recommendations first; put a
  hosted provider behind an explicit feature flag and retention decision.
- [ ] Test malformed output, prompt injection, unknown fields, executable input,
  stale versions, secret minimization, cancellation, and provider outage.

**Pass:** assistant output cannot mutate authority or bypass the same draft and
catalog checks; disabling the provider leaves the complete manual path intact.

## Lane C2 — Release operations (2–4 days; after stable storage schemas)

- [ ] Separate shallow liveness from dependency-aware readiness.
- [ ] Define reproducible clean-tree/tag/image/SBOM promotion and rollback.
- [ ] Implement consistent encrypted off-platform backup across databases,
  artifacts, schema metadata, and separately inventoried signing configuration.
- [ ] Restore into isolation and meet RPO <= 1 hour / RTO <= 4 hours; verify
  integrity, counts, artifact digests, publication bindings/signatures, and
  registration read/export without pretending a different origin is public.
- [ ] Rehearse public disaster recovery separately with a predeclared recovery
  hostname and genuine route/DNS cutover and failback.
- [ ] Fix Railway replica/autoscaling count at one, prove the expected volume is
  mounted, and ensure deploy/rollback overlap cannot create two SQLite writers.
- [ ] Implement and test the supported operator-assisted account reset: verified
  second-channel contact, 15-minute single-use locator, rate limits, session
  rotation, value-free audit, and no operator access to the new password.
- [ ] Add external uptime/readiness, 5xx, latency, restart, disk, backup age,
  certificate, DNS, and registration-failure/abuse alerts with responders.
- [ ] Document incident severity, support contact, request correlation, customer
  notice, rollback, privacy incident, and postmortem procedure.

**Pass:** one restore succeeds and one synthetic alert reaches primary and backup;
persistent volume alone is never described as backup.

## G1 — Assembled-system verification (3–5 days; after all required lanes)

- [ ] Start the real Go service on an ephemeral port with isolated databases,
  artifact root, generated secrets, and two clean browser contexts.
- [ ] Automate fresh Architect signup/workspace → guided draft → signed preview →
  hosted publish → Maintainer and Viewer invite/accept → anonymous visitor/
  registration → Architect read/export and role/cross-workspace denial →
  Maintainer site edit/build plus authority denials → registration delete →
  Architect replace/rollback → remove both members → unpublish → supported
  workspace/account cleanup.
- [ ] Cover refresh/resume, wrong workspace, Unicode/oversize, stale versions,
  ambiguous responses, expired preview, dependency failure, cross-tenant access,
  and old session/link/file denial.
- [ ] Run automated WCAG checks and manual keyboard/screen-reader/200% zoom/400%
  reflow/320px/reduced-motion/forced-colors/contrast checks on the critical path.

**Pass:** all assertions cross real handlers/storage/artifacts; 0 automated
critical/serious accessibility findings and no A/AA blocker.

## G1b — Moderated mosque usability on technical-alpha staging

- [ ] Deploy the technically passing candidate to an isolated staging URL.
- [ ] Run the five-person moderated study, including a mosque organizer, and
  record time, next-action comprehension, state comprehension, wrong turns, and
  Advanced use without treating automation as human evidence.
- [ ] Fix every P0/P1 pain point, rerun its affected journey, and return any code
  change to G1 before freezing the final candidate.

**Pass:** >=4/5 publish without facilitator help or Advanced mode, median first
preview <=10 minutes, median public site <=15 minutes, and zero open P0/P1 pain.

## G2 — Freeze, run one final integrated sweep, and promote

- [ ] Freeze and commit the release candidate first; require a clean tree and
  record the exact commit before tests or image construction.
- [ ] Run all Go tests, browser/runtime suites, both production builds, diff/
  formatting checks, supported race tests, vulnerability/secret/dependency/
  container scans, and an independent authorization/security review once against
  that exact commit.
- [ ] Require zero failures, zero known exploitable high/critical findings, zero
  secrets, and zero unaccepted P0/P1 security issues.
- [ ] Build from that commit, record build/image/SBOM digests and rollback target,
  then tag or annotate the already-tested commit with its evidence.
- [ ] Deploy that exact artifact and wait for terminal success.
- [ ] Run the production acceptance and cleanup ledger in `release-gates.md` and
  `docs/builder-live-qa-runbook.md` against fresh synthetic fixtures.

**Pass:** the release decision is based on observed evidence mapped to the exact
deployment, not fixture tests or a dirty-tree upload.

## Optional custom-domain gate

- [ ] If custom domains are exposed in alpha, complete real authoritative/public
  DNS, browser-trusted TLS, exact Host, edit/replacement, rollback, and revoke
  using a controlled hostname and independent network evidence.

Failure here disables custom domains for alpha; it does not block the mandatory
Taawun-hosted URL. No simulation may be recorded as success.

## Lane ownership and merge discipline

- **Lane A — Guided product:** persistent draft, Guided/Advanced, assistant.
- **Lane B — Useful public site:** Mosque Essentials, hosted publication,
  registration.
- **Lane C — Trust and operations:** invitation atomicity, auth hardening,
  readiness, recovery, monitoring, and real-server harness foundations.
- **Integration/verification:** assembled journey and independent security/release
  approval after authoring lanes converge.

Lane A exclusively owns `src/web/index.html` until it is modularized. Lanes B/C
work through server packages and small adapters to avoid concurrent edits to the
existing monolith.
