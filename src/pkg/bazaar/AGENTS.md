# Template Bazaar boundaries

## Trust and lifecycle

- Listings advance only through `draft -> submitted -> compliance_review -> shura_review -> approved -> published`. Rejection, suspension, and retirement are explicit audited transitions.
- Creator actions require persisted workspace capabilities. Compliance, Shura approval, rejection, and suspension are trusted platform actions.
- Listing revisions are immutable snapshots; events are append-only. Mutable rows hold only current lifecycle pointers and optimistic versions.
- Public discovery, detail, and test-drive redirects expose only `published` listings. Publishing rechecks the exact active verified-domain publication and signed artifact content hash.

## Disclosures and external systems

- Every revision binds the creator, signed artifact, template and primitive versions, price/license, hosting limits, SSO seats, relay allowance, AZOA capacity, custody statement, limitations, madhhab review metadata, Shura decision, and a 10,000-basis-point revenue split.
- The package depends on interfaces supplied by artifact, domain, compliance, Shura, workspace, and financial packages. Do not reproduce those authorities here.
- Checkout creates only the vetted marketplace-escrow quest. Bazaar does not hold funds or initiate payouts; the disclosed revenue split is validated against the vetted financial flow for later settlement handling.
- Entitlements are created only after the financial authority reports `SETTLED` with a reconciliation reference. Installation is pinned to the purchased immutable revision and an authorized target workspace.

## Preview exception

- Authenticated preview bundles may use configured Taawun control/staging origins before a creator owns a domain.
- This preview authority is lifecycle-scoped and never authorizes Bazaar publication or public Host delivery; those still require an active verified domain publication.
