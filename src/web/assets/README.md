# Vendored browser runtime

`datastar-v1.0.2.js` is the official Datastar v1.0.2 browser bundle from
<https://github.com/starfederation/datastar/blob/v1.0.2/bundles/datastar.js>.

Datastar is distributed under the MIT License. The vendored version is pinned
so the dedicated Taawun builder does not depend on a third-party CDN at runtime.

Bundle SHA-256: `2837D87ACF6EE0BA8E4E63765926C25A98D63883B02F88BE194A86B81D3FD24A`.

The bundle lives in `assets/` because the Go module's existing `vendor/` ignore
rule would otherwise omit it from the embedded application.
