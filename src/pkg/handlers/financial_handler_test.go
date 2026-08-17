package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"taawun/pkg/financial"
	"taawun/pkg/models"
	"taawun/pkg/shura"
)

func TestFinancialCreateInjectsAuthenticatedActorAfterWorkspaceAndShuraChecks(t *testing.T) {
	orchestrator := &financialOrchestratorStub{}
	handler, err := NewFinancialHTTPHandler(orchestrator, financialWorkspaceStub{}, financialDecisionStub{approved: true}, CurrentUser)
	if err != nil {
		t.Fatalf("NewFinancialHTTPHandler() error = %v", err)
	}
	body := `{"workspaceId":7,"idempotencyKey":"quest-1","flowId":"donation","currency":"USD","amountMinor":500,"feeMinor":0,"parties":["taawun:user:9"],"shuraDecisionRef":"decision-1"}`
	request := httptest.NewRequest(http.MethodPost, "/api/financial/quests", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(WithCurrentUser(request.Context(), &models.User{ID: 9}))
	response := httptest.NewRecorder()

	handler.CreateQuest(response, request)

	if response.Code != http.StatusCreated || orchestrator.created.ActorID != "taawun:user:9" || orchestrator.created.WorkspaceID != 7 {
		t.Fatalf("financial create = status=%d request=%+v body=%s", response.Code, orchestrator.created, response.Body.String())
	}
}

func TestFinancialCreateRejectsClientActorAndMissingApprovedDecision(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		decision   bool
		wantStatus int
	}{
		{name: "client actor", body: `{"workspaceId":7,"actorId":"taawun:user:999"}`, decision: true, wantStatus: http.StatusBadRequest},
		{name: "missing decision", body: `{"workspaceId":7,"idempotencyKey":"quest-1","flowId":"donation","currency":"USD","amountMinor":500,"parties":["taawun:user:9"]}`, decision: true, wantStatus: http.StatusUnprocessableEntity},
		{name: "unapproved decision", body: `{"workspaceId":7,"idempotencyKey":"quest-1","flowId":"donation","currency":"USD","amountMinor":500,"parties":["taawun:user:9"],"shuraDecisionRef":"decision-1"}`, decision: false, wantStatus: http.StatusConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			orchestrator := &financialOrchestratorStub{}
			handler, err := NewFinancialHTTPHandler(orchestrator, financialWorkspaceStub{}, financialDecisionStub{approved: test.decision}, CurrentUser)
			if err != nil {
				t.Fatalf("NewFinancialHTTPHandler() error = %v", err)
			}
			request := httptest.NewRequest(http.MethodPost, "/api/financial/quests", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			request = request.WithContext(WithCurrentUser(request.Context(), &models.User{ID: 9}))
			response := httptest.NewRecorder()

			handler.CreateQuest(response, request)

			if response.Code != test.wantStatus || orchestrator.createCalls != 0 {
				t.Fatalf("financial create = status=%d calls=%d body=%s", response.Code, orchestrator.createCalls, response.Body.String())
			}
		})
	}
}

type financialOrchestratorStub struct {
	created     financial.CreateQuestRequest
	createCalls int
}

func (s *financialOrchestratorStub) CreateQuest(_ context.Context, request financial.CreateQuestRequest) (*financial.CreateQuestResult, error) {
	s.created, s.createCalls = request, s.createCalls+1
	return &financial.CreateQuestResult{Quest: &financial.Quest{ID: "quest-1", WorkspaceID: request.WorkspaceID}}, nil
}

func (*financialOrchestratorStub) GetQuest(context.Context, string) (*financial.Quest, error) {
	return &financial.Quest{ID: "quest-1", WorkspaceID: 7}, nil
}

func (*financialOrchestratorStub) Events(context.Context, string) ([]financial.QuestEvent, error) {
	return nil, nil
}

func (*financialOrchestratorStub) Approve(context.Context, string, string, int64) (*financial.Quest, error) {
	return &financial.Quest{}, nil
}

func (*financialOrchestratorStub) Execute(context.Context, string, string, int64) (*financial.Quest, error) {
	return &financial.Quest{}, nil
}

func (*financialOrchestratorStub) Reconcile(context.Context, string, string, int64) (*financial.Quest, error) {
	return &financial.Quest{}, nil
}

func (*financialOrchestratorStub) Cancel(context.Context, string, string, int64) (*financial.Quest, error) {
	return &financial.Quest{}, nil
}

type financialWorkspaceStub struct{}

func (financialWorkspaceStub) AuthorizeWorkspaceCapability(_ *models.User, workspaceID int, _ models.WorkspaceCapability) (*models.Workspace, error) {
	if workspaceID != 7 {
		return nil, context.Canceled
	}
	return &models.Workspace{ID: 7}, nil
}

type financialDecisionStub struct{ approved bool }

func (s financialDecisionStub) ResolveDecision(_ context.Context, workspaceID int, reference string) (shura.DecisionResolution, error) {
	return shura.DecisionResolution{Reference: reference, WorkspaceID: workspaceID, Approved: s.approved}, nil
}
