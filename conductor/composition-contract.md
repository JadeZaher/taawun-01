---
type: architecture
title: Declarative composition contract
status: accepted
---

# Declarative composition contract

Taawun's hosted MCP composes applications from reviewed primitives. It does not
accept arbitrary Go, JavaScript, HTML, SQL, container, or shell payloads. This is
the production boundary that lets a user's chosen LLM “vibe code” a useful app
without turning the MCP service into a remote-code execution service.

## Contract shape

`taawun.composition/v1` contains:

- an immutable template ID and version;
- application and organization display metadata;
- ordered pages with canonical relative routes;
- ordered slots containing primitive instances;
- a unique instance ID, primitive ID, primitive version, and validated
  primitive-specific configuration for every instance;
- references to declared capabilities, server signals, local collections, and
  versioned host events;
- curated theme tokens; and
- requested card, isolated-embed, and monolithic delivery modes.

The artifact signature covers the normalized composition, subject, workspace,
verified origins, lifecycle, expiry, runtime version, file digests, compliance
evidence, and financial boundary. A composition with the same visible copy but a
different capability or origin is therefore a different signed artifact.

## Safe customization

Primitive schemas may expose bounded display copy, labels, field definitions,
choice lists, visibility rules, and references to other primitive instances.
Validation rejects unknown fields, unsafe markup, executable expressions,
absolute routes, undeclared signals, and capabilities outside the reviewed
primitive version. Custom CSS is not accepted; users customize the documented
namespaced design tokens.

Financial descriptions compile to references to vetted server-owned AZOA flow
definitions. They never compile to browser collections, balances, or settlement
logic. Governance actions compile to Shura capability checks. Compliance status
and citations remain visible in the resulting manifest and product surface.

## Microfrontend rules

- A card owns exactly one stable DOM root and does not query or mutate another
  card's subtree.
- The default third-party integration is a sandboxed iframe. The optional
  `<taawun-card>` loader creates that boundary and uses a versioned host contract.
- A monolith renders the same primitive instances and contracts, rather than a
  separate implementation.
- Browser-to-host messages and DOM events are namespaced, versioned, validated,
  and scoped to the artifact and primitive instance.
- Styles are rooted beneath the Taawun surface/card boundary and use namespaced
  tokens. Assets resolve from the signed artifact base URL, never the embedding
  page's relative URL.
- Network, embedding, and passive-resource origins are separate exact allowlists.
  CORS is not treated as authentication; the signed subject/workspace and server
  authorization still apply.

## MCP authoring loop

1. The LLM lists or inspects templates and primitive schemas.
2. It drafts a composition for one authorized workspace.
3. Taawun validates structure, capabilities, financial boundaries, and the
   selected madhhab-tagged compliance baseline.
4. Taawun renders an authenticated staging preview.
5. An authorized user explicitly publishes it to a verified domain.
6. The public delivery handler serves only the active signed artifact for that
   exact host. Rollback changes the active pointer; it never mutates an artifact.

Preview, publish, and rollback are distinct audited actions. An LLM may prepare a
draft, but it cannot grant itself a domain, workspace role, compliance approval,
financial settlement, or publication authority.
