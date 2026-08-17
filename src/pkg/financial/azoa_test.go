package financial

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteSandboxQuestLifecycleIsDurableAndAudited(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "financial", "azoa.db")
	orchestrator, err := NewSQLiteAzoaSandbox(databasePath)
	if err != nil {
		t.Fatalf("open sandbox: %v", err)
	}

	created, err := orchestrator.CreateQuest(ctx, CreateQuestRequest{
		WorkspaceID: 7, IdempotencyKey: "checkout-1042", FlowID: FlowMarketplaceEscrow,
		Currency: "USD", AmountMinor: 2500, FeeMinor: 100, ActorID: "buyer-1",
		Parties: []string{"creator-1", "buyer-1"}, Memo: "Order 1042",
	})
	if err != nil {
		t.Fatalf("create quest: %v", err)
	}
	quest := created.Quest
	if !created.Created || quest.Status != QuestPending || quest.Version != 1 || quest.Intent.AmountMinor != 2500 {
		t.Fatalf("unexpected created quest: %+v", created)
	}
	quest, err = orchestrator.Approve(ctx, quest.ID, "buyer-1", quest.Version)
	if err != nil || quest.Status != QuestPending {
		t.Fatalf("first approval: quest=%+v err=%v", quest, err)
	}
	quest, err = orchestrator.Approve(ctx, quest.ID, "creator-1", quest.Version)
	if err != nil || quest.Status != QuestApproved {
		t.Fatalf("second approval: quest=%+v err=%v", quest, err)
	}
	if _, err := orchestrator.Execute(ctx, quest.ID, "buyer-1", quest.Version-1); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale execution error = %v, want ErrVersionConflict", err)
	}
	quest, err = orchestrator.Execute(ctx, quest.ID, "buyer-1", quest.Version)
	if err != nil || quest.Status != QuestExecuting || quest.ProviderReference == "" || quest.ReconciliationReference != "" {
		t.Fatalf("execute quest: quest=%+v err=%v", quest, err)
	}
	quest, err = orchestrator.Reconcile(ctx, quest.ID, "reconciler-1", quest.Version)
	if !errors.Is(err, ErrReconciliationPending) || quest.Status != QuestExecuting {
		t.Fatalf("pending reconciliation: quest=%+v err=%v", quest, err)
	}
	quest, err = orchestrator.ApplySandboxReconciliation(ctx, quest.ID, "reconciler-1", quest.Version, ProviderReconciliation{
		Status: ProviderReconciliationSettled, ReconciliationReference: "sandbox-recon-1042",
	})
	if err != nil || quest.Status != QuestSettled || quest.ReconciliationReference != "sandbox-recon-1042" {
		t.Fatalf("settle sandbox quest: quest=%+v err=%v", quest, err)
	}

	events, err := orchestrator.Events(ctx, quest.ID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 7 {
		t.Fatalf("event count = %d, want 7", len(events))
	}
	for index, event := range events {
		if event.Hash == "" {
			t.Fatalf("event %d has no hash", index)
		}
		if index == 0 && event.PreviousHash != "" {
			t.Fatalf("first event previous hash = %q", event.PreviousHash)
		}
		if index > 0 && event.PreviousHash != events[index-1].Hash {
			t.Fatalf("event %d does not chain to its predecessor", index)
		}
	}
	if _, err := orchestrator.db.ExecContext(ctx, `UPDATE quest_events SET event_type = 'TAMPERED' WHERE quest_id = ?`, quest.ID); err == nil {
		t.Fatal("append-only event update unexpectedly succeeded")
	}
	if err := orchestrator.Close(); err != nil {
		t.Fatalf("close sandbox: %v", err)
	}

	reopened, err := NewSQLiteAzoaSandbox(databasePath)
	if err != nil {
		t.Fatalf("reopen sandbox: %v", err)
	}
	defer reopened.Close()
	persisted, err := reopened.GetQuest(ctx, quest.ID)
	if err != nil || persisted.Status != QuestSettled || persisted.Version != quest.Version {
		t.Fatalf("durable quest: quest=%+v err=%v", persisted, err)
	}
}

func TestQuestCreationIsIdempotentAndVersioned(t *testing.T) {
	ctx := context.Background()
	orchestrator := newTestSandbox(t)
	request := CreateQuestRequest{
		WorkspaceID: 3, IdempotencyKey: "donation-1", FlowID: FlowDonation, Currency: "USD",
		AmountMinor: 5000, ActorID: "donor-1", Parties: []string{"charity-1"},
	}
	first, err := orchestrator.CreateQuest(ctx, request)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	second, err := orchestrator.CreateQuest(ctx, request)
	if err != nil || second.Created || second.Quest.ID != first.Quest.ID {
		t.Fatalf("idempotent create: result=%+v err=%v", second, err)
	}
	request.AmountMinor++
	if _, err := orchestrator.CreateQuest(ctx, request); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("conflicting create error = %v, want ErrIdempotencyConflict", err)
	}
	if _, err := orchestrator.Approve(ctx, first.Quest.ID, "charity-1", first.Quest.Version+1); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale approval error = %v, want ErrVersionConflict", err)
	}
	if _, err := orchestrator.Approve(ctx, first.Quest.ID, "outsider-1", first.Quest.Version); !errors.Is(err, ErrInvalidQuest) {
		t.Fatalf("outsider approval error = %v, want ErrInvalidQuest", err)
	}
}

func TestVettedFlowCatalogAndValidation(t *testing.T) {
	orchestrator := newTestSandbox(t)
	ctx := context.Background()
	requests := []CreateQuestRequest{
		{WorkspaceID: 1, IdempotencyKey: "flow-donation", FlowID: FlowDonation, Currency: "USD", AmountMinor: 100, ActorID: "actor-1", Parties: []string{"party-1"}},
		{WorkspaceID: 1, IdempotencyKey: "flow-escrow", FlowID: FlowMarketplaceEscrow, Currency: "USD", AmountMinor: 100, FeeMinor: 5, ActorID: "actor-1", Parties: []string{"party-1", "party-2"}},
		{WorkspaceID: 1, IdempotencyKey: "flow-split", FlowID: FlowRevenueSplit, Currency: "USD", AmountMinor: 100, FeeMinor: 5, ActorID: "actor-1", Parties: []string{"party-1", "party-2"}, Allocations: []Allocation{{PartyID: "party-1", BasisPoints: 6000}, {PartyID: "party-2", BasisPoints: 4000}}},
		{WorkspaceID: 1, IdempotencyKey: "flow-zakat", FlowID: FlowZakat, Currency: "USD", AmountMinor: 100, ActorID: "actor-1", Parties: []string{"party-1"}},
		{WorkspaceID: 1, IdempotencyKey: "flow-qard", FlowID: FlowQardHasan, Currency: "USD", AmountMinor: 100, ActorID: "actor-1", Parties: []string{"party-1", "party-2"}},
		{WorkspaceID: 1, IdempotencyKey: "flow-stipend", FlowID: FlowVolunteerStipend, Currency: "USD", AmountMinor: 100, FeeMinor: 5, ActorID: "actor-1", Parties: []string{"party-1", "party-2"}},
		{WorkspaceID: 1, IdempotencyKey: "flow-multiparty", FlowID: FlowMultiPartyApproval, Currency: "USD", AmountMinor: 100, FeeMinor: 5, ActorID: "actor-1", Parties: []string{"party-1", "party-2", "party-3"}, ApprovalsRequired: 2},
	}
	for _, request := range requests {
		if _, err := orchestrator.CreateQuest(ctx, request); err != nil {
			t.Errorf("create vetted flow %s: %v", request.FlowID, err)
		}
	}
	invalid := requests[0]
	invalid.IdempotencyKey = "bad-zakat-fee"
	invalid.FlowID = FlowZakat
	invalid.FeeMinor = 1
	if _, err := orchestrator.CreateQuest(ctx, invalid); !errors.Is(err, ErrInvalidQuest) {
		t.Errorf("zakat fee error = %v", err)
	}
	invalid = requests[2]
	invalid.IdempotencyKey = "bad-split"
	invalid.Allocations[0].BasisPoints = 5000
	if _, err := orchestrator.CreateQuest(ctx, invalid); !errors.Is(err, ErrInvalidQuest) {
		t.Errorf("invalid split error = %v", err)
	}
	invalid = requests[0]
	invalid.IdempotencyKey = "bad-currency"
	invalid.Currency = "usd"
	if _, err := orchestrator.CreateQuest(ctx, invalid); !errors.Is(err, ErrInvalidQuest) {
		t.Errorf("lowercase currency error = %v", err)
	}
	invalid = requests[6]
	invalid.IdempotencyKey = "bad-threshold"
	invalid.ApprovalsRequired = 4
	if _, err := orchestrator.CreateQuest(ctx, invalid); !errors.Is(err, ErrInvalidQuest) {
		t.Errorf("multi-party threshold error = %v", err)
	}
}

func TestProviderFailureAndMissingReconciliationFailClosed(t *testing.T) {
	ctx := context.Background()
	failing := &testProvider{startErr: errors.New("node offline")}
	orchestrator := newTestOrchestrator(t, failing)
	quest := createApprovedDonation(t, orchestrator)
	unconfirmed, err := orchestrator.Execute(ctx, quest.ID, "operator-1", quest.Version)
	if !errors.Is(err, ErrProviderUnavailable) || unconfirmed.Status != QuestExecuting || unconfirmed.ReconciliationReference != "" {
		t.Fatalf("provider failure: quest=%+v err=%v", unconfirmed, err)
	}
	failing.startErr = nil
	failing.execution.ProviderReference = "provider-retry-1"
	retried, err := orchestrator.Execute(ctx, quest.ID, "operator-1", unconfirmed.Version)
	if err != nil || retried.Status != QuestExecuting || retried.ProviderReference != "provider-retry-1" {
		t.Fatalf("retry unconfirmed provider start: quest=%+v err=%v", retried, err)
	}

	provider := &testProvider{execution: ProviderExecution{ProviderReference: "provider-intent-1"}, reconciliation: ProviderReconciliation{Status: ProviderReconciliationSettled}}
	orchestrator2 := newTestOrchestrator(t, provider)
	quest = createApprovedDonation(t, orchestrator2)
	quest, err = orchestrator2.Execute(ctx, quest.ID, "operator-1", quest.Version)
	if err != nil {
		t.Fatalf("execute with provider: %v", err)
	}
	if _, err := orchestrator2.Reconcile(ctx, quest.ID, "reconciler-1", quest.Version); !errors.Is(err, ErrInvalidQuest) {
		t.Fatalf("missing reference error = %v, want ErrInvalidQuest", err)
	}
	unchanged, err := orchestrator2.GetQuest(ctx, quest.ID)
	if err != nil || unchanged.Status != QuestExecuting || unchanged.Version != quest.Version {
		t.Fatalf("missing reference changed quest: quest=%+v err=%v", unchanged, err)
	}
	provider.reconciliation.ReconciliationReference = "provider-recon-1"
	settled, err := orchestrator2.Reconcile(ctx, quest.ID, "reconciler-1", quest.Version)
	if err != nil || settled.Status != QuestSettled || settled.ReconciliationReference != "provider-recon-1" {
		t.Fatalf("provider settlement: quest=%+v err=%v", settled, err)
	}

	provider3 := &testProvider{
		execution: ProviderExecution{ProviderReference: "provider-intent-3"},
		reconciliation: ProviderReconciliation{
			Status: ProviderReconciliationFailed, FailureCode: "recipient_rejected",
		},
	}
	orchestrator3 := newTestOrchestrator(t, provider3)
	quest = createApprovedDonation(t, orchestrator3)
	quest, err = orchestrator3.Execute(ctx, quest.ID, "operator-1", quest.Version)
	if err != nil {
		t.Fatalf("execute failed-outcome quest: %v", err)
	}
	failed, err := orchestrator3.Reconcile(ctx, quest.ID, "reconciler-1", quest.Version)
	if err != nil || failed.Status != QuestFailed || failed.FailureCode != "recipient_rejected" {
		t.Fatalf("definitive provider failure: quest=%+v err=%v", failed, err)
	}
}

func TestFederationIntentUsesExistingEnvelopeWithoutSettling(t *testing.T) {
	ctx := context.Background()
	orchestrator := newTestSandbox(t)
	quest := createApprovedDonation(t, orchestrator)
	signer, publicKey, err := GenerateFederationSigner("node-alpha")
	if err != nil {
		t.Fatalf("generate signer: %v", err)
	}
	envelope, err := orchestrator.FederationIntent(ctx, quest.ID, "node-beta", signer, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatalf("create federation intent: %v", err)
	}
	if err := VerifyFederationEnvelope(envelope, publicKey); err != nil {
		t.Fatalf("verify federation intent: %v", err)
	}
	if envelope.Type != FederationSettlementIntent || envelope.SettlementID != quest.Intent.ID || envelope.QuestID != quest.ID {
		t.Fatalf("unexpected envelope routing: %+v", envelope)
	}
	var projection FederationQuestProjection
	if err := json.Unmarshal(envelope.Payload, &projection); err != nil {
		t.Fatalf("decode projection: %v", err)
	}
	if projection.AmountMinor != quest.Intent.AmountMinor || projection.Status != QuestApproved || projection.OrchestrationVersion != AzoaOrchestrationVersion {
		t.Fatalf("unexpected federation projection: %+v", projection)
	}
	unchanged, err := orchestrator.GetQuest(ctx, quest.ID)
	if err != nil || unchanged.Status != QuestApproved || unchanged.Version != quest.Version {
		t.Fatalf("federation intent changed settlement state: quest=%+v err=%v", unchanged, err)
	}
}

func TestLegacyAzoaClientFailsClosed(t *testing.T) {
	client := NewAzoaClient("https://invented.invalid", "secret")
	if _, err := client.CreateMarketplaceEscrowQuest("buyer", "creator", 100, 5); !errors.Is(err, ErrLegacyAzoaDisabled) {
		t.Fatalf("legacy marketplace error = %v", err)
	}
	if _, err := client.CreateZakatDriveQuest("donor", 100, "charity"); !errors.Is(err, ErrLegacyAzoaDisabled) {
		t.Fatalf("legacy zakat error = %v", err)
	}
	if _, err := NewSQLiteAzoaSandbox(":memory:"); !errors.Is(err, ErrInvalidQuest) {
		t.Fatalf("in-memory database error = %v", err)
	}
}

type testProvider struct {
	execution      ProviderExecution
	startErr       error
	reconciliation ProviderReconciliation
	reconcileErr   error
}

func (p *testProvider) Start(context.Context, ProviderIntent) (ProviderExecution, error) {
	return p.execution, p.startErr
}

func (p *testProvider) Reconcile(context.Context, string) (ProviderReconciliation, error) {
	return p.reconciliation, p.reconcileErr
}

func newTestSandbox(t *testing.T) *AzoaOrchestrator {
	t.Helper()
	orchestrator, err := NewSQLiteAzoaSandbox(filepath.Join(t.TempDir(), "azoa.db"))
	if err != nil {
		t.Fatalf("open sandbox: %v", err)
	}
	t.Cleanup(func() { _ = orchestrator.Close() })
	return orchestrator
}

func newTestOrchestrator(t *testing.T, provider AzoaProvider) *AzoaOrchestrator {
	t.Helper()
	orchestrator, err := NewSQLiteAzoaOrchestrator(filepath.Join(t.TempDir(), "azoa.db"), provider)
	if err != nil {
		t.Fatalf("open orchestrator: %v", err)
	}
	t.Cleanup(func() { _ = orchestrator.Close() })
	return orchestrator
}

func createApprovedDonation(t *testing.T, orchestrator *AzoaOrchestrator) *Quest {
	t.Helper()
	created, err := orchestrator.CreateQuest(context.Background(), CreateQuestRequest{
		WorkspaceID: 9, IdempotencyKey: "donation-approved", FlowID: FlowDonation,
		Currency: "USD", AmountMinor: 5000, ActorID: "donor-1", Parties: []string{"charity-1", "donor-1"},
	})
	if err != nil {
		t.Fatalf("create donation: %v", err)
	}
	quest, err := orchestrator.Approve(context.Background(), created.Quest.ID, "donor-1", created.Quest.Version)
	if err != nil {
		t.Fatalf("approve donation: %v", err)
	}
	return quest
}
