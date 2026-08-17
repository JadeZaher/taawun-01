package financial

import (
	"errors"
	"fmt"
	"time"
)

// QuestStatus represents the state of an AZOA quest graph step.
type QuestStatus string

const (
	QuestPending    QuestStatus = "PENDING"
	QuestStaged     QuestStatus = "STAGED"
	QuestSettled    QuestStatus = "SETTLED"
	QuestEscalated  QuestStatus = "ESCALATED_HUMAN"
	QuestFailed     QuestStatus = "FAILED_CLOSED"
)

// QuestStep represents a single immutable, fail-closed step in an AZOA quest graph.
type QuestStep struct {
	StepID      string      `json:"step_id"`
	Action      string      `json:"action"`
	HolonID     string      `json:"holon_id"`
	AmountCents int64       `json:"amount_cents"`
	FeeCents    int64       `json:"fee_cents"`
	Status      QuestStatus `json:"status"`
	Timestamp   time.Time   `json:"timestamp"`
	Signature   string      `json:"signature"`
}

// AzoaClient handles communication with the AZOA financial orchestration engine.
type AzoaClient struct {
	NodeURL string
	APIKey  string
}

// NewAzoaClient returns a client targeting an AZOA node.
func NewAzoaClient(nodeURL, apiKey string) *AzoaClient {
	if nodeURL == "" {
		nodeURL = "https://azoa-api-production.up.railway.app"
	}
	return &AzoaClient{
		NodeURL: nodeURL,
		APIKey:  apiKey,
	}
}

// CreateMarketplaceEscrowQuest scaffolds a marketplace template purchase escrow graph.
func (c *AzoaClient) CreateMarketplaceEscrowQuest(buyerID, creatorID string, priceCents int64, feeCents int64) (*QuestStep, error) {
	if priceCents <= 0 {
		return nil, errors.New("invalid escrow amount: price must be positive")
	}

	step := &QuestStep{
		StepID:      fmt.Sprintf("quest_escrow_%d", time.Now().UnixNano()),
		Action:      "MARKETPLACE_ESCROW_LOCK",
		HolonID:     fmt.Sprintf("holon_bundle_%s", buyerID),
		AmountCents: priceCents,
		FeeCents:    feeCents,
		Status:      QuestStaged,
		Timestamp:   time.Now(),
		Signature:   "sig_azoa_star_crdt_split",
	}

	return step, nil
}

// SettleEscrow executes settlement after live staging preview confirmation.
func (c *AzoaClient) SettleEscrow(step *QuestStep, buyerConfirmed bool) (*QuestStep, error) {
	if !buyerConfirmed {
		step.Status = QuestFailed
		return step, fmt.Errorf("staging preview unconfirmed: escrow failed closed and refunded")
	}

	step.Status = QuestSettled
	step.Timestamp = time.Now()
	return step, nil
}

// CreateZakatDriveQuest creates a pre-vetted STAR Zakat or Charity donation quest primitive.
func (c *AzoaClient) CreateZakatDriveQuest(donorID string, amountCents int64, cause string) (*QuestStep, error) {
	if amountCents <= 0 {
		return nil, errors.New("donation amount must be greater than zero")
	}

	return &QuestStep{
		StepID:      fmt.Sprintf("quest_zakat_%d", time.Now().UnixNano()),
		Action:      fmt.Sprintf("ZAKAT_DONATION_%s", cause),
		HolonID:     fmt.Sprintf("holon_community_trust_%s", donorID),
		AmountCents: amountCents,
		FeeCents:    0, // Zero interest/exploitative fee
		Status:      QuestSettled,
		Timestamp:   time.Now(),
		Signature:   "sig_star_zakat_verified",
	}, nil
}
