package shura

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"taawun/pkg/models"
)

type CurrentUserFunc func(*http.Request) (*models.User, error)

type HTTPHandler struct {
	service     *Service
	currentUser CurrentUserFunc
}

func NewHTTPHandler(service *Service, currentUser CurrentUserFunc) (*HTTPHandler, error) {
	if service == nil || currentUser == nil {
		return nil, ErrGovernanceInvalid
	}
	return &HTTPHandler{service: service, currentUser: currentUser}, nil
}

func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodGet && r.URL.Path == "/.well-known/jwks.json" {
		w.Header().Set("Cache-Control", "public, max-age=300")
		writeJSON(w, http.StatusOK, h.service.issuer.KeyMetadata())
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/capabilities" {
		h.issueCapability(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/capabilities/revoke" {
		h.revokeCapability(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/invitations" {
		h.createInvitation(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/invitations/accept" {
		h.acceptInvitation(w, r)
		return
	}
	segments := pathSegments(r.URL.Path)
	if r.Method == http.MethodPost && len(segments) == 4 && segments[0] == "v1" && segments[1] == "invitations" && segments[3] == "revoke" {
		h.revokeInvitation(w, r, segments[2])
		return
	}
	if r.Method == http.MethodPost && len(segments) == 4 && segments[0] == "v1" && segments[1] == "workspaces" && segments[3] == "proposals" {
		h.createProposal(w, r, segments[2])
		return
	}
	if len(segments) >= 3 && segments[0] == "v1" && segments[1] == "proposals" {
		switch {
		case r.Method == http.MethodGet && len(segments) == 3:
			h.getProposal(w, r, segments[2])
		case r.Method == http.MethodPost && len(segments) == 4 && segments[3] == "deliberation":
			h.addDeliberation(w, r, segments[2])
		case r.Method == http.MethodPost && len(segments) == 4 && segments[3] == "votes":
			h.recordVote(w, r, segments[2])
		case r.Method == http.MethodPost && len(segments) == 4 && segments[3] == "decision":
			h.recordDecision(w, r, segments[2])
		case r.Method == http.MethodPost && len(segments) == 4 && segments[3] == "cancel":
			h.cancelProposal(w, r, segments[2])
		case r.Method == http.MethodGet && len(segments) == 4 && segments[3] == "audit":
			h.proposalAudit(w, r, segments[2])
		default:
			writeProblem(w, http.StatusNotFound, ErrGovernanceNotFound)
		}
		return
	}
	writeProblem(w, http.StatusNotFound, ErrGovernanceNotFound)
}

func (h *HTTPHandler) issueCapability(w http.ResponseWriter, r *http.Request) {
	actor, err := h.currentUser(r)
	if err != nil || actor == nil {
		writeProblem(w, http.StatusUnauthorized, ErrCapabilityInvalid)
		return
	}
	var body struct {
		WorkspaceID int     `json:"workspace_id"`
		Role        Role    `json:"role"`
		Scopes      []Scope `json:"scopes"`
		Audience    string  `json:"audience"`
		TTLSeconds  int64   `json:"ttl_seconds"`
	}
	if decodeRequest(w, r, &body) != nil {
		return
	}
	raw, claims, err := h.service.IssueCapability(r.Context(), actor, IssueCapabilityRequest{WorkspaceID: body.WorkspaceID, Role: body.Role, Scopes: body.Scopes, Audience: body.Audience, TTL: time.Duration(body.TTLSeconds) * time.Second})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"token": raw, "claims": claims})
}

func (h *HTTPHandler) revokeCapability(w http.ResponseWriter, r *http.Request) {
	actor, err := h.currentUser(r)
	if err != nil || actor == nil {
		writeProblem(w, http.StatusUnauthorized, ErrCapabilityInvalid)
		return
	}
	var body struct {
		WorkspaceID int    `json:"workspace_id"`
		TokenID     string `json:"jti"`
		Reason      string `json:"reason"`
	}
	if decodeRequest(w, r, &body) != nil {
		return
	}
	if err := h.service.RevokeCapability(r.Context(), actor, body.WorkspaceID, body.TokenID, body.Reason); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTPHandler) createInvitation(w http.ResponseWriter, r *http.Request) {
	actor, err := h.currentUser(r)
	if err != nil || actor == nil {
		writeProblem(w, http.StatusUnauthorized, ErrCapabilityInvalid)
		return
	}
	var body struct {
		WorkspaceID int    `json:"workspace_id"`
		Invitee     string `json:"invitee"`
		Role        Role   `json:"role"`
		TTLSeconds  int64  `json:"ttl_seconds"`
	}
	if decodeRequest(w, r, &body) != nil {
		return
	}
	grant, err := h.service.CreateInvitation(r.Context(), actor, body.WorkspaceID, body.Invitee, body.Role, time.Duration(body.TTLSeconds)*time.Second)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, grant)
}

func (h *HTTPHandler) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	actor, err := h.currentUser(r)
	if err != nil || actor == nil {
		writeProblem(w, http.StatusUnauthorized, ErrCapabilityInvalid)
		return
	}
	var body struct {
		Token string `json:"token"`
	}
	if decodeRequest(w, r, &body) != nil {
		return
	}
	invitation, err := h.service.AcceptInvitation(r.Context(), actor, body.Token)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, invitation)
}

func (h *HTTPHandler) revokeInvitation(w http.ResponseWriter, r *http.Request, invitationID string) {
	actor, err := h.currentUser(r)
	if err != nil || actor == nil {
		writeProblem(w, http.StatusUnauthorized, ErrCapabilityInvalid)
		return
	}
	var body struct {
		ExpectedVersion int64 `json:"expected_version"`
	}
	if decodeRequest(w, r, &body) != nil {
		return
	}
	invitation, err := h.service.RevokeInvitation(r.Context(), actor, invitationID, body.ExpectedVersion)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, invitation)
}

func (h *HTTPHandler) createProposal(w http.ResponseWriter, r *http.Request, workspaceValue string) {
	workspaceID, err := strconv.Atoi(workspaceValue)
	if err != nil || workspaceID <= 0 {
		writeProblem(w, http.StatusBadRequest, ErrGovernanceInvalid)
		return
	}
	var body struct {
		Title  string         `json:"title"`
		Body   string         `json:"body"`
		Policy ProposalPolicy `json:"policy"`
	}
	if decodeRequest(w, r, &body) != nil {
		return
	}
	proposal, err := h.service.CreateProposal(r.Context(), bearerToken(r), CreateProposalRequest{WorkspaceID: workspaceID, Title: body.Title, Body: body.Body, Policy: body.Policy})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, proposal)
}

func (h *HTTPHandler) getProposal(w http.ResponseWriter, r *http.Request, proposalID string) {
	record, err := h.service.GetProposal(r.Context(), bearerToken(r), proposalID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (h *HTTPHandler) addDeliberation(w http.ResponseWriter, r *http.Request, proposalID string) {
	var body struct {
		ExpectedVersion int64  `json:"expected_version"`
		Body            string `json:"body"`
	}
	if decodeRequest(w, r, &body) != nil {
		return
	}
	entry, proposal, err := h.service.AddDeliberation(r.Context(), bearerToken(r), proposalID, body.ExpectedVersion, body.Body)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"entry": entry, "proposal": proposal})
}

func (h *HTTPHandler) recordVote(w http.ResponseWriter, r *http.Request, proposalID string) {
	var body struct {
		ExpectedVersion int64      `json:"expected_version"`
		Choice          VoteChoice `json:"choice"`
		Rationale       string     `json:"rationale"`
	}
	if decodeRequest(w, r, &body) != nil {
		return
	}
	vote, proposal, err := h.service.RecordVote(r.Context(), bearerToken(r), proposalID, body.ExpectedVersion, body.Choice, body.Rationale)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"vote": vote, "proposal": proposal})
}

func (h *HTTPHandler) recordDecision(w http.ResponseWriter, r *http.Request, proposalID string) {
	var body struct {
		ExpectedVersion int64           `json:"expected_version"`
		Outcome         DecisionOutcome `json:"outcome"`
		Rationale       string          `json:"rationale"`
	}
	if decodeRequest(w, r, &body) != nil {
		return
	}
	decision, proposal, err := h.service.RecordDecision(r.Context(), bearerToken(r), proposalID, body.ExpectedVersion, body.Outcome, body.Rationale)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"decision": decision, "proposal": proposal})
}

func (h *HTTPHandler) cancelProposal(w http.ResponseWriter, r *http.Request, proposalID string) {
	var body struct {
		ExpectedVersion int64  `json:"expected_version"`
		Reason          string `json:"reason"`
	}
	if decodeRequest(w, r, &body) != nil {
		return
	}
	proposal, err := h.service.CancelProposal(r.Context(), bearerToken(r), proposalID, body.ExpectedVersion, body.Reason)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, proposal)
}

func (h *HTTPHandler) proposalAudit(w http.ResponseWriter, r *http.Request, proposalID string) {
	events, err := h.service.ProposalAudit(r.Context(), bearerToken(r), proposalID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

func decodeRequest(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeProblem(w, http.StatusBadRequest, ErrGovernanceInvalid)
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeProblem(w, http.StatusBadRequest, ErrGovernanceInvalid)
		return ErrGovernanceInvalid
	}
	return nil
}

func bearerToken(r *http.Request) string {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

func pathSegments(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func writeServiceError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, ErrCapabilityInvalid), errors.Is(err, ErrCapabilityExpired), errors.Is(err, ErrCapabilityNotActive):
		status = http.StatusUnauthorized
	case errors.Is(err, ErrCapabilityForbidden), errors.Is(err, ErrWorkspaceDenied):
		status = http.StatusForbidden
	case errors.Is(err, ErrGovernanceInvalid):
		status = http.StatusBadRequest
	case errors.Is(err, ErrGovernanceNotFound):
		status = http.StatusNotFound
	case errors.Is(err, ErrGovernanceConflict), errors.Is(err, ErrGovernanceTransition):
		status = http.StatusConflict
	}
	if status == http.StatusInternalServerError {
		err = errors.New("internal server error")
	}
	writeProblem(w, status, err)
}

func writeProblem(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": http.StatusText(status), "message": fmt.Sprint(err)})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
