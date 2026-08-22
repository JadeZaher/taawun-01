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
iframe, receipt, and active trust presentation. For Maintainer and Architect,
the existing curated restore control becomes “Prepare fresh preview”; it remains
a zero-network local draft action and then focuses the separate explicit staging
preview submit. Current workspace and approved origins must be reviewed before
that submit creates a distinct signature. Viewer guidance remains inspect-only,
and no path extends or rewrites the old authorization. Once exact expiry is
established, the old record's reopen control becomes a disabled “Signed preview
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
transparent full-viewport layer rather than a framed object. Six authored scene
states group geometry right, right, left, left, right, right while readable
content uses the inverse left, left, right, right, left, left sequence. Those two
intentional sways avoid forcing the reader's eye across the screen at every
section. Same-side section changes perform no lateral interpolation. Only the
two authored side changes traverse the page, using an additional smootherstep
inside the fixed-rate critically damped response so the crossings begin and end
slowly instead of following wheel or swipe speed. The desktop resting offset is
22vw, keeping those two traversals restrained. Renderer eligibility follows the
entire public `main` region rather than ending with the sixth marker. The three
post-Templates sections add no geometry states: they retain state 5 on the right
and use a left-weighted translucent background so cards and text stay readable
without hard-occluding the fixed scene. Small
screens keep only restrained static edge peeks that fade beyond the viewport.
Their 1.2-second transform uses a smooth non-overshooting curve, so swipe or
wheel rate never changes the transition duration. Static fallback scaling is
limited to 3.5% and occurs only for a real side change. Continuation sections use
a stronger uniform reading surface on mobile, reduced-transparency, increased-
contrast, and forced-color paths. The resting pattern
is a disciplined repeated eight-point star and rosette tessellation with narrow
polygonal straps. Shared rosette corners form an outlined octagonal connector
with an open center. Paired diagonal bridge rails enter at the real offset
outer-arm intersection near lattice radius 0.406 and end on that octagon's
boundary, making the connector part of the strap network without cutting through
either interior. Alternating strap gaps
establish a visible over/under interlock. Crossing parity comes from the shared
global lattice coordinate: a
vertical edge uses `round(x)+floor(y)` and a horizontal edge uses
`floor(x)+round(y)`, so both cell halves agree which physical diagonal passes on
top. Emerald, gold, and rust refraction stays within adjacent parallel samples
of that exact lattice. Rail separation is authored in cell space from 0.010 to a
maximum 0.022, then divided by the active lattice scale; the mirrored rail is a
true small perpendicular offset, not a second scaled drawing elsewhere on the
surface. Chromatic rails keep only a subtle baseline outside the localized lens
envelope. Refraction remains visible without deforming or obscuring the crisp
base edges. The static HTML and CSS fallback carry the complete product story.

WebGPU may enhance only on capable desktop devices after idle, never moves text
or captures scrolling. Scroll events update only a normalized target. A bounded
critically damped response advances at a fixed 60Hz physics step with capped
elapsed time and velocity; an explicit target-crossing guard prevents numerical
overshoot. A separate internal phase eases toward a fixed 0.55 radians per second
while that response is moving, then eases its velocity to zero before sleeping;
wheel or swipe speed can move the target but cannot set this phase rate. This
short fixed-step tail prevents the localized lens and caustic from freezing at a
nonzero velocity when the scroll spring reaches its target. Its bounded envelope
evolves the rosette scale, perpendicular mirror depth, and at most 0.0008 cell of
rail
separation even through same-side spans, so scroll progress never freezes the
decoration. The resting lattice orientation remains invariant across all six
sections. Only while an authored side transition and physical settling overlap
does it add at most 0.018 radians of combined travel-only lattice rotation plus
another 0.0012 cell of rail shimmer. At same-side strength the outer star
contracts by at most 1.575% and the inner rosette expands by at most 2.025%; a
side crossing may reach the existing 3.5% and 4.5% bounds. Nested scale, extra
rail separation, and mirror depth return to their exact authored values on
settlement, while the localized color phase simply freezes until the next scroll.
This restrained kaleidoscope overlap changes only the nested star outlines.
Lattice scale, octagonal
connectors, bridge rails, and over/under straps remain fixed spatial anchors, so
there is no whole-field zoom or added lateral swing. The broad radial field is
also multiplied by a narrow viewport-aware feather computed from unshifted canvas
coordinates and the actual aspect ratio. Alpha reaches zero just inside every
physical edge even on square or portrait-capable desktop viewports, so the
shifted lattice cannot reveal the exact canvas boundary. It renders a
sharper final tessellation and stops scheduling frames once both displacement and
velocity settle. The subtle navigation motion button uses `aria-pressed` and
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
observer or duplicate listener. The signed-in cockpit brand is a 44px semantic
link back to the public `/` story; the account document itself remains noindex
and contains no decorative renderer.
