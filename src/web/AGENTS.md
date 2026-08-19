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
