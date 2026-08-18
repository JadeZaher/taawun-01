# Artifact generation

This package turns a validated curated template request into an immutable,
content-addressed artifact bundle. User input is data only: it may populate
escaped template slots and theme tokens, but it must never become executable
JavaScript, an output path, or an arbitrary template name.

`community-iftar` is the only P0 template. Its financial surface is a
sandbox-only handoff shell: generated artifacts do not custody funds, create a
ledger, or claim settlement. Compliance metadata comes from the built-in ethics
seed corpus and must remain labelled reference-only and pending qualified
review; it is never scholar approval or a fatwa.

Bundles support a monolithic dedicated-domain app and isolated modular cards
without changing their data or compliance semantics. Both are HTTPS Datastar
surfaces whose endpoint bindings are supplied by their serving host. MCP is the
LLM-facing control plane that invokes the builder, not an artifact render mode.
Keep browser state, transactional server state, and opaque relay signaling
explicit and separate.

Builds use a temporary directory under the configured output root and become
visible through one atomic directory rename. Artifact identifiers are generated
with `crypto/rand`; content hashes use SHA-256. Never derive a filesystem path
from request content and never overwrite an existing artifact directory.

Theme foregrounds are selected from the Swiss ink/light palette by measured
WCAG contrast. A pure-black fallback covers the narrow mid-luminance range
where neither palette foreground reaches 4.5:1 for normal text. Text placed over
the hero gradient's fixed ink segment uses its own opaque ink/white treatment.
