package conductor

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"taawun/pkg/models"
)

func TestTrackSummaryListOrdersPaginatesAndRedacts(t *testing.T) {
	harness := newConductorHarness(t)
	ctx := context.Background()

	empty, err := harness.service.ListTracks(ctx, harness.owner, 42, 2, "")
	if err != nil || empty.Tracks == nil || len(empty.Tracks) != 0 || empty.NextCursor != "" {
		t.Fatalf("empty list = %+v, err = %v", empty, err)
	}

	sharedTime := time.Date(2026, 8, 18, 7, 25, 0, 123000000, time.UTC)
	expiresAt := time.Date(2026, 8, 19, 7, 25, 0, 0, time.UTC)
	insertTrackSummaryFixture(t, harness.repository, "track_z", 42, "community-iftar", TrackPreviewReady, 6, sharedTime, true, &expiresAt, false)
	insertTrackSummaryFixture(t, harness.repository, "track_a", 42, "ramadan-campaign", TrackPublicationRequested, 7, sharedTime, true, &expiresAt, true)
	insertTrackSummaryFixture(t, harness.repository, "track_old", 42, "mutual-aid", TrackFailed, 3, sharedTime.Add(-time.Hour), false, nil, false)
	insertTrackSummaryFixture(t, harness.repository, "track_other", 99, "community-iftar", TrackPublished, 8, sharedTime.Add(time.Hour), true, &expiresAt, true)

	first, err := harness.service.ListTracks(ctx, harness.owner, 42, 2, "")
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if len(first.Tracks) != 2 || first.Tracks[0].ID != "track_z" || first.Tracks[1].ID != "track_a" || first.NextCursor == "" {
		t.Fatalf("first page order/cursor = %+v", first)
	}
	if first.Tracks[0].TemplateID != "community-iftar" || first.Tracks[0].Status != TrackPreviewReady || first.Tracks[0].Version != 6 || !first.Tracks[0].PreviewPresent || !first.Tracks[0].ArtifactPresent || first.Tracks[0].PublicationPresent || first.Tracks[0].AuthorizationExpiresAt == nil || !first.Tracks[0].AuthorizationExpiresAt.Equal(expiresAt) {
		t.Fatalf("first summary = %+v", first.Tracks[0])
	}
	second, err := harness.service.ListTracks(ctx, harness.owner, 42, 2, first.NextCursor)
	if err != nil || len(second.Tracks) != 1 || second.Tracks[0].ID != "track_old" || second.NextCursor != "" {
		t.Fatalf("second page = %+v, err = %v", second, err)
	}

	viewer, err := harness.service.ListTracks(ctx, &models.User{ID: 8}, 42, 50, "")
	if err != nil || len(viewer.Tracks) != 3 {
		t.Fatalf("viewer list = %+v, err = %v", viewer, err)
	}
	if _, err := harness.service.ListTracks(ctx, &models.User{ID: 9}, 42, 20, ""); !errors.Is(err, ErrWorkspaceForbidden) {
		t.Fatalf("outsider error = %v", err)
	}

	encoded, err := json.Marshal(struct {
		First  TrackSummaryPage `json:"first"`
		Second TrackSummaryPage `json:"second"`
	}{First: first, Second: second})
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"component-document-secret", "actor-secret", "signature-secret", "failure-secret", "token-secret",
		"workspaceId", "createdBy", "failureCode", "artifactId", "contentHash", "claimId", "publicationId",
	} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("summary JSON leaked %q: %s", forbidden, encoded)
		}
	}

	var indexCount int
	if err := harness.repository.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_conductor_tracks_workspace_updated'`).Scan(&indexCount); err != nil || indexCount != 1 {
		t.Fatalf("workspace list index count = %d, err = %v", indexCount, err)
	}
}

func TestTrackSummaryListRejectsInvalidBoundsAndCursor(t *testing.T) {
	harness := newConductorHarness(t)
	ctx := context.Background()
	for _, limit := range []int{0, -1, MaximumTrackListLimit + 1} {
		if _, err := harness.service.ListTracks(ctx, harness.owner, 42, limit, ""); !errors.Is(err, ErrInvalidTrackQuery) {
			t.Fatalf("limit %d error = %v", limit, err)
		}
	}
	for _, cursor := range []string{"not-a-cursor", "eAo", strings.Repeat("a", 257)} {
		if _, err := harness.service.ListTracks(ctx, harness.owner, 42, 20, cursor); !errors.Is(err, ErrInvalidTrackQuery) {
			t.Fatalf("cursor %q error = %v", cursor, err)
		}
	}
}

func insertTrackSummaryFixture(t *testing.T, repository *Repository, id string, workspaceID int, templateID string, status TrackStatus, version int64, updatedAt time.Time, artifactPresent bool, expiresAt *time.Time, publicationPresent bool) {
	t.Helper()
	requestJSON, err := json.Marshal(map[string]any{
		"templateId": templateID,
		"components": []any{map[string]any{"data": map[string]any{"secret": "component-document-secret"}}},
		"token":      "token-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	artifactJSON := ""
	if artifactPresent {
		artifactJSON = `{"artifactId":"artifact-secret","contentHash":"hash-secret","signature":"signature-secret"}`
	}
	previewJSON := ""
	if expiresAt != nil {
		previewJSONBytes, err := json.Marshal(map[string]any{
			"authorizationExpiresAt": expiresAt.UTC(),
			"signature":              "signature-secret",
		})
		if err != nil {
			t.Fatal(err)
		}
		previewJSON = string(previewJSONBytes)
	}
	publicationJSON := ""
	if publicationPresent {
		publicationJSON = `{"id":"publication-secret","token":"token-secret"}`
	}
	_, err = repository.db.Exec(`INSERT INTO conductor_tracks
		(id, workspace_id, idempotency_key, request_hash, request_json, build_request_json, status, version,
		 compliance_json, artifact_json, preview_json, claim_id, publication_json, failure_code, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, workspaceID, "idem_"+id, "hash_"+id, requestJSON, `{"actor":"actor-secret"}`, status, version,
		`{"actor":"actor-secret"}`, artifactJSON, previewJSON, "claim-secret", publicationJSON, "failure-secret", 991,
		updatedAt.Add(-time.Hour).UnixMilli(), updatedAt.UnixMilli())
	if err != nil {
		t.Fatalf("insert track summary fixture %s: %v", id, err)
	}
}
