package conductor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"taawun/pkg/artifacts"
	"taawun/pkg/domains"
	"taawun/pkg/ethics"
	"taawun/pkg/models"
)

func TestCompositionCreatesDurableSignedPreviewBeforeExplicitPublication(t *testing.T) {
	harness := newConductorHarness(t)
	ctx := context.Background()
	request := validCompositionRequest()
	result, err := harness.service.Compose(ctx, harness.owner, request)
	if err != nil {
		t.Fatalf("compose preview: %v", err)
	}
	track := result.Track
	if !result.Created || track.Status != TrackPreviewReady || track.Version != 6 || track.Artifact == nil || track.Preview == nil {
		t.Fatalf("unexpected preview track: %+v", result)
	}
	if !track.Preview.AuthenticationRequired || track.Preview.ContentHash != track.Artifact.ContentHash || track.Preview.WorkspaceID != 42 || track.Publication != nil || harness.publisher.publishCalls != 0 {
		t.Fatalf("preview implied publication: track=%+v calls=%d", track, harness.publisher.publishCalls)
	}
	if track.Compliance.ReviewStatus != ReviewPendingQualified || track.Compliance.ReferenceID == "" || track.Compliance.Disclaimer == "" {
		t.Fatalf("compliance evidence is not visible: %+v", track.Compliance)
	}

	if err := harness.repository.Close(); err != nil {
		t.Fatalf("close repository: %v", err)
	}
	reopened, err := OpenRepository(harness.databasePath)
	if err != nil {
		t.Fatalf("reopen repository: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	harness.repository = reopened
	harness.service, err = NewService(reopened, harness.workspaces, ActorSubjectResolver{}, CuratedCompositionValidator{}, harness.auditor, harness.builder, harness.origins, harness.publisher)
	if err != nil {
		t.Fatalf("recreate service: %v", err)
	}
	persisted, err := harness.service.GetTrack(ctx, harness.owner, track.ID)
	if err != nil || persisted.Status != TrackPreviewReady || persisted.Artifact.ContentHash != track.Artifact.ContentHash {
		t.Fatalf("durable preview: track=%+v err=%v", persisted, err)
	}

	requested, err := harness.service.RequestPublication(ctx, harness.owner, track.ID, persisted.Version, "claim-community")
	if err != nil || requested.Status != TrackPublicationRequested || harness.publisher.publishCalls != 0 {
		t.Fatalf("request publication: track=%+v calls=%d err=%v", requested, harness.publisher.publishCalls, err)
	}
	if _, err := harness.service.ActivatePublication(ctx, harness.owner, track.ID, persisted.Version); !errors.Is(err, ErrTrackVersionConflict) {
		t.Fatalf("stale activation error = %v", err)
	}
	published, err := harness.service.ActivatePublication(ctx, harness.owner, track.ID, requested.Version)
	if err != nil || published.Status != TrackPublished || published.Publication == nil || !published.Publication.Active || harness.publisher.publishCalls != 1 {
		t.Fatalf("activate publication: track=%+v calls=%d err=%v", published, harness.publisher.publishCalls, err)
	}

	events, err := harness.service.Events(ctx, harness.owner, track.ID)
	if err != nil || len(events) != 9 {
		t.Fatalf("track events: len=%d err=%v", len(events), err)
	}
	for index, event := range events {
		if event.Hash == "" || (index == 0 && event.PreviousHash != "") || (index > 0 && event.PreviousHash != events[index-1].Hash) {
			t.Fatalf("broken event chain at %d: %+v", index, event)
		}
	}
	if _, err := reopened.db.ExecContext(ctx, `UPDATE conductor_track_events SET event_type = 'TAMPERED' WHERE track_id = ?`, track.ID); err == nil {
		t.Fatal("append-only event update unexpectedly succeeded")
	}
	repeated, err := harness.service.Compose(ctx, harness.owner, request)
	if err != nil || repeated.Created || repeated.Track.ID != track.ID || repeated.Track.Status != TrackPublished {
		t.Fatalf("idempotent composition: result=%+v err=%v", repeated, err)
	}
	request.AppName = "Different app"
	if _, err := harness.service.Compose(ctx, harness.owner, request); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("conflicting idempotency error = %v", err)
	}
}

func TestInvalidOrRejectedCompositionFailsBeforeArtifactBuild(t *testing.T) {
	harness := newConductorHarness(t)
	request := validCompositionRequest()
	request.TemplateID = "arbitrary-code-template"
	result, err := harness.service.Compose(context.Background(), harness.owner, request)
	if err == nil || result.Track.Status != TrackFailed || result.Track.FailureCode != "composition_validation_failed" || harness.builder.buildCalls != 0 {
		t.Fatalf("invalid composition: result=%+v builds=%d err=%v", result, harness.builder.buildCalls, err)
	}

	harness2 := newConductorHarness(t)
	harness2.auditor.reject = true
	request = validCompositionRequest()
	result, err = harness2.service.Compose(context.Background(), harness2.owner, request)
	if err == nil || result.Track.Status != TrackFailed || result.Track.Compliance == nil || result.Track.Compliance.Disposition != ComplianceReject || harness2.builder.buildCalls != 0 {
		t.Fatalf("rejected compliance: result=%+v builds=%d err=%v", result, harness2.builder.buildCalls, err)
	}
}

func TestTransientArtifactFailureCanResumeWithoutDockerFallback(t *testing.T) {
	harness := newConductorHarness(t)
	harness.builder.buildErr = errors.New("signer temporarily unavailable")
	result, err := harness.service.Compose(context.Background(), harness.owner, validCompositionRequest())
	if !errors.Is(err, ErrDependencyUnavailable) || result.Track.Status != TrackComplianceAudited || result.Track.Artifact != nil {
		t.Fatalf("transient build failure: result=%+v err=%v", result, err)
	}
	harness.builder.buildErr = nil
	resumed, err := harness.service.Resume(context.Background(), harness.owner, result.Track.ID, result.Track.Version)
	if err != nil || resumed.Status != TrackPreviewReady || resumed.Artifact == nil || harness.builder.buildCalls != 2 {
		t.Fatalf("resume signed preview: track=%+v builds=%d err=%v", resumed, harness.builder.buildCalls, err)
	}
}

func TestLegacyConductorFailsClosed(t *testing.T) {
	legacy := NewConductorTrack(nil, nil)
	if _, err := legacy.ExecuteTrack(context.Background(), &TrackRequest{}); !errors.Is(err, ErrLegacyWorkflowDisabled) {
		t.Fatalf("legacy execution error = %v", err)
	}
}

type conductorHarness struct {
	databasePath string
	repository   *Repository
	service      *Service
	workspaces   *fakeWorkspaceAuthorizer
	auditor      *fakeComplianceAuditor
	builder      *fakeSignedBuilder
	origins      *fakeOriginAuthority
	publisher    *fakePublicationService
	owner        *models.User
}

func newConductorHarness(t *testing.T) *conductorHarness {
	t.Helper()
	databasePath := filepath.Join(t.TempDir(), "conductor.db")
	repository, err := OpenRepository(databasePath)
	if err != nil {
		t.Fatalf("open repository: %v", err)
	}
	t.Cleanup(func() { _ = repository.Close() })
	workspaces := &fakeWorkspaceAuthorizer{}
	auditor := &fakeComplianceAuditor{}
	builder := &fakeSignedBuilder{}
	origins := &fakeOriginAuthority{}
	publisher := &fakePublicationService{}
	service, err := NewService(repository, workspaces, ActorSubjectResolver{}, CuratedCompositionValidator{}, auditor, builder, origins, publisher)
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	return &conductorHarness{databasePath: databasePath, repository: repository, service: service, workspaces: workspaces, auditor: auditor, builder: builder, origins: origins, publisher: publisher, owner: &models.User{ID: 7}}
}

func validCompositionRequest() CompositionRequest {
	return CompositionRequest{
		IdempotencyKey: "compose-community-1", WorkspaceID: 42,
		AppName: "Community Iftar", OrganizationName: "Northside Mosque", City: "Denver",
		Madhhab: ethics.MadhhabHanafi, TemplateID: artifacts.TemplateCommunityIftar,
		Theme:            artifacts.ThemeRequest{AccentColor: "#166534"},
		Modules:          []string{artifacts.ModuleAnnouncements, artifacts.ModuleRegistration},
		RequestedOrigins: artifacts.OriginPolicy{Surfaces: []string{"https://preview.taawun.example"}}, TTLHours: 24,
	}
}

type fakeWorkspaceAuthorizer struct{}

func (*fakeWorkspaceAuthorizer) AuthorizeWorkspaceCapability(actor *models.User, workspaceID int, _ models.WorkspaceCapability) (*models.Workspace, error) {
	if actor == nil || actor.ID != 7 || workspaceID != 42 {
		return nil, errors.New("denied")
	}
	return &models.Workspace{ID: 42, Status: models.WorkspaceStatusActive}, nil
}

type fakeComplianceAuditor struct{ reject bool }

func (a *fakeComplianceAuditor) AuditComposition(_ context.Context, request artifacts.BuildRequest) (ComplianceEvidence, error) {
	passed := !a.reject
	disposition := CompliancePass
	if a.reject {
		disposition = ComplianceReject
	}
	return ComplianceEvidence{
		ReferenceID: "audit-reference-1", Madhhab: request.Madhhab, Disposition: disposition,
		ReviewStatus: ReviewPendingQualified, Audit: &ethics.ComplianceResult{Passed: passed},
		Disclaimer: "Reference-only evidence pending qualified review; not a fatwa.",
	}, nil
}

type fakeSignedBuilder struct {
	buildCalls int
	buildErr   error
}

func (*fakeSignedBuilder) ProductionReady() bool { return true }

func (b *fakeSignedBuilder) Build(_ context.Context, request artifacts.BuildRequest) (artifacts.BuildResult, error) {
	b.buildCalls++
	if b.buildErr != nil {
		return artifacts.BuildResult{}, b.buildErr
	}
	encoded, _ := json.Marshal(request)
	digest := sha256.Sum256(encoded)
	contentHash := hex.EncodeToString(digest[:])
	artifactID := "artifact_signed_1"
	manifest := artifacts.Manifest{
		ArtifactID: artifactID, ContentHash: contentHash, WorkspaceID: request.WorkspaceID,
		Authorization: artifacts.BundleAuthorization{
			Subject:        artifacts.BundleSubject{ID: request.Subject.ID, UserID: request.Subject.UserID, WorkspaceID: request.WorkspaceID},
			AllowedOrigins: request.AllowedOrigins, ExpiresAt: request.ExpiresAt, Lifecycle: request.Lifecycle,
		},
		Signature: artifacts.BundleSignature{Algorithm: "Ed25519", Value: "verified-test-signature"},
	}
	return artifacts.BuildResult{ArtifactID: artifactID, ContentHash: contentHash, Manifest: manifest}, nil
}

type fakeOriginAuthority struct{}

func (*fakeOriginAuthority) AuthorizeOriginsForLifecycle(_ context.Context, _ *models.User, _ *models.Workspace, lifecycle artifacts.BundleLifecycle, requested artifacts.OriginPolicy) (artifacts.OriginPolicy, error) {
	if lifecycle != artifacts.BundleLifecyclePreview || len(requested.Surfaces) != 1 || requested.Surfaces[0] != "https://preview.taawun.example" {
		return artifacts.OriginPolicy{}, errors.New("origin denied")
	}
	return requested, nil
}

type fakePublicationService struct {
	publishCalls int
	history      []domains.Publication
}

func (p *fakePublicationService) Publish(_ context.Context, _ *models.User, workspaceID int, claimID, contentHash string) (domains.Publication, error) {
	p.publishCalls++
	publication := domains.Publication{ID: "publication-1", WorkspaceID: workspaceID, ClaimID: claimID, Origin: "https://community.example", Host: "community.example", ContentHash: contentHash, ArtifactID: "artifact_signed_1", Active: true, ActivatedAt: time.Now().UTC()}
	p.history = []domains.Publication{publication}
	return publication, nil
}

func (p *fakePublicationService) PublicationHistory(context.Context, *models.User, int, string) ([]domains.Publication, error) {
	return append([]domains.Publication(nil), p.history...), nil
}
