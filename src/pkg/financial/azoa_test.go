package financial

import (
	"testing"
)

func TestAzoaClient_MarketplaceEscrow(t *testing.T) {
	client := NewAzoaClient("", "test-key")

	step, err := client.CreateMarketplaceEscrowQuest("buyer_123", "creator_456", 2500, 100)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if step.Status != QuestStaged {
		t.Errorf("expected QuestStaged, got: %s", step.Status)
	}

	// Confirm staging preview
	settledStep, err := client.SettleEscrow(step, true)
	if err != nil {
		t.Fatalf("expected settlement success, got: %v", err)
	}
	if settledStep.Status != QuestSettled {
		t.Errorf("expected QuestSettled, got: %s", settledStep.Status)
	}

	// Test unconfirmed preview -> fail closed
	staged2, _ := client.CreateMarketplaceEscrowQuest("buyer_123", "creator_456", 2500, 100)
	failedStep, err := client.SettleEscrow(staged2, false)
	if err == nil {
		t.Error("expected error for unconfirmed staging preview")
	}
	if failedStep.Status != QuestFailed {
		t.Errorf("expected QuestFailed, got: %s", failedStep.Status)
	}
}

func TestAzoaClient_ZakatDriveQuest(t *testing.T) {
	client := NewAzoaClient("", "test-key")
	quest, err := client.CreateZakatDriveQuest("donor_789", 5000, "Ramadan_Food_Bank")
	if err != nil {
		t.Fatalf("expected zero error, got: %v", err)
	}
	if quest.Status != QuestSettled {
		t.Errorf("expected QuestSettled, got: %s", quest.Status)
	}
	if quest.FeeCents != 0 {
		t.Errorf("expected zero interest fee, got: %d", quest.FeeCents)
	}
}
