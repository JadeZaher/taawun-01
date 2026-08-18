# MCP control-plane boundary

This directory serves a hosted remote MCP endpoint for LLM-driven product composition. It intentionally does not implement MCP Apps, `ui://` resources, AppBridge, inline UI, arbitrary source execution, package installation, shell commands, or container deployment.

Authentication happens at the HTTP boundary through the OAuth-only resource middleware in `pkg/oauth`; first-party platform JWTs are not valid MCP credentials. Every tool has an explicit fail-closed OAuth scope gate (`taawun:read`, `taawun:build`, or `taawun:publish`). Tool handlers still resolve the authenticated user from request context, require the workspace to be present in consent, and re-authorize every workspace operation through the persisted role/capability check. Tool inputs may name a workspace, but they never select the user or assert ownership. Bundle composition derives the signed subject from the authenticated user.

Artifacts are curated, declarative, immutable, and content-addressed. Before signing, an injected origin authorizer proves that every requested surface, embedder, connection, and resource origin is an approved exact subset; user input alone can never create a domain claim. Retrieval verifies the artifact and its workspace binding before returning a manifest-listed UTF-8 file. The storage directory never crosses the MCP boundary. An optional publisher is registered only when a concrete integration exists and can stage only to a domain already bound into the signed manifest.

The Streamable HTTP handler owns long-lived MCP responses. Application server write deadlines must therefore exclude `/mcp`, while ordinary routes retain bounded write deadlines. Exact Host and browser Origin allowlists wrap the authenticated MCP handler; non-browser clients without an `Origin` remain supported unless browser fetch metadata identifies a cross-site request.

The compose tool accepts the same stable component IDs/types and bounded JSON
data objects as HTTP. Its generated tool schema keeps `data` as an object, then
marshals through the artifact package's canonical validator before build; MCP
cannot use component data to widen any workspace, origin, signer, lifecycle, or
financial authority.
