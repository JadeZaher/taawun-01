---
type: product-spec
title: Guided AI Co-builder for Non-technical Organizers
status: in-progress
---

# Guided AI Co-builder for Non-technical Organizers

## Problem statement

The current cockpit exposes templates, components, identity fields, trust
receipts, domains, collaboration, governance, finance, Bazaar, and account
operations in one surface. A first-time mosque or community organizer must
translate their intent into product vocabulary before they have seen a useful
page. Live QA confirmed that even workspace creation is hidden behind a
collapsed disclosure, and component customization can lose a renamed custom
field during rerender.

The default experience should feel like working with a patient co-builder: ask
one understandable question, recommend a safe next step, assemble a draft from
typed parts, and let the organizer review the result before any consequential
action.

## Goals

- At least 80% of invited first-time testers reach a verified preview without
  opening Advanced controls or asking what a template, module, track, manifest,
  or origin means.
- Median time from successful registration to first meaningful preview is under
  10 minutes, with no more than eight deliberate control activations excluding
  typing and network waits.
- At every step, at least 4 of 5 moderated testers can identify the single
  recommended next action and explain whether the page is draft, preview,
  protected review, or public.
- No assistant proposal can bypass catalog validation, workspace authorization,
  signed-artifact verification, role limits, or explicit confirmation gates.
- Draft progress survives interruption and can be resumed without silently
  repeating an invitation, build, publication, revocation, or deletion.

## Non-goals

- The first slice does not generate or execute arbitrary HTML, JavaScript,
  server code, containers, permissions, or routes; safe declarative parts remain
  the execution boundary.
- The assistant does not autonomously publish, invite people, remove access,
  delete data, claim domains, move money, or represent religious approval.
- The assistant does not replace the exact manifest/receipt review or conceal
  material safety and publication boundaries.
- Voice, image generation, open-ended marketplace installation, and every
  component becoming a server-backed workflow are later tracks.

## Primary guided journey

1. **Purpose** — “What are you trying to help people do?” Offer recognizable
   examples and plain-language free text.
2. **Audience and timing** — ask who the page is for and whether it is a one-time
   event, ongoing workspace, or cooperative offering.
3. **Recommended starting shape** — propose one template and explain the choice
   in one sentence; alternatives remain available but are not shown as a wall.
4. **Choose useful sections** — recommend a small initial set of parts. Ask
   outcome-oriented questions such as “Should people register?” rather than
   exposing module IDs.
5. **Make it theirs** — walk through only selected parts, one card at a time,
   with examples, validation, Back, Skip, Undo, and a visible saved-state cue.
6. **Review the assembled draft** — summarize what will be included, what will
   stay browser-local, and what is still demonstration-only.
7. **Create and inspect preview** — build the signed preview, show the page first,
   then offer plain-language proof details with technical evidence expandable.
8. **Choose the next outcome** — invite a teammate, copy a protected review
   link, keep editing, or prepare a controlled-domain launch. Show only actions
   allowed by the current role and authoritative state.

## User stories

- As a first-time community organizer, I want to describe my intended outcome
  in ordinary language so that I do not need to understand Taawun’s architecture.
- As a hesitant organizer, I want one recommended next step with examples and a
  way back so that I can make progress without fearing irreversible changes.
- As an experienced operator, I want to switch to Advanced controls without
  losing my guided draft so that the product does not slow me down.
- As a Viewer, I want the assistant to explain why I can review but not edit so
  that a disabled control does not feel broken.
- As an Architect, I want every invitation and publication proposal summarized
  before confirmation so that the assistant never expands access silently.
- As a returning organizer, I want to resume the last safe step and understand
  what changed since my last preview.

## Requirements

### P0 — Guided builder shell

- Guided mode is the default for new/empty workspaces and presents one primary
  task per step, a short progress indicator, Back, Save and exit, and “Advanced
  controls.”
- The guide maps answers into the existing versioned catalog, component
  documents, theme tokens, and preview request. It cannot invent unsupported
  fields or code.
- Recommendations include a concise reason and remain editable; users can reject
  them without restarting.
- Only fields for selected parts are shown. Technical IDs, raw JSON, origins,
  receipts, and history are progressively disclosed when needed.
- Every mutation distinguishes proposed, in progress, succeeded, failed, and
  ambiguous outcomes. A reload reconciles authoritative state before retry.
- Draft edits invalidate the prior preview immediately. The guide explains why
  a fresh preview is needed in user language.
- Keyboard, screen-reader, mobile, reduced-motion, 200% zoom, and error recovery
  follow the same step order as the visual experience.

### P1 — AI co-builder

- An assistant may turn natural-language answers into a typed draft proposal,
  but the server validates it through the same catalog and artifact contracts as
  manual editing.
- Every proposal includes “what I changed,” “why,” and material limitations;
  Apply, Edit, and Reject are explicit.
- The assistant receives only the minimum current-workspace draft context and
  never receives passwords, invitation tokens, bearer tokens, private keys, or
  unrelated workspace data.
- The assistant may recommend a preview or collaboration action, but invitation,
  access removal, domain proof, publication, rollback, revocation, and deletion
  remain explicit user-confirmed product actions.
- Provider unavailability falls back to the deterministic guide without losing
  draft state or blocking manual completion.

### P2 — Learning and personalization

- Remember optional organization-level preferences only with clear controls and
  deletion behavior.
- Offer safe reusable recipes derived from approved templates, never from one
  customer’s private content.
- Add multilingual guided copy only after the English task model and safety
  boundaries pass comprehension testing.

## Acceptance criteria

- [ ] A new organizer can create a workspace, choose a purpose, accept or alter
  one recommended template, customize two parts, and reach a verified preview
  without seeing raw JSON or more than one primary call to action per step.
- [ ] Switching Guided → Advanced → Guided preserves the exact typed draft and
  current step without duplicating any mutation.
- [ ] Invalid assistant output is rejected before it changes the draft, with the
  last valid state retained and an actionable explanation.
- [ ] Viewer, Maintainer, and Architect guidance derives from authoritative role
  data and never makes a disabled mutation appear available.
- [ ] Offline/provider failure leaves a fully usable deterministic guided path.
- [ ] A proposed invitation or publication cannot complete without the same
  explicit confirmation and server authorization required outside the assistant.
- [ ] Custom-field rename, type, and value survive every rerender and are exact
  in the signed manifest and reopened draft.
- [ ] Moderated comprehension results are recorded as observed evidence; no
  participant outcome or conversion metric is fabricated.

## Open questions

- **Product/design, blocking:** Is Guided mode workspace-scoped, user-scoped, or
  both when experienced users share a workspace with first-time collaborators?
- **Engineering, blocking for P1:** Which provider/model contract supplies typed
  proposals, and what is the explicit retention boundary for prompts and drafts?
- **Product, non-blocking:** Should “vibe coding” appear in customer copy, or
  remain internal language while the UI says “guided co-builder”?
- **Research, non-blocking:** Which three organizer intents should seed the first
  recommendation examples?

## Phasing

1. Ship the deterministic guided shell and fix the observed create-workspace and
   custom-field continuity failures.
2. Run five-person moderated comprehension testing and revise step language.
3. Add typed AI proposals behind the same validation and confirmation boundary.
4. Expand only after first-preview completion and trust comprehension meet the
   stated thresholds.
