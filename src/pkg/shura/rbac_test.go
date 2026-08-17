package shura

import (
	"testing"
	"time"
)

func TestTokenVerifier_Capabilities(t *testing.T) {
	verifier := NewTokenVerifier("pub-key-123")

	architectToken := &CapabilityToken{
		TokenID:     "tok_1",
		SubjectID:   "user_arch",
		WorkspaceID: "ws_mosque",
		Role:        RoleArchitect,
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	}

	maintainerToken := &CapabilityToken{
		TokenID:        "tok_2",
		SubjectID:      "user_maint",
		WorkspaceID:    "ws_mosque",
		Role:           RoleMaintainer,
		CRDTWriteScope: []string{"convergent:events", "convergent:announcements"},
		ExpiresAt:      time.Now().Add(1 * time.Hour),
	}

	viewerToken := &CapabilityToken{
		TokenID:     "tok_3",
		SubjectID:   "user_view",
		WorkspaceID: "ws_mosque",
		Role:        RoleViewer,
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	}

	// Architect can write CRDT and execute AZOA quest
	if err := verifier.CanWriteCRDT(architectToken, "convergent:events"); err != nil {
		t.Errorf("Architect failed CRDT write: %v", err)
	}
	if err := verifier.CanExecuteAzoaQuest(architectToken, "ZAKAT_DISBURSEMENT"); err != nil {
		t.Errorf("Architect failed AZOA quest exec: %v", err)
	}

	// Maintainer can write convergent CRDT but NOT execute AZOA quest
	if err := verifier.CanWriteCRDT(maintainerToken, "convergent:events"); err != nil {
		t.Errorf("Maintainer failed convergent CRDT write: %v", err)
	}
	if err := verifier.CanExecuteAzoaQuest(maintainerToken, "ZAKAT_DISBURSEMENT"); err == nil {
		t.Error("Maintainer should NOT be allowed to execute AZOA quest")
	}

	// Viewer cannot write CRDT
	if err := verifier.CanWriteCRDT(viewerToken, "convergent:events"); err == nil {
		t.Error("Viewer should NOT be allowed to write CRDT")
	}
}
