---
type: research
title: Signed Starter Path private-beta study control sheet
status: awaiting-human-input
date: 2026-08-19
approved_protocol_sha256: FE19E054AFDC6682677EC6C6CC49660C548C95166470F28D7A58735B6DD5E419
study_state: not-started
---

# Signed Starter Path private-beta study control sheet

## Purpose and authority boundary

This fill-in sheet operationalizes the control-room-approved protocol template
without changing its approved bytes. It is not a recruitment, scheduling, study,
fixture-creation, deployment, or feature authorization.

Approved immutable template:

- File: `docs/reviews/signed-starter-private-beta-usability-protocol.md`
- SHA-256: `FE19E054AFDC6682677EC6C6CC49660C548C95166470F28D7A58735B6DD5E419`
- Control-room decision: protocol design/template approved; execution not approved
- Current study state: `not-started`
- Accepted application baseline: app `c1e4aa5`, Railway deployment
  `431bc9b3-21b2-4e12-a4e1-c2a4a1fbfc39`

Do not edit the approved template. After every control below is complete, make a
separate runnable copy, calculate its new SHA-256, and return that exact copy to
the control room for final execution approval.

## Locked decisions

These values are not open questions:

| Control | Locked value |
| --- | --- |
| Recording | `OFF`; no screen or audio recording |
| Raw worksheet retention | No more than 30 calendar days after that participant's session; insert the exact deletion date before consent |
| Invitation handoff | Private, in-session visual/manual transfer to the isolated staff Viewer fixture |
| Prohibited handoff media | No clipboard, chat, email, screenshot, recording, or retained channel |
| Research access | Least privilege; only the named study owner, privacy contact, incident contact, and cleanup operator |
| Participant source | Five genuine participants arranged by the user with explicit consent |
| Cleanup | Authenticated supported application routes only; no raw database access, fabricated JWT, credential recovery, or optimistic participant self-cleanup |
| Application state | Keep the accepted app frozen unless the protocol is explicitly re-reviewed for a later deployment |

## Preliminary recruitment and scheduling authorization

This sheet does not authorize contacting, recruiting, screening, or scheduling a
participant. Before any participant row in section C or attestation in section E
may be filled from real activity, the control room must issue a separate decision
whose exact label is `RECRUITMENT/SCHEDULING APPROVED`.

That decision must name the immutable protocol SHA, restrict activity to
user-managed participant contact, screening, and scheduling, state an expiry or
permitted window, and explicitly authorize no session, application fixture, or
participant data entry into Taawun.

| Preliminary authorization field | Required value |
| --- | --- |
| Decision status | `[REQUIRED after decision: exact RECRUITMENT/SCHEDULING APPROVED label; current state is NOT AUTHORIZED]` |
| Control-room decision reference and timestamp | `[REQUIRED BEFORE RECRUITMENT]` |
| Immutable protocol SHA named in decision | `[REQUIRED BEFORE RECRUITMENT]` |
| Permitted user-managed contact/screening/scheduling scope | `[REQUIRED BEFORE RECRUITMENT]` |
| Authorization window/expiry | `[REQUIRED BEFORE RECRUITMENT]` |
| Decision explicitly says no sessions or fixtures | `[REQUIRED BEFORE RECRUITMENT]` |

## A. Named human roles

Every field is required. Codex must not infer or invent a person, contact method,
or authority. If one human fills multiple roles, repeat the exact name and contact
in each row and have the control room explicitly accept the concentration of
access.

| Role | Exact human name | Reachable contact or escalation method | Scope of access | Confirmed by / date |
| --- | --- | --- | --- | --- |
| Study owner | `[REQUIRED]` | `[REQUIRED]` | Participant coordination, consent, session conduct, de-identified synthesis | `[REQUIRED]` |
| Privacy contact | `[REQUIRED]` | `[REQUIRED]` | Consent questions, withdrawal, raw-note access/deletion | `[REQUIRED]` |
| Incident contact | `[REQUIRED]` | `[REQUIRED]` | Immediate safety/technical escalation and stop decision | `[REQUIRED]` |
| Supported cleanup operator | `[REQUIRED]` | `[REQUIRED]` | Independent application-admin cleanup after completion, abort, withdrawal, or browser closure | `[REQUIRED]` |

Approved raw-note access list, using only the names above:

```text
[REQUIRED: exact names]
```

Storage location for the filled contact sheet and raw worksheets:

```text
[REQUIRED: approved restricted location and access owner]
```

Do not commit a filled sheet containing personal contacts or participant data to
a repository whose audience exceeds the approved access list.

## B. Supported cleanup authority

Participant self-deletion is not the cleanup control. The named cleanup operator
must have an independent, authenticated Taawun application-admin principal that
remains usable if a participant withdraws, closes the browser, or loses their
session.

Fill without recording credentials, cookies, bearer tokens, response bodies, or
unredacted request IDs:

| Evidence | Required value |
| --- | --- |
| Cleanup operator's non-secret application principal ID/username | `[REQUIRED]` |
| `GET /api/profile` preflight timestamp | `[REQUIRED]` |
| Profile response | `[REQUIRED: 200 and platform role admin]` |
| Aggregate-only admin preflight | `[REQUIRED: GET /api/admin/statistics, timestamp, 200 status]` |
| Redacted preflight request ID | `[REQUIRED: first four and last two characters only]` |
| Independent of participant browser/session | `[REQUIRED: yes, verified by named operator]` |
| Operator confirms supported workspace, membership, invitation and user lifecycle runbook | `[REQUIRED: yes, date]` |

The preflight is read-only and proves the supported authority exists. Retain only
the route, timestamp, status, and redacted request ID; discard the response body.
Do not use identity- or workspace-listing admin routes for this preflight. It does
not authorize deletion or fixture creation. Full request IDs, if needed, belong
only in the restricted operator ledger.

Cleanup routes and outcomes remain those in the approved protocol:

- revoke an unaccepted invitation with its recorded non-secret invitation ID and
  expected version;
- remove an accepted Viewer membership;
- delete the synthetic workspace and study accounts through supported routes;
- prove prior sessions are invalid with bounded `401` readbacks;
- stop later sessions and escalate if any supported cleanup step fails.

## C. Session and deletion-date register

The user supplies the dates; Codex must not recruit or schedule anyone. Use
participant codes only in this sheet. Do not place names, emails, organizations,
or screening free text here.

The raw-note deletion date must be an exact calendar date no later than 30
calendar days after the session date. Calculate it before that participant sees
or hears the consent script.

| Code | Session date/time + timezone | Exact raw-note deletion date | Genuine participant arranged by user | Screening complete |
| --- | --- | --- | --- | --- |
| P01 | `[REQUIRED]` | `[REQUIRED: <= session + 30 days]` | `[REQUIRED]` | `[REQUIRED]` |
| P02 | `[REQUIRED]` | `[REQUIRED: <= session + 30 days]` | `[REQUIRED]` | `[REQUIRED]` |
| P03 | `[REQUIRED]` | `[REQUIRED: <= session + 30 days]` | `[REQUIRED]` | `[REQUIRED]` |
| P04 | `[REQUIRED]` | `[REQUIRED: <= session + 30 days]` | `[REQUIRED]` | `[REQUIRED]` |
| P05 | `[REQUIRED]` | `[REQUIRED: <= session + 30 days]` | `[REQUIRED]` | `[REQUIRED]` |

No participant may be marked `arranged` from a synthetic persona, staff Viewer
fixture, colleague impersonation, generated response, or unconsented contact.

## D. Runnable consent substitutions

Create a runnable copy only after sections A–C are complete. Replace the approved
template's four bracketed consent fields as follows:

| Approved placeholder | Required runnable value |
| --- | --- |
| `[approved roles/names]` | Exact names from the approved least-privilege access list |
| `[exact date]` | The exact deletion date for the participant receiving that copy |
| `[name/contact]` | Exact named privacy contact and reachable method |
| `[off / separately described here]` | `off; there will be no screen or audio recording` |

Also lock the generic invitation-channel wording to this exact operating method:

> Private in-session visual/manual transfer to the isolated staff Viewer
> fixture. Do not use a clipboard, chat, email, screenshot, recording, or any
> retained channel.

Use one runnable copy per participant unless a single common deletion date is no
later than 30 calendar days after every included session and the control room
explicitly approves that shared copy. A participant must receive the exact copy
whose deletion date applies to them.

## E. Participant and fixture readiness attestations

These attestations are completed by the named human study owner, not Codex:

- [ ] Five genuine participants were arranged by the user without automated or
  unsolicited contact.
- [ ] Each participant passed the approved screen; exclusions/conflicts were
  handled without collecting unnecessary sensitive data.
- [ ] Compensation, if any, is disclosed and does not depend on task success or
  favorable feedback.
- [ ] Each session will use a unique synthetic organizer identity and an isolated
  staff Viewer fixture; the fixture is never counted as a participant or result.
- [ ] The staff Viewer understands the visual/manual no-retention token handoff
  and will not copy, photograph, record, message, or retain the token.
- [ ] The study owner can stop the session before sensitive data, credentials, or
  tokens enter notes or a shared view.
- [ ] No account, workspace, invitation, track, or artifact will be created before
  final execution approval for the applicable runnable SHA.

Study owner attestation:

```text
Exact name: [REQUIRED]
Date/time + timezone: [REQUIRED]
Attestation: [REQUIRED: yes]
```

## F. Exact re-approval procedure

0. **Obtain preliminary recruitment/scheduling approval.** The control room must
   issue the scoped `RECRUITMENT/SCHEDULING APPROVED` decision described above
   before the user or study team contacts, screens, or schedules participants.
   This preliminary decision authorizes no session or application fixture.
1. **Preserve the template.** Recompute the approved file's SHA-256 and require
   `FE19E054AFDC6682677EC6C6CC49660C548C95166470F28D7A58735B6DD5E419`.
   A mismatch stops the process.
2. **Complete this control sheet.** Fill every `[REQUIRED]` value through named
   human input. Do not add credentials, tokens, participant identities, or
   unredacted request IDs.
3. **Verify independent cleanup authority.** The named operator performs the
   read-only supported preflight in section B. If production application-admin
   authority is absent, expired, participant-dependent, or ambiguous, the study
   stays `not-started`.
4. **Create the runnable copy or copies.** Copy the approved protocol rather than
   editing it. Make only the consent, recording, handoff, and participant-specific
   deletion-date substitutions authorized in section D. Do not weaken abort,
   cleanup, scoring, evidence, privacy, or technical boundaries.
5. **Validate dates.** Independently calculate each deletion deadline and prove
   the inserted date is no more than 30 calendar days after its session.
6. **Generate the approval manifest.** For each runnable copy record its filename,
   SHA-256, participant code or shared scope, session date, deletion date, and a
   concise diff classification against the approved template. Do not include
   participant identity or credentials.
7. **Run the frozen-baseline preflight.** Confirm the intended application commit
   and Railway deployment, one health `200`, one root `200`, terminal deployment
   success, and a bounded `>=500` readback. If the app changed from the approved
   baseline, return the protocol for technical/source-fit review before execution.
8. **Return the exact package to control room.** Submit this completed control
   sheet, every runnable-copy SHA, the redacted cleanup-authority preflight,
   baseline readback, and the named-role/access/deletion-date attestations.
9. **Wait for an explicit execution decision.** The control room must name the
   approved runnable SHA or SHAs, deployment, session window, and cleanup operator
   in an `EXECUTION APPROVED` decision. Template approval alone is insufficient.
10. **Freeze approved bytes.** Any later runnable-copy byte, role, contact,
    session date, deletion date, recording, handoff, authority, deployment, or
    access change invalidates execution approval and returns to step 1.

Recruitment or scheduling may begin only after a separate control-room decision
explicitly lifts the current hold on those activities; that narrower decision
still does not authorize a session or fixture. Only after step 9 may the named
humans create just-in-time synthetic session fixtures and conduct a session.
Execute one session at a time, complete supported cleanup, and close any incident
before proceeding to the next.

## G. Approval manifest template

```text
Approved design/template SHA:
Completed control-sheet SHA:
Recruitment/scheduling decision reference/window:

Runnable copy P01/shared:
Runnable SHA:
Session date/timezone:
Raw-note deletion date:
Authorized substitutions only: yes/no

Runnable copy P02:
Runnable SHA:
Session date/timezone:
Raw-note deletion date:
Authorized substitutions only: yes/no

Runnable copy P03:
Runnable SHA:
Session date/timezone:
Raw-note deletion date:
Authorized substitutions only: yes/no

Runnable copy P04:
Runnable SHA:
Session date/timezone:
Raw-note deletion date:
Authorized substitutions only: yes/no

Runnable copy P05:
Runnable SHA:
Session date/timezone:
Raw-note deletion date:
Authorized substitutions only: yes/no

Application commit / Railway deployment:
Health/root timestamp and status:
Bounded Railway >=500 result:
Cleanup-authority preflight timestamp/status/redacted request ID:

Control-room decision: PENDING
Execution-approved SHA(s): NONE
Execution-approved session window: NONE
Named cleanup operator: PENDING
```

## Current readiness

| Gate | State |
| --- | --- |
| Protocol design/template | `APPROVED` at the immutable SHA above |
| Recruitment/scheduling decision | `NOT AUTHORIZED` |
| Named human roles/contacts | `BLOCKED — human input required` |
| Supported application-admin cleanup authority | `BLOCKED — not currently configured/verified` |
| Session dates and deletion dates | `BLOCKED — user input required` |
| Five genuine participants | `BLOCKED — user arrangement required` |
| Runnable copy SHA | `NOT CREATED` |
| Recruitment/scheduling/sessions | `NOT AUTHORIZED` |
| Application fixtures | `NOT AUTHORIZED` |
| Final execution approval | `PENDING` |
