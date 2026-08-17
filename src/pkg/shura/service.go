package shura

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"sort"
	"strconv"
	"strings"
	"time"

	"taawun/pkg/models"
)

type InvitationStatus string

const (
	InvitationPending  InvitationStatus = "PENDING"
	InvitationAccepted InvitationStatus = "ACCEPTED"
	InvitationExpired  InvitationStatus = "EXPIRED"
	InvitationRevoked  InvitationStatus = "REVOKED"
)

type Invitation struct {
	ID          string           `json:"id"`
	WorkspaceID int              `json:"workspace_id"`
	Invitee     string           `json:"invitee"`
	Role        Role             `json:"role"`
	Status      InvitationStatus `json:"status"`
	Version     int64            `json:"version"`
	CreatedBy   string           `json:"created_by"`
	AcceptedBy  string           `json:"accepted_by,omitempty"`
	ExpiresAt   time.Time        `json:"expires_at"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

type InvitationGrant struct {
	Invitation *Invitation `json:"invitation"`
	Token      string      `json:"token"`
}

type ProposalStatus string

const (
	ProposalOpen      ProposalStatus = "OPEN"
	ProposalDecided   ProposalStatus = "DECIDED"
	ProposalCancelled ProposalStatus = "CANCELLED"
)

type VoteChoice string

const (
	VoteApprove VoteChoice = "APPROVE"
	VoteReject  VoteChoice = "REJECT"
	VoteAbstain VoteChoice = "ABSTAIN"
)

type DecisionOutcome string

const (
	DecisionApproved DecisionOutcome = "APPROVED"
	DecisionRejected DecisionOutcome = "REJECTED"
)

type ProposalPolicy struct {
	Quorum            int      `json:"quorum"`
	ApprovalThreshold int      `json:"approval_threshold"`
	RequiredApprovers []string `json:"required_approvers,omitempty"`
}

type Proposal struct {
	ID          string         `json:"id"`
	WorkspaceID int            `json:"workspace_id"`
	Title       string         `json:"title"`
	Body        string         `json:"body"`
	Policy      ProposalPolicy `json:"policy"`
	Status      ProposalStatus `json:"status"`
	Version     int64          `json:"version"`
	CreatedBy   string         `json:"created_by"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type DeliberationEntry struct {
	ID              string    `json:"id"`
	ProposalID      string    `json:"proposal_id"`
	Author          string    `json:"author"`
	Body            string    `json:"body"`
	ProposalVersion int64     `json:"proposal_version"`
	CreatedAt       time.Time `json:"created_at"`
}

type Vote struct {
	ID              string     `json:"id"`
	ProposalID      string     `json:"proposal_id"`
	Voter           string     `json:"voter"`
	Choice          VoteChoice `json:"choice"`
	Rationale       string     `json:"rationale,omitempty"`
	Role            Role       `json:"role"`
	ProposalVersion int64      `json:"proposal_version"`
	CreatedAt       time.Time  `json:"created_at"`
}

type Decision struct {
	ID              string          `json:"id"`
	ProposalID      string          `json:"proposal_id"`
	Outcome         DecisionOutcome `json:"outcome"`
	Rationale       string          `json:"rationale"`
	DecidedBy       string          `json:"decided_by"`
	VoteCount       int             `json:"vote_count"`
	ApprovalCount   int             `json:"approval_count"`
	RejectionCount  int             `json:"rejection_count"`
	ProposalVersion int64           `json:"proposal_version"`
	CreatedAt       time.Time       `json:"created_at"`
}

type ProposalRecord struct {
	Proposal     *Proposal           `json:"proposal"`
	Deliberation []DeliberationEntry `json:"deliberation"`
	Votes        []Vote              `json:"votes"`
	Decision     *Decision           `json:"decision,omitempty"`
}

type AuditEvent struct {
	Sequence      int64           `json:"sequence"`
	ID            string          `json:"id"`
	WorkspaceID   int             `json:"workspace_id"`
	EntityType    string          `json:"entity_type"`
	EntityID      string          `json:"entity_id"`
	EntityVersion int64           `json:"entity_version"`
	EventType     string          `json:"event_type"`
	Actor         string          `json:"actor"`
	Detail        json.RawMessage `json:"detail"`
	PreviousHash  string          `json:"previous_hash,omitempty"`
	Hash          string          `json:"hash"`
	CreatedAt     time.Time       `json:"created_at"`
}

type IssueCapabilityRequest struct {
	WorkspaceID int
	Role        Role
	Scopes      []Scope
	Audience    string
	TTL         time.Duration
}

type CreateProposalRequest struct {
	WorkspaceID int            `json:"workspace_id"`
	Title       string         `json:"title"`
	Body        string         `json:"body"`
	Policy      ProposalPolicy `json:"policy"`
}

type Service struct {
	repository  *Repository
	issuer      *CapabilityIssuer
	verifier    *CapabilityVerifier
	workspaces  WorkspaceAuthorizer
	memberships MembershipAccepter
	now         func() time.Time
}

// MembershipAccepter joins an accepted invitation to the authoritative workspace membership store.
type MembershipAccepter interface {
	AcceptWorkspaceInvitation(context.Context, *models.User, int, string) error
}

func NewService(repository *Repository, issuer *CapabilityIssuer, verifier *CapabilityVerifier) (*Service, error) {
	return NewServiceWithMembership(repository, issuer, verifier, nil)
}

// NewServiceWithMembership adds the production invitation-to-membership bridge.
func NewServiceWithMembership(repository *Repository, issuer *CapabilityIssuer, verifier *CapabilityVerifier, memberships MembershipAccepter) (*Service, error) {
	if repository == nil || issuer == nil || verifier == nil || verifier.workspaces == nil {
		return nil, ErrGovernanceInvalid
	}
	return &Service{repository: repository, issuer: issuer, verifier: verifier, workspaces: verifier.workspaces, memberships: memberships, now: time.Now}, nil
}

func (s *Service) IssueCapability(ctx context.Context, actor *models.User, request IssueCapabilityRequest) (string, *CapabilityClaims, error) {
	if actor == nil || actor.ID <= 0 || request.WorkspaceID <= 0 || request.Audience != CapabilityAudience || request.TTL < time.Second || request.TTL > maxCapabilityTTL || !validRole(request.Role) {
		return "", nil, ErrCapabilityInvalid
	}
	if err := authorizeWorkspaceRole(s.workspaces, actor, request.WorkspaceID, request.Role); err != nil {
		return "", nil, err
	}
	scopes := append([]Scope(nil), request.Scopes...)
	sort.Slice(scopes, func(i, j int) bool { return scopes[i] < scopes[j] })
	for index, scope := range scopes {
		if !scopeAllowed(request.Role, scope) || (index > 0 && scopes[index-1] == scope) {
			return "", nil, ErrCapabilityForbidden
		}
	}
	if len(scopes) == 0 {
		return "", nil, ErrCapabilityInvalid
	}
	tokenID, err := secureID("cap")
	if err != nil {
		return "", nil, err
	}
	now := s.now().UTC().Truncate(time.Second)
	claims := CapabilityClaims{
		KeyID: s.issuer.keyID, Issuer: s.issuer.issuer, Audience: request.Audience,
		TokenID: tokenID, Subject: strconv.Itoa(actor.ID), WorkspaceID: request.WorkspaceID,
		Role: request.Role, Scopes: scopes, IssuedAt: now.Unix(), NotBefore: now.Unix(),
		ExpiresAt: now.Add(request.TTL).Unix(),
	}
	raw, err := s.issuer.sign(claims)
	if err != nil {
		return "", nil, err
	}
	hash := sha256.Sum256([]byte(raw))
	scopesJSON, _ := json.Marshal(scopes)
	tx, err := s.repository.db.BeginTx(ctx, nil)
	if err != nil {
		return "", nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO shura_capabilities
        (jti, token_hash, issuer, kid, subject, workspace_id, role, audience, scopes_json, status, version, issued_at, expires_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'ACTIVE', 1, ?, ?)`,
		claims.TokenID, hex.EncodeToString(hash[:]), claims.Issuer, claims.KeyID, claims.Subject, claims.WorkspaceID,
		claims.Role, claims.Audience, scopesJSON, claims.IssuedAt, claims.ExpiresAt); err != nil {
		return "", nil, err
	}
	if err := appendAudit(ctx, tx, claims.WorkspaceID, "capability", claims.TokenID, 1, "CAPABILITY_ISSUED", claims.Subject, map[string]any{"role": claims.Role, "scopes": scopes}, now); err != nil {
		return "", nil, err
	}
	if err := tx.Commit(); err != nil {
		return "", nil, err
	}
	return raw, &claims, nil
}

func (s *Service) RevokeCapability(ctx context.Context, actor *models.User, workspaceID int, tokenID, reason string) error {
	reason = strings.TrimSpace(reason)
	if actor == nil || actor.ID <= 0 || workspaceID <= 0 || !validIdentifier(tokenID) || !validText(reason, 300) {
		return ErrGovernanceInvalid
	}
	tx, err := s.repository.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var subject, status string
	var version int64
	if err := tx.QueryRowContext(ctx, `SELECT subject, status, version FROM shura_capabilities WHERE jti = ? AND workspace_id = ?`, tokenID, workspaceID).Scan(&subject, &status, &version); errors.Is(err, sql.ErrNoRows) {
		return ErrGovernanceNotFound
	} else if err != nil {
		return err
	}
	actorID := strconv.Itoa(actor.ID)
	if actorID != subject {
		if err := authorizeWorkspaceRole(s.workspaces, actor, workspaceID, RoleArchitect); err != nil {
			return err
		}
	}
	if status != "ACTIVE" {
		return ErrCapabilityNotActive
	}
	now := s.now().UTC()
	result, err := tx.ExecContext(ctx, `UPDATE shura_capabilities SET status = 'REVOKED', version = version + 1 WHERE jti = ? AND version = ? AND status = 'ACTIVE'`, tokenID, version)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return ErrGovernanceConflict
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO shura_revocations (jti, reason, revoked_by, revoked_at) VALUES (?, ?, ?, ?)`, tokenID, reason, actorID, now.UnixMilli()); err != nil {
		return err
	}
	if err := appendAudit(ctx, tx, workspaceID, "capability", tokenID, version+1, "CAPABILITY_REVOKED", actorID, map[string]any{"reason": reason}, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) CreateInvitation(ctx context.Context, actor *models.User, workspaceID int, invitee string, role Role, ttl time.Duration) (*InvitationGrant, error) {
	invitee = strings.TrimSpace(strings.ToLower(invitee))
	if actor == nil || actor.ID <= 0 || workspaceID <= 0 || !validInvitee(invitee) || !validRole(role) || ttl <= 0 || ttl > 7*24*time.Hour {
		return nil, ErrGovernanceInvalid
	}
	if err := authorizeWorkspaceRole(s.workspaces, actor, workspaceID, RoleArchitect); err != nil {
		return nil, err
	}
	id, err := secureID("invite")
	if err != nil {
		return nil, err
	}
	raw, err := secureID("accept")
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256([]byte(raw))
	now := s.now().UTC().Truncate(time.Millisecond)
	invitation := &Invitation{ID: id, WorkspaceID: workspaceID, Invitee: invitee, Role: role, Status: InvitationPending, Version: 1, CreatedBy: strconv.Itoa(actor.ID), ExpiresAt: now.Add(ttl), CreatedAt: now, UpdatedAt: now}
	tx, err := s.repository.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO shura_invitations
        (id, workspace_id, invitee, role, token_hash, status, version, created_by, expires_at, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?, ?, ?)`, id, workspaceID, invitee, role, hex.EncodeToString(hash[:]), InvitationPending, invitation.CreatedBy, invitation.ExpiresAt.UnixMilli(), now.UnixMilli(), now.UnixMilli()); err != nil {
		return nil, err
	}
	if err := appendAudit(ctx, tx, workspaceID, "invitation", id, 1, "INVITATION_CREATED", invitation.CreatedBy, map[string]any{"role": role}, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &InvitationGrant{Invitation: invitation, Token: raw}, nil
}

// AcceptInvitation records acceptance; WorkspaceService still owns membership changes.
func (s *Service) AcceptInvitation(ctx context.Context, actor *models.User, rawToken string) (*Invitation, error) {
	if actor == nil || actor.ID <= 0 || rawToken == "" {
		return nil, ErrGovernanceInvalid
	}
	hash := sha256.Sum256([]byte(rawToken))
	tx, err := s.repository.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	invitation, err := scanInvitation(tx.QueryRowContext(ctx, invitationSelect+` WHERE token_hash = ?`, hex.EncodeToString(hash[:])))
	if err != nil {
		return nil, err
	}
	actorID := strconv.Itoa(actor.ID)
	if invitation.Invitee != strings.ToLower(actor.Email) && invitation.Invitee != actorID {
		return nil, ErrCapabilityForbidden
	}
	if invitation.Status == InvitationAccepted {
		if invitation.AcceptedBy != actorID {
			return nil, ErrCapabilityForbidden
		}
		// An accepted token is idempotent only for an existing membership. Replaying it
		// must not restore a member whom an administrator later removed.
		if err := authorizeWorkspaceRole(s.workspaces, actor, invitation.WorkspaceID, RoleViewer); err != nil {
			return invitation, err
		}
		return invitation, nil
	}
	if invitation.Status != InvitationPending {
		return nil, ErrGovernanceTransition
	}
	now := s.now().UTC().Truncate(time.Millisecond)
	status := InvitationAccepted
	acceptedBy := actorID
	if !now.Before(invitation.ExpiresAt) {
		status = InvitationExpired
		acceptedBy = ""
	}
	result, err := tx.ExecContext(ctx, `UPDATE shura_invitations SET status = ?, version = version + 1, accepted_by = ?, updated_at = ? WHERE id = ? AND version = ? AND status = 'PENDING'`, status, acceptedBy, now.UnixMilli(), invitation.ID, invitation.Version)
	if err != nil {
		return nil, err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return nil, ErrGovernanceTransition
	}
	eventType := "INVITATION_ACCEPTED"
	if status == InvitationExpired {
		eventType = "INVITATION_EXPIRED"
	}
	if err := appendAudit(ctx, tx, invitation.WorkspaceID, "invitation", invitation.ID, invitation.Version+1, eventType, actorID, map[string]any{}, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	invitation.Status, invitation.Version, invitation.AcceptedBy, invitation.UpdatedAt = status, invitation.Version+1, acceptedBy, now
	if status == InvitationExpired {
		return invitation, ErrCapabilityExpired
	}
	if err := s.acceptMembership(ctx, actor, invitation); err != nil {
		return invitation, err
	}
	return invitation, nil
}

func (s *Service) acceptMembership(ctx context.Context, actor *models.User, invitation *Invitation) error {
	if s.memberships == nil {
		return nil
	}
	role, err := workspaceRoleForInvitation(invitation.Role)
	if err != nil {
		return err
	}
	return s.memberships.AcceptWorkspaceInvitation(ctx, actor, invitation.WorkspaceID, role)
}

func workspaceRoleForInvitation(role Role) (string, error) {
	switch role {
	case RoleArchitect:
		return models.WorkspaceRoleAdmin, nil
	case RoleMaintainer:
		return models.WorkspaceRoleMember, nil
	case RoleViewer:
		return models.WorkspaceRoleViewer, nil
	default:
		return "", ErrGovernanceInvalid
	}
}

// ResolveDecision verifies one final decision as internal evidence for another trusted service.
func (s *Service) ResolveDecision(ctx context.Context, workspaceID int, reference string) (DecisionResolution, error) {
	if s == nil || s.repository == nil || workspaceID <= 0 || !validIdentifier(reference) {
		return DecisionResolution{}, ErrGovernanceNotFound
	}
	var outcome string
	err := s.repository.db.QueryRowContext(ctx, `SELECT d.outcome
		FROM shura_decisions d JOIN shura_proposals p ON p.id = d.proposal_id
		WHERE d.id = ? AND p.workspace_id = ? AND p.status = 'DECIDED'`, reference, workspaceID).Scan(&outcome)
	if errors.Is(err, sql.ErrNoRows) {
		return DecisionResolution{}, ErrGovernanceNotFound
	}
	if err != nil {
		return DecisionResolution{}, err
	}
	return DecisionResolution{Reference: reference, WorkspaceID: workspaceID, Approved: DecisionOutcome(outcome) == DecisionApproved}, nil
}

// DecisionResolution is a narrow, immutable decision verification result.
type DecisionResolution struct {
	Reference   string
	WorkspaceID int
	Approved    bool
}

func (s *Service) RevokeInvitation(ctx context.Context, actor *models.User, invitationID string, expectedVersion int64) (*Invitation, error) {
	invitation, err := s.getInvitation(ctx, invitationID)
	if err != nil {
		return nil, err
	}
	if err := authorizeWorkspaceRole(s.workspaces, actor, invitation.WorkspaceID, RoleArchitect); err != nil {
		return nil, err
	}
	if invitation.Version != expectedVersion {
		return nil, ErrGovernanceConflict
	}
	if invitation.Status != InvitationPending {
		return nil, ErrGovernanceTransition
	}
	now := s.now().UTC().Truncate(time.Millisecond)
	tx, err := s.repository.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE shura_invitations SET status = 'REVOKED', version = version + 1, updated_at = ? WHERE id = ? AND version = ? AND status = 'PENDING'`, now.UnixMilli(), invitation.ID, expectedVersion)
	if err != nil {
		return nil, err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return nil, ErrGovernanceConflict
	}
	if err := appendAudit(ctx, tx, invitation.WorkspaceID, "invitation", invitation.ID, expectedVersion+1, "INVITATION_REVOKED", strconv.Itoa(actor.ID), map[string]any{}, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	invitation.Status, invitation.Version, invitation.UpdatedAt = InvitationRevoked, expectedVersion+1, now
	return invitation, nil
}

func (s *Service) CreateProposal(ctx context.Context, rawCapability string, request CreateProposalRequest) (*Proposal, error) {
	claims, err := s.verifier.Verify(ctx, rawCapability, request.WorkspaceID, CapabilityAudience, ScopePropose)
	if err != nil {
		return nil, err
	}
	request.Title = strings.TrimSpace(request.Title)
	request.Body = strings.TrimSpace(request.Body)
	policy, err := validatePolicy(request.Policy)
	if !validText(request.Title, 160) || !validText(request.Body, 10000) || err != nil {
		return nil, ErrGovernanceInvalid
	}
	id, err := secureID("proposal")
	if err != nil {
		return nil, err
	}
	now := s.now().UTC().Truncate(time.Millisecond)
	proposal := &Proposal{ID: id, WorkspaceID: request.WorkspaceID, Title: request.Title, Body: request.Body, Policy: policy, Status: ProposalOpen, Version: 1, CreatedBy: claims.Subject, CreatedAt: now, UpdatedAt: now}
	policyJSON, _ := json.Marshal(policy)
	tx, err := s.repository.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO shura_proposals
        (id, workspace_id, title, body, policy_json, status, version, created_by, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, 'OPEN', 1, ?, ?, ?)`, id, request.WorkspaceID, request.Title, request.Body, policyJSON, claims.Subject, now.UnixMilli(), now.UnixMilli()); err != nil {
		return nil, err
	}
	if err := appendAudit(ctx, tx, request.WorkspaceID, "proposal", id, 1, "PROPOSAL_OPENED", claims.Subject, map[string]any{"policy": policy}, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return proposal, nil
}

func (s *Service) AddDeliberation(ctx context.Context, rawCapability, proposalID string, expectedVersion int64, body string) (*DeliberationEntry, *Proposal, error) {
	proposal, err := s.getProposal(ctx, proposalID)
	if err != nil {
		return nil, nil, err
	}
	claims, err := s.verifier.Verify(ctx, rawCapability, proposal.WorkspaceID, CapabilityAudience, ScopeDeliberate)
	if err != nil {
		return nil, nil, err
	}
	body = strings.TrimSpace(body)
	if !validText(body, 5000) || proposal.Status != ProposalOpen {
		return nil, nil, ErrGovernanceTransition
	}
	entryID, err := secureID("deliberation")
	if err != nil {
		return nil, nil, err
	}
	newVersion, now, tx, err := s.beginProposalMutation(ctx, proposal, expectedVersion)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO shura_deliberation_entries (id, proposal_id, author, body, proposal_version, created_at) VALUES (?, ?, ?, ?, ?, ?)`, entryID, proposal.ID, claims.Subject, body, newVersion, now.UnixMilli()); err != nil {
		return nil, nil, err
	}
	if err := appendAudit(ctx, tx, proposal.WorkspaceID, "proposal", proposal.ID, newVersion, "DELIBERATION_RECORDED", claims.Subject, map[string]any{"entry_id": entryID}, now); err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	proposal.Version, proposal.UpdatedAt = newVersion, now
	return &DeliberationEntry{ID: entryID, ProposalID: proposal.ID, Author: claims.Subject, Body: body, ProposalVersion: newVersion, CreatedAt: now}, proposal, nil
}

func (s *Service) RecordVote(ctx context.Context, rawCapability, proposalID string, expectedVersion int64, choice VoteChoice, rationale string) (*Vote, *Proposal, error) {
	proposal, err := s.getProposal(ctx, proposalID)
	if err != nil {
		return nil, nil, err
	}
	claims, err := s.verifier.Verify(ctx, rawCapability, proposal.WorkspaceID, CapabilityAudience, ScopeVote)
	if err != nil {
		return nil, nil, err
	}
	if proposal.Status != ProposalOpen || !validVoteChoice(choice) || len(rationale) > 1000 || strings.ContainsRune(rationale, '\x00') {
		return nil, nil, ErrGovernanceInvalid
	}
	voteID, err := secureID("vote")
	if err != nil {
		return nil, nil, err
	}
	newVersion, now, tx, err := s.beginProposalMutation(ctx, proposal, expectedVersion)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO shura_votes (id, proposal_id, voter, choice, rationale, role, proposal_version, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, voteID, proposal.ID, claims.Subject, choice, rationale, claims.Role, newVersion, now.UnixMilli()); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, nil, ErrGovernanceConflict
		}
		return nil, nil, err
	}
	if err := appendAudit(ctx, tx, proposal.WorkspaceID, "proposal", proposal.ID, newVersion, "VOTE_RECORDED", claims.Subject, map[string]any{"vote_id": voteID, "choice": choice}, now); err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	proposal.Version, proposal.UpdatedAt = newVersion, now
	return &Vote{ID: voteID, ProposalID: proposal.ID, Voter: claims.Subject, Choice: choice, Rationale: rationale, Role: claims.Role, ProposalVersion: newVersion, CreatedAt: now}, proposal, nil
}

func (s *Service) RecordDecision(ctx context.Context, rawCapability, proposalID string, expectedVersion int64, outcome DecisionOutcome, rationale string) (*Decision, *Proposal, error) {
	proposal, err := s.getProposal(ctx, proposalID)
	if err != nil {
		return nil, nil, err
	}
	claims, err := s.verifier.Verify(ctx, rawCapability, proposal.WorkspaceID, CapabilityAudience, ScopeDecide)
	if err != nil {
		return nil, nil, err
	}
	if proposal.Status != ProposalOpen || (outcome != DecisionApproved && outcome != DecisionRejected) || !validText(strings.TrimSpace(rationale), 2000) {
		return nil, nil, ErrGovernanceInvalid
	}
	votes, err := s.listVotes(ctx, proposal.ID)
	if err != nil {
		return nil, nil, err
	}
	approvals, rejections := 0, 0
	approvedBy := make(map[string]struct{})
	for _, vote := range votes {
		switch vote.Choice {
		case VoteApprove:
			approvals++
			approvedBy[vote.Voter] = struct{}{}
		case VoteReject:
			rejections++
		}
	}
	if len(votes) < proposal.Policy.Quorum {
		return nil, nil, fmt.Errorf("%w: quorum not met", ErrGovernanceTransition)
	}
	if outcome == DecisionApproved {
		if approvals < proposal.Policy.ApprovalThreshold {
			return nil, nil, fmt.Errorf("%w: approval threshold not met", ErrGovernanceTransition)
		}
		for _, required := range proposal.Policy.RequiredApprovers {
			if _, ok := approvedBy[required]; !ok {
				return nil, nil, fmt.Errorf("%w: required approver %s has not approved", ErrGovernanceTransition, required)
			}
		}
	}
	decisionID, err := secureID("decision")
	if err != nil {
		return nil, nil, err
	}
	newVersion, now, tx, err := s.beginProposalMutation(ctx, proposal, expectedVersion)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE shura_proposals SET status = 'DECIDED' WHERE id = ? AND version = ? AND status = 'OPEN'`, proposal.ID, newVersion)
	if err != nil {
		return nil, nil, err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return nil, nil, ErrGovernanceConflict
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO shura_decisions
        (id, proposal_id, outcome, rationale, decided_by, vote_count, approval_count, rejection_count, proposal_version, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, decisionID, proposal.ID, outcome, strings.TrimSpace(rationale), claims.Subject, len(votes), approvals, rejections, newVersion, now.UnixMilli()); err != nil {
		return nil, nil, err
	}
	if err := appendAudit(ctx, tx, proposal.WorkspaceID, "proposal", proposal.ID, newVersion, "DECISION_RECORDED", claims.Subject, map[string]any{"decision_id": decisionID, "outcome": outcome}, now); err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	proposal.Status, proposal.Version, proposal.UpdatedAt = ProposalDecided, newVersion, now
	decision := &Decision{ID: decisionID, ProposalID: proposal.ID, Outcome: outcome, Rationale: strings.TrimSpace(rationale), DecidedBy: claims.Subject, VoteCount: len(votes), ApprovalCount: approvals, RejectionCount: rejections, ProposalVersion: newVersion, CreatedAt: now}
	return decision, proposal, nil
}

func (s *Service) CancelProposal(ctx context.Context, rawCapability, proposalID string, expectedVersion int64, reason string) (*Proposal, error) {
	proposal, err := s.getProposal(ctx, proposalID)
	if err != nil {
		return nil, err
	}
	claims, err := s.verifier.Verify(ctx, rawCapability, proposal.WorkspaceID, CapabilityAudience, ScopeDecide)
	if err != nil {
		return nil, err
	}
	reason = strings.TrimSpace(reason)
	if proposal.Status != ProposalOpen || !validText(reason, 1000) {
		return nil, ErrGovernanceTransition
	}
	newVersion, now, tx, err := s.beginProposalMutation(ctx, proposal, expectedVersion)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE shura_proposals SET status = 'CANCELLED' WHERE id = ? AND version = ? AND status = 'OPEN'`, proposal.ID, newVersion)
	if err != nil {
		return nil, err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return nil, ErrGovernanceConflict
	}
	if err := appendAudit(ctx, tx, proposal.WorkspaceID, "proposal", proposal.ID, newVersion, "PROPOSAL_CANCELLED", claims.Subject, map[string]any{"reason": reason}, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	proposal.Status, proposal.Version, proposal.UpdatedAt = ProposalCancelled, newVersion, now
	return proposal, nil
}

func (s *Service) GetProposal(ctx context.Context, rawCapability, proposalID string) (*ProposalRecord, error) {
	proposal, err := s.getProposal(ctx, proposalID)
	if err != nil {
		return nil, err
	}
	if _, err := s.verifier.Verify(ctx, rawCapability, proposal.WorkspaceID, CapabilityAudience, ScopeRead); err != nil {
		return nil, err
	}
	deliberation, err := s.listDeliberation(ctx, proposal.ID)
	if err != nil {
		return nil, err
	}
	votes, err := s.listVotes(ctx, proposal.ID)
	if err != nil {
		return nil, err
	}
	decision, err := s.getDecision(ctx, proposal.ID)
	if err != nil && !errors.Is(err, ErrGovernanceNotFound) {
		return nil, err
	}
	return &ProposalRecord{Proposal: proposal, Deliberation: deliberation, Votes: votes, Decision: decision}, nil
}

func (s *Service) ProposalAudit(ctx context.Context, rawCapability, proposalID string) ([]AuditEvent, error) {
	proposal, err := s.getProposal(ctx, proposalID)
	if err != nil {
		return nil, err
	}
	if _, err := s.verifier.Verify(ctx, rawCapability, proposal.WorkspaceID, CapabilityAudience, ScopeRead); err != nil {
		return nil, err
	}
	rows, err := s.repository.db.QueryContext(ctx, `SELECT sequence, id, workspace_id, entity_type, entity_id, entity_version, event_type, actor, detail_json, previous_hash, event_hash, created_at
        FROM shura_audit_events WHERE entity_type = 'proposal' AND entity_id = ? ORDER BY sequence`, proposalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []AuditEvent
	for rows.Next() {
		var event AuditEvent
		var createdAt int64
		if err := rows.Scan(&event.Sequence, &event.ID, &event.WorkspaceID, &event.EntityType, &event.EntityID, &event.EntityVersion, &event.EventType, &event.Actor, &event.Detail, &event.PreviousHash, &event.Hash, &createdAt); err != nil {
			return nil, err
		}
		event.CreatedAt = time.UnixMilli(createdAt).UTC()
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *Service) beginProposalMutation(ctx context.Context, proposal *Proposal, expectedVersion int64) (int64, time.Time, *sql.Tx, error) {
	if proposal.Version != expectedVersion {
		return 0, time.Time{}, nil, ErrGovernanceConflict
	}
	tx, err := s.repository.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, time.Time{}, nil, err
	}
	now := s.now().UTC().Truncate(time.Millisecond)
	newVersion := expectedVersion + 1
	result, err := tx.ExecContext(ctx, `UPDATE shura_proposals SET version = ?, updated_at = ? WHERE id = ? AND version = ? AND status = 'OPEN'`, newVersion, now.UnixMilli(), proposal.ID, expectedVersion)
	if err != nil {
		_ = tx.Rollback()
		return 0, time.Time{}, nil, err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		_ = tx.Rollback()
		return 0, time.Time{}, nil, ErrGovernanceConflict
	}
	return newVersion, now, tx, nil
}

const invitationSelect = `SELECT id, workspace_id, invitee, role, status, version, created_by, accepted_by, expires_at, created_at, updated_at FROM shura_invitations`

func (s *Service) getInvitation(ctx context.Context, id string) (*Invitation, error) {
	if !validIdentifier(id) {
		return nil, ErrGovernanceNotFound
	}
	return scanInvitation(s.repository.db.QueryRowContext(ctx, invitationSelect+` WHERE id = ?`, id))
}

func scanInvitation(row interface{ Scan(...any) error }) (*Invitation, error) {
	var invitation Invitation
	var role, status string
	var expiresAt, createdAt, updatedAt int64
	err := row.Scan(&invitation.ID, &invitation.WorkspaceID, &invitation.Invitee, &role, &status, &invitation.Version, &invitation.CreatedBy, &invitation.AcceptedBy, &expiresAt, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGovernanceNotFound
	}
	if err != nil {
		return nil, err
	}
	invitation.Role, invitation.Status = Role(role), InvitationStatus(status)
	invitation.ExpiresAt, invitation.CreatedAt, invitation.UpdatedAt = time.UnixMilli(expiresAt).UTC(), time.UnixMilli(createdAt).UTC(), time.UnixMilli(updatedAt).UTC()
	return &invitation, nil
}

func (s *Service) getProposal(ctx context.Context, id string) (*Proposal, error) {
	if !validIdentifier(id) {
		return nil, ErrGovernanceNotFound
	}
	var proposal Proposal
	var policyJSON []byte
	var status string
	var createdAt, updatedAt int64
	err := s.repository.db.QueryRowContext(ctx, `SELECT id, workspace_id, title, body, policy_json, status, version, created_by, created_at, updated_at FROM shura_proposals WHERE id = ?`, id).Scan(
		&proposal.ID, &proposal.WorkspaceID, &proposal.Title, &proposal.Body, &policyJSON, &status, &proposal.Version, &proposal.CreatedBy, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGovernanceNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(policyJSON, &proposal.Policy); err != nil {
		return nil, err
	}
	proposal.Status = ProposalStatus(status)
	proposal.CreatedAt, proposal.UpdatedAt = time.UnixMilli(createdAt).UTC(), time.UnixMilli(updatedAt).UTC()
	return &proposal, nil
}

func (s *Service) listDeliberation(ctx context.Context, proposalID string) ([]DeliberationEntry, error) {
	rows, err := s.repository.db.QueryContext(ctx, `SELECT id, proposal_id, author, body, proposal_version, created_at FROM shura_deliberation_entries WHERE proposal_id = ? ORDER BY proposal_version`, proposalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []DeliberationEntry
	for rows.Next() {
		var entry DeliberationEntry
		var createdAt int64
		if err := rows.Scan(&entry.ID, &entry.ProposalID, &entry.Author, &entry.Body, &entry.ProposalVersion, &createdAt); err != nil {
			return nil, err
		}
		entry.CreatedAt = time.UnixMilli(createdAt).UTC()
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (s *Service) listVotes(ctx context.Context, proposalID string) ([]Vote, error) {
	rows, err := s.repository.db.QueryContext(ctx, `SELECT id, proposal_id, voter, choice, rationale, role, proposal_version, created_at FROM shura_votes WHERE proposal_id = ? ORDER BY proposal_version`, proposalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var votes []Vote
	for rows.Next() {
		var vote Vote
		var choice, role string
		var createdAt int64
		if err := rows.Scan(&vote.ID, &vote.ProposalID, &vote.Voter, &choice, &vote.Rationale, &role, &vote.ProposalVersion, &createdAt); err != nil {
			return nil, err
		}
		vote.Choice, vote.Role, vote.CreatedAt = VoteChoice(choice), Role(role), time.UnixMilli(createdAt).UTC()
		votes = append(votes, vote)
	}
	return votes, rows.Err()
}

func (s *Service) getDecision(ctx context.Context, proposalID string) (*Decision, error) {
	var decision Decision
	var outcome string
	var createdAt int64
	err := s.repository.db.QueryRowContext(ctx, `SELECT id, proposal_id, outcome, rationale, decided_by, vote_count, approval_count, rejection_count, proposal_version, created_at FROM shura_decisions WHERE proposal_id = ?`, proposalID).Scan(
		&decision.ID, &decision.ProposalID, &outcome, &decision.Rationale, &decision.DecidedBy, &decision.VoteCount, &decision.ApprovalCount, &decision.RejectionCount, &decision.ProposalVersion, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGovernanceNotFound
	}
	if err != nil {
		return nil, err
	}
	decision.Outcome, decision.CreatedAt = DecisionOutcome(outcome), time.UnixMilli(createdAt).UTC()
	return &decision, nil
}

func validatePolicy(policy ProposalPolicy) (ProposalPolicy, error) {
	if policy.Quorum <= 0 || policy.Quorum > 100 || policy.ApprovalThreshold <= 0 || policy.ApprovalThreshold > policy.Quorum || len(policy.RequiredApprovers) > 100 {
		return ProposalPolicy{}, ErrGovernanceInvalid
	}
	policy.RequiredApprovers = append([]string(nil), policy.RequiredApprovers...)
	sort.Strings(policy.RequiredApprovers)
	for index, approver := range policy.RequiredApprovers {
		if !validIdentifier(approver) || (index > 0 && policy.RequiredApprovers[index-1] == approver) {
			return ProposalPolicy{}, ErrGovernanceInvalid
		}
	}
	if len(policy.RequiredApprovers) > policy.ApprovalThreshold {
		return ProposalPolicy{}, ErrGovernanceInvalid
	}
	return policy, nil
}

func validVoteChoice(choice VoteChoice) bool {
	return choice == VoteApprove || choice == VoteReject || choice == VoteAbstain
}

func validText(value string, maximum int) bool {
	return len(value) > 0 && len(value) <= maximum && !strings.ContainsRune(value, '\x00')
}

func validInvitee(value string) bool {
	if len(value) == 0 || len(value) > 254 || strings.ContainsAny(value, "\x00\r\n") {
		return false
	}
	if _, err := strconv.Atoi(value); err == nil {
		return true
	}
	address, err := mail.ParseAddress(value)
	return err == nil && address.Address == value && strings.Contains(strings.SplitN(value, "@", 2)[1], ".")
}

func secureID(prefix string) (string, error) {
	bytes := make([]byte, 18)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return prefix + "_" + base64.RawURLEncoding.EncodeToString(bytes), nil
}
