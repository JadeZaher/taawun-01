package ethics

import (
	"testing"
	"time"
)

func TestHaramCheckEngine_CompliantPrompt(t *testing.T) {
	engine := NewHaramCheckEngine()
	res, err := engine.AuditPrompt("Build a Zakat distribution tracker for my local mosque.")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !res.Passed {
		t.Errorf("Expected compliant prompt to pass, got violations: %v", res.Violations)
	}
}

func TestHaramCheckEngine_RibaDetection(t *testing.T) {
	engine := NewHaramCheckEngine()
	res, err := engine.AuditPrompt("Create a conventional personal loan calculator with compound interest rate.")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if res.Passed {
		t.Errorf("Expected Riba prompt to fail ethics audit, but it passed.")
	}
	if len(res.Violations) == 0 {
		t.Errorf("Expected violations list to contain Riba detection notice.")
	}
}

func TestAntiGhararValidator(t *testing.T) {
	validator := NewAntiGhararValidator()
	spec := &DeploymentSpec{
		AppName:         "Halal Directory",
		WorkspaceID:     1,
		CPULimit:        "0.5 vCPU",
		MemoryLimitMB:   512,
		StorageMB:       1024,
		UptimeGuarantee: "99.9%",
		MonthlyFeeUSD:   10.0,
		PreviewURL:      "http://localhost:8081",
		PreviewExpiry:   time.Now().Add(1 * time.Hour),
		IsStagingVetted: true,
	}

	if err := validator.ValidateForCheckout(spec); err != nil {
		t.Errorf("Expected valid spec to pass Anti-Gharar audit, got: %v", err)
	}

	// Test invalid spec without staging vetting
	spec.IsStagingVetted = false
	if err := validator.ValidateForCheckout(spec); err == nil {
		t.Errorf("Expected unvetted staging spec to fail Anti-Gharar check")
	}
}
