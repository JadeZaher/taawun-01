# Cockpit continuity and trust state

The cockpit presents server-authorized workspace data without treating presence
as verification. Build History summaries may identify a track and its durable
status, but only `includeVerifiedPreview=true` may replace the trusted iframe and
receipt. Plain track and event reads never change the local draft or trusted
preview, and event `detail` is never rendered.

Restoring a build copies only its curated request into a new local draft. It does
not copy a track ID, creator, signer, lifecycle, origin, publication, or
idempotency authority. Viewer access is inspect-only; Maintainer and Architect
editing still results in a new server-authorized track.

Server-confirmed preview expiry keeps the immutable build record but removes its
iframe, receipt, and active trust presentation. The builder exposes the signed
lifetime before creation and uses seven days as its guided default; the server
still validates the one-hour through 90-day boundary. Every signed build exposes
“Re-sign as new preview” to a current Maintainer or Architect, so recovery does
not depend on a client-clock guess or a failed reopen attempt. That action asks
the server to copy only the exact curated request into a new composition, while
rechecking present workspace capability and origins and issuing a new subject,
artifact, signature, track, expiry, and review URL. The old authorization and
track remain unchanged. The separate restore control remains available when the
content also needs editing. Viewer guidance stays inspect-only. Once expiry is
known, the old record's reopen control becomes a disabled “Signed preview
expired” state rather than inviting another request that must fail.

All async workspace evidence uses principal/workspace generation guards.
Same-workspace domain and publication operations additionally bind the exact
claim, origin, track, version, draft, and verified-preview generation. A stale
response may not replace a newer draft, receipt, claim selection, or publication
state. Failures retain the last valid draft and verified preview while showing a
specific retry/recovery state.

Signed-preview reopen also binds the captured selected track ID and track-record
epoch before any trust-surface commit or expiry neutralization. Inspecting a new
track invalidates an older delayed reopen, including a delayed server-confirmed
expiry, so one record can never apply its status or recovery guidance to another.

## Signed starter derivation

The Signed Starter Path is ephemeral cockpit guidance, not persisted workflow
state. It appears only after an authorized Build History request independently
proves that the selected workspace has zero tracks; loading, errors, nonempty
history, and no workspace remain distinct states.

`activateWorkspaceScope` is the single boundary for direct selection, a newly
created preferred workspace, and server-list fallback after deletion. Its
`workspaceScopeId` comparison invalidates prior principal/workspace requests and
clears builder, receipt, track, domain, invitation-token, retry, and live-region
state before restoring only that principal/workspace's validated component
draft. A restored local draft remains editable but does not recreate starter
milestones automatically; separate intentional controls may reconfirm its
template, component choices, and exact customization without rewriting them.
Signed completion is derived from authorized verified history. The
workspace-creation POST is also session- and scope-guarded so a response after a
workspace or principal switch cannot select or mutate the newer cockpit. Scope
activation resets workspace mutation controls, while their stale finalizers are
generation-guarded so they cannot relabel controls owned by the new scope. This
includes invitation acceptance and publication: an intentional successful
acceptance may select its returned workspace, but a user-selected newer scope
invalidates the pending response; publication never carries busy or retry state
across a scope change.

On a later login, a nonempty summary never counts as signed progress. The
cockpit verifies only the selected track when it is in the loaded page, otherwise
the newest applicable summary that reports both preview and artifact presence,
using one `includeVerifiedPreview=true` read. Presence only selects the candidate;
it never supplies trust. Exact verified
component documents are then compared with validated catalog defaults. No event
scan, per-row verification, or summary-presence shortcut is allowed.

People, Build History, and domain evidence load independently. Role-sensitive
buttons remain disabled until People resolves, while history and domain failures
keep their own honest retry states. Viewer invitations remain session-only;
Architect may invite a Viewer, Maintainer coordinates with an Architect or an
existing member, and Viewer remains inspect-only. None of this guidance adds a
backend record, durable milestone, or analytics event.

## Signed preview file execution

The cockpit treats the manifest file list as the delivery boundary, not merely
display metadata. Before replacing either receipt or iframe it binds
`index.html`, `theme.css`, `app.css`, and the bundled runtime to the exact
authorized track file route, fetches each with the current Bearer token, and
forces a no-store read before checking raw byte length plus SHA-256 and strict
UTF-8 decoding. Descriptor paths must be unique and traversal-free; preview
URLs may not switch tracks or
add query/fragment data.

The document may name the public pinned runtime for portable bundle semantics,
but the cockpit never fetches or executes that public path. It requires that tag
to match the signed runtime requirement, then executes only the separately
verified runtime bytes from the track-scoped bundle. A missing, altered, delayed,
or stale file leaves the prior trusted iframe and receipt untouched and keeps
publication disabled.

Removing the cockpit's public runtime import also removes its incidental
`data-bind` behavior. Small builder-only presentation links, such as reflecting
the plain App name in `.sample-title`, stay explicit vanilla DOM updates; they do
not justify fetching or executing unsigned runtime bytes in the parent cockpit.

## Protected review links

A review URL is a locator for an immutable signed track, never a bearer grant.
Its fragment carries the exact track plus a non-authoritative workspace hint so
an already-authorized multi-workspace recipient lands deterministically. The
hint is accepted only when the workspace appears in the authenticated workspace
list; normal track membership authorization still decides access.

Opening the link must run the same verified reopen path as Build History: fetch
the exact track with `includeVerifiedPreview=true`, validate the receipt and
every manifest-listed file, then replace the trusted iframe. Retained fragments
are retried after invitation acceptance and deliberate workspace selection.
Draft edits immediately disable copying because the visible preview no longer
matches the draft. An old URL still locates its old immutable build until its
server authorization expires or membership is removed; it is not independently
revocable until a review-session grant primitive is implemented.

## Guided builder continuity

Empty workspaces must expose workspace creation immediately; the first required
action cannot live behind an unopened disclosure. The planned Guided mode maps
plain-language intent into the same catalog and component-document contracts as
Advanced mode. It may recommend and explain, but it does not create another
authority path or a second draft representation.

Custom-field name and value edits commit into the validated draft on input
without rerendering the editor row. Rerendering on every rename discards focus
and can let the visible control diverge from the document signed by a subsequent
build. The row updates its own key metadata and accessibility labels in place;
type changes replace only the value control, while full rerenders remain for
structural add, remove, and advanced-JSON operations.

## Destructive workspace controls

People offboarding is an Architect-only presentation of the existing membership
DELETE. Owner and current-principal rows never receive a control, confirmation
binds the exact member and workspace epoch, and success is not announced until
an authoritative People readback omits that member. Viewer and Maintainer views
remain inspect-only.

Invitation revocation applies only to pending grants created in the current
browser session. It sends the displayed record's positive `expected_version`,
accepts only the exact next-version `REVOKED` response, and clears the matching
secret. A conflict has no readback path, so the UI clears the secret and reports
status as unknown instead of inventing a durable invitation index.

Domain revocation binds the exact selected claim and issues one DELETE followed
by exact claim and publication-history reads. A revoked claim hides DNS proof
and cannot verify, publish, replace, or roll back. Its immutable history and the
last trusted staging iframe may remain visible, but that iframe is not public
serving authority.

Member and invitation confirmations expose stable target IDs plus exact
workspace identity. Once a request starts, Cancel is disabled and cannot claim
that no request was sent. Invitation create/revoke and domain claim/verify/revoke
use one in-flight mutation lane per primitive; domain confirmation has a separate
epoch so merely opening or cancelling it cannot invalidate another operation.
Domain claim mutations and publication replacement or rollback are mutually
exclusive in both directions, and long-running publication work remains bound
to the unchanged exact workspace and claim fingerprint. Alternate domain
renderers and evidence reloads preserve that exclusion; native claim controls
remain disabled until authoritative publication completion clears its own busy
state. The inverse also holds: a domain-claims read owns its exact loader epoch,
synchronously rerenders every native claim and publication control as disabled,
keeps replacement and rollback unavailable through claim/publication readback,
and cannot commit stale results across a publication epoch.
An ambiguous member or domain response fails closed until authoritative People
or exact claim-and-history reload resolves the outcome.

## Account and editing continuity

The Account tab exposes self-only password rotation and account deletion; it has
no administrator primitive. Password rotation uses the server's 12-character
minimum and deliberately returns to sign-in because every session is invalidated.
Account deletion treats `owned_workspaces_remaining` as a non-mutating recovery
state, and a successful zero-owned deletion signs out while explaining retained
audit and control records. Message-bearing account sign-outs select the login
tab before rendering their final live outcome so tab setup cannot erase it.

Password rotation, selected-owner workspace cleanup, and account deletion share
one explicit Account mutation guard. Network or 5xx outcomes never assert that
credentials, sessions, workspaces, or accounts are unchanged: password and
account ambiguity signs out for re-authentication, while workspace ambiguity
requires a workspace reload. Owned workspace cleanup uses the existing
owner-authorized workspace DELETE before retrying self-deletion. Its exact
workspace stays scope-locked while the request is in flight; ambiguous cleanup
releases the busy guard but keeps account deletion disabled until an
authoritative workspace-list reload resolves ownership. Its recovery opens the
Build surface so the exact outcome and workspace retry control are visible and
focusable. Cleanup never treats deleting one workspace as evidence that every
owned workspace is gone.

Component editor rerenders explicitly restore focus for JSON Apply, custom-field
Add/Remove, selection changes, and template-switch Apply/Cancel. Invalid or
reserved JSON never replaces the last valid document. The dedicated account
document keeps login, registration, and the authenticated cockpit in one page so
the in-memory bearer never crosses a navigation boundary.

## Public landing and search contract

The control-host root is a dedicated crawlable public explanation. Its title,
description, visible H1, social metadata, and JSON-LD must describe the same
current private-beta capability. Ordinary links lead to `/account#register` and
`/account#login`; the account document is `noindex` and retains the exact
same-document auth and cockpit lifecycle. Search language may name a community
app or mosque page builder, but visible copy must immediately bound that promise
to the three curated templates, eleven customizable building blocks, authorized
workspace collaboration, and an exact private preview.

Do not imply arbitrary generated applications, live shared registration data,
generally available customer-domain serving, money movement or settlement,
scholar approval, a live marketplace, or proven federation, TURN, and E2EE.
Product counts come from the catalog, not marketing estimates. Financial and
religious-reference copy always says practice/reference-only. Information copy
retains the browser-first community-record boundary plus centrally retained
identity, preview, audit, and control records.

All indexable content stays in the root HTML with one visible H1, ordered
headings, working fragment navigation, and account links at the top of the
journey. The account forms and dynamic messages remain `data-nosnippet`, and the
account response also carries `X-Robots-Tag: noindex`. The robots file is crawl
guidance only and never a security boundary. Canonical, sitemap, Open Graph, and
structured-data URLs move together when a controlled branded hostname replaces
the current Railway service hostname. Structured data never adds pricing,
ratings, testimonials, organization facts, or availability that is absent from
the visible page.

The Islamic geometric scene is progressive decoration. It occupies one
transparent full-viewport layer rather than a framed object. Six authored
content and static-fallback states place geometry right, right, left, left,
right, right while readable content uses the inverse sequence. Enhanced WebGPU
treats the whole tessellation as one rigid body on a single scroll timeline: a
heavy over-critically damped spring chases normalized scroll progress at a
fixed 60Hz physics step with capped elapsed time and a velocity ceiling, and
every visual dimension — the lateral slide, a slow one-direction full-tile
rotation, a gentle zoom-in, and a growing reveal radius — derives from that
one lagged value. Motion therefore only progresses start-to-finish with
momentum: no side-to-side returns, no lift or lean, no bounce, and no wiggle
after scrolling stops. Renderer eligibility follows the
entire public `main` region rather than ending with the sixth marker. The three
post-Templates sections add no geometry states: they retain state 5 on the right
and use a left-weighted translucent background so cards and text stay readable
without hard-occluding the fixed scene. Small
screens enhance too when the device reports WebGPU with adequate memory and
cores: they use wider side anchors, a tighter resolution budget, and the same
settle-and-stop scheduler. Devices that stay on the fallback keep restrained
static edge peeks whose 1.8-second transform starts slowly and uses a
non-overshooting curve, so swipe or wheel rate never changes the transition
duration and no transition scale reads as a snap. Continuation sections use
a stronger uniform reading surface on mobile, reduced-transparency, increased-
contrast, and forced-color paths. The resting pattern
is a disciplined repeated eight-point star and rosette tessellation with narrow
polygonal straps. Its static fallback and WebGPU enhancement share a fully
opaque 2.5 CSS px center core; the surrounding color and field stay muted so
the line remains crisp without competing with content. The enhanced material is
silver mirror-chrome in the ayeneh-kari spirit: every rail is a beveled
polished bar shaded by one procedural high-contrast studio environment —
near-white zenith and horizon flash over deep ink — and the palette lives in
the reflections as an emerald under-band, a gold high band, and a rust sliver,
with one bevel flank catching gold and the other emerald so color never sits on
the geometry itself. The core survives as the fully opaque polished face of
each rail. Shared rosette corners form an outlined octagonal connector
with an open center. Paired diagonal bridge rails enter at the real offset
outer-arm intersection near lattice radius 0.406 and end on that octagon's
boundary, making the connector part of the strap network without cutting through
either interior. Alternating strap gaps
establish a visible over/under interlock. Crossing parity comes from the shared
global lattice coordinate: a
vertical edge uses `round(x)+floor(y)` and a horizontal edge uses
`floor(x)+round(y)`, so both cell halves agree which physical diagonal passes on
top. Beneath the rails, large diamond mirror panes set at 45 degrees echo the
strap diagonals and fill the voids as a dim mosaic ground that glints toward
the live light. The pointer or touch point is a roaming light source, not a
lens: rails carry a moving specular band, panes flare and die as it passes,
one restrained anamorphic streak rides it, and inside a tight radius the
reflections smear and flow like molten metal — that melt lives entirely in
reflection space, so lattice coordinates never deform. Neither path uses blur
or filters. In CSS the fallback keeps its stable 6px/-4px, 3.25-degree
authored refraction overlap beneath an aligned halo. WebGPU composites panes,
rails, and glare as premultiplied layers. Scroll never changes the exact core
width, the material, or the softness; the timeline's gentle zoom scales the
tessellation as one whole. The static HTML and CSS fallback carry the complete
product story.

WebGPU may enhance only on capable devices after idle, never moves text
or captures scrolling. Scroll events update only the normalized timeline
target; a tiny final positional normalization happens only inside the
settlement tolerances. There is no crossing wiggle or moving-frame blur.
Pointer/touch input is a light source only — the star geometry never
articulates — and it still settles through the one guarded interaction
frame. Base lattice coordinates, octagonal connectors, bridge rails, and
over/under straps remain fixed spatial anchors throughout that bounded local
envelope. The CSS fallback expresses the same intent with cell-sized rosette
accents above an aligned halo, never by transforming the crisp copy. The canvas
renders into a bounded 40px overscan and the shader feathers against separately
reported visible-viewport bounds; the fallback uses a closest-side radial fade
and the fixed header/side vignette finishes the edge. Alpha therefore settles
naturally before the stage clip without creating document overflow. Every travel frame
retains the same crisp center and the renderer stops scheduling frames once both
displacement and velocity settle. A draw over 50ms fails directly to the still design; two
consecutive draws over 20ms also fail, while any intervening healthy draw clears
the slow-frame strike. A nonfatal resolution reconfiguration schedules exactly
one replacement draw even when the spring was already settled, so a newly
configured canvas never remains blank and cannot start an idle redraw loop.
Every renderer stop restores the current authored static state before exposing
the fallback, preventing the continuous enhanced side from leaking into the
categorical still layout. The subtle
navigation motion button uses `aria-pressed` and
keeps a reversible user preference. The progressively enhanced mobile menu
remains readable without JavaScript, uses 44px controls, and restores focus when
Escape closes it. Reduced motion, forced colors, increased contrast, reduced
transparency, reduced data, low-power, small-screen, shader, and device-loss
paths stay on the still design. Sacred text remains semantic, selectable, and
separate from the decorative renderer. The public excerpt is Qur'an 5:2 with its
exact citation and a visibly labeled English translation attributed to Dr.
Mustafa Khattab, The Clear Quran; adjacent copy does not interpret the verse as
a visual metaphor or product claim.

A persisted `pagehide` is a back/forward-cache suspension, not an unload: pending
loader frames are cancelled and the existing renderer pauses without destroying
its observer, preference, or device. The matching persisted `pageshow`
recomputes main-region visibility and resumes that one renderer or re-enters the
same eligibility gate if initialization was pending. A non-persisted pagehide
still disconnects and destroys normally. No restoration path registers a second
observer or duplicate listener. Persisted restoration also cancels the frozen
canvas opacity transition, applies the already-authoritative enhanced opacity
for the retained renderer lifetime, then removes only the temporary transition
override so future transitions remain CSS-owned. Every stop, capability fallback,
pause, and device-loss path clears both inline properties. The signed-in cockpit brand is a 44px semantic
link back to the public `/` story; the account document itself remains noindex
and contains no decorative renderer.

Canvas visibility changes immediately with the authoritative enhanced state; it
does not use an opacity fade that a reload or browser history restoration can
strand at its initial value. Decorative movement combines the inertial panel
with one separately guarded, slowly eased local pointer/touch light.
The CSS fallback masks separate refraction, halo, cell-local shape, and
crisp-main copies around the input; WebGPU applies its light-and-melt response
through the existing draw scheduler. Neither path deforms the base lattice, intercepts
touch scrolling, or creates another renderer loop; the only whole-field
transform is the panel's own rigid inertial motion. Touch release, pointer
leave, blur, hidden-page, preference, pause, and pagehide paths clear the
local state. Reduced motion keeps the crisp static tiling and suppresses all
local movement.

The executable contract is a deliberate smoke tier as of 2026-08-23: landing
layer/core/overscan structure, cleanup and no-overflow behavior, and mocked
desktop and mobile enhancement journeys. The former exhaustive suite (cockpit
trust flows, exact shader constants) was retired by decision and lives in git
history.
