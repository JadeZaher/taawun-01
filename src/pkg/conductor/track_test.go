package conductor

import (
	"context"
	"os"
	"testing"

	"taawun/pkg/primitives"
)

func TestConductorTrack_ExecutionSuccess(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "taawun_conductor_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	p2pHub := primitives.NewP2PRelayHub()
	ltapService := primitives.NewLTAPStorageService(tempDir)
	track := NewConductorTrack(p2pHub, ltapService)

	req := &TrackRequest{
		AppName:     "Test Standup App",
		Prompt:      "Build a team standup tracker for 5 engineers",
		WorkspaceID: 1,
		Environment: "staging",
	}

	res, err := track.ExecuteTrack(context.Background(), req)
	if err != nil {
		t.Fatalf("Expected Conductor Track execution to succeed, got: %v", err)
	}

	if res.Status != StepCompleted {
		t.Errorf("Expected status %s, got %s", StepCompleted, res.Status)
	}

	if len(res.Events) != 6 {
		t.Errorf("Expected 6 pipeline events, got %d", len(res.Events))
	}

	if res.EthicsAudit == nil || !res.EthicsAudit.Passed {
		t.Errorf("Expected ethics audit to pass")
	}
}

func TestConductorTrack_EthicsFailure(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "taawun_conductor_test_*")
	defer os.RemoveAll(tempDir)

	p2pHub := primitives.NewP2PRelayHub()
	ltapService := primitives.NewLTAPStorageService(tempDir)
	track := NewConductorTrack(p2pHub, ltapService)

	req := &TrackRequest{
		AppName:     "Invalid Financial App",
		Prompt:      "Create a payday loan interest calculator with late fee compound interest",
		WorkspaceID: 1,
		Environment: "staging",
	}

	res, err := track.ExecuteTrack(context.Background(), req)
	if err == nil {
		t.Fatalf("Expected Conductor Track to fail due to Riba ethics violation")
	}

	if res.Status != StepFailed {
		t.Errorf("Expected status %s, got %s", StepFailed, res.Status)
	}
}
