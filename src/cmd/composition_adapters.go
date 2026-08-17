package main

import (
	"context"
	"fmt"
	"strings"

	"taawun/pkg/bazaar"
	"taawun/pkg/ethics"
	"taawun/pkg/shura"
)

type bazaarShuraDecisionResolver struct {
	service *shura.Service
}

func (r bazaarShuraDecisionResolver) ResolveDecision(ctx context.Context, workspaceID int, reference string) (bazaar.ShuraDecision, error) {
	if r.service == nil {
		return bazaar.ShuraDecision{}, fmt.Errorf("Shura decision service is unavailable")
	}
	decision, err := r.service.ResolveDecision(ctx, workspaceID, reference)
	if err != nil {
		return bazaar.ShuraDecision{}, err
	}
	return bazaar.ShuraDecision{Reference: decision.Reference, WorkspaceID: decision.WorkspaceID, Approved: decision.Approved}, nil
}

type referenceBazaarComplianceReviewer struct {
	corpus *ethics.ComplianceCorpus
}

func (r referenceBazaarComplianceReviewer) ReviewListing(ctx context.Context, revision bazaar.ListingRevision) (bazaar.ComplianceDisclosure, error) {
	if r.corpus == nil {
		return bazaar.ComplianceDisclosure{}, fmt.Errorf("reference compliance corpus is unavailable")
	}
	terms := []string{revision.Title, revision.Summary, revision.TemplateID}
	for _, primitive := range revision.Primitives {
		terms = append(terms, primitive.ID)
	}
	references, err := r.corpus.Retrieve(strings.Join(terms, " "), revision.Compliance.Madhhab, 8)
	if err != nil {
		return bazaar.ComplianceDisclosure{}, err
	}
	citations := make([]string, 0, len(references))
	for _, reference := range references {
		citations = append(citations, reference.ID+": "+reference.Citation)
	}
	if len(citations) == 0 {
		return bazaar.ComplianceDisclosure{}, fmt.Errorf("no reference compliance citations were found")
	}
	return bazaar.ComplianceDisclosure{Madhhab: revision.Compliance.Madhhab, Status: "passed", Citations: citations}, nil
}
