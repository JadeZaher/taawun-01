package conductor

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"taawun/pkg/models"
)

func TestReissueCreatesFreshImmutablePreviewForCurrentBuilder(t *testing.T) {
	harness := newConductorHarness(t)
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	harness.service.now = func() time.Time { return now }
	created, err := harness.service.Compose(context.Background(), harness.owner, validCompositionRequest())
	if err != nil {
		t.Fatalf("compose source: %v", err)
	}
	sourceBefore := cloneTrack(created.Track)

	viewer := &models.User{ID: 8}
	if result, err := harness.service.Reissue(context.Background(), viewer, created.Track.ID, created.Track.Version, "viewer-reissue", 24); result != nil || !errors.Is(err, ErrWorkspaceForbidden) {
		t.Fatalf("viewer reissue = result:%+v err:%v", result, err)
	}
	if result, err := harness.service.Reissue(context.Background(), harness.owner, created.Track.ID, created.Track.Version-1, "stale-reissue", 24); result != nil || !errors.Is(err, ErrTrackVersionConflict) {
		t.Fatalf("stale reissue = result:%+v err:%v", result, err)
	}

	now = now.Add(48 * time.Hour)
	builder := &models.User{ID: 10}
	reissued, err := harness.service.Reissue(context.Background(), builder, created.Track.ID, created.Track.Version, "fresh-reissue", 72)
	if err != nil || reissued == nil || !reissued.Created || reissued.Track == nil {
		t.Fatalf("reissue = %+v err=%v", reissued, err)
	}
	if reissued.Track.ID == created.Track.ID || reissued.Track.Request.IdempotencyKey != "fresh-reissue" || reissued.Track.Request.TTLHours != 72 {
		t.Fatalf("new track identity/request = %+v", reissued.Track)
	}
	if reissued.Track.BuildRequest == nil || reissued.Track.BuildRequest.Subject.UserID != builder.ID || reissued.Track.CreatedBy != builder.ID {
		t.Fatalf("fresh actor authority = track:%+v build:%+v", reissued.Track, reissued.Track.BuildRequest)
	}
	if harness.origins.calls != 4 {
		t.Fatalf("origin authorization calls = %d, want source and reissue preflight plus race checks", harness.origins.calls)
	}
	wantExpiry := now.Add(72 * time.Hour)
	if reissued.Track.Preview == nil || !reissued.Track.Preview.AuthorizationExpiresAt.Equal(wantExpiry) ||
		reissued.Track.Artifact == nil || !reissued.Track.Artifact.Manifest.Authorization.ExpiresAt.Equal(wantExpiry) {
		t.Fatalf("fresh expiry = preview:%+v artifact:%+v want:%s", reissued.Track.Preview, reissued.Track.Artifact, wantExpiry)
	}
	if reissued.Track.Request.AppName != sourceBefore.Request.AppName ||
		!reflect.DeepEqual(reissued.Track.Request.Theme, sourceBefore.Request.Theme) ||
		!reflect.DeepEqual(reissued.Track.Request.Modules, sourceBefore.Request.Modules) ||
		!reflect.DeepEqual(reissued.Track.Request.Components, sourceBefore.Request.Components) ||
		!reflect.DeepEqual(reissued.Track.Request.RequestedOrigins, sourceBefore.Request.RequestedOrigins) {
		t.Fatalf("curated request changed: source=%+v reissued=%+v", sourceBefore.Request, reissued.Track.Request)
	}
	retried, err := harness.service.Reissue(context.Background(), builder, created.Track.ID, created.Track.Version, "fresh-reissue", 72)
	if err != nil || retried == nil || retried.Created || retried.Track == nil || retried.Track.ID != reissued.Track.ID {
		t.Fatalf("same-key reissue retry = %+v err=%v", retried, err)
	}
	sourceAfter, err := harness.service.GetTrack(context.Background(), harness.owner, created.Track.ID)
	if err != nil || !reflect.DeepEqual(sourceAfter, sourceBefore) {
		t.Fatalf("source mutated: before=%+v after=%+v err=%v", sourceBefore, sourceAfter, err)
	}
}

func TestReissueRejectsSourceWhoseCuratedRequestNoLongerMatchesArtifact(t *testing.T) {
	harness := newConductorHarness(t)
	created, err := harness.service.Compose(context.Background(), harness.owner, validCompositionRequest())
	if err != nil {
		t.Fatalf("compose source: %v", err)
	}
	tampered := created.Track.Request
	tampered.AppName = "Tampered"
	requestJSON, err := json.Marshal(tampered)
	if err != nil {
		t.Fatalf("marshal tampered fixture: %v", err)
	}
	if _, err := harness.repository.db.Exec(`UPDATE conductor_tracks SET request_json = ? WHERE id = ?`, requestJSON, created.Track.ID); err != nil {
		t.Fatalf("tamper fixture: %v", err)
	}
	if result, err := harness.service.Reissue(context.Background(), harness.owner, created.Track.ID, created.Track.Version, "tampered-reissue", 24); result != nil || !errors.Is(err, ErrTrackTransition) {
		t.Fatalf("tampered reissue = result:%+v err:%v", result, err)
	}
}
