# Taawun private beta

Taawun is a hosted control plane for composing curated community-app primitives
into immutable, Ed25519-signed Datastar cards. A bundle is bound to the
issuing workspace, its expiry, and exact approved origins. The generated app
keeps convergent community records in the browser; Taawun centrally owns only
identity, governance, marketplace, financial-orchestration control records, and
opaque relay transit.

This repository is at a sellable private-beta checkpoint. It includes a real
authenticated journey:

1. Register or sign in, then create/select a workspace.
2. Inspect the curated template catalog and compose an immutable staging
   preview.
3. Claim and verify an exact HTTPS domain by DNS TXT proof.
4. Refresh the preview with the verified origin, then request and activate its
   publication.
5. Serve the signed card by exact `Host`, with signature, expiry, workspace,
   origin, and file-digest checks on every request.

The Bazaar, Shura, financial sandbox, hosted MCP control plane, and
identity-bound relay ticket service are all mounted in the same application.
They remain deliberately bounded: no generated app holds a ledger; financial
flows are durable manual-reconciliation sandbox orchestration; compliance
references are not fatwas or scholar approval.

## Run locally

Requirements: Go 1.25 and Node.js 20+ for the browser-runtime tests.

Copy `.env.example` to `.env`, replace every placeholder with locally generated
values, then run:

```powershell
go run ./src/cmd
```

Open `http://localhost:8080`. The first screen guides the private-beta builder
journey. No default credentials are created: register an account or configure
the optional bootstrap-admin variables explicitly.

For Docker, supply the same secrets in `.env` and use:

```powershell
docker compose up --build
```

The image is non-root and read-only apart from `/data`; it stores the control
database, AZOA sandbox database, and artifacts there.

## Verification

From the repository root:

```powershell
go test ./src/pkg/... ./src/cmd/...
node --test src/web/index.test.mjs src/web/runtime/runtime.test.mjs
go build -trimpath -o <temp>\taawun.exe ./src/cmd
go build -trimpath -o <temp>\taawun-relay.exe ./src/cmd/relay
git diff --check
```

The browser runtime's two-tab demo lives at
`src/web/runtime/demo.html`. A cross-device relay test needs fresh,
identity-bound relay tickets from the authenticated control plane; tickets are
one-use and never belong in a URL.

## Deployment boundary

Set the exact public application origin in `TAWUN_PUBLIC_BASE_URL`,
`TAWUN_ALLOWED_ORIGINS`, `TAWUN_MCP_ALLOWED_HOSTS`, and
`RELAY_ALLOWED_ORIGINS`. Customer verified domains are intentionally not added
to the control-plane CORS allowlist. Public cards are served only by exact Host
after a verified-domain publication; central `/api` access remains limited to
the Taawun application origin.

The Railway live-QA deployment details are recorded in
`docs/deployment-live-qa.md` once the checkpoint is promoted.

## Explicitly deferred

- Device/member workspace-key enrollment, rotation, and recovery; see
  `conductor/e2ee-workspace-keys.md`.
- Durable relay federation inbox/outbox and a shared replay store for
  multi-instance relays.
- Operator-provisioned short-lived TURN credentials and a two-network ICE
  demonstration.
- Live AZOA settlement, customer TLS/routing operations, and qualified scholar
  review authority.

See `PASSOFF.md` and `conductor/feature-parity.md` for the current product
truth, scope guardrails, and remaining acceptance gates.
