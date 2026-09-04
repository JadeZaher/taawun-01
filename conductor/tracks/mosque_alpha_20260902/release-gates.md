---
type: test-plan
title: Mosque Alpha Release Gates
status: proposed
---

# Mosque alpha release gates

## Decision levels

- **Technical alpha:** scope/ownership, required implementation, assembled local
  verification, accessibility, security, reproducible promotion, recovery,
  monitoring, support, and hosted-public lifecycle all pass.
- **Mosque-facing alpha:** technical alpha plus the full fresh production journey,
  human-usability thresholds, supported cleanup, and zero open P0/P1 issues.
- **Custom-domain alpha:** mosque-facing alpha plus genuine DNS and trusted TLS.

Current verdict: **NO-GO**. The live deployment is a technical preview until the
unchecked work in `plan.md` and this evidence ledger passes.

## Gate ledger

| Gate | Required observable evidence | Fails when |
|:---|:---|:---|
| Scope and ownership | Signed scope, owners, registration-data matrix, hidden/sandboxed exclusions | Money/religious authority is ambiguous or PII/support has no owner |
| Authority correctness | Fault-injected invitation transaction, role matrix, removal and anti-replay | Invitation is consumed without membership or removed access persists |
| Guided product | Resumable shared draft, one-action stages, lossless Advanced, offline/provider fallback | Raw JSON/technical vocabulary is required or work is lost |
| Useful content | Fully typed Mosque Essentials signed round trip | Published page contains hard-coded demo values |
| Hosted public site | Real Taawun HTTPS `/s/{slug}`, exact signed artifact, replace/rollback/unpublish | Customer DNS is required or control-plane data leaks |
| Registration | Real submit/read/export/close/delete with privacy/abuse/idempotency tests | Responses are tab-local, cross-tenant readable, or unbounded |
| Real-server E2E | Fresh two-context journey crosses real handlers, stores and artifact files | Fixture server substitutes for any release-critical stage |
| Accessibility | 0 critical/serious automated findings plus manual assistive-tech record | Any WCAG A/AA blocker or inaccessible critical action |
| Human usability | >=4/5 success and comprehension, time thresholds, pain ledger | Technical assistance is required or P0/P1 pain remains |
| Security | Independent authority review, scans, safe headers/logs, zero accepted high/critical | Secret/PII leakage or exploitable/unanalyzed high-risk finding |
| Promotion | Clean tagged source and exact commit/build/image/SBOM/rollback evidence | Deployment cannot be traced to immutable source |
| Readiness/soak | Dependency checks, graceful restart, 24h low-volume soak, no unexplained 5xx | Static-only health or recurring crashes/errors |
| Recovery | Encrypted off-platform backup and isolated restore inside RPO/RTO | Persistent disk is the only copy or restore is unproved |
| Monitoring/support | Synthetic alert reaches two responders; support and incident paths work | No responder, no customer route, or diagnostics expose secrets |
| Production lifecycle | Publish, register, edit, rollback, revoke/unpublish, clean up | Synthetic production state remains or evidence uses unsupported cleanup |
| Optional custom domain | Real public DNS and browser-trusted TLS from independent network | Host override, fake DNS/TLS, or uncontrolled hostname |

## Performance and reliability targets

- Public page p75 LCP < 2.5 seconds across 20 cold-cache navigations in Chromium
  at 390x844, 4x CPU slowdown, 150 ms RTT, 1.6 Mbps down and 750 Kbps up; record
  each sample and nearest-rank p75. No navigation has an uncaught browser error.
- Authenticated alpha mutations reconcile safely after ambiguous responses; all
  critical creates/activations/submissions are idempotent or version guarded.
- A 10-minute 20-user smoke (80% public GET, 15% registration submit, 5%
  authenticated read/mutation) has nearest-rank p95 server response < 1 second;
  registration capacity remains correct under contention.
- RPO <= 1 hour, RTO <= 4 hours, encrypted backup retention 30 days.
- External liveness/readiness checks run each minute and alert after 5 consecutive
  failures; 5xx/latency/restart/disk checks evaluate every 5 minutes; backup age
  alerts at 90 minutes and certificates at 21 days. Each alert reaches the two
  named responders within 10 minutes.

## Exact production acceptance

1. Promote a clean tagged artifact and record only safe deployment identifiers.
2. Register a fresh Architect and create a mosque workspace.
3. Build Mosque Essentials through Guided mode, close/reopen, and resume.
4. Publish to a Taawun-hosted URL; anonymously verify exact content on mobile.
5. Invite and accept a Maintainer and Viewer; prove retry safety.
6. Submit a registration twice with the same idempotency key and observe one row.
7. Trigger or await a consistent backup while that publication and response are
   active. Record its safe ID, completion, source commit/deployment, schema
   version, and timestamp; verify its age is inside the one-hour RPO.
8. Read/export as Architect; prove Maintainer, Viewer, and another workspace
   cannot enumerate or read the response.
9. As Maintainer, edit allowed site/event content and build; prove People,
   domain, publication, and registration-response mutations remain denied.
10. Delete the registration as Architect.
11. As Architect, activate the successor and prove
   old/new immutable review behavior.
12. Roll back once, remove Maintainer and Viewer, and prove old
    session/link/open-file denial.
13. Unpublish and prove public serving stops while authorized history remains.
14. Clean up through supported APIs and prove old sessions return `401`.
15. Restore that exact backup in isolation and verify integrity, expected counts,
    publication bindings/signatures, registration count/digest, and artifact
    digests without claiming public serving from the recovery origin.
16. Destroy the isolated recovery fixtures through the supported environment
    procedure and retain only the safe evidence ledger.

## Severity rule

- **P0:** authority bypass, secret/private-data exposure, unrecoverable data loss,
  false public state, unsafe consequential action, or unusable primary journey.
- **P1:** likely customer failure with no clear recovery, critical accessibility
  barrier, inconsistent registration/publication, or missing operational response.
- **P2:** bounded friction or polish issue with a documented safe workaround.

No P0/P1 may be waived by a deployment deadline. External blockers are recorded
as blocked and the affected optional feature remains unavailable.
