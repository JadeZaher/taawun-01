# Signed component-document contract

This is the P0 contract for Taawun's component-based low-code builder. It is a
checkpoint specification for the current draft PR, not evidence that the
checkpoint has passed live QA. The last-known-good production rollback remains
application `09cd897` / Railway deployment
`c2374acf-7102-4471-91fd-e568f7bdb5ce`.

The governing product boundary remains: compose reviewed templates and modules
from data, never execute user code. See the [product guide](../conductor/product.md),
[feature-parity matrix](../conductor/feature-parity.md), and
[sellable-MVP passoff](../PASSOFF.md).

## P0 model

- The template is selected explicitly from the authenticated real catalog. No
  template ID is implied or hard-coded by the client.
- A template declares its allowed module types. A composition selects one or
  more of those types.
- Each selected module has exactly one component instance. In P0 its stable
  `id` equals its curated `type`; duplicate instances and reordering are
  deferred. IDs are at most 64 bytes, begin with a lowercase ASCII letter, and
  otherwise contain lowercase ASCII letters, digits, or `-`.
- If both `modules` and `components` are supplied, their type sets must match
  exactly. The server sorts the canonical component set by type, so input order
  does not change the artifact.
- Each component has a root JSON object in `data`. The catalog currently
  declares required `title` and `summary` strings, with limits of 120 and 600
  Unicode code points respectively. Module-specific defaults come from the
  catalog, not the browser.
- Other safe keys are user-defined presentation fields. Their values may be a
  JSON string, number, boolean, `null`, array, or object. They are rendered as
  escaped, generic reference data; they never select behavior or authority.

Example explicit input:

```json
{
  "modules": ["announcements"],
  "components": [
    {
      "id": "announcements",
      "type": "announcements",
      "data": {
        "title": "Neighbourhood update",
        "summary": "Doors open at sunset.",
        "accessibility-note": "Step-free entrance",
        "schedule": ["18:30", "19:05"]
      }
    }
  ]
}
```

## Validation and canonical JSON

The catalog's module response exposes the same fixed
`componentDocumentPolicy` used by HTTP, MCP, Conductor, and the artifact
builder. The current limits are:

| Limit | Value |
|:---|---:|
| Canonical bytes per component document | 8,192 |
| Canonical bytes across all component documents | 32,768 |
| JSON depth, counting the root object as depth 1 | 6 |
| Key length | 64 bytes |
| Fields in any object | 32 |
| Items in any array | 32 |
| Keys across one component document | 128 |
| String length | 2,048 Unicode code points |
| Number token length | 64 bytes |

Keys begin with an ASCII letter and then use only ASCII letters, digits, `_`,
or `-`. Duplicate keys are rejected before decoding into a map. Validation is
recursive: every nested object key and value is subject to the same policy.
Control characters are rejected in strings except tab and newline; documents
must be valid Unicode scalar sequences and UTF-8. Raw JSON string tokens with
an unpaired high or low UTF-16 surrogate escape are rejected before decoding
can substitute a replacement character. Valid surrogate pairs and literal
Unicode such as emoji canonicalize to the same exact UTF-8 value.

Numbers use their JSON token, not a binary floating-point conversion. Valid
JSON integer and decimal forms are accepted only when already canonical:
exponents, negative zero, and fractional values ending in `0` are rejected.
Objects are serialized with deterministic key order, component instances are
sorted by type, and array order is preserved. The resulting compact JSON bytes
are the authoritative document bytes used for persistence, hashing, rendering,
and comparison.

## Reserved keys

Key checks are case-insensitive and ignore `_` and `-` before matching. A key
is rejected at any depth when it begins with `on` or contains one of these
reserved terms:

- executable/prototype: `proto`, `prototype`, `constructor`, `script`, `html`,
  `css`, `style`;
- network/origin: `endpoint`, `url`, `uri`, `origin`, `surface`, `embedder`,
  `connection`, `resource`;
- identity/authority: `authority`, `permission`, `capability`, `workspace`,
  `principal`, `subject`, `user`, `actor`, `signer`, `signature`, `auth`,
  `lifecycle`, `expiry`, `expires`;
- governance/financial control: `shura`, `finance`, `payment`, `settlement`,
  `escrow`;
- artifact trust: `artifactid`, `contenthash`, `manifestdigest`,
  `contractversion`, `templateid`, `moduleid`, `componentid`;
- credentials: `token`, `password`, `credential`, `secret`, `cookie`, `apikey`.

This intentionally disallows apparently convenient fields such as a custom URL
or payment setting. Those are reviewed catalog/server contracts, not content.
Component data cannot add markup, scripts, CSS, Datastar expressions, signals,
routes, endpoints, origins, permissions, lifecycle, identities, Shura authority,
finance actors, settlement behavior, or signer fields.

## Signed data chain

The same canonical bytes must survive every seam:

1. Authenticated HTTP or OAuth-protected MCP receives `modules` plus optional
   `components`. Client-supplied workspace or role claims do not grant access.
2. Conductor validates and canonicalizes explicit component data before origin
   authorization, durable track creation, or idempotency consumption. Invalid
   data therefore creates no durable track and does not strand the key.
3. The durable track stores the canonical composition request. Its resolved
   build request contains the exact component documents used for the build.
4. The builder revalidates the request and safely renders declared fields into
   curated slots. Unknown fields appear only in an escaped generic presentation.
5. A v2 manifest embeds each component's exact canonical `data` and binds its
   `components/<id>.json` path and SHA-256 digest. `components.json` contains the
   canonical aggregate and is also manifest-digested.
6. File digests feed the semantic content hash. The Ed25519 v2 signature covers
   the complete manifest, including `components`, subject/workspace, lifecycle,
   expiry, exact origins, compliance boundary, and files.
7. A positive preview receipt is returned only after reopening the immutable
   bundle and comparing request, build request, stored manifest, reopened
   manifest, files, workspace, and preview metadata. The response includes the
   exact serialized manifest plus its SHA-256 digest for complete browser-side
   comparison.
8. Track inspection may request `includeVerifiedPreview=true`. That path reopens
   and verifies the immutable artifact before returning the receipt. Persisted
   track JSON alone is never proof of a verified preview. A verified reload
   rehydrates documents from the verified build/manifest binding, byte for byte.

Missing, altered, or non-canonical signed documents; post-signature reordering;
mismatched module and component sets; missing component files; wrong file
digests; component-bearing v1 manifests; incomplete v2 manifests; signature
mismatches; and expired active authorization all fail closed. No negative state
may be labelled active or verified. Request order itself is harmless because it
is canonicalized before persistence and signing.

HTTP composition validation uses a nested `422 invalid_composition` envelope
with only bounded `componentId`, safe key path, and reason class. It never echoes
the document value. Invalid or reserved key paths may be withheld entirely to
avoid reflecting sensitive names. Integrity failures use a bounded conflict
response and retain the last previously verified preview in the client.

## Legacy compatibility

Omission and explicit emptiness are different inputs:

- Omitted `components` with a non-empty legacy `modules` list preserves the
  legacy composition/idempotency hash. Catalog defaults are resolved only into
  the durable build request. New builds still emit the complete v2 component
  contract.
- Explicit `components: []` means the caller chose the component contract but
  supplied no instances. It is rejected before origin checks, track creation,
  or idempotency consumption; it never falls back to defaults.
- Existing v1 bundles remain readable only when manifest, authorization, and
  signature all use their exact v1 versions and exact v1 signed-field allowlist,
  with no components. A component-bearing v1 bundle is invalid.
- A v2 bundle requires at least one fully bound component and the exact v2
  signed-field allowlist. Readers do not reinterpret incomplete v2 data as v1.

No database schema migration is required: the existing durable request/build
JSON columns carry the additive component fields. Required audit and Shura
history remain untouched.

## Authorization

- Architect/organizer and Maintainer mutations require the server-issued
  selected-workspace build capability. HTTP, MCP consent/scope, Conductor, and
  the artifact builder recheck their respective boundaries.
- A Viewer may inspect an authorized workspace, track, and verified preview but
  cannot compose, resume, mutate component documents, publish, or gain a
  financial/governance capability from component data.
- Every response and local draft is scoped by authenticated principal,
  workspace, and template. Late responses from an earlier principal or
  workspace generation are ignored.
- Publication remains a separate, stricter server-authorized operation against
  a currently verified domain claim. A signed preview is not publication.

## Builder recovery and reconciliation

The browser UI must preserve both editing work and the last trustworthy result:

- Catalog loading, empty, failure, and retry states are explicit. A template is
  chosen through an accessible dropdown showing its real description and
  allowed components; ordinary failures do not reset it.
- Drafts are isolated in session storage by principal, workspace, and template.
  Invalid editor JSON may remain visible for correction, but it never overwrites
  the last valid document and can never be composed.
- Every raw JSON entry surface, including custom object and array fields, applies
  the same duplicate-key, canonical-number, trailing-token, control-text, and
  Unicode-scalar checks before decoding.
- A compatible template switch applies immediately and carries compatible
  component documents. A switch that would drop selected incompatible types
  requires an explicit Apply/Cancel reconcile step; the entire old template
  draft is retained for recovery and document data is never silently discarded.
- Network/offline, validation, conflict, stale-principal/workspace, and build
  failures retain the last valid draft and last verified iframe/receipt. The UI
  labels that preview stale or unverified relative to the edited draft and
  disables publication until a fresh exact verified receipt is present.
- Reload uses the durable track ID and verified-preview endpoint. Only an active
  exact receipt restores signed component documents. Expired, missing, or
  tampered evidence may update an honest warning, but it never replaces an
  existing verified iframe and cannot be published.
- Viewer mode uses the same component presentation in read-only form. Disabled
  controls explain the role boundary without attempting a request.
- A principal or workspace change clears the visible preview and track before
  loading the new scope; generation guards suppress every late response from the
  prior scope.
- Reconciliation, errors, and successful rebuilds use keyboard-accessible focus
  and live announcements and must reflow without horizontal overflow at 320 px,
  400 px, and 200% zoom.

## Workspace build history and activation continuity

The history endpoint is deliberately a small recovery index over existing
Conductor tracks:

```text
GET /api/conductor/tracks?workspaceId=<positive>&limit=<positive>&cursor=<opaque>
```

- `workspaceId` is required and freshly authorized with the workspace View
  capability. Viewer, Maintainer, and Architect may list only a workspace they
  can already view; an outsider receives the same bounded workspace denial.
- `limit` defaults to 20. HTTP values above 50 are capped at 50. The optional
  cursor is an opaque base64url keyset over `(updatedAt, trackId)` and is valid
  only as an input to a freshly authorized workspace query.
- Results are ordered by `updatedAt DESC, id DESC`. The response contains
  `tracks` and an optional `nextCursor`.
- Each summary contains only ID, template ID, status, version, updated time,
  artifact/preview/publication presence, and optional preview-authorization
  expiry. Presence is not called verified. Summaries omit workspace IDs,
  component documents, app/customer content, actors, subjects, artifact IDs,
  hashes, signatures, origins, claims/publication identities, failure reasons,
  tokens, and event details.
- The composite SQLite index is additive; existing tracks require no data
  migration. Legacy v1 and v2 tracks are listed from their durable request and
  status metadata, while their existing version-specific verification rules
  remain unchanged.

Selecting a summary first loads the authorized durable track and a timeline made
only from event type, resulting status, version, and time. Raw event detail is
never rendered. Reopening then calls the existing verified-preview endpoint,
which reopens the immutable bundle and applies the complete v1/v2 verification
contract before changing the iframe or receipt.

Build-capable roles may copy the exact curated request into a new local draft.
This recovery never transfers creator, signer, track, idempotency, lifecycle,
origin, claim, or publication authority. Viewer remains inspect-only. The
existing Resume operation is shown only for a resumable status and remains
server-gated to the original creator, a fresh Build capability, and exact
expected version. Authorization and creator checks occur before the version
comparison, preventing a track-version oracle. A conflict reloads authoritative
status/version without replacing the current draft or trusted preview.

Architect delivery state is recovered through the existing domain-claim and
publication-history APIs. Pending, verified, revoked, and expired states show
their real next action. A pending claim never reconstructs a TXT proof; issuing
or rotating a proof uses the supported claim endpoint. Publication still
requires a currently verified claim whose exact origin appears in a fresh signed
preview. If request succeeds but activation fails, the cockpit retains the
durable `PUBLICATION_REQUESTED` version, reloads safe status/events, and retries
activation without repeating the publication request. DNS control, reviewer,
and Bazaar gates remain unchanged.

## Signed Starter Path boundary

The Signed Starter Path is approved for implementation as a first-use aid; it
is not yet live acceptance evidence and does not add a persistence or authority
primitive.

- It appears only after a selected, authorized workspace history request has
  completed successfully with zero rows. An absent workspace, a loading
  request, or a failed history request is not an empty-workspace signal.
- Progress is derived ephemeral UI state. There is no completion flag,
  analytics state, or backend record. After reload or fresh login, the signed
  step is derived only by reopening one applicable authorized track through the
  complete verified-preview path; a summary presence flag is insufficient.
- Invitation tokens and invitation progress remain session-only. The path may
  create and copy an invitation for delivery over a trusted channel, but it
  must not restore or describe that handoff as durable after reload.
- The five-person comprehension check is pending manual, post-deployment
  private-beta validation with real participants. Results must not be
  fabricated, and the study does not block the technical deployment gate.

Existing non-empty workspace composition, history, domain recovery, and role
controls remain unchanged.

## Rollback and operational boundary

Use the last-known-good application `09cd897` / Railway deployment
`c2374acf-7102-4471-91fd-e568f7bdb5ce` only for an application-caused health or
core organizer-to-Viewer regression. It is the discoverability baseline and
does **not** support component-document editing.

1. Correlate health, runtime logs, metrics, and request evidence first. Do not
   roll back solely for a recovered Railway edge/routing interval.
2. Use Railway's supported production rollback/redeploy action for project
   `43973172-fd19-4c41-8097-369cf805cbe6`, environment
   `e967d839-d678-4d56-9984-b60e7fc2029d`, service
   `3a254bda-f83c-4b47-aaa6-03a045546e88`, targeting the known-good deployment.
   Do not reset the Git branch, edit production data, or ask QA to mutate
   production.
3. Require the resulting deployment to reach terminal `SUCCESS` rather than
   treating `QUEUED` as acceptance. Verify `/api/health` and `/`, run a short
   low-rate stability soak, then smoke the organizer-to-Viewer baseline.
4. Record the new deployment ID and evidence in
   [deployment live QA](deployment-live-qa.md) and hand the exact result to the
   independent QA task.

The rollback binary predates v2. Preserve v2 tracks and immutable artifacts;
do not resume, reinterpret, delete, or manually rewrite them through the older
application. Component publication must not be activated before this checkpoint
passes QA. If a v2 publication is ever activated later, roll its domain pointer
back through the supported publication lifecycle before reverting the app.

Controlled DNS verification and public serving still require a genuine customer
hostname and must not be bypassed. Bazaar remains published-only and must show
the honest empty/external-fixture state until controlled DNS, an authorized
reviewer, and cleanup-safe listing lifecycle exist; sandbox escrow is not real
custody or settlement. Device-key E2EE, durable relay federation, configured
TURN, and AZOA federation remain explicitly deferred. Component documents do
not weaken or claim completion of any of those boundaries.
