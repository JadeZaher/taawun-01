package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"taawun/pkg/financial"
	"taawun/pkg/models"
	"taawun/pkg/shura"
)

const maximumFinancialBodyBytes = 64 << 10

type financialOrchestrator interface {
	CreateQuest(context.Context, financial.CreateQuestRequest) (*financial.CreateQuestResult, error)
	GetQuest(context.Context, string) (*financial.Quest, error)
	Events(context.Context, string) ([]financial.QuestEvent, error)
	Approve(context.Context, string, string, int64) (*financial.Quest, error)
	Execute(context.Context, string, string, int64) (*financial.Quest, error)
	Reconcile(context.Context, string, string, int64) (*financial.Quest, error)
	Cancel(context.Context, string, string, int64) (*financial.Quest, error)
}

type financialWorkspaceAuthorizer interface {
	AuthorizeWorkspaceCapability(*models.User, int, models.WorkspaceCapability) (*models.Workspace, error)
}

type financialDecisionResolver interface {
	ResolveDecision(context.Context, int, string) (shura.DecisionResolution, error)
}

// FinancialHTTPHandler injects the platform principal into sandbox orchestration calls.
type FinancialHTTPHandler struct {
	orchestrator financialOrchestrator
	workspaces   financialWorkspaceAuthorizer
	decisions    financialDecisionResolver
	currentUser  currentPrincipal
}

// NewFinancialHTTPHandler keeps client JSON from selecting a financial actor.
func NewFinancialHTTPHandler(orchestrator financialOrchestrator, workspaces financialWorkspaceAuthorizer, decisions financialDecisionResolver, currentUser func(context.Context) (*models.User, bool)) (*FinancialHTTPHandler, error) {
	if orchestrator == nil || workspaces == nil || decisions == nil || currentUser == nil {
		return nil, errors.New("financial orchestrator, workspace authorization, Shura decisions, and current-user resolver are required")
	}
	return &FinancialHTTPHandler{orchestrator: orchestrator, workspaces: workspaces, decisions: decisions, currentUser: currentUser}, nil
}

// Flows lists the fixed, vetted orchestration catalog.
func (h *FinancialHTTPHandler) Flows(w http.ResponseWriter, r *http.Request) {
	writeFinancialJSON(w, http.StatusOK, map[string]any{"flows": financial.ListFlowDefinitions()})
}

type createFinancialQuestInput struct {
	WorkspaceID       int64                  `json:"workspaceId"`
	IdempotencyKey    string                 `json:"idempotencyKey"`
	FlowID            financial.FlowID       `json:"flowId"`
	Currency          string                 `json:"currency"`
	AmountMinor       int64                  `json:"amountMinor"`
	FeeMinor          int64                  `json:"feeMinor"`
	Parties           []string               `json:"parties"`
	Allocations       []financial.Allocation `json:"allocations,omitempty"`
	ApprovalsRequired int                    `json:"approvalsRequired,omitempty"`
	Memo              string                 `json:"memo,omitempty"`
	ShuraDecisionRef  string                 `json:"shuraDecisionRef"`
}

// CreateQuest starts one vetted sandbox quest after workspace and final-Shura checks.
func (h *FinancialHTTPHandler) CreateQuest(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	var input createFinancialQuestInput
	if !decodeFinancialJSON(w, r, &input) {
		return
	}
	workspaceID, ok := financialWorkspaceID(w, input.WorkspaceID)
	if !ok || !h.authorizeAndResolveDecision(w, r, actor, workspaceID, models.WorkspaceCapabilityBuild, input.ShuraDecisionRef) {
		return
	}
	result, err := h.orchestrator.CreateQuest(r.Context(), financial.CreateQuestRequest{
		WorkspaceID: input.WorkspaceID, IdempotencyKey: input.IdempotencyKey, FlowID: input.FlowID,
		Currency: input.Currency, AmountMinor: input.AmountMinor, FeeMinor: input.FeeMinor,
		ActorID: financialActorID(actor), Parties: append([]string(nil), input.Parties...),
		Allocations: append([]financial.Allocation(nil), input.Allocations...), ApprovalsRequired: input.ApprovalsRequired, Memo: input.Memo,
	})
	if err != nil {
		writeFinancialServiceError(w, err)
		return
	}
	writeFinancialJSON(w, http.StatusCreated, result)
}

// GetQuest returns a workspace-authorized quest without exposing actor selection.
func (h *FinancialHTTPHandler) GetQuest(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	quest, ok := h.authorizeQuest(w, r, actor, models.WorkspaceCapabilityView)
	if !ok {
		return
	}
	writeFinancialJSON(w, http.StatusOK, quest)
}

// Events returns the verified, append-only lifecycle for a workspace-authorized quest.
func (h *FinancialHTTPHandler) Events(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	quest, ok := h.authorizeQuest(w, r, actor, models.WorkspaceCapabilityView)
	if !ok {
		return
	}
	events, err := h.orchestrator.Events(r.Context(), quest.ID)
	if err != nil {
		writeFinancialServiceError(w, err)
		return
	}
	writeFinancialJSON(w, http.StatusOK, map[string]any{"events": events})
}

type financialMutationInput struct {
	ExpectedVersion  int64  `json:"expectedVersion"`
	ShuraDecisionRef string `json:"shuraDecisionRef"`
}

// Approve records the authenticated principal as a declared quest party after final-Shura validation.
func (h *FinancialHTTPHandler) Approve(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, func(ctx context.Context, questID, actorID string, expectedVersion int64) (*financial.Quest, error) {
		return h.orchestrator.Approve(ctx, questID, actorID, expectedVersion)
	})
}

// Execute sends a previously approved sandbox quest to its configured provider boundary.
func (h *FinancialHTTPHandler) Execute(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, func(ctx context.Context, questID, actorID string, expectedVersion int64) (*financial.Quest, error) {
		return h.orchestrator.Execute(ctx, questID, actorID, expectedVersion)
	})
}

// Reconcile reads the provider boundary and records only its durable result.
func (h *FinancialHTTPHandler) Reconcile(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, func(ctx context.Context, questID, actorID string, expectedVersion int64) (*financial.Quest, error) {
		return h.orchestrator.Reconcile(ctx, questID, actorID, expectedVersion)
	})
}

// Cancel terminates only a pre-execution sandbox quest after final-Shura validation.
func (h *FinancialHTTPHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, func(ctx context.Context, questID, actorID string, expectedVersion int64) (*financial.Quest, error) {
		return h.orchestrator.Cancel(ctx, questID, actorID, expectedVersion)
	})
}

func (h *FinancialHTTPHandler) mutate(w http.ResponseWriter, r *http.Request, operation func(context.Context, string, string, int64) (*financial.Quest, error)) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	var input financialMutationInput
	if !decodeFinancialJSON(w, r, &input) {
		return
	}
	quest, ok := h.authorizeQuest(w, r, actor, models.WorkspaceCapabilityPublish)
	if !ok || !h.authorizeAndResolveDecision(w, r, actor, int(quest.WorkspaceID), models.WorkspaceCapabilityPublish, input.ShuraDecisionRef) {
		return
	}
	updated, err := operation(r.Context(), quest.ID, financialActorID(actor), input.ExpectedVersion)
	if err != nil && !errors.Is(err, financial.ErrReconciliationPending) {
		writeFinancialServiceError(w, err)
		return
	}
	status := http.StatusOK
	if errors.Is(err, financial.ErrReconciliationPending) {
		status = http.StatusAccepted
	}
	writeFinancialJSON(w, status, updated)
}

func (h *FinancialHTTPHandler) authorizeQuest(w http.ResponseWriter, r *http.Request, actor *models.User, capability models.WorkspaceCapability) (*financial.Quest, bool) {
	quest, err := h.orchestrator.GetQuest(r.Context(), mux.Vars(r)["quest_id"])
	if err != nil {
		writeFinancialServiceError(w, err)
		return nil, false
	}
	if quest.WorkspaceID <= 0 || int64(int(quest.WorkspaceID)) != quest.WorkspaceID {
		writeFinancialError(w, http.StatusNotFound, "financial_quest_not_found", "Financial quest was not found.")
		return nil, false
	}
	if _, err := h.workspaces.AuthorizeWorkspaceCapability(actor, int(quest.WorkspaceID), capability); err != nil {
		writeFinancialError(w, http.StatusForbidden, "workspace_forbidden", "This account is not authorized for that workspace operation.")
		return nil, false
	}
	return quest, true
}

func (h *FinancialHTTPHandler) authorizeAndResolveDecision(w http.ResponseWriter, r *http.Request, actor *models.User, workspaceID int, capability models.WorkspaceCapability, reference string) bool {
	if workspaceID <= 0 || reference == "" {
		writeFinancialError(w, http.StatusUnprocessableEntity, "shura_decision_required", "A final approved Shura decision reference is required.")
		return false
	}
	if _, err := h.workspaces.AuthorizeWorkspaceCapability(actor, workspaceID, capability); err != nil {
		writeFinancialError(w, http.StatusForbidden, "workspace_forbidden", "This account is not authorized for that workspace operation.")
		return false
	}
	decision, err := h.decisions.ResolveDecision(r.Context(), workspaceID, reference)
	if err != nil || !decision.Approved || decision.Reference != reference || decision.WorkspaceID != workspaceID {
		writeFinancialError(w, http.StatusConflict, "shura_decision_not_approved", "A final approved Shura decision for this workspace is required.")
		return false
	}
	return true
}

func financialWorkspaceID(w http.ResponseWriter, value int64) (int, bool) {
	workspaceID := int(value)
	if value <= 0 || int64(workspaceID) != value {
		writeFinancialError(w, http.StatusBadRequest, "invalid_workspace", "Workspace ID must be positive.")
		return 0, false
	}
	return workspaceID, true
}

func (h *FinancialHTTPHandler) actor(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
	actor, ok := h.currentUser(r.Context())
	if !ok || actor == nil || actor.ID <= 0 {
		writeFinancialError(w, http.StatusUnauthorized, "authentication_required", "Authentication is required.")
		return nil, false
	}
	return actor, true
}

func financialActorID(actor *models.User) string {
	return "taawun:user:" + strconv.Itoa(actor.ID)
}

func decodeFinancialJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	if mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mediaType != "application/json" {
		writeFinancialError(w, http.StatusUnsupportedMediaType, "content_type_required", "Content-Type must be application/json.")
		return false
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maximumFinancialBodyBytes))
	if err != nil {
		writeFinancialError(w, http.StatusRequestEntityTooLarge, "request_too_large", "Request body is too large.")
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeFinancialError(w, http.StatusBadRequest, "invalid_request", "Request body must be one valid JSON object.")
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeFinancialError(w, http.StatusBadRequest, "invalid_request", "Request body must contain one JSON object.")
		return false
	}
	return true
}

func writeFinancialServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, financial.ErrQuestNotFound):
		writeFinancialError(w, http.StatusNotFound, "financial_quest_not_found", "Financial quest was not found.")
	case errors.Is(err, financial.ErrInvalidQuest):
		writeFinancialError(w, http.StatusUnprocessableEntity, "invalid_financial_request", "Financial request is not a vetted sandbox quest.")
	case errors.Is(err, financial.ErrVersionConflict), errors.Is(err, financial.ErrInvalidTransition), errors.Is(err, financial.ErrIdempotencyConflict), errors.Is(err, financial.ErrApprovalRequired):
		writeFinancialError(w, http.StatusConflict, "financial_conflict", err.Error())
	case errors.Is(err, financial.ErrProviderUnavailable):
		writeFinancialError(w, http.StatusBadGateway, "financial_provider_unavailable", "The sandbox provider is unavailable.")
	default:
		writeFinancialError(w, http.StatusInternalServerError, "financial_operation_failed", "Financial operation could not be completed.")
	}
}

func writeFinancialError(w http.ResponseWriter, status int, code, message string) {
	writeFinancialJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func writeFinancialJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
