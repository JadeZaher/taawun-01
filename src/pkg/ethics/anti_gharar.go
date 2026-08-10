package ethics

import (
	"fmt"
	"time"
)

// DeploymentSpec defines explicit infrastructure specs to eliminate Gharar (uncertainty).
type DeploymentSpec struct {
	AppName         string        `json:"app_name"`
	WorkspaceID     int           `json:"workspace_id"`
	CPULimit        string        `json:"cpu_limit"`       // e.g. "0.5 vCPU"
	MemoryLimitMB   int           `json:"memory_limit_mb"` // e.g. 512
	StorageMB       int           `json:"storage_mb"`      // e.g. 2048
	UptimeGuarantee string        `json:"uptime_guarantee"`// e.g. "99.9%"
	MonthlyFeeUSD   float64       `json:"monthly_fee_usd"` // Flat fee, zero Riba late penalty
	PreviewURL      string        `json:"preview_url"`
	PreviewExpiry   time.Time     `json:"preview_expiry"`
	IsStagingVetted bool          `json:"is_staging_vetted"`
}

// AntiGhararValidator ensures user previews and approves exact specs before payment.
type AntiGhararValidator struct{}

func NewAntiGhararValidator() *AntiGhararValidator {
	return &AntiGhararValidator{}
}

// ValidateForCheckout checks that an app has a valid staging preview and explicit specs.
func (v *AntiGhararValidator) ValidateForCheckout(spec *DeploymentSpec) error {
	if spec.AppName == "" {
		return fmt.Errorf("Gharar violation: App name is undefined")
	}
	if !spec.IsStagingVetted {
		return fmt.Errorf("Gharar violation: App must be test-driven in a free staging preview environment before purchasing deployment")
	}
	if spec.PreviewURL == "" {
		return fmt.Errorf("Gharar violation: Live preview sandbox URL missing")
	}
	if time.Now().After(spec.PreviewExpiry) {
		return fmt.Errorf("Gharar violation: Staging preview session expired. Please re-verify staging state")
	}
	if spec.MemoryLimitMB <= 0 || spec.StorageMB <= 0 {
		return fmt.Errorf("Gharar violation: Infrastructure limits must be explicitly specified (RAM/Storage/vCPU)")
	}
	return nil
}
