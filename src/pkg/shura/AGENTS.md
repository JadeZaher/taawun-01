# Shura governance boundary

Shura records invitations, deliberation, votes, and explicit human decisions. It
does not execute AZOA intents or move money. Financial code may consume a final
decision ID as evidence, but must independently enforce its own approvals and
reconciliation.

Workspace membership remains owned by `WorkspaceService`. Shura asks an injected
workspace authorizer for current view, build, or publish permission and maps
those capabilities to Viewer, Maintainer, or Architect. It does not maintain a
second membership-role table.

Capabilities are short-lived Ed25519-signed claims from an injected, stable
issuer key. Verification binds issuer, key ID, audience, workspace, subject,
role, scope, and time, then checks both the durable issued-token record and the
online revocation table. Any registry, database, or membership-check failure
denies access.

Governance history is append-only. Proposal mutations use optimistic versions;
votes are immutable once recorded; and satisfying quorum never creates a
decision automatically. An Architect must explicitly record the final outcome
under the proposal's quorum, approval threshold, and required-approver policy.
