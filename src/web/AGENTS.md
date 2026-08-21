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

All async workspace evidence uses principal/workspace generation guards.
Same-workspace domain and publication operations additionally bind the exact
claim, origin, track, version, draft, and verified-preview generation. A stale
response may not replace a newer draft, receipt, claim selection, or publication
state. Failures retain the last valid draft and verified preview while showing a
specific retry/recovery state.

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
reserved JSON never replaces the last valid document. The logged-out landing
keeps account access immediate while describing outcomes before the later trust,
custody, and retention caveats; its claims remain bounded to existing primitives.

## Public landing and search contract

The logged-out document is both the account entrance and the crawlable public
explanation. Its title, description, visible H1, social metadata, and JSON-LD
must describe the same current private-beta capability. Search language may name
a community app or mosque page builder, but visible copy must immediately bound
that promise to the three curated templates, eleven customizable building
blocks, authorized workspace collaboration, and an exact private preview.

Do not imply arbitrary generated applications, live shared registration data,
generally available customer-domain serving, money movement or settlement,
scholar approval, a live marketplace, or proven federation, TURN, and E2EE.
Product counts come from the catalog, not marketing estimates. Financial and
religious-reference copy always says practice/reference-only. Information copy
retains the browser-first community-record boundary plus centrally retained
identity, preview, audit, and control records.

All indexable content stays in the initial HTML with one visible H1, ordered
headings, working fragment navigation, and account access at the top of the
journey. Auth forms and their dynamic messages are `data-nosnippet`. The robots
file is crawl guidance only and never a security boundary. Canonical, sitemap,
Open Graph, and structured-data URLs move together when a controlled branded
hostname replaces the current Railway service hostname. Structured data never
adds pricing, ratings, testimonials, organization facts, or availability that
is absent from the visible page.
