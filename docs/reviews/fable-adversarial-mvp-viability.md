---
type: review
title: Fable adversarial MVP viability review
status: complete
reviewed_application_commit: 4a9b1c68044e0324105fddccfaf72810d502626c
evidence_head: 0d6d5fb133cd3a672a5f2052891629f123df359d
railway_deployment: f8e86558-b90a-43a1-8599-245a8c30684d
runtime_image: sha256:85726c4a1411319f10be0420db3b1f96237f3082012d9ab98accc9bef13a1eb8
qa_ledger_sha256: 013AC4D84CC5B38D823505861A07B6E4FD1AE6F8D8AD76E5F0FCA9722B288A2B
completeness_audit_sha256: C3C60EAF4DA7CD92A1D73BE748C36CFB719F990449F57ADB193E4F93FBBBF95A
invocation_started_utc: 2026-08-21T19:56:48.0998964Z
one_review_invocation: true
cli: Claude Code 2.1.233
model: claude-fable-5
invocation_result: success
permission_scope: Read, Glob, and Grep only; no session persistence or browser
---

# Verdict

**CONDITIONAL GO** — for an invited, hand-held, free-or-nominal-fee **design-partner private beta only**, with this exact allowed promise:

> *"Taawun lets a small Muslim community organization (mosque committee, student association, charity circle) compose a signed, tamper-evident community page from three approved templates and eleven approved components, review it in a verified sandboxed staging preview, collaborate under Architect/Maintainer/Viewer roles with real invitations and offboarding, record Shura decisions, and rehearse ethical-finance workflows in an explicitly no-money sandbox. Public serving on the community's own domain, marketplace purchases, live money, and scholar-approved rulings are not yet available and are stated as pending."*

This is **not** a GO for charging self-serve customers, promising public hosting, or claiming a working shared-data community app. The commercial question — will anyone pay — is unanswered by design: the five-person study is `not-started`.

**Falsifiable conditions that flip the verdict:**
- **Upgrade to GO (paid beta):** (1) one controlled customer-DNS claim → TXT verify → activation → exact-Host public serving → expiry/replacement/rollback smoke passes live; (2) the five-person comprehension/demand/willingness-to-pay study completes with genuine participants and shows first-session comprehension and directional willingness to pay; (3) operator provisions an audited admin authority and clears the known residue (workspace `47`, users `8`, `19`, inferred `30`).
- **Downgrade to NO-GO:** the deployed commit drifts from `4a9b1c6` without re-acceptance; any fresh live pass finds a P0 (signed-file auth bypass, receipt-trust regression, whole-service outage recurrence); or marketing exceeds the claim boundaries in the Claim audit below.

## Evidence and limitations

**Reviewed:** PASSOFF.md, conductor/product.md, conductor/feature-parity.md, the full 1,476-line QA ledger (`docs/reviews/continuous-live-qa.md`), the full 757-line completeness audit, and current application source including `src/cmd/main.go`, `src/web/index.html`, `src/web/index.test.mjs`, `src/web/AGENTS.md`, and backend packages (`conductor`, `domains`, `financial`, `mcp`, `oauth`, `artifacts`). The QA ledger's final section accepts exactly application commit `4a9b1c6…` / Railway deployment `f8e86558…` / image `sha256:85726c4a…` with no open P0/P1 application finding, matching the stated review identity. Source I inspected is consistent with that acceptance: the lifecycle controls, publication `servingState` truth model, 90-day replacement/rollback, and `409 owned_workspaces_remaining` contract all exist in current code (`src/web/index.html:771-789, 3148-3446, 5007`; `src/cmd/main.go:118-122, 314-323`).

**Not proven, and I state this plainly:**
- **No external DNS/TLS evidence exists.** Railway's domain inventory shows only the Railway hostname; every verification attempt correctly returned `422`. Verified-domain activation, Host-bound public serving, expiry shutoff, and rollback are proven only by controlled-clock/service/promoted-Chromium tests, never live.
- **No Bazaar creator/reviewer/purchase fixture exists.** The live catalog is truthfully empty; review transitions need a real admin reviewer; the fixture is externally blocked and was correctly not fabricated.
- **The five-person study is not started.** Zero participants, zero demand or willingness-to-pay evidence. It must not be counted as technical evidence, and the ledger does not.
- Federation, TURN/ICE, E2EE device-key lifecycle, live settlement, scholar approval, and a real external MCP-client authorization are all absent, per PASSOFF's own "do not claim finished" list.
- **My own limits:** read-only tools cannot recompute the stated SHA-256 values; I read the ledger at 1,476 physical lines versus the stated 1,475 (almost certainly EOF-newline accounting, but I note it). I did not execute tests or touch the live site; live claims rest on the ledger's recorded evidence, which is internally consistent, request-ID-correlated, and adversarially structured.

## Proven readiness

- **Signed-artifact trust chain, live:** exact `manifestJson` SHA-256 recomputation, Ed25519 receipt binding to workspace/actor/components/origins/lifecycle/expiry, and — after a real P1 was caught and fixed — byte-exact verification of the *executed* runtime (34,083 LF bytes, digest `2837…d24a`) before iframe/receipt commit. Negative matrices cover tampered digests, expired authorization, missing evidence, and wrong-track substitution.
- **Component-document exactness:** live round-trip of custom scalar/array/object/emoji/Unicode data with deterministic `422` rejection of surrogates, duplicate keys, reserved keys, non-canonical numbers, control characters, and oversize documents; idempotency keys survive denial.
- **Role safety, live:** Viewer inspect-only, Maintainer build-without-publish, Architect-only invite/remove/revoke/publish; generic `403`s with no existence oracles; member removal, invitation revoke, and domain revoke now in the cockpit with confirmation/focus discipline (deployed `4a9b1c6`, live-green).
- **Lifecycle honesty:** self-delete returns non-destructive `409 owned_workspaces_remaining` with race serialization proven via independent-handle `BEGIN IMMEDIATE` tests; password rotation invalidates all sessions live; publication expiry yields `active=true` + `servingState=expired` with all "Active/Verified" wording removed (controlled-clock proven).
- **Shura → Finance continuity, live:** approved decision ID displayed and carried; server rejects wrong/missing decisions (`409`/`422`); seven sandbox flows with hash-linked audit and no settlement language.
- **Operations:** zero unexplained 5xx across every bounded sample, p95 latencies in tens of milliseconds, clean starts, disciplined synthetic-fixture cleanup with `204`/`401` readbacks throughout the ledger.

## Severity-ranked findings

**No P0 findings.** I looked for one and did not find one; the previously open P0 (publication lifetime) is closed in current source with honest serving-state truth, and I will not invent a P0 to fill the rank.

| Severity | Finding | Concrete evidence | Customer consequence | Smallest honest next action |
|---|---|---|---|---|
| P1 | The delivered "community app" has unproven shared-data utility: generated-card controls largely record tab-local/illustrative data; E2EE workspace key has no provisioning lifecycle; TURN/ICE absent; no live two-device sync demo exists | Audit parity row citing `src/pkg/artifacts/html.go:293-303`, `render.go:233-234`; QA: registration control "reported a tab-local preview save"; PASSOFF deferrals 1–2; feature-parity ship gate "Live two-device signed relay demonstration" never recorded | A community that publishes expecting members to see each other's registrations may get a beautiful signed page whose interactivity is per-visitor | Scope the beta promise to "signed page + staging collaboration" in writing, or run one live two-browser relay sync demonstration before promising shared records |
| P2 | Cockpit publish path still defaults to a 24-hour authorization (`buildPayload` omits `ttlHours`; handler defaults 24 at `composition_handler.go:153-154`); the 90-day path exists only as post-activation "Prepare 90-day replacement" | Source read; `src/web/index.html:3994-3998, 5007` | First customer publication goes dark in a day unless a second action is taken (currently moot — publish is DNS-blocked — but it is the first thing the DNS smoke test will hit) | Offer the 90-day TTL at first publication, or fold it into the DNS-smoke runbook |
| P2 | Advertised revenue-split flow cannot complete from the cockpit (no allocation inputs; `createQuest` never sends `allocations`, which the server requires) | `src/web/index.html:5794-5810`; `src/pkg/financial/azoa.go:555-580` | Honest server `422`, but an advertised flow dead-ends | Hide/label the flow "API only" until the allocation form ships |
| P2 | Shura proposals and finance quests have no workspace recovery index (paste-ID only); MCP/OAuth — the differentiator — is invisible in the cockpit; OAuth consent permits a predictably invalid zero-workspace submit | `questLookup` input `index.html:897`; sole MCP mention is a caption at `:651`; `oauth/http.go:413-423` | Fresh login or device loss makes governance/finance work undiscoverable; no beta operator can find the LLM control plane | Bounded redacted list contracts (already specified in the audit); read-only MCP connection guidance |
| P2 | Operator residue and authority gap: orphaned workspace `47`, users `8`/`19`/`30` uncleanable; no production admin account, backup/restore drill, or customer-domain runbook | QA ledger cleanup sections; feature-parity Deployability gate | Beta support requests cannot be honored without raw DB access, which is rightly prohibited | Provision one audited admin authority and write the runbook before first customer |
| P2 | Relay is single-process with process-local replay tracking; scaling beyond one instance breaks one-use tickets | PASSOFF deferral 2 | None at current load; sticky-routing constraint must be documented | Keep the single-instance constraint in the operator runbook |
| Market | Zero demand evidence: five-person study not started; Bazaar has no listing, creator, or buyer; no pricing signal of any kind | Ledger: "The five-person study remains `not-started`"; empty published-only catalog | The product may be technically sound and commercially unwanted | Run the study with real participants before any pricing decision; nothing technical substitutes for it |

## Claim audit

**Trust/custody/retention.** The deployed landing copy is already honest and is the ceiling: *"Taawun keeps account identity, signed artifacts, and required audit or control records. Community app records remain local-first. Relay transit is encrypted… Retention follows the record's purpose, including required audit history after account deletion"* (`index.html:465`). **May say:** signed tamper-evident artifacts; community records live in the browser; encrypted relay transit; no funds custody. **Must not say:** "Taawun does not hold your data" or "zero data custody" unqualified (product.md's thesis wording) — Taawun durably holds identity, signed artifacts, Shura/finance/audit chains, OAuth grants, and retains audit records after account deletion; the E2EE claim that "the platform cannot decrypt" is not yet earnable because key enrollment/rotation doesn't exist and the workspace key is in-memory only.

**Compliance/Islamic jurisprudence.** **May say:** a tagged seed reference corpus with citations, and receipts that literally print "Reference-only; not scholar approval" (live-verified). **Must not say:** "fiqh-compliant," "scholar-authored guardrail corpus" (product.md), "Shariah-certified," or anything implying religious authority. No qualified scholar has approved anything; the ethics audit is a keyword-class engine plus seed references.

**Finance/AZOA.** **May say:** durable sandbox orchestration of seven vetted flow shapes with hash-linked audit and Shura gating; explicitly "no funds, no custody, no settlement" (the cockpit already says this). **Must not say:** payments, escrow that settles, Zakat calculation as a religious determination, or "federated financial engine" — federation is a signed-envelope boundary with no durable inbox/outbox and no second node.

**Low-code/vibecoding/MCP.** **May say:** a hosted OAuth-protected MCP control plane exists with typed discovery/audit/compose/inspect tools over Streamable HTTP, bounded to three templates and eleven primitives with structured component documents. **Must not say:** "describe any app and get a working product" (product.md's Lovable comparison), "your LLM deploys it" (the MCP publish tool is honestly unregistered — `Publisher` is nil in `main.go:202`, and the tool self-reports `publishingAvailable:false`), or that external MCP clients are proven — no real external-client authorization smoke test has run, and the cockpit gives users no way to discover MCP exists.

**Bazaar.** **May say:** a durable marketplace substrate with anti-gharar disclosures and published-only discovery exists behind the API, and the beta catalog is intentionally empty pending reviewer and DNS authority. **Must not say:** "marketplace," "buy templates," "creator revenue" as present-tense offerings — no listing has ever been created, reviewed, published, test-driven, or purchased, and a draft cannot even be deleted.

## Journey continuity

- **Landing → account → workspace:** **live-proven.** Plain-language H1, honest custody caveat, keyboard-navigable tabs, 12-char password enforcement, honest zero-workspace and zero-history states.
- **Template/components → custom component documents:** **live-proven.** Three templates/eleven modules always explicit; exact JSON round-trip including Unicode/emoji; adversarial input bounds; last-valid-draft retention.
- **Signed preview/files/receipt:** **live-proven at the strongest level in the product** — executed runtime bytes are digest-verified against the signed manifest before the receipt commits.
- **Build History and actor-bound restore:** **live-proven,** including bounded pagination, redacted summaries, Maintainer clone into a new actor-bound track, and fresh-login recovery.
- **Architect/Viewer/Maintainer safety, People/invitations, offboarding:** **live-proven,** including post-removal access loss and session-only invitation honesty.
- **Shura decision → finance sandbox:** **live-proven** for donation-class flows; revenue split is cockpit-discontinuous; fresh-login recovery of proposals/quests is paste-ID only (**commercially discontinuous**).
- **Domain claim/verification:** claim, TXT issuance, pending rediscovery, revoke — **live-proven**; verification onward — **externally blocked** (no controlled hostname exists anywhere in Railway).
- **Publication lifetime/replacement/rollback and Host serving:** **promoted-test-only** (controlled-clock, Host handler, real-Chromium). Well-designed and honest, but zero live activations have ever occurred.
- **Bazaar purchase:** **externally blocked** (reviewer + DNS); buyer surface truthfully empty.
- **First-session comprehension/demand:** **not started** — the only journey segment with no evidence of any kind.

The through-line: everything a customer can reach today ends at a verified *staging* preview shared with invited members. Nothing has ever been publicly served. That is a coherent design-partner offering only if sold as exactly that.

## Prioritized next actions

**Operator/external proof (highest decision value — unblocks the core promise):**
1. Supply one controlled customer hostname and run the already-written smoke: claim → TXT verify → publish (use 90-day TTL) → exact-Host serve → replacement → rollback → revoke shutoff. This converts the largest promoted-test-only block into live truth.
2. Provision the audited application-admin authority; clean workspace `47` and users `8`/`19`/`30`; write the backup/restore and customer-domain runbooks.
3. Authorize one Bazaar reviewer and run the sanctioned synthetic listing/test-drive/purchase fixture.

**Real-user market validation (equal priority — the actual commercial question):**
4. Run the five-person comprehension/demand/willingness-to-pay study with genuine participants. Do not price, publicize, or scale before it. No technical work below substitutes for this.

**Product work (only what changes the beta's honesty or safety):**
5. Decide the shared-data promise: either demonstrate live two-device relay sync or put "per-visitor data; shared records coming" in the beta agreement.
6. Default first publication to the 90-day TTL (one-line-class change already contract-supported).
7. Hide or label the revenue-split flow; add the bounded Shura/finance recovery lists per the audit's existing contracts; add read-only MCP connection guidance.

The engineering culture evidenced here — fail-closed defaults, refusal to fabricate DNS/reviewer/scholar/study evidence, adversarial self-QA that caught and fixed a real runtime-trust P1 — is itself a sellable asset. The honest boundary in feature-parity.md is correct: ship the workflow, never the fabricated authority. Hold that line in the sales conversation and this is a defensible design-partner beta today; cross it and the same ledger that proves readiness becomes the record of the overclaim.
