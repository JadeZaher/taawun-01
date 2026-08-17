package shura

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"taawun/pkg/models"
)

func TestCapabilitySignatureBindingLeastPrivilegeAndRevocation(t *testing.T) {
	harness := newShuraHarness(t)
	ctx := context.Background()
	maintainer := harness.users[2]
	raw, claims, err := harness.service.IssueCapability(ctx, maintainer, IssueCapabilityRequest{
		WorkspaceID: 42, Role: RoleMaintainer,
		Scopes:   []Scope{ScopeVote, ScopeRead, ScopePropose, ScopeDeliberate},
		Audience: CapabilityAudience, TTL: 10 * time.Minute,
	})
	if err != nil {
		t.Fatalf("issue capability: %v", err)
	}
	if claims.KeyID != "key-2026-01" || claims.Issuer != "https://issuer.taawun.example" || claims.TokenID == "" || claims.Subject != "2" || claims.WorkspaceID != 42 || claims.IssuedAt == 0 || claims.NotBefore == 0 || claims.ExpiresAt <= claims.IssuedAt {
		t.Fatalf("incomplete claims: %+v", claims)
	}
	verified, err := harness.verifier.Verify(ctx, raw, 42, CapabilityAudience, ScopePropose)
	if err != nil || verified.Subject != "2" || verified.Role != RoleMaintainer {
		t.Fatalf("verify capability: claims=%+v err=%v", verified, err)
	}
	if _, err := harness.verifier.Verify(ctx, raw, 41, CapabilityAudience, ScopePropose); !errors.Is(err, ErrCapabilityInvalid) {
		t.Fatalf("wrong workspace error = %v", err)
	}
	if _, err := harness.verifier.Verify(ctx, raw, 42, CapabilityAudience, ScopeDecide); !errors.Is(err, ErrCapabilityForbidden) {
		t.Fatalf("least privilege error = %v", err)
	}
	delete(harness.workspaces.roles, 2)
	if _, err := harness.verifier.Verify(ctx, raw, 42, CapabilityAudience, ScopePropose); !errors.Is(err, ErrWorkspaceDenied) {
		t.Fatalf("removed workspace member error = %v", err)
	}
	harness.workspaces.roles[2] = RoleMaintainer
	if _, _, err := harness.service.IssueCapability(ctx, maintainer, IssueCapabilityRequest{WorkspaceID: 42, Role: RoleMaintainer, Scopes: []Scope{ScopeDecide}, Audience: CapabilityAudience, TTL: time.Minute}); !errors.Is(err, ErrCapabilityForbidden) {
		t.Fatalf("privilege escalation issue error = %v", err)
	}
	tampered := raw[:len(raw)-1] + differentBase64Character(raw[len(raw)-1])
	if _, err := harness.verifier.Verify(ctx, tampered, 42, CapabilityAudience, ScopePropose); !errors.Is(err, ErrCapabilityInvalid) {
		t.Fatalf("tampered signature error = %v", err)
	}
	harness.verifier.now = func() time.Time { return time.Unix(claims.ExpiresAt, 0).Add(maxClockSkew + time.Second) }
	if _, err := harness.verifier.Verify(ctx, raw, 42, CapabilityAudience, ScopePropose); !errors.Is(err, ErrCapabilityExpired) {
		t.Fatalf("expired capability error = %v", err)
	}
	harness.verifier.now = time.Now
	if err := harness.service.RevokeCapability(ctx, maintainer, 42, claims.TokenID, "device lost"); err != nil {
		t.Fatalf("revoke capability: %v", err)
	}
	if _, err := harness.verifier.Verify(ctx, raw, 42, CapabilityAudience, ScopePropose); !errors.Is(err, ErrCapabilityNotActive) {
		t.Fatalf("revoked capability error = %v", err)
	}
}

func TestCapabilityVerificationFailsClosedWhenRevocationStoreIsUnavailable(t *testing.T) {
	harness := newShuraHarness(t)
	raw, _, err := harness.service.IssueCapability(context.Background(), harness.users[3], IssueCapabilityRequest{
		WorkspaceID: 42, Role: RoleViewer, Scopes: []Scope{ScopeRead}, Audience: CapabilityAudience, TTL: time.Minute,
	})
	if err != nil {
		t.Fatalf("issue viewer capability: %v", err)
	}
	if err := harness.repository.Close(); err != nil {
		t.Fatalf("close revocation store: %v", err)
	}
	if _, err := harness.verifier.Verify(context.Background(), raw, 42, CapabilityAudience, ScopeRead); !errors.Is(err, ErrRevocationCheck) {
		t.Fatalf("closed revocation store error = %v", err)
	}
}

func TestGovernanceLifecycleRequiresQuorumAndExplicitDecision(t *testing.T) {
	harness := newShuraHarness(t)
	ctx := context.Background()
	architectToken := issueTestCapability(t, harness, 1, RoleArchitect, []Scope{ScopeRead, ScopeDeliberate, ScopeVote, ScopePropose, ScopeDecide})
	maintainerToken := issueTestCapability(t, harness, 2, RoleMaintainer, []Scope{ScopeRead, ScopeDeliberate, ScopeVote, ScopePropose})
	viewerToken := issueTestCapability(t, harness, 3, RoleViewer, []Scope{ScopeRead})

	proposal, err := harness.service.CreateProposal(ctx, maintainerToken, CreateProposalRequest{
		WorkspaceID: 42, Title: "Extend pantry hours", Body: "Open two evenings each week.",
		Policy: ProposalPolicy{Quorum: 2, ApprovalThreshold: 2, RequiredApprovers: []string{"1"}},
	})
	if err != nil || proposal.Status != ProposalOpen || proposal.Version != 1 {
		t.Fatalf("create proposal: proposal=%+v err=%v", proposal, err)
	}
	entry, proposal, err := harness.service.AddDeliberation(ctx, maintainerToken, proposal.ID, proposal.Version, "Volunteer coverage is confirmed.")
	if err != nil || entry.ProposalVersion != 2 || proposal.Version != 2 {
		t.Fatalf("add deliberation: entry=%+v proposal=%+v err=%v", entry, proposal, err)
	}
	if _, _, err := harness.service.RecordVote(ctx, maintainerToken, proposal.ID, 1, VoteApprove, "stale"); !errors.Is(err, ErrGovernanceConflict) {
		t.Fatalf("stale vote error = %v", err)
	}
	_, proposal, err = harness.service.RecordVote(ctx, maintainerToken, proposal.ID, proposal.Version, VoteApprove, "Capacity is available.")
	if err != nil || proposal.Version != 3 {
		t.Fatalf("maintainer vote: proposal=%+v err=%v", proposal, err)
	}
	if _, _, err := harness.service.RecordDecision(ctx, architectToken, proposal.ID, proposal.Version, DecisionApproved, "Proceed."); !errors.Is(err, ErrGovernanceTransition) {
		t.Fatalf("premature decision error = %v", err)
	}
	beforeDecision, err := harness.service.GetProposal(ctx, viewerToken, proposal.ID)
	if err != nil || beforeDecision.Decision != nil || beforeDecision.Proposal.Status != ProposalOpen {
		t.Fatalf("proposal auto-decided: record=%+v err=%v", beforeDecision, err)
	}
	_, proposal, err = harness.service.RecordVote(ctx, architectToken, proposal.ID, proposal.Version, VoteApprove, "Governance review complete.")
	if err != nil || proposal.Version != 4 {
		t.Fatalf("architect vote: proposal=%+v err=%v", proposal, err)
	}
	decision, proposal, err := harness.service.RecordDecision(ctx, architectToken, proposal.ID, proposal.Version, DecisionApproved, "Approved after explicit review.")
	if err != nil || proposal.Status != ProposalDecided || decision.VoteCount != 2 || decision.ApprovalCount != 2 || decision.ID == "" {
		t.Fatalf("record decision: decision=%+v proposal=%+v err=%v", decision, proposal, err)
	}
	record, err := harness.service.GetProposal(ctx, viewerToken, proposal.ID)
	if err != nil || record.Decision.ID != decision.ID || len(record.Deliberation) != 1 || len(record.Votes) != 2 {
		t.Fatalf("read final record: record=%+v err=%v", record, err)
	}
	if _, err := harness.repository.db.ExecContext(ctx, `UPDATE shura_votes SET choice = 'REJECT' WHERE proposal_id = ?`, proposal.ID); err == nil {
		t.Fatal("append-only vote update unexpectedly succeeded")
	}
	if _, err := harness.repository.db.ExecContext(ctx, `DELETE FROM shura_audit_events WHERE entity_id = ?`, proposal.ID); err == nil {
		t.Fatal("append-only audit delete unexpectedly succeeded")
	}
	events, err := harness.service.ProposalAudit(ctx, viewerToken, proposal.ID)
	if err != nil || len(events) != 5 {
		t.Fatalf("proposal audit: events=%+v err=%v", events, err)
	}
	for index, event := range events {
		if event.Hash == "" || (index == 0 && event.PreviousHash != "") || (index > 0 && event.PreviousHash != events[index-1].Hash) {
			t.Fatalf("broken audit chain at event %d: %+v", index, event)
		}
	}
}

func TestInvitationsRecordAcceptedExpiredAndRevokedStates(t *testing.T) {
	harness := newShuraHarness(t)
	ctx := context.Background()
	base := time.Date(2026, 8, 16, 20, 0, 0, 0, time.UTC)
	harness.service.now = func() time.Time { return base }
	grant, err := harness.service.CreateInvitation(ctx, harness.users[1], 42, "viewer@example.com", RoleViewer, time.Hour)
	if err != nil || grant.Token == "" || grant.Invitation.Status != InvitationPending {
		t.Fatalf("create invitation: grant=%+v err=%v", grant, err)
	}
	accepted, err := harness.service.AcceptInvitation(ctx, harness.users[3], grant.Token)
	if err != nil || accepted.Status != InvitationAccepted || accepted.AcceptedBy != "3" {
		t.Fatalf("accept invitation: invitation=%+v err=%v", accepted, err)
	}
	expiring, err := harness.service.CreateInvitation(ctx, harness.users[1], 42, "2", RoleMaintainer, time.Second)
	if err != nil {
		t.Fatalf("create expiring invitation: %v", err)
	}
	harness.service.now = func() time.Time { return base.Add(2 * time.Second) }
	expired, err := harness.service.AcceptInvitation(ctx, harness.users[2], expiring.Token)
	if !errors.Is(err, ErrCapabilityExpired) || expired.Status != InvitationExpired {
		t.Fatalf("expire invitation: invitation=%+v err=%v", expired, err)
	}
	harness.service.now = func() time.Time { return base }
	revoking, err := harness.service.CreateInvitation(ctx, harness.users[1], 42, "new@example.com", RoleViewer, time.Hour)
	if err != nil {
		t.Fatalf("create revoked invitation: %v", err)
	}
	revoked, err := harness.service.RevokeInvitation(ctx, harness.users[1], revoking.Invitation.ID, revoking.Invitation.Version)
	if err != nil || revoked.Status != InvitationRevoked {
		t.Fatalf("revoke invitation: invitation=%+v err=%v", revoked, err)
	}
}

func TestHTTPHandlerPublishesJWKSAndRequiresBearerCapability(t *testing.T) {
	harness := newShuraHarness(t)
	handler, err := NewHTTPHandler(harness.service, func(*http.Request) (*models.User, error) { return harness.users[1], nil })
	if err != nil {
		t.Fatalf("create HTTP handler: %v", err)
	}
	jwksRequest := httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil)
	jwksResponse := httptest.NewRecorder()
	handler.ServeHTTP(jwksResponse, jwksRequest)
	if jwksResponse.Code != http.StatusOK {
		t.Fatalf("JWKS status = %d", jwksResponse.Code)
	}
	var keys JWKSet
	if err := json.Unmarshal(jwksResponse.Body.Bytes(), &keys); err != nil || len(keys.Keys) != 1 || keys.Keys[0].KeyID != "key-2026-01" || keys.Keys[0].X == "" {
		t.Fatalf("JWKS response: keys=%+v err=%v", keys, err)
	}
	raw := issueTestCapability(t, harness, 2, RoleMaintainer, []Scope{ScopePropose})
	body := `{"title":"Repair the roof","body":"Fund the urgent repair.","policy":{"quorum":1,"approval_threshold":1}}`
	request := httptest.NewRequest(http.MethodPost, "/v1/workspaces/42/proposals", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+raw)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create proposal status = %d body=%s", response.Code, response.Body.String())
	}
	unauthorized := httptest.NewRequest(http.MethodPost, "/v1/workspaces/42/proposals", strings.NewReader(body))
	unauthorizedResponse := httptest.NewRecorder()
	handler.ServeHTTP(unauthorizedResponse, unauthorized)
	if unauthorizedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("missing bearer status = %d body=%s", unauthorizedResponse.Code, unauthorizedResponse.Body.String())
	}
}

func TestAcceptedInvitationAddsMappedWorkspaceMembershipAndRetriesSafely(t *testing.T) {
	harness := newShuraHarness(t)
	delete(harness.workspaces.roles, 3)
	memberships := &recordingMembershipAccepter{roles: harness.workspaces.roles}
	service, err := NewServiceWithMembership(harness.repository, harness.service.issuer, harness.verifier, memberships)
	if err != nil {
		t.Fatalf("NewServiceWithMembership() error = %v", err)
	}
	grant, err := service.CreateInvitation(context.Background(), harness.users[1], 42, harness.users[3].Email, RoleArchitect, time.Hour)
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}
	accepted, err := service.AcceptInvitation(context.Background(), harness.users[3], grant.Token)
	if err != nil || accepted.Status != InvitationAccepted {
		t.Fatalf("AcceptInvitation() = %+v, %v", accepted, err)
	}
	if len(memberships.calls) != 1 || memberships.calls[0].workspaceID != 42 || memberships.calls[0].role != models.WorkspaceRoleAdmin || memberships.calls[0].actorID != 3 {
		t.Fatalf("membership calls = %+v", memberships.calls)
	}
	if _, err := service.AcceptInvitation(context.Background(), harness.users[3], grant.Token); err != nil {
		t.Fatalf("idempotent AcceptInvitation() error = %v", err)
	}
	if len(memberships.calls) != 1 {
		t.Fatalf("idempotent retry re-added membership: %+v", memberships.calls)
	}
	delete(harness.workspaces.roles, 3)
	if _, err := service.AcceptInvitation(context.Background(), harness.users[3], grant.Token); !errors.Is(err, ErrWorkspaceDenied) {
		t.Fatalf("replayed accepted invitation restored removed membership: %v", err)
	}
}

type shuraHarness struct {
	repository *Repository
	service    *Service
	verifier   *CapabilityVerifier
	workspaces *fakeWorkspaceAuthorizer
	users      map[int]*models.User
}

func newShuraHarness(t *testing.T) *shuraHarness {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate issuer key: %v", err)
	}
	issuer, err := NewCapabilityIssuer("https://issuer.taawun.example", "key-2026-01", privateKey)
	if err != nil {
		t.Fatalf("create issuer: %v", err)
	}
	registry, err := NewStaticKeyRegistry("https://issuer.taawun.example", "key-2026-01", issuer.PublicKey())
	if err != nil {
		t.Fatalf("create key registry: %v", err)
	}
	repository, err := OpenRepository(filepath.Join(t.TempDir(), "shura.db"))
	if err != nil {
		t.Fatalf("open Shura repository: %v", err)
	}
	t.Cleanup(func() { _ = repository.Close() })
	workspaces := &fakeWorkspaceAuthorizer{roles: map[int]Role{1: RoleArchitect, 2: RoleMaintainer, 3: RoleViewer}}
	verifier, err := NewCapabilityVerifier(registry, repository, workspaces)
	if err != nil {
		t.Fatalf("create verifier: %v", err)
	}
	service, err := NewService(repository, issuer, verifier)
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	return &shuraHarness{
		repository: repository, service: service, verifier: verifier, workspaces: workspaces,
		users: map[int]*models.User{
			1: {ID: 1, Email: "architect@example.com"},
			2: {ID: 2, Email: "maintainer@example.com"},
			3: {ID: 3, Email: "viewer@example.com"},
		},
	}
}

func issueTestCapability(t *testing.T, harness *shuraHarness, userID int, role Role, scopes []Scope) string {
	t.Helper()
	raw, _, err := harness.service.IssueCapability(context.Background(), harness.users[userID], IssueCapabilityRequest{WorkspaceID: 42, Role: role, Scopes: scopes, Audience: CapabilityAudience, TTL: 10 * time.Minute})
	if err != nil {
		t.Fatalf("issue %s capability: %v", role, err)
	}
	return raw
}

type fakeWorkspaceAuthorizer struct {
	roles map[int]Role
}

type membershipCall struct {
	actorID     int
	workspaceID int
	role        string
}

type recordingMembershipAccepter struct {
	calls []membershipCall
	roles map[int]Role
}

func (a *recordingMembershipAccepter) AcceptWorkspaceInvitation(_ context.Context, actor *models.User, workspaceID int, role string) error {
	a.calls = append(a.calls, membershipCall{actorID: actor.ID, workspaceID: workspaceID, role: role})
	switch role {
	case models.WorkspaceRoleAdmin:
		a.roles[actor.ID] = RoleArchitect
	case models.WorkspaceRoleMember:
		a.roles[actor.ID] = RoleMaintainer
	case models.WorkspaceRoleViewer:
		a.roles[actor.ID] = RoleViewer
	}
	return nil
}

func (a *fakeWorkspaceAuthorizer) AuthorizeWorkspaceCapability(actor *models.User, workspaceID int, capability models.WorkspaceCapability) (*models.Workspace, error) {
	if actor == nil || workspaceID != 42 {
		return nil, errors.New("denied")
	}
	role, ok := a.roles[actor.ID]
	if !ok {
		return nil, errors.New("denied")
	}
	allowed := capability == models.WorkspaceCapabilityView ||
		(role == RoleMaintainer && capability == models.WorkspaceCapabilityBuild) ||
		(role == RoleArchitect && (capability == models.WorkspaceCapabilityBuild || capability == models.WorkspaceCapabilityPublish))
	if !allowed {
		return nil, errors.New("denied")
	}
	return &models.Workspace{ID: workspaceID, Status: models.WorkspaceStatusActive}, nil
}

func differentBase64Character(value byte) string {
	if value == 'A' {
		return "B"
	}
	return "A"
}
