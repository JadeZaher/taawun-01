package conductor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"taawun/pkg/artifacts"
	"taawun/pkg/ethics"
)

// ReferenceComplianceAuditor exposes seed evidence without claiming scholar approval.
type ReferenceComplianceAuditor struct {
	engine *ethics.HaramCheckEngine
	corpus *ethics.ComplianceCorpus
}

func NewReferenceComplianceAuditor(engine *ethics.HaramCheckEngine, corpus *ethics.ComplianceCorpus) (*ReferenceComplianceAuditor, error) {
	if engine == nil || corpus == nil {
		return nil, ErrInvalidComposition
	}
	return &ReferenceComplianceAuditor{engine: engine, corpus: corpus}, nil
}

func (a *ReferenceComplianceAuditor) AuditComposition(ctx context.Context, request artifacts.BuildRequest) (ComplianceEvidence, error) {
	if err := contextError(ctx); err != nil {
		return ComplianceEvidence{}, err
	}
	declarative, err := json.Marshal(request)
	if err != nil {
		return ComplianceEvidence{}, err
	}
	audit, err := a.engine.AuditPrompt(string(declarative))
	if err != nil {
		return ComplianceEvidence{}, err
	}
	query := strings.Join(append([]string{request.AppName, request.OrganizationName, request.City, request.TemplateID}, request.Modules...), " ")
	references, err := a.corpus.Retrieve(query, request.Madhhab, 8)
	if err != nil && !errors.Is(err, ethics.ErrEmptyComplianceQuery) {
		return ComplianceEvidence{}, err
	}
	digest := sha256.Sum256(declarative)
	disposition := CompliancePass
	if !audit.Passed {
		disposition = ComplianceReject
	}
	return ComplianceEvidence{
		ReferenceID: "compliance_" + hex.EncodeToString(digest[:12]),
		Madhhab:     request.Madhhab, Disposition: disposition, ReviewStatus: ReviewPendingQualified,
		Audit: audit, References: references,
		Disclaimer: "Reference-only evidence pending qualified review; not a fatwa or scholar approval.",
	}, nil
}
