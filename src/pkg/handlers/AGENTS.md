# HTTP handlers

Handlers translate strict HTTP contracts into package/service calls. Keep domain
logic, persistence, and authentication outside this directory.

`ArtifactHTTPHandler` must be mounted behind the application's authentication
and workspace-authorization middleware. It accepts a resolver for the trusted
subject, workspace, and verified-origin policy established upstream; it does
not parse credentials or mint sessions. Client authority fields are replaced
or rejected, and cross-authority reads are reported as not found.

Artifact downloads are content-addressed and immutable. Every read goes through
the artifact package's manifest and file-digest verification before bytes are
served. Never join a URL path directly to the artifact output root.
