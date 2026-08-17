package financial

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// FederationQuestProjection is the non-secret quest data carried by federation.
type FederationQuestProjection struct {
	OrchestrationVersion string       `json:"orchestration_version"`
	FlowID               FlowID       `json:"flow_id"`
	FlowVersion          int          `json:"flow_version"`
	IntentVersion        int64        `json:"intent_version"`
	Status               QuestStatus  `json:"status"`
	Currency             string       `json:"currency"`
	AmountMinor          int64        `json:"amount_minor"`
	FeeMinor             int64        `json:"fee_minor"`
	Parties              []string     `json:"parties"`
	Allocations          []Allocation `json:"allocations,omitempty"`
}

// FederationIntent signs a quest projection with the existing federation protocol.
func (o *AzoaOrchestrator) FederationIntent(ctx context.Context, questID, targetNodeID string, signer *FederationSigner, expiresAt time.Time) (*FederationEnvelope, error) {
	quest, err := o.GetQuest(ctx, questID)
	if err != nil {
		return nil, err
	}
	if quest.Status != QuestApproved && quest.Status != QuestExecuting {
		return nil, fmt.Errorf("%w: only approved or executing quests may federate", ErrInvalidTransition)
	}
	payload, err := json.Marshal(FederationQuestProjection{
		OrchestrationVersion: AzoaOrchestrationVersion,
		FlowID:               quest.FlowID,
		FlowVersion:          quest.FlowVersion,
		IntentVersion:        quest.Intent.Version,
		Status:               quest.Status,
		Currency:             quest.Intent.Currency,
		AmountMinor:          quest.Intent.AmountMinor,
		FeeMinor:             quest.Intent.FeeMinor,
		Parties:              append([]string(nil), quest.Intent.Parties...),
		Allocations:          append([]Allocation(nil), quest.Intent.Allocations...),
	})
	if err != nil {
		return nil, fmt.Errorf("encode federation quest projection: %w", err)
	}
	return signer.SignSettlementIntent(targetNodeID, quest.ID, quest.Intent.ID, payload, expiresAt)
}
