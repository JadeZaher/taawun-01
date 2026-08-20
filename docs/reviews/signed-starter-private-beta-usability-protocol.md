---
type: research
title: Signed Starter Path five-person private-beta usability protocol
status: proposed-for-control-room-approval
date: 2026-08-19
source_checkpoint: c1e4aa5
deployment: 431bc9b3-21b2-4e12-a4e1-c2a4a1fbfc39
study_size: 5
study_state: not-started
---

# Signed Starter Path five-person private-beta usability protocol

## Decision and scope

This is a runnable 20–30 minute, lightly moderated usability and comprehension
study for the live Taawun Signed Starter Path. The user will arrange five genuine
participants and explicit consent. This document does not authorize recruitment,
contact, impersonation, session execution, application changes, analytics, DNS,
publication, Bazaar activity, real financial activity, or the use of real
community data.

The study is a post-deployment private-beta validation. It cannot reopen the
accepted technical gate by itself, prove market demand from five people, or
justify new platform primitives. It may identify evidence-backed friction in the
existing catalog, component editor, signed preview, receipt, Build History,
People, invitations, and supported lifecycle routes.

## Objective and falsifiable hypotheses

**Objective:** determine whether community organizers with varied technical
comfort can independently turn a new empty workspace into a meaningfully
customized, exact workspace-bound signed preview, recover it after a fresh login,
and hand it to a read-only Viewer while accurately understanding the trust and
publication boundaries.

The sample is deliberately small and directional. Each hypothesis is evaluated
against observed behavior and unaided answers, not moderator interpretation.

| ID | Falsifiable hypothesis | Pass rule across five participants |
| --- | --- | --- |
| H1 — time to value | A new organizer can reach an actively verified, meaningfully customized preview quickly. | At least 4/5 reach active verified preview within five minutes of the timed start, without coaching. |
| H2 — path salience and economy | The empty-workspace path is visibly primary rather than hidden inside the full toolbox. | At least 4/5 independently use the Starter Path's primary action before entering the generic composer and finish in no more than eight counted control activations; no participant requires a moderator to name a control. |
| H3 — trust comprehension | The receipt explains what is actually signed and what authority it does not confer. | At least 4/5 independently answer all four critical questions correctly: exact content, workspace binding, not published, and reviewer next action. |
| H4 — continuity | A returning organizer can find and verify the result without remembering or pasting its opaque track ID. | At least 4/5 sign out, sign in afresh, recognize the automatically derived applicable verified track and separate history confirmation, and reach its exact verified preview without assistance or a copied track ID. A manual reopen is required only when automatic recovery does not already complete it. |
| H5 — role handoff | An Architect understands the session-only Viewer handoff and the Viewer boundary. | At least 4/5 create the Viewer invitation, use the designated trusted handoff channel, and correctly state that Viewer may inspect but not edit/build. |
| H6 — error recovery | Recoverable failures do not cause users to discard the valid draft or mistake stale output for verified/public output. | Do not inject failures. If a natural error occurs, report `recovered opportunities / total opportunities` by error class and whether valid work remained. If none occurs, report `N/A`, never pass. No participant may call stale/unverified output published. |

The study-level comprehension gate passes only if the **same at least four of
five participants** both reach active verified preview within five minutes and
earn four critical-comprehension scores of `2`, with no critical safety, privacy,
cross-workspace, or false-verification incident. H2, H4, and H5 diagnose the
surrounding journey and are reported independently. H6 is conditional opportunity
evidence and is never included in an aggregate gate.

## Participant profile and screening

### Target mix

Recruit only through a user-managed, consented process. Do not recruit from this
task or contact anyone automatically.

- Five adults who have organized or operated a mosque, charity, student group,
  mutual-aid group, cooperative, or comparable community program in the last
  twelve months.
- All should have created or maintained at least one event, form, announcement,
  volunteer process, shared workspace, or community-facing page.
- Aim for varied technical comfort: two low, two moderate, and one high by the
  self-rating below. Prior no-code experience is useful variation, not a
  requirement.
- Use a desktop or laptop capable of running the current supported browser. Note
  assistive technology and accommodations with consent; do not exclude someone
  because they use them.
- The current cockpit is English-language. Participants must be comfortable
  completing this study in English; record this as a product limitation, not a
  judgment about the participant.

### Screening questions

Ask before scheduling. Store only the participant code and categorical answers.

1. In the last twelve months, have you helped run a community event, program,
   organization, or cooperative? What kind of work did you do?
2. Which of these have you personally created or maintained: registration form,
   announcement, event page, volunteer schedule, shared workspace, website, or
   app?
3. On a 1–5 scale, how comfortable are you configuring a new digital tool without
   step-by-step help? (`1–2 = low`, `3 = moderate`, `4–5 = high`.)
4. Have you used a no-code, website-builder, spreadsheet-database, or workflow
   tool? Which category? Do not collect employer or customer names unless the
   participant volunteers them and consents to recording them.
5. Are you 18 or older, able to use the study in English, and willing to use only
   fictional study data and a study-only credential?
6. Do you need an accessibility accommodation for the session? Record only the
   requested accommodation, not medical information.
7. Do you have any current employment, contracting, investment, or contribution
   relationship with Taawun or this codebase?

### Exclusion and conflict notes

- Exclude Taawun employees, contractors, code contributors, investors, current
  implementation/QA participants, and anyone who has already seen this exact
  study script. Record only the conflict category.
- Exclude minors and anyone unwilling to use exclusively synthetic data.
- Prior familiarity with low-code tools is not a conflict; use it to assign the
  technical-comfort mix.
- Do not ask about religious practice, beneficiary status, financial need,
  immigration status, health, donations, or other sensitive traits. Community
  operations experience is sufficient.
- Compensation, if the user chooses to provide it, must be disclosed before
  consent and must not depend on task success or favorable feedback.

## Required study preconditions

The study owner completes this checklist before scheduling the first session.
If any item is false, do not begin.

- [ ] Control room approved this exact protocol version and named a study owner,
  privacy contact, incident contact, and supported cleanup operator.
- [ ] Application remains `c1e4aa5` on Railway deployment
  `431bc9b3-21b2-4e12-a4e1-c2a4a1fbfc39`, or the protocol was explicitly reviewed
  again for a later deployment.
- [ ] One low-rate `GET /api/health` and one `GET /` return `200`; the deployment
  is terminal `SUCCESS`; the bounded Railway `>=500` query is empty or every
  entry is explained before the session.
- [ ] The operator has authenticated, supported application authority to delete
  the exact synthetic workspace, memberships, organizer account, and study
  Viewer account after each session. Lack of this authority is a hard blocker;
  never substitute raw database access or a fabricated JWT.
- [ ] Each session has a unique participant code (`P01`–`P05`), unique synthetic
  organizer identity, unique isolated synthetic Viewer identity, and an empty
  clipboard. Accounts and invitation tokens are never reused across sessions.
- [ ] The designated handoff channel is private to that session and will not log
  or retain the invitation token. The channel is a fixture, not a participant.
- [ ] Research notes have an approved access list and deletion date. Recommended
  default: delete raw session worksheets within 30 days after synthesis; retain
  only de-identified aggregate findings and the approved decision record.
- [ ] Screen/audio recording is off by default. If recording is separately
  approved and consented, password entry and token handoff are paused or masked,
  and the recording follows the same deletion date.

## Synthetic scenario and assigned data

Read the scenario exactly. Do not substitute a real organization, person,
beneficiary, donor, payment, domain, or credential.

> You are organizing a fictional neighborhood community meal for **Crescent
> Commons Collective** in **Example City**. You need a small workspace that can
> announce the event and collect attendance interest. Build a signed preview that
> another person can review. This is only a private-beta preview: do not publish
> it, claim a domain, enter real names or contact details, or use real financial
> information.

Use these exact fictional values when the interface asks for content:

- App name: `Crescent Commons Community Meal`
- Organization: `Crescent Commons Collective`
- City: `Example City`
- Announcement title: `Welcome, neighbors`
- Announcement summary: `Doors open at 6:30 PM. Fictional private-beta scenario.`
- Reference baseline: leave the catalog's prefilled study value unchanged. It is
  fictional fixture data and does not represent or ask about the participant's
  religious belief or practice.
- Workspace name: `Pxx Crescent Commons Study`, replacing `Pxx` with the assigned
  participant code.

The participant chooses the template and components. The task wording describes
the job rather than naming the controls. The expected source-fit choice is the
Community iftar template with Community announcements and Iftar registration;
record a different explicit choice rather than correcting it during the timed
task.

## Session schedule and exact tasks

The scored five-person method is lightly moderated because reliable timestamps,
ordered control counts, privacy intervention, token handoff, staff-fixture
acceptance, and supported cleanup are required without adding analytics. An
unmoderated pilot requires a separately approved protocol and separate analysis;
it cannot contribute to H1–H6, and missing H2 observations may never be backfilled
from memory or inference.

| Minute | Stage | Participant task | Moderator action |
| --- | --- | --- | --- |
| 0–3 | Consent and safety | Hear/read consent; ask questions; confirm synthetic-data rule. | Read the neutral consent script. Confirm recording state and abort contacts. |
| 3–5 | Brief context | Describe current community-operations workflow at a high level. | Ask only the three warm-up questions below. Do not show Taawun controls. |
| 5–8 | Account and workspace | Register with the assigned synthetic email and a study-only password; log in; create/select the assigned empty workspace. | Stop sharing/looking during password entry. Record account/workspace IDs only for cleanup. Confirm successful zero-history load; do not start the five-minute timer before it. |
| 8–13 | Timed Signed Starter Path | From the selected empty workspace, choose the fitting template, choose components for announcements and attendance interest, change the supplied fictional content, and create a signed preview. | Start `T0` when the authorized zero-history starter is ready. Count controls silently. Stop at active verified preview (`T1`) or five minutes. Do not coach. |
| 13–16 | Trust comprehension | Inspect the visible receipt/preview and explain what happened. | Ask the four critical comprehension questions verbatim. Record answers before probing. |
| 16–20 | Continuity | Confirm the new Build History item; sign out; sign in afresh; recognize the automatically derived applicable verified track and history confirmation; reach the exact verified preview without using a copied track ID. Reopen manually only if automatic recovery did not already do so. | Record automatic recovery, history-confirmation, and any manual-reopen timestamps separately. Never supply the track ID or control name. |
| 20–23 | Viewer handoff | Invite the assigned synthetic study Viewer and use the designated trusted channel for the session-only acceptance token. Explain what the Viewer should do and what they may change. | Never copy the token into research notes. A disclosed staff fixture accepts it in an isolated session; this fixture is not a participant and its actions are excluded from participant scoring. |
| 23–27 | Debrief | Answer post-task questions and distinguish usefulness from willingness to pay. | Ask neutrally; do not pitch features or defend the product. |
| 27–30 | Cleanup | Confirm the session is ending and synthetic records will be removed. | Run the supported cleanup checklist, verify old tokens return `401`, and perform the post-session health/log check. |

### Warm-up questions

1. What kind of digital artifact would you normally create for this fictional
   event: a form, page, spreadsheet, message, or something else?
2. When another organizer reviews your work, what evidence helps them trust it?
3. If you return to a tool tomorrow, where do you expect to find yesterday's
   work?

## Neutral moderator script

### Consent and privacy language

> Thank you for considering this private-beta usability session. We are testing
> the product, not you. Participation is voluntary; you may pause, skip a question,
> or stop at any time without penalty. The session takes about 25 minutes.
>
> Please use only the fictional scenario and the assigned synthetic account.
> Never enter a real person's name, email, phone number, beneficiary information,
> donation, payment, domain, password used elsewhere, or other sensitive data.
> Create a unique study-only password and do not say or paste it to me. I will not
> record passwords, bearer tokens, invitation tokens, request bodies, or real
> personal data.
>
> This session uses the live private-beta service. The synthetic account,
> workspace, component documents, build, and receipt are stored by Taawun until
> supported cleanup at the end of the session. Supported cleanup removes access
> to the workspace/build and deactivates or anonymizes the synthetic identities;
> required de-identified or immutable audit/control references may remain under
> the operator's stated retention policy. Minimal request metadata may also remain
> in Railway under its existing retention policy. Raw research notes are
> accessible only to [approved roles/names] and are scheduled for deletion on
> [exact date]. If you withdraw before synthesis, we will stop, perform supported
> cleanup, and delete your identifiable raw notes; retained de-identified or
> immutable audit/control records may not be erasable. After findings have been
> combined and de-identified, removing one contribution may no longer be possible.
> The privacy contact is [name/contact].
>
> We will record a participant code, task timestamps, counted control activations,
> observed errors, your answers, and redacted request IDs if troubleshooting is
> needed. Recording is [off / separately described here]. De-identified findings
> will be combined across five participants. Usability results do not prove that
> people will buy the product. Do you consent to proceed?

Replace every bracketed consent field before recruitment. Record `yes`, timestamp,
protocol version, and recording choice. Do not record a signature unless the
user's approved consent process requires one.

### Task introduction

> Please work as you normally would. Think aloud when convenient: tell me what you
> expect, what you notice, and what you are deciding. I will mostly stay quiet. I
> can repeat the task or resolve a study-safety issue, but I will not tell you
> which control to choose. If you get stuck, continue with whatever you would try
> on your own.

Read each task block once. Do not mention expected template/module names unless
they were already present in the scenario. Do not describe signed fields before
the participant answers the comprehension questions.

### No-coaching rules

The moderator may:

- repeat the task verbatim;
- say, “What would you try next?” after 60 seconds of inactivity;
- clarify the fictional scenario or safety rule;
- stop accidental entry of real/sensitive data;
- resolve a study infrastructure problem unrelated to product usability.

The moderator may not:

- point, move the cursor, name a control, or reveal the expected template;
- define “signed,” “workspace-bound,” “history,” “Viewer,” or “published” before
  the participant's first answer;
- confirm that an answer or control choice is correct;
- supply a track ID, invitation token, password, or recovery step;
- convert a failure into success in the worksheet.

After one neutral prompt, mark the task `prompted`. After 90 additional seconds
without progress, mark it `assisted/incomplete`, end the timed task, and continue
to the comprehension/debrief section without pretending it succeeded.

## Safe credential and token handling

- Assign synthetic addresses under `example.test`; never use a participant's real
  email. Use a unique account per session.
- The participant creates a unique password used only for this session. Password
  entry is masked and screen observation/recording pauses. The password is never
  spoken, copied into chat, placed in notes, or reused.
- Bearer/JWT/OAuth tokens and request/response bodies are never collected.
- The Viewer invitation token may travel only through the pre-approved private
  session channel. Do not place it in the worksheet, recording, screenshot, log,
  or support message. Clear clipboard/channel state immediately after acceptance.
- Track IDs are selectors, not authority. They may be recorded for cleanup and
  troubleshooting, but never described as granting access.

## Manual instrumentation — no analytics

No new telemetry, cookies, event collectors, screen instrumentation, or product
state is authorized. Use a stopwatch and one worksheet per participant.

### Required timestamps

Record ISO timestamp and elapsed seconds for:

1. consent;
2. workspace selected and authorized zero history successfully loaded (`T0`);
3. explicit template choice;
4. component choices complete;
5. first meaningful component change;
6. Create signed preview activation;
7. active verified preview visible (`T1`);
8. durable history confirmation visible;
9. sign-out;
10. fresh sign-in;
11. history record reopened and verified;
12. Viewer invitation created;
13. Viewer acceptance confirmed;
14. cleanup completed.

`Time to verified preview = T1 − T0`. If active verification never appears,
record `incomplete`, not an inferred time.

### Counted control activations

Between `T0` and `T1`, count each activated button/tab/link and each committed
select/toggle change, whether performed by mouse, tap, Enter, or Space. Opening a
select or moving focus is not a separate activation from committing its choice.
Count a retry separately. Do not count text entry, pointer movement, Tab/arrow
focus navigation, passive scrolling, network wait, or moderator actions. Preserve
the ordered action labels in the worksheet; do not record keystroke content.

### Error and request evidence

For each observed error record:

- timestamp and study stage;
- visible error/status copy verbatim;
- participant's next unaided action;
- whether the last valid draft/verified preview remained;
- status code if visible to the approved operator;
- a redacted request ID such as `5t2z…Xw` (first four and last two characters);
- recovery result and elapsed time.

Full request IDs, if required for Railway correlation, go only in the restricted
operator incident log with timestamp, route category, deployment, and status.
Never place participant answers, credentials, tokens, origins, request bodies, or
real identifiers in that log. The research worksheet retains only the redacted
form.

## Scorecard

### Task score

| Measure | 2 — pass | 1 — partial/prompted | 0 — fail |
| --- | --- | --- | --- |
| Customized verified preview | Active verified preview within 5:00, meaningful supplied text change present, no coaching | Completed after 5:00 or after one neutral prompt | Not completed, wrong/stale evidence treated as success, or moderator named a control |
| Starter salience and control economy | Uses the Starter Path primary action before generic composer and finishes in 8 or fewer counted activations | Uses the Starter Path after an unaided detour or finishes in 9–10 activations | Requires control naming, enters generic composer first and cannot recover, exceeds 10, or is incomplete |
| History continuity | Fresh login automatically derives the applicable verified track and separate history confirmation, or participant manually reopens it only when needed; no track ID/help | Completed after neutral prompt or obvious detour | Cannot recover, pastes track ID, or opens unverifiable/stale result |
| Viewer handoff | Creates Viewer invitation, uses designated channel, and states Viewer is inspect-only | Invitation created but token/session or permission boundary is partly unclear | Cannot hand off, exposes token, or says Viewer may edit/build |

### Critical comprehension score

Ask each question before any probe. Score `2` only when the participant states the
complete boundary in their own words.

| Question | 2 — correct | 1 — partial | 0 — critical misconception |
| --- | --- | --- | --- |
| What was signed? | In plain language: the complete artifact manifest/receipt, including exact configured component documents and artifact/file digests plus workspace/subject, origin, lifecycle, expiry and signer bindings. The participant need not recite every field, but must identify the exact configured content and bounded receipt rather than a generic app. | Says “the preview/app” and exact configured content but cannot explain that the receipt also carries its trust/authorization bindings. | Says a person, organization, compliance status, payment, domain, future edits, or unrelated authority were approved/signed. |
| Which workspace does it belong to? | The selected Crescent Commons study workspace, evidenced by the receipt/binding. | Names the workspace but cannot identify any binding evidence. | Says global, transferable, or any workspace. |
| Is it published now? | No; it is a staging/private-beta preview, and publication needs separate verified-domain/review steps. | Says no but cannot distinguish the next gate. | Says public/live/deployed or assumes the preview URL is publication. |
| What should the reviewer do next? | Accept the session-only Viewer invitation through the trusted channel, inspect the exact build/receipt, and remain unable to edit/build. | Identifies review or Viewer but misses token/session or read-only boundary. | Shares a public link/token broadly, transfers owner authority, or says Viewer should edit/build/publish. |

H3 passes for a participant only with four scores of `2`. Do not average away a
critical misconception.

## Error taxonomy and severity

Classify observed behavior after the session, not while coaching.

| Code | Category | Examples |
| --- | --- | --- |
| D — discoverability | Cannot find starter action, template, component, Build History, receipt, or invite action. |
| C — comprehension/trust | Misunderstands exact signed content, workspace binding, expiry, preview/publication, reference-only review, or track selector. |
| V — validation/content | Cannot make or recover a valid meaningful edit; unclear field/JSON error. |
| R — recovery/continuity | Refresh/fresh login loses discoverability; retry appears destructive; stale output is unclear. |
| A — authority/handoff | Viewer/Architect/Maintainer boundary or session-only invitation is misunderstood. |
| X — accessibility/input | Keyboard, focus, zoom, target size, screen reader, motion, or input-method barrier. |
| S — service/performance | Transport/offline failure, unexpected service error or 5xx, unavailable loader, missing request ID, or material latency. Expected 422 validation belongs under `V`; expected 401/403 authorization behavior belongs under `A`. |
| P — privacy/safety | Real/sensitive data, credential/token exposure, cross-principal/workspace evidence, false Verified/publication, or misleading authority claim. |

| Severity | Research meaning | Action |
| --- | --- | --- |
| S0 — observation | Preference or comment with no task effect. | Record; do not create a backlog item alone. |
| S1 — minor friction | Hesitation under 30 seconds; self-recovers with correct understanding. | Aggregate with similar observations. |
| S2 — material friction | Detour over 30 seconds, one neutral prompt, >5-minute completion, or partial critical answer. | Candidate P2 only after source-fit review and recurrence/impact evidence. |
| S3 — blocker | Task incomplete, repeated failure, or critical concept wrong; no technical/security boundary breach. | Candidate P1; freeze feature ideation until exact repro and existing-primitive fit are documented. |
| S4 — safety/technical incident | Cross-scope data, credential/token leak, false Active Verified, publication/custody/compliance misrepresentation, destructive loss, or unexplained recurring 5xx. | Abort session and all later sessions; invoke incident path. Do not classify as ordinary usability evidence. |

## Abort and escalation conditions

Stop the current session immediately when:

- the participant withdraws, becomes distressed, or requests a pause;
- real personal, beneficiary, donor, payment, domain, password, credential, or
  sensitive data is entered or exposed;
- a password, bearer token, OAuth credential, or invitation token enters notes,
  chat, recording, screenshots, or logs;
- another principal/workspace's names, draft, receipt, track, or token appear;
- Active Verified appears without the complete exact receipt/runtime binding, or
  a stale/unverified preview enables publication;
- a real publication/domain/Bazaar/financial action becomes possible or is
  triggered;
- cleanup cannot be completed through supported authority.

Pause the whole study and notify the control room when:

- two consecutive low-rate health checks fail over at least 30 seconds;
- Railway shows an unexplained `>=500` correlated to the session;
- one S4 incident occurs;
- the deployed application/commit changes mid-study;
- two sessions encounter the same S3 blocker, because later participants would
  repeat a known failure rather than test the intended path.

Do not debug with participant credentials, weaken DNS/review/custody boundaries,
run load/adversarial probes, or deploy a fix during a session.

## Post-task questions

Ask in this order and record the first answer before probing:

1. In your own words, what did Taawun produce for you?
2. What exactly was signed? What, if anything, was not signed or approved?
3. Which workspace does the result belong to, and what on the screen supports
   your answer?
4. Is anyone on the public internet able to use it now? What would need to happen
   before publication?
5. What should the reviewer do next? What may and may they not change?
6. If you returned tomorrow on a fresh sign-in, where would you look for this
   work?
7. What was the first moment the product's value became clear, if any?
8. What was the most confusing moment? What did you expect instead?
9. What tool or process would you otherwise use for this fictional job?
10. Would you consider a real private-beta pilot for your organization? Why or
    why not? What approval, risk, or workflow would determine that decision?
11. Do you influence or own budget for tools like this? Do not ask for income,
    financial need, payment details, or a forced price answer.

Questions 1–8 are usability/comprehension evidence. Questions 9–11 are separate,
directional demand evidence. Interest, praise, or a hypothetical price from five
participants is not willingness-to-pay validation; an actual approved pilot,
budgeted purchase step, or other observable commitment would be stronger evidence
and still requires separate commercial review.

## Per-participant evidence template

```text
Protocol version / deployment:
Participant code: P0_
Consent timestamp / recording choice:
Technical comfort: low | moderate | high
Prior tool category (optional):
Device/browser/input/accommodation (non-medical):

Preflight health/root/deployment/5xx:
Synthetic organizer user ID:
Synthetic Viewer user ID:
Workspace ID:
Created track ID(s):
Viewer invitation ID / expected version / final status:

T0 authorized zero-history ready:
Template chosen / timestamp:
Components chosen / timestamp:
Meaningful edit observed / timestamp:
Create preview activation:
T1 active verified / elapsed:
Counted control actions in order:
History confirmed:
Fresh login / exact reopen:
Viewer handoff / acceptance:

Task scores (preview, controls, history, handoff):
Critical answers and 0/1/2 scores:
Errors: code, severity, visible copy, redacted request ID, recovery:
Neutral prompts or assistance (exact wording/time):
Unaided participant quotes (consented, de-identified):
Usability observations:
Demand observations (separate):

Cleanup membership/workspace/users status:
Old organizer/Viewer token checks = 401:
Post-session health/log result:
Incident/escalation reference, if any:
```

## Synthesis and ICE decision rules

1. Lock all five worksheets before synthesis. Never backfill missing timestamps,
   actions, answers, or request IDs from memory.
2. Report individual denominators (`4/5`, not percentages alone), medians and
   ranges for time/controls, every critical misconception, and all assisted or
   incomplete cases.
3. Separate three evidence columns in every finding:
   - **technical evidence:** status, request/log correlation, receipt/runtime
     binding, authorization, cleanup;
   - **usability evidence:** observed behavior, time, controls, errors, unaided
     answers and consented de-identified quotes;
   - **demand evidence:** current alternative, urgency, pilot intent, budget
     influence, and observable commitment. Never infer one column from another.
4. Combine only genuinely similar observations. A preference from one participant
   is not a product requirement. One reproducible S4 technical incident is still
   an incident, not a majority vote.
5. Map a candidate change only to existing Taawun primitives and documented
   constraints. Exclude DNS/Bazaar/reviewer fixtures, federation, TURN, E2EE,
   enterprise abstractions, fake activity, new analytics, and new authority types.
6. Score candidate slices with:
   - `Impact` 1–10: effect on the signed first-success/recovery/review job;
   - `Confidence` 1–10: cap at 3 for one participant, 5 for two, 7 for three, and
     8 for four/five; independently reproducible technical evidence may be scored
     separately;
   - `Ease` 1–10: existing primitive and bounded UI/test change scores higher;
   - `ICE = Impact × Confidence × Ease / 10`.
7. Rank at most three slices. Recommend only the smallest coherent slice whose
   acceptance criterion is falsifiable. The control room—not the moderator or
   participant—decides whether implementation is authorized.
8. Decision outcomes:
   - **Comprehension validated:** the same at least 4/5 satisfy H1 and all four H3
     anchors, no S4 occurs, and remaining H2/H4/H5 gaps are explicitly reported.
   - **Revise and retest:** H1 or H3 fails, or the same S3 recurs twice. Produce
     one evidence-backed existing-primitive prompt; do not invent expansion.
   - **Technical incident:** any S4. Stop study, preserve minimal incident
     evidence, and return to the technical QA/incident loop.
   - **Demand unknown:** default outcome unless separate observable commercial
     commitment exists. Usability success is not willingness to pay.

## Supported cleanup and monitoring runbook

### After every session

1. Stop participant observation before cleanup; do not ask for or expose the
   password/token.
2. Record only synthetic user/workspace/track IDs needed for exact cleanup. From
   the Viewer-invitation creation result, the operator separately records only
   its non-secret invitation ID and expected version, then discards the response
   body and never places the invitation token in the worksheet or operator ledger.
3. If a Viewer invitation is still pending/unaccepted, revoke it through the
   supported `POST /api/shura/v1/invitations/{invitation_id}/revoke` route using
   its recorded expected version before deleting anything else. If it was
   accepted, remove the Viewer membership. Then delete the synthetic workspace,
   isolated Viewer, and organizer accounts through authenticated supported routes,
   and capture status/request IDs in the restricted operator ledger. Expected
   lifecycle status is `204` where defined.
4. Prove both old sessions are invalid with a bounded profile/read check returning
   `401`. Do not retain the bearer values.
5. Clear the invitation token from clipboard and the designated session channel.
6. Confirm the deleted workspace/track is inaccessible through supported reads.
7. If any cleanup step fails, stop future sessions and escalate for supported
   application-admin handling. Never use raw database deletion, credential
   recovery, a fabricated JWT, or an unrelated participant's authority.

### Live monitoring boundaries

- Before and after a session, issue only one health and one root probe; confirm the
  exact deployment and inspect a bounded `>=500` Railway window.
- During a session, do not poll more often than once per minute and do not generate
  synthetic background traffic. Observe the participant journey only.
- Correlate a serious failure by timestamp and full request ID in the restricted
  incident ledger. App research notes keep only the redacted ID.
- Never log credentials, tokens, request bodies, component content, participant
  identity, or invitation values.
- Do not deploy, modify infrastructure, alter DNS, invent qualified-reviewer
  evidence, represent the disclosed isolated Viewer fixture as a participant or
  result, or create Bazaar/financial fixtures during this study.

## Evidence boundaries and final deliverable

The synthesis report must have three separate sections:

1. **Technical readback:** deployment, health/log window, exact signed/role/history
   boundaries observed, incidents, and cleanup. This can confirm that the tested
   system behaved as designed; it cannot prove ease of use.
2. **Usability/comprehension:** participant counts, task times, control counts,
   scores, observed errors, unaided answers, and de-identified quotes. This can
   falsify the five-person hypotheses; it cannot prove broad population rates.
3. **Demand/willingness to pay:** current alternatives, pilot interest, budget
   influence, and any observable commitment. This remains unknown unless genuine
   customer evidence exists and must never be inferred from technical correctness
   or polite positive feedback.

## Preparation monitoring snapshot

At `2026-08-19T09:53Z`, Railway still reported deployment
`431bc9b3-21b2-4e12-a4e1-c2a4a1fbfc39` as terminal `SUCCESS`. One low-rate health
probe returned `200` / `status=ok` in 382 ms, and one root probe returned `200`
with the Taawun Builder HTML in 421 ms. The bounded 43-request Railway sample
contained 37 `2xx`, six expected `4xx`, and zero `5xx`; p50 was 4 ms, p95 61 ms,
and p99 68 ms. No application fixture, participant account, invitation, workspace,
track, artifact, DNS record, Bazaar listing, or financial record was created in
this research cycle. A final low-rate confirmation at `2026-08-19T09:58:07Z`
again returned `200` for health in 391 ms and `200` for root in 323 ms. Railway
still showed the exact deployment as `SUCCESS`; its bounded 45-request readback
contained 39 `2xx`, six expected `4xx`, and zero `5xx`.

Until the user supplies genuine participants, consent, supported cleanup
authority, and control-room approval, the study state remains `not-started` and
no findings may be fabricated.
