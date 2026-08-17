package financial

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
	"sort"
	"strings"
	"time"
)

const AzoaOrchestrationVersion = "taawun-azoa-orchestration/v1"

const maxQuestParties = 100

var (
	ErrInvalidQuest          = errors.New("invalid financial quest")
	ErrQuestNotFound         = errors.New("financial quest not found")
	ErrIdempotencyConflict   = errors.New("financial idempotency key conflict")
	ErrVersionConflict       = errors.New("financial quest version conflict")
	ErrInvalidTransition     = errors.New("invalid financial quest transition")
	ErrApprovalRequired      = errors.New("financial quest approval required")
	ErrProviderUnavailable   = errors.New("AZOA provider is unavailable")
	ErrReconciliationPending = errors.New("financial reconciliation is pending")
	ErrSandboxOnly           = errors.New("operation is available only in the sandbox provider")
	ErrLegacyAzoaDisabled    = errors.New("legacy AZOA client is disabled; configure the durable orchestrator")
)

// QuestStatus is the durable orchestration state, not a balance or payment state.
type QuestStatus string

const (
	QuestPending   QuestStatus = "PENDING"
	QuestApproved  QuestStatus = "APPROVED"
	QuestExecuting QuestStatus = "EXECUTING"
	QuestSettled   QuestStatus = "SETTLED"
	QuestFailed    QuestStatus = "FAILED"
	QuestCancelled QuestStatus = "CANCELLED"

	// Deprecated compatibility aliases never imply settlement.
	QuestStaged    QuestStatus = QuestApproved
	QuestEscalated QuestStatus = QuestPending
)

type FlowID string

const (
	FlowDonation           FlowID = "donation"
	FlowMarketplaceEscrow  FlowID = "marketplace-escrow"
	FlowRevenueSplit       FlowID = "revenue-split"
	FlowZakat              FlowID = "zakat"
	FlowQardHasan          FlowID = "qard-hasan"
	FlowVolunteerStipend   FlowID = "volunteer-stipend"
	FlowMultiPartyApproval FlowID = "multi-party-approval"
)

// FlowDefinition is a vetted orchestration policy, never executable code.
type FlowDefinition struct {
	ID               FlowID `json:"id"`
	Version          int    `json:"version"`
	Title            string `json:"title"`
	DefaultApprovals int    `json:"defaultApprovals"`
	MinimumParties   int    `json:"minimumParties"`
	ZeroFee          bool   `json:"zeroFee"`
	RequiresSplit    bool   `json:"requiresSplit"`
}

var flowCatalog = map[FlowID]FlowDefinition{
	FlowDonation:           {ID: FlowDonation, Version: 1, Title: "Donation", DefaultApprovals: 1, MinimumParties: 1, ZeroFee: true},
	FlowMarketplaceEscrow:  {ID: FlowMarketplaceEscrow, Version: 1, Title: "Marketplace escrow", DefaultApprovals: 2, MinimumParties: 2},
	FlowRevenueSplit:       {ID: FlowRevenueSplit, Version: 1, Title: "Revenue split", DefaultApprovals: 2, MinimumParties: 2, RequiresSplit: true},
	FlowZakat:              {ID: FlowZakat, Version: 1, Title: "Zakat", DefaultApprovals: 1, MinimumParties: 1, ZeroFee: true},
	FlowQardHasan:          {ID: FlowQardHasan, Version: 1, Title: "Qard Hasan", DefaultApprovals: 2, MinimumParties: 2, ZeroFee: true},
	FlowVolunteerStipend:   {ID: FlowVolunteerStipend, Version: 1, Title: "Volunteer stipend", DefaultApprovals: 2, MinimumParties: 2},
	FlowMultiPartyApproval: {ID: FlowMultiPartyApproval, Version: 1, Title: "Multi-party approval", DefaultApprovals: 3, MinimumParties: 3},
}

// ListFlowDefinitions returns immutable catalog copies in stable order.
func ListFlowDefinitions() []FlowDefinition {
	flows := make([]FlowDefinition, 0, len(flowCatalog))
	for _, definition := range flowCatalog {
		flows = append(flows, definition)
	}
	sort.Slice(flows, func(i, j int) bool { return flows[i].ID < flows[j].ID })
	return flows
}

type Allocation struct {
	PartyID     string `json:"partyId"`
	BasisPoints int    `json:"basisPoints"`
}

type CreateQuestRequest struct {
	WorkspaceID       int64        `json:"workspaceId"`
	IdempotencyKey    string       `json:"idempotencyKey"`
	FlowID            FlowID       `json:"flowId"`
	Currency          string       `json:"currency"`
	AmountMinor       int64        `json:"amountMinor"`
	FeeMinor          int64        `json:"feeMinor"`
	ActorID           string       `json:"actorId"`
	Parties           []string     `json:"parties"`
	Allocations       []Allocation `json:"allocations,omitempty"`
	ApprovalsRequired int          `json:"approvalsRequired,omitempty"`
	Memo              string       `json:"memo,omitempty"`
}

type Quest struct {
	ID                      string      `json:"id"`
	WorkspaceID             int64       `json:"workspaceId"`
	FlowID                  FlowID      `json:"flowId"`
	FlowVersion             int         `json:"flowVersion"`
	Status                  QuestStatus `json:"status"`
	Version                 int64       `json:"version"`
	ApprovalsRequired       int         `json:"approvalsRequired"`
	ApprovalsReceived       int         `json:"approvalsReceived"`
	ProviderReference       string      `json:"providerReference,omitempty"`
	ReconciliationReference string      `json:"reconciliationReference,omitempty"`
	FailureCode             string      `json:"failureCode,omitempty"`
	Intent                  QuestIntent `json:"intent"`
	CreatedAt               time.Time   `json:"createdAt"`
	UpdatedAt               time.Time   `json:"updatedAt"`
}

type QuestIntent struct {
	ID          string       `json:"id"`
	QuestID     string       `json:"questId"`
	Currency    string       `json:"currency"`
	AmountMinor int64        `json:"amountMinor"`
	FeeMinor    int64        `json:"feeMinor"`
	Parties     []string     `json:"parties"`
	Allocations []Allocation `json:"allocations,omitempty"`
	Memo        string       `json:"memo,omitempty"`
	Status      QuestStatus  `json:"status"`
	Version     int64        `json:"version"`
}

type QuestEvent struct {
	Sequence     int64           `json:"sequence"`
	ID           string          `json:"id"`
	QuestID      string          `json:"questId"`
	QuestVersion int64           `json:"questVersion"`
	Type         string          `json:"type"`
	FromStatus   QuestStatus     `json:"fromStatus"`
	ToStatus     QuestStatus     `json:"toStatus"`
	ActorID      string          `json:"actorId"`
	Detail       json.RawMessage `json:"detail"`
	PreviousHash string          `json:"previousHash,omitempty"`
	Hash         string          `json:"hash"`
	CreatedAt    time.Time       `json:"createdAt"`
}

type CreateQuestResult struct {
	Quest   *Quest `json:"quest"`
	Created bool   `json:"created"`
}

type ProviderIntent struct {
	QuestID        string       `json:"questId"`
	IntentID       string       `json:"intentId"`
	IdempotencyKey string       `json:"idempotencyKey"`
	FlowID         FlowID       `json:"flowId"`
	Currency       string       `json:"currency"`
	AmountMinor    int64        `json:"amountMinor"`
	FeeMinor       int64        `json:"feeMinor"`
	Parties        []string     `json:"parties"`
	Allocations    []Allocation `json:"allocations,omitempty"`
}

type ProviderExecution struct {
	ProviderReference string `json:"providerReference"`
}

type ProviderReconciliationStatus string

const (
	ProviderReconciliationPending ProviderReconciliationStatus = "PENDING"
	ProviderReconciliationSettled ProviderReconciliationStatus = "SETTLED"
	ProviderReconciliationFailed  ProviderReconciliationStatus = "FAILED"
)

type ProviderReconciliation struct {
	Status                  ProviderReconciliationStatus `json:"status"`
	ReconciliationReference string                       `json:"reconciliationReference,omitempty"`
	FailureCode             string                       `json:"failureCode,omitempty"`
}

// AzoaProvider owns external credentials and must start idempotently by intent key.
type AzoaProvider interface {
	Start(context.Context, ProviderIntent) (ProviderExecution, error)
	Reconcile(context.Context, string) (ProviderReconciliation, error)
}

// SandboxProvider starts durable sandbox intents but never reports settlement itself.
type SandboxProvider struct{}

func (SandboxProvider) Start(_ context.Context, _ ProviderIntent) (ProviderExecution, error) {
	reference, err := financialID("sandbox")
	if err != nil {
		return ProviderExecution{}, err
	}
	return ProviderExecution{ProviderReference: reference}, nil
}

func (SandboxProvider) Reconcile(context.Context, string) (ProviderReconciliation, error) {
	return ProviderReconciliation{Status: ProviderReconciliationPending}, nil
}

// AzoaOrchestrator persists policy state and delegates execution without holding funds.
type AzoaOrchestrator struct {
	db       *sql.DB
	provider AzoaProvider
	sandbox  bool
	now      func() time.Time
}

// NewSQLiteAzoaOrchestrator opens a durable SQLite store around an injected provider.
func NewSQLiteAzoaOrchestrator(databasePath string, provider AzoaProvider) (*AzoaOrchestrator, error) {
	return newSQLiteAzoaOrchestrator(databasePath, provider, false)
}

// NewSQLiteAzoaSandbox opens the durable manual-reconciliation sandbox.
func NewSQLiteAzoaSandbox(databasePath string) (*AzoaOrchestrator, error) {
	return newSQLiteAzoaOrchestrator(databasePath, SandboxProvider{}, true)
}

func newSQLiteAzoaOrchestrator(databasePath string, provider AzoaProvider, sandbox bool) (*AzoaOrchestrator, error) {
	if strings.TrimSpace(databasePath) == "" || databasePath == ":memory:" {
		return nil, fmt.Errorf("%w: durable SQLite path is required", ErrInvalidQuest)
	}
	if provider == nil {
		return nil, ErrProviderUnavailable
	}
	db, err := openAzoaDatabase(databasePath)
	if err != nil {
		return nil, err
	}
	orchestrator := &AzoaOrchestrator{db: db, provider: provider, sandbox: sandbox, now: time.Now}
	if err := orchestrator.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return orchestrator, nil
}

func (o *AzoaOrchestrator) Close() error {
	if o == nil || o.db == nil {
		return nil
	}
	return o.db.Close()
}

func (o *AzoaOrchestrator) CreateQuest(ctx context.Context, request CreateQuestRequest) (*CreateQuestResult, error) {
	normalized, definition, requestHash, err := validateCreateQuest(request)
	if err != nil {
		return nil, err
	}
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	tx, err := o.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin quest creation: %w", err)
	}
	defer tx.Rollback()

	existing, existingHash, err := getQuestByIdempotencyTx(ctx, tx, normalized.WorkspaceID, normalized.IdempotencyKey)
	if err == nil {
		if existingHash != requestHash {
			return nil, ErrIdempotencyConflict
		}
		return &CreateQuestResult{Quest: existing, Created: false}, nil
	}
	if !errors.Is(err, ErrQuestNotFound) {
		return nil, err
	}
	questID, err := financialID("quest")
	if err != nil {
		return nil, err
	}
	intentID, err := financialID("intent")
	if err != nil {
		return nil, err
	}
	now := o.now().UTC().Truncate(time.Millisecond)
	partiesJSON, _ := json.Marshal(normalized.Parties)
	allocationsJSON, _ := json.Marshal(normalized.Allocations)
	approvals := definition.DefaultApprovals
	if normalized.ApprovalsRequired > 0 {
		approvals = normalized.ApprovalsRequired
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO quests
        (id, workspace_id, flow_id, flow_version, idempotency_key, request_hash, status, version, approvals_required, approvals_received, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, 0, ?, ?)`,
		questID, normalized.WorkspaceID, normalized.FlowID, definition.Version, normalized.IdempotencyKey, requestHash, QuestPending, approvals, now.UnixMilli(), now.UnixMilli()); err != nil {
		return nil, fmt.Errorf("insert quest: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO intents
        (id, quest_id, currency, amount_minor, fee_minor, parties_json, allocations_json, memo, status, version)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 1)`,
		intentID, questID, normalized.Currency, normalized.AmountMinor, normalized.FeeMinor, partiesJSON, allocationsJSON, normalized.Memo, QuestPending); err != nil {
		return nil, fmt.Errorf("insert quest intent: %w", err)
	}
	if err := appendQuestEvent(ctx, tx, questID, 1, "QUEST_CREATED", "", QuestPending, normalized.ActorID, map[string]any{"flowId": normalized.FlowID, "requestHash": requestHash}, now); err != nil {
		return nil, err
	}
	quest, err := getQuestTx(ctx, tx, questID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit quest creation: %w", err)
	}
	return &CreateQuestResult{Quest: quest, Created: true}, nil
}

func (o *AzoaOrchestrator) GetQuest(ctx context.Context, questID string) (*Quest, error) {
	if !validFinancialID(questID) {
		return nil, ErrQuestNotFound
	}
	return getQuestDB(ctx, o.db, questID)
}

func (o *AzoaOrchestrator) Events(ctx context.Context, questID string) ([]QuestEvent, error) {
	if _, err := o.GetQuest(ctx, questID); err != nil {
		return nil, err
	}
	return listQuestEvents(ctx, o.db, questID)
}

func (o *AzoaOrchestrator) Approve(ctx context.Context, questID, actorID string, expectedVersion int64) (*Quest, error) {
	if !validFinancialID(actorID) {
		return nil, fmt.Errorf("%w: invalid approval actor", ErrInvalidQuest)
	}
	tx, err := o.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	quest, err := getQuestTx(ctx, tx, questID)
	if err != nil {
		return nil, err
	}
	if quest.Status != QuestPending {
		return nil, fmt.Errorf("%w: approvals require PENDING status", ErrInvalidTransition)
	}
	partyIndex := sort.SearchStrings(quest.Intent.Parties, actorID)
	if partyIndex == len(quest.Intent.Parties) || quest.Intent.Parties[partyIndex] != actorID {
		return nil, fmt.Errorf("%w: approval actor is not a quest party", ErrInvalidQuest)
	}
	var present int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM approvals WHERE quest_id = ? AND actor_id = ?`, questID, actorID).Scan(&present); err != nil {
		return nil, err
	}
	if present > 0 {
		return quest, nil
	}
	if quest.Version != expectedVersion {
		return nil, ErrVersionConflict
	}
	now := o.now().UTC().Truncate(time.Millisecond)
	if _, err := tx.ExecContext(ctx, `INSERT INTO approvals (quest_id, actor_id, created_at) VALUES (?, ?, ?)`, questID, actorID, now.UnixMilli()); err != nil {
		return nil, err
	}
	toStatus := QuestPending
	if quest.ApprovalsReceived+1 >= quest.ApprovalsRequired {
		toStatus = QuestApproved
	}
	quest, err = updateQuestState(ctx, tx, quest, toStatus, actorID, "APPROVAL_RECORDED", map[string]any{"approvalsReceived": quest.ApprovalsReceived + 1}, "", "", "", now, true)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return quest, nil
}

func (o *AzoaOrchestrator) Execute(ctx context.Context, questID, actorID string, expectedVersion int64) (*Quest, error) {
	quest, err := o.GetQuest(ctx, questID)
	if err != nil {
		return nil, err
	}
	if quest.Version != expectedVersion {
		return nil, ErrVersionConflict
	}
	switch {
	case quest.Status == QuestApproved:
		quest, err = o.transition(ctx, questID, actorID, expectedVersion, QuestApproved, QuestExecuting, "EXECUTION_REQUESTED", nil, "", "", "")
	case quest.Status == QuestExecuting && quest.ProviderReference == "":
		quest, err = o.transition(ctx, questID, actorID, expectedVersion, QuestExecuting, QuestExecuting, "PROVIDER_START_RETRY_REQUESTED", nil, "", "", "")
	default:
		return nil, fmt.Errorf("%w: execution requires an approved quest or an unconfirmed provider start", ErrInvalidTransition)
	}
	if err != nil {
		return nil, err
	}
	execution, providerErr := o.provider.Start(ctx, providerIntentFromQuest(quest))
	if providerErr != nil || !validFinancialID(execution.ProviderReference) {
		unconfirmed, transitionErr := o.transition(ctx, quest.ID, "system", quest.Version, QuestExecuting, QuestExecuting, "PROVIDER_START_UNCONFIRMED", map[string]any{"code": "provider_start_unconfirmed"}, "", "", "")
		if transitionErr != nil {
			return nil, errors.Join(providerErr, transitionErr)
		}
		if providerErr == nil {
			providerErr = ErrProviderUnavailable
		}
		return unconfirmed, fmt.Errorf("%w: %v", ErrProviderUnavailable, providerErr)
	}
	return o.transition(ctx, quest.ID, "provider", quest.Version, QuestExecuting, QuestExecuting, "PROVIDER_INTENT_ACCEPTED", nil, execution.ProviderReference, "", "")
}

func (o *AzoaOrchestrator) Reconcile(ctx context.Context, questID, actorID string, expectedVersion int64) (*Quest, error) {
	quest, err := o.GetQuest(ctx, questID)
	if err != nil {
		return nil, err
	}
	if quest.Status != QuestExecuting || quest.Version != expectedVersion || quest.ProviderReference == "" {
		if quest.Version != expectedVersion {
			return nil, ErrVersionConflict
		}
		return nil, fmt.Errorf("%w: reconciliation requires an executing provider intent", ErrInvalidTransition)
	}
	reconciliation, providerErr := o.provider.Reconcile(ctx, quest.ProviderReference)
	if providerErr != nil {
		updated, updateErr := o.transition(ctx, questID, actorID, expectedVersion, QuestExecuting, QuestExecuting, "RECONCILIATION_FAILED_CLOSED", map[string]any{"error": "provider unavailable"}, "", "", "")
		if updateErr != nil {
			return nil, errors.Join(providerErr, updateErr)
		}
		return updated, fmt.Errorf("%w: %v", ErrProviderUnavailable, providerErr)
	}
	switch reconciliation.Status {
	case ProviderReconciliationPending:
		updated, err := o.transition(ctx, questID, actorID, expectedVersion, QuestExecuting, QuestExecuting, "RECONCILIATION_PENDING", nil, "", "", "")
		if err != nil {
			return nil, err
		}
		return updated, ErrReconciliationPending
	case ProviderReconciliationSettled:
		if !validReconciliationReference(reconciliation.ReconciliationReference) {
			return nil, fmt.Errorf("%w: settled result requires reconciliation reference", ErrInvalidQuest)
		}
		return o.transition(ctx, questID, actorID, expectedVersion, QuestExecuting, QuestSettled, "RECONCILIATION_SETTLED", nil, "", reconciliation.ReconciliationReference, "")
	case ProviderReconciliationFailed:
		if !validFailureCode(reconciliation.FailureCode) {
			return nil, fmt.Errorf("%w: failed result requires failure code", ErrInvalidQuest)
		}
		if reconciliation.ReconciliationReference != "" && !validReconciliationReference(reconciliation.ReconciliationReference) {
			return nil, fmt.Errorf("%w: malformed reconciliation reference", ErrInvalidQuest)
		}
		return o.transition(ctx, questID, actorID, expectedVersion, QuestExecuting, QuestFailed, "RECONCILIATION_FAILED", nil, "", reconciliation.ReconciliationReference, reconciliation.FailureCode)
	default:
		return nil, fmt.Errorf("%w: unsupported provider reconciliation", ErrInvalidQuest)
	}
}

func (o *AzoaOrchestrator) ApplySandboxReconciliation(ctx context.Context, questID, actorID string, expectedVersion int64, reconciliation ProviderReconciliation) (*Quest, error) {
	if !o.sandbox {
		return nil, ErrSandboxOnly
	}
	if reconciliation.Status == ProviderReconciliationSettled {
		if !validReconciliationReference(reconciliation.ReconciliationReference) {
			return nil, fmt.Errorf("%w: reconciliation reference required", ErrInvalidQuest)
		}
		return o.transition(ctx, questID, actorID, expectedVersion, QuestExecuting, QuestSettled, "SANDBOX_RECONCILIATION_SETTLED", nil, "", reconciliation.ReconciliationReference, "")
	}
	if reconciliation.Status == ProviderReconciliationFailed && validFailureCode(reconciliation.FailureCode) &&
		(reconciliation.ReconciliationReference == "" || validReconciliationReference(reconciliation.ReconciliationReference)) {
		return o.transition(ctx, questID, actorID, expectedVersion, QuestExecuting, QuestFailed, "SANDBOX_RECONCILIATION_FAILED", nil, "", reconciliation.ReconciliationReference, reconciliation.FailureCode)
	}
	return nil, fmt.Errorf("%w: sandbox outcome must be SETTLED or FAILED", ErrInvalidQuest)
}

func (o *AzoaOrchestrator) Cancel(ctx context.Context, questID, actorID string, expectedVersion int64) (*Quest, error) {
	quest, err := o.GetQuest(ctx, questID)
	if err != nil {
		return nil, err
	}
	if quest.Status != QuestPending && quest.Status != QuestApproved {
		return nil, fmt.Errorf("%w: only pending or approved quests can be cancelled", ErrInvalidTransition)
	}
	return o.transition(ctx, questID, actorID, expectedVersion, quest.Status, QuestCancelled, "QUEST_CANCELLED", nil, "", "", "")
}

func (o *AzoaOrchestrator) transition(ctx context.Context, questID, actorID string, expectedVersion int64, from, to QuestStatus, eventType string, detail any, providerReference, reconciliationReference, failureCode string) (*Quest, error) {
	if !validFinancialID(actorID) {
		return nil, fmt.Errorf("%w: invalid actor", ErrInvalidQuest)
	}
	tx, err := o.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	quest, err := getQuestTx(ctx, tx, questID)
	if err != nil {
		return nil, err
	}
	if quest.Version != expectedVersion {
		return nil, ErrVersionConflict
	}
	if quest.Status != from || !allowedQuestTransition(from, to) {
		return nil, fmt.Errorf("%w: %s to %s", ErrInvalidTransition, quest.Status, to)
	}
	quest, err = updateQuestState(ctx, tx, quest, to, actorID, eventType, detail, providerReference, reconciliationReference, failureCode, o.now().UTC().Truncate(time.Millisecond), false)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return quest, nil
}

func allowedQuestTransition(from, to QuestStatus) bool {
	if from == to {
		return from == QuestPending || from == QuestExecuting
	}
	switch from {
	case QuestPending:
		return to == QuestApproved || to == QuestCancelled
	case QuestApproved:
		return to == QuestExecuting || to == QuestCancelled
	case QuestExecuting:
		return to == QuestSettled || to == QuestFailed
	default:
		return false
	}
}

func providerIntentFromQuest(quest *Quest) ProviderIntent {
	return ProviderIntent{
		QuestID: quest.ID, IntentID: quest.Intent.ID, IdempotencyKey: quest.ID,
		FlowID: quest.FlowID, Currency: quest.Intent.Currency, AmountMinor: quest.Intent.AmountMinor,
		FeeMinor: quest.Intent.FeeMinor, Parties: append([]string(nil), quest.Intent.Parties...),
		Allocations: append([]Allocation(nil), quest.Intent.Allocations...),
	}
}

func validateCreateQuest(request CreateQuestRequest) (CreateQuestRequest, FlowDefinition, string, error) {
	definition, supported := flowCatalog[request.FlowID]
	if !supported || request.WorkspaceID <= 0 || !validFinancialID(request.IdempotencyKey) || !validFinancialID(request.ActorID) || !validCurrency(request.Currency) || request.AmountMinor <= 0 || request.FeeMinor < 0 {
		return CreateQuestRequest{}, FlowDefinition{}, "", ErrInvalidQuest
	}
	if definition.ZeroFee && request.FeeMinor != 0 {
		return CreateQuestRequest{}, FlowDefinition{}, "", fmt.Errorf("%w: %s requires zero fee", ErrInvalidQuest, request.FlowID)
	}
	if request.FeeMinor > request.AmountMinor || len(request.Memo) > 500 || strings.ContainsAny(request.Memo, "\x00\r\n") {
		return CreateQuestRequest{}, FlowDefinition{}, "", ErrInvalidQuest
	}
	request.Currency = strings.ToUpper(request.Currency)
	if len(request.Parties) > maxQuestParties || len(request.Allocations) > maxQuestParties {
		return CreateQuestRequest{}, FlowDefinition{}, "", fmt.Errorf("%w: too many quest parties", ErrInvalidQuest)
	}
	request.Parties = append([]string(nil), request.Parties...)
	for _, party := range request.Parties {
		if !validFinancialID(party) {
			return CreateQuestRequest{}, FlowDefinition{}, "", ErrInvalidQuest
		}
	}
	sort.Strings(request.Parties)
	request.Parties = compactStrings(request.Parties)
	if len(request.Parties) < definition.MinimumParties {
		return CreateQuestRequest{}, FlowDefinition{}, "", fmt.Errorf("%w: %s requires at least %d parties", ErrInvalidQuest, request.FlowID, definition.MinimumParties)
	}
	request.Allocations = append([]Allocation(nil), request.Allocations...)
	sort.Slice(request.Allocations, func(i, j int) bool { return request.Allocations[i].PartyID < request.Allocations[j].PartyID })
	if definition.RequiresSplit {
		total := 0
		seen := make(map[string]struct{}, len(request.Allocations))
		for _, allocation := range request.Allocations {
			if !validFinancialID(allocation.PartyID) || allocation.BasisPoints <= 0 || allocation.BasisPoints > 10000 {
				return CreateQuestRequest{}, FlowDefinition{}, "", ErrInvalidQuest
			}
			partyIndex := sort.SearchStrings(request.Parties, allocation.PartyID)
			if partyIndex == len(request.Parties) || request.Parties[partyIndex] != allocation.PartyID {
				return CreateQuestRequest{}, FlowDefinition{}, "", fmt.Errorf("%w: allocation recipient is not a quest party", ErrInvalidQuest)
			}
			if _, duplicate := seen[allocation.PartyID]; duplicate {
				return CreateQuestRequest{}, FlowDefinition{}, "", ErrInvalidQuest
			}
			seen[allocation.PartyID] = struct{}{}
			total += allocation.BasisPoints
		}
		if total != 10000 || len(request.Allocations) != len(request.Parties) {
			return CreateQuestRequest{}, FlowDefinition{}, "", fmt.Errorf("%w: revenue split must total 10000 basis points", ErrInvalidQuest)
		}
	} else if len(request.Allocations) != 0 {
		return CreateQuestRequest{}, FlowDefinition{}, "", fmt.Errorf("%w: allocations are allowed only for revenue split", ErrInvalidQuest)
	}
	if request.FlowID == FlowMultiPartyApproval {
		threshold := request.ApprovalsRequired
		if threshold == 0 {
			threshold = definition.DefaultApprovals
		}
		if threshold < 2 || threshold > len(request.Parties) {
			return CreateQuestRequest{}, FlowDefinition{}, "", fmt.Errorf("%w: multi-party approval threshold is invalid", ErrInvalidQuest)
		}
	} else if request.ApprovalsRequired != 0 && request.ApprovalsRequired != definition.DefaultApprovals {
		return CreateQuestRequest{}, FlowDefinition{}, "", fmt.Errorf("%w: approval threshold is fixed by flow", ErrInvalidQuest)
	}
	canonical, _ := json.Marshal(request)
	digest := sha256.Sum256(canonical)
	return request, definition, hex.EncodeToString(digest[:]), nil
}

// ValidateCreateQuestRequest checks a request against the vetted flow catalog without creating a quest.
func ValidateCreateQuestRequest(request CreateQuestRequest) error {
	_, _, _, err := validateCreateQuest(request)
	return err
}

func compactStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}

func validCurrency(value string) bool {
	if len(value) != 3 {
		return false
	}
	for _, character := range value {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
}

func validFinancialID(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || strings.ContainsRune("._:-", character) {
			continue
		}
		return false
	}
	return true
}

func validReconciliationReference(value string) bool {
	return validFinancialID(value)
}

func validFailureCode(value string) bool {
	return validFinancialID(value)
}

func financialID(prefix string) (string, error) {
	bytes := make([]byte, 18)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return prefix + "_" + base64.RawURLEncoding.EncodeToString(bytes), nil
}

func contextErr(ctx context.Context) error {
	if ctx == nil {
		return errors.New("financial context is required")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// QuestStep is a deprecated read-only compatibility projection.
type QuestStep struct {
	StepID      string      `json:"step_id"`
	Action      string      `json:"action"`
	HolonID     string      `json:"holon_id"`
	AmountCents int64       `json:"amount_cents"`
	FeeCents    int64       `json:"fee_cents"`
	Status      QuestStatus `json:"status"`
	Timestamp   time.Time   `json:"timestamp"`
	Version     int64       `json:"version"`
}

// AzoaClient is a fail-closed legacy adapter; use AzoaOrchestrator directly.
type AzoaClient struct{ orchestrator *AzoaOrchestrator }

func NewAzoaClient(_, _ string) *AzoaClient { return &AzoaClient{} }

func NewAzoaClientWithOrchestrator(orchestrator *AzoaOrchestrator) *AzoaClient {
	return &AzoaClient{orchestrator: orchestrator}
}

func (c *AzoaClient) CreateMarketplaceEscrowQuest(_, _ string, _, _ int64) (*QuestStep, error) {
	return nil, ErrLegacyAzoaDisabled
}

func (c *AzoaClient) SettleEscrow(step *QuestStep, _ bool) (*QuestStep, error) {
	return step, ErrLegacyAzoaDisabled
}

func (c *AzoaClient) CreateZakatDriveQuest(_ string, _ int64, _ string) (*QuestStep, error) {
	return nil, ErrLegacyAzoaDisabled
}
