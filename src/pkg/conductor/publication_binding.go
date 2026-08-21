package conductor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"taawun/pkg/domains"
)

// ResolvePublicationTrackBindings performs one bounded indexed lookup and returns only fully validated candidates.
func (r *Repository) ResolvePublicationTrackBindings(ctx context.Context, workspaceID int, claimID string, publications []domains.Publication) (map[string][]domains.PublicationTrackBinding, error) {
	bindings := make(map[string][]domains.PublicationTrackBinding, len(publications))
	if workspaceID <= 0 || claimID == "" || len(publications) == 0 {
		return bindings, nil
	}
	expected := make(map[string]domains.Publication, len(publications))
	placeholders := make([]string, 0, len(publications))
	arguments := []any{workspaceID, claimID}
	for _, publication := range publications {
		if publication.WorkspaceID != workspaceID || publication.ClaimID != claimID || !validIdentifier(publication.ID) {
			continue
		}
		if _, duplicate := expected[publication.ID]; duplicate {
			continue
		}
		expected[publication.ID] = publication
		placeholders = append(placeholders, "?")
		arguments = append(arguments, publication.ID)
	}
	if len(placeholders) == 0 {
		return bindings, nil
	}
	query := `WITH candidates AS (
		SELECT id, workspace_id, status, version, claim_id, publication_id, artifact_json, publication_json,
			COUNT(*) OVER (PARTITION BY publication_id) AS candidate_count,
			ROW_NUMBER() OVER (PARTITION BY publication_id ORDER BY id) AS candidate_row
		FROM conductor_tracks WHERE workspace_id = ? AND claim_id = ? AND publication_id IN (` + strings.Join(placeholders, ",") + `)
	) SELECT id, workspace_id, status, version, claim_id, publication_id, artifact_json, publication_json, candidate_count
		FROM candidates WHERE candidate_row = 1`
	rows, err := r.db.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("query Conductor publication bindings: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var trackID, status, trackClaimID, publicationID string
		var trackWorkspaceID int
		var version, candidateCount int64
		var artifactJSON, publicationJSON []byte
		if err := rows.Scan(&trackID, &trackWorkspaceID, &status, &version, &trackClaimID, &publicationID, &artifactJSON, &publicationJSON, &candidateCount); err != nil {
			return nil, fmt.Errorf("scan Conductor publication binding: %w", err)
		}
		publication, ok := expected[publicationID]
		if !ok || candidateCount != 1 || trackWorkspaceID != workspaceID || trackClaimID != claimID || status != string(TrackPublished) || version <= 0 || !validIdentifier(trackID) {
			continue
		}
		var artifact ArtifactReference
		var snapshot domains.Publication
		if json.Unmarshal(artifactJSON, &artifact) != nil || json.Unmarshal(publicationJSON, &snapshot) != nil {
			continue
		}
		if snapshot.ID != publication.ID || snapshot.WorkspaceID != publication.WorkspaceID || snapshot.ClaimID != publication.ClaimID ||
			snapshot.Origin != publication.Origin || snapshot.Host != publication.Host || snapshot.ArtifactID != publication.ArtifactID ||
			snapshot.ContentHash != publication.ContentHash || !validTrackArtifactPublicationLink(artifact, publication) {
			continue
		}
		bindings[publicationID] = append(bindings[publicationID], domains.PublicationTrackBinding{TrackID: trackID, Status: status, Version: version})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read Conductor publication bindings: %w", err)
	}
	return bindings, nil
}

func validTrackArtifactPublicationLink(artifact ArtifactReference, publication domains.Publication) bool {
	return publication.WorkspaceID > 0 && publication.ArtifactID != "" && publication.ContentHash != "" &&
		artifact.ArtifactID == publication.ArtifactID && artifact.ContentHash == publication.ContentHash &&
		artifact.Manifest.WorkspaceID == publication.WorkspaceID && artifact.Manifest.ArtifactID == artifact.ArtifactID &&
		artifact.Manifest.ContentHash == artifact.ContentHash
}
