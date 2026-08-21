package conductor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
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
	if len(persisted.BuildRequest.Components) != len(persisted.Request.Modules) || !reflect.DeepEqual(persisted.BuildRequest.Components[0].Data, persisted.Artifact.Manifest.Components[0].Data) {
		t.Fatalf("durable component documents = request:%#v build:%#v manifest:%#v", persisted.Request.Components, persisted.BuildRequest.Components, persisted.Artifact.Manifest.Components)
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
	if err != nil {
		t.Fatalf("track events: len=%d err=%v", len(events), err)
	}
	expectedEvents := []struct {
		eventType  string
		fromStatus TrackStatus
		toStatus   TrackStatus
	}{
		{eventType: "TRACK_DRAFTED", toStatus: TrackDraft},
		{eventType: "COMPOSITION_STAGED", fromStatus: TrackDraft, toStatus: TrackStaged},
		{eventType: "COMPOSITION_VALIDATED", fromStatus: TrackStaged, toStatus: TrackValidated},
		{eventType: "COMPLIANCE_AUDITED", fromStatus: TrackValidated, toStatus: TrackComplianceAudited},
		{eventType: "ARTIFACT_SIGNED", fromStatus: TrackComplianceAudited, toStatus: TrackArtifactSigned},
		{eventType: "PREVIEW_READY", fromStatus: TrackArtifactSigned, toStatus: TrackPreviewReady},
		{eventType: "PUBLICATION_REQUESTED", fromStatus: TrackPreviewReady, toStatus: TrackPublicationRequested},
		{eventType: "PUBLICATION_ACTIVATED", fromStatus: TrackPublicationRequested, toStatus: TrackPublished},
	}
	if len(events) != len(expectedEvents) {
		t.Fatalf("track event sequence length=%d, want %d: %+v", len(events), len(expectedEvents), events)
	}
	for index, event := range events {
		expected := expectedEvents[index]
		if event.Type != expected.eventType || event.TrackVersion != int64(index+1) ||
			event.FromStatus != expected.fromStatus || event.ToStatus != expected.toStatus {
			t.Fatalf("track event %d = type:%s version:%d %s->%s, want type:%s version:%d %s->%s",
				index, event.Type, event.TrackVersion, event.FromStatus, event.ToStatus,
				expected.eventType, index+1, expected.fromStatus, expected.toStatus)
		}
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
	if !errors.Is(err, ErrInvalidComposition) || result != nil || harness.builder.buildCalls != 0 {
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

func TestUnverifiedPreviewOriginsDoNotConsumeDurableIdempotency(t *testing.T) {
	tests := []struct {
		name   string
		policy artifacts.OriginPolicy
	}{
		{name: "surfaces", policy: artifacts.OriginPolicy{Surfaces: []string{"https://unverified-surface.example"}}},
		{name: "embedders", policy: artifacts.OriginPolicy{Surfaces: []string{"https://preview.taawun.example"}, Embedders: []string{"https://unverified-embedder.example"}}},
		{name: "connections", policy: artifacts.OriginPolicy{Surfaces: []string{"https://preview.taawun.example"}, Connections: []string{"https://unverified-connection.example"}}},
		{name: "resources", policy: artifacts.OriginPolicy{Surfaces: []string{"https://preview.taawun.example"}, Resources: []string{"https://unverified-resource.example"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			harness := newConductorHarness(t)
			harness.origins.authorizeErr = fmt.Errorf("%s origin denied: %w", test.name, domains.ErrOriginNotVerified)
			request := validCompositionRequest()
			request.IdempotencyKey = "unverified-" + test.name
			request.RequestedOrigins = test.policy

			for attempt := 0; attempt < 2; attempt++ {
				result, err := harness.service.Compose(context.Background(), harness.owner, request)
				if result != nil || !errors.Is(err, ErrInvalidComposition) || !errors.Is(err, ErrPreviewOriginDenied) || harness.builder.buildCalls != 0 {
					t.Fatalf("attempt %d result=%+v builds=%d err=%v", attempt+1, result, harness.builder.buildCalls, err)
				}
			}
			var tracks int
			if err := harness.repository.db.QueryRow(`SELECT COUNT(*) FROM conductor_tracks`).Scan(&tracks); err != nil || tracks != 0 {
				t.Fatalf("denial persisted %d tracks: %v", tracks, err)
			}

			harness.origins.authorizeErr = nil
			retried, err := harness.service.Compose(context.Background(), harness.owner, request)
			if err != nil || !retried.Created || retried.Track.Status != TrackPreviewReady {
				t.Fatalf("retry after verification = %+v err=%v", retried, err)
			}
		})
	}
}

func TestInvalidComponentDocumentDoesNotConsumeTrackOrIdempotency(t *testing.T) {
	harness := newConductorHarness(t)
	request := validCompositionRequest()
	request.Components = []artifacts.ComponentInstance{
		{ID: artifacts.ModuleAnnouncements, Type: artifacts.ModuleAnnouncements, Data: json.RawMessage(`{"title":"Updates","summary":"Safe","actorId":7}`)},
		{ID: artifacts.ModuleRegistration, Type: artifacts.ModuleRegistration, Data: json.RawMessage(`{"title":"Register","summary":"Safe"}`)},
	}
	result, err := harness.service.Compose(context.Background(), harness.owner, request)
	var validation *artifacts.ComponentValidationError
	if result != nil || !errors.Is(err, ErrInvalidComposition) || !errors.As(err, &validation) || validation.Reason != "reserved_key" || harness.builder.buildCalls != 0 {
		t.Fatalf("invalid component result=%#v validation=%#v builds=%d err=%v", result, validation, harness.builder.buildCalls, err)
	}
	request.Components[0].Data = json.RawMessage(`{"title":"Updates","summary":"Safe","audience":"families"}`)
	corrected, err := harness.service.Compose(context.Background(), harness.owner, request)
	if err != nil || corrected == nil || !corrected.Created || corrected.Track.Status != TrackPreviewReady {
		t.Fatalf("corrected same-key composition = %#v err=%v", corrected, err)
	}
	if got := string(corrected.Track.BuildRequest.Components[0].Data); got != `{"audience":"families","summary":"Safe","title":"Updates"}` {
		t.Fatalf("canonical persisted document = %s", got)
	}
}

func TestInvalidComponentUnicodeDoesNotConsumeTrackOrIdempotency(t *testing.T) {
	for _, test := range []struct {
		name string
		data json.RawMessage
	}{
		{name: "root high surrogate", data: json.RawMessage(`{"title":"Unicode","summary":"\ud800"}`)},
		{name: "nested low surrogate", data: json.RawMessage(`{"title":"Unicode","summary":"Safe","details":{"note":"\udc00"}}`)},
	} {
		t.Run(test.name, func(t *testing.T) {
			harness := newConductorHarness(t)
			request := validCompositionRequest()
			request.IdempotencyKey = "unicode-scalar-retry"
			request.Components = []artifacts.ComponentInstance{
				{ID: artifacts.ModuleAnnouncements, Type: artifacts.ModuleAnnouncements, Data: test.data},
				{ID: artifacts.ModuleRegistration, Type: artifacts.ModuleRegistration, Data: json.RawMessage(`{"title":"Register","summary":"Safe"}`)},
			}

			result, err := harness.service.Compose(context.Background(), harness.owner, request)
			var validation *artifacts.ComponentValidationError
			if result != nil || !errors.Is(err, ErrInvalidComposition) || !errors.As(err, &validation) || validation.Reason != "invalid_unicode_scalar" || harness.origins.calls != 0 || harness.builder.buildCalls != 0 {
				t.Fatalf("invalid Unicode result=%#v validation=%#v origins=%d builds=%d err=%v", result, validation, harness.origins.calls, harness.builder.buildCalls, err)
			}
			var tracks int
			if err := harness.repository.db.QueryRow(`SELECT COUNT(*) FROM conductor_tracks`).Scan(&tracks); err != nil || tracks != 0 {
				t.Fatalf("invalid Unicode persisted %d tracks: %v", tracks, err)
			}

			request.Components[0].Data = json.RawMessage(`{"title":"Unicode","summary":"\ud83d\ude00","details":{"note":"🚀"}}`)
			corrected, err := harness.service.Compose(context.Background(), harness.owner, request)
			if err != nil || corrected == nil || !corrected.Created || corrected.Track.Status != TrackPreviewReady {
				t.Fatalf("corrected Unicode same-key composition = %#v err=%v", corrected, err)
			}
			if got := string(corrected.Track.BuildRequest.Components[0].Data); got != `{"details":{"note":"🚀"},"summary":"😀","title":"Unicode"}` {
				t.Fatalf("canonical persisted Unicode document = %s", got)
			}
		})
	}
}

func TestExplicitEmptyComponentsFailBeforeTrackCreation(t *testing.T) {
	harness := newConductorHarness(t)
	request := validCompositionRequest()
	request.Components = []artifacts.ComponentInstance{}
	result, err := harness.service.Compose(context.Background(), harness.owner, request)
	var validation *artifacts.ComponentValidationError
	if result != nil || !errors.As(err, &validation) || validation.Reason != "components_required" || harness.origins.calls != 0 || harness.builder.buildCalls != 0 {
		t.Fatalf("explicit empty components = result:%#v validation:%#v origins:%d builds:%d err:%v", result, validation, harness.origins.calls, harness.builder.buildCalls, err)
	}
}

func TestViewerCanInspectButCannotComposeComponentDocuments(t *testing.T) {
	harness := newConductorHarness(t)
	created, err := harness.service.Compose(context.Background(), harness.owner, validCompositionRequest())
	if err != nil {
		t.Fatalf("owner compose: %v", err)
	}
	viewer := &models.User{ID: 8}
	read, err := harness.service.GetTrack(context.Background(), viewer, created.Track.ID)
	if err != nil || read.ID != created.Track.ID || len(read.BuildRequest.Components) == 0 {
		t.Fatalf("viewer read = %#v err=%v", read, err)
	}
	request := validCompositionRequest()
	request.IdempotencyKey = "viewer-component-build"
	if result, err := harness.service.Compose(context.Background(), viewer, request); result != nil || !errors.Is(err, ErrWorkspaceForbidden) {
		t.Fatalf("viewer compose = %#v err=%v", result, err)
	}
}

func TestPreviewOriginRevocationRaceLeavesTrackResumable(t *testing.T) {
	harness := newConductorHarness(t)
	harness.origins.denyAtCall = 2
	request := validCompositionRequest()

	first, err := harness.service.Compose(context.Background(), harness.owner, request)
	if !errors.Is(err, ErrPreviewOriginDenied) || first == nil || first.Track.Status != TrackStaged || !first.Created || harness.builder.buildCalls != 0 || first.Track.BuildRequest != nil || first.Track.Artifact != nil {
		t.Fatalf("revocation race = %+v err=%v", first, err)
	}
	replayed, err := harness.service.Compose(context.Background(), harness.owner, request)
	if err != nil || replayed.Created || replayed.Track.ID != first.Track.ID || replayed.Track.Status != TrackPreviewReady {
		t.Fatalf("resumable replay = %+v err=%v", replayed, err)
	}
}

func TestPreviewOriginErrorClassificationPreservesOperationalFailures(t *testing.T) {
	tests := []struct {
		name      string
		input     error
		want      error
		clientErr bool
	}{
		{name: "unknown dependency", input: errors.New("database unavailable"), want: ErrDependencyUnavailable},
		{name: "canceled", input: context.Canceled, want: context.Canceled},
		{name: "deadline", input: context.DeadlineExceeded, want: context.DeadlineExceeded},
		{name: "workspace forbidden", input: domains.ErrForbidden, want: ErrWorkspaceForbidden},
		{name: "unverified origin", input: domains.ErrOriginNotVerified, want: ErrPreviewOriginDenied, clientErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := classifyPreviewOriginError(test.input)
			if !errors.Is(err, test.want) {
				t.Fatalf("classification error = %v, want %v", err, test.want)
			}
			if got := errors.Is(err, ErrInvalidComposition) || errors.Is(err, ErrPreviewOriginDenied); got != test.clientErr {
				t.Fatalf("client classification = %t, want %t: %v", got, test.clientErr, err)
			}
		})
	}
}

func TestDefaultPreviewOriginIsPreflightedAndReauthorized(t *testing.T) {
	harness := newConductorHarness(t)
	request := validCompositionRequest()
	result, err := harness.service.Compose(context.Background(), harness.owner, request)
	if err != nil || result.Track.Status != TrackPreviewReady || harness.origins.calls != 2 {
		t.Fatalf("default preview origin = result:%+v calls:%d err:%v", result, harness.origins.calls, err)
	}
	if result.Track.BuildRequest == nil || len(result.Track.BuildRequest.AllowedOrigins.Surfaces) != 1 || result.Track.BuildRequest.AllowedOrigins.Surfaces[0] != "https://preview.taawun.example" {
		t.Fatalf("authorized default origin = %+v", result.Track.BuildRequest)
	}
}

func TestPublicationRequiresCurrentlyVerifiedClaim(t *testing.T) {
	harness := newConductorHarness(t)
	result, err := harness.service.Compose(context.Background(), harness.owner, validCompositionRequest())
	if err != nil {
		t.Fatalf("compose preview: %v", err)
	}
	harness.publisher.claimStatus = domains.StatusPending
	if _, err := harness.service.RequestPublication(context.Background(), harness.owner, result.Track.ID, result.Track.Version, "claim-community"); !errors.Is(err, ErrPublicationClaimState) {
		t.Fatalf("pending claim request error = %v", err)
	}

	harness.publisher.claimStatus = domains.StatusVerified
	requested, err := harness.service.RequestPublication(context.Background(), harness.owner, result.Track.ID, result.Track.Version, "claim-community")
	if err != nil {
		t.Fatalf("request verified publication: %v", err)
	}
	harness.publisher.publishErr = domains.ErrOriginNotVerified
	activated, err := harness.service.ActivatePublication(context.Background(), harness.owner, result.Track.ID, requested.Version)
	if !errors.Is(err, ErrPublicationClaimState) || activated == nil || activated.Status != TrackPublicationRequested {
		t.Fatalf("revoked activation = track:%+v err:%v", activated, err)
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

func TestResumeAuthorizesWorkspaceAndCreatorBeforeVersion(t *testing.T) {
	harness := newConductorHarness(t)
	harness.builder.buildErr = errors.New("signer temporarily unavailable")
	result, err := harness.service.Compose(context.Background(), harness.owner, validCompositionRequest())
	if !errors.Is(err, ErrDependencyUnavailable) || result == nil || result.Track == nil {
		t.Fatalf("create resumable track: result=%+v err=%v", result, err)
	}
	correctVersion := result.Track.Version
	wrongVersion := correctVersion + 100
	for _, test := range []struct {
		name  string
		actor *models.User
	}{
		{name: "outsider", actor: &models.User{ID: 9}},
		{name: "different build member", actor: &models.User{ID: 10}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var messages []string
			for _, expectedVersion := range []int64{correctVersion, wrongVersion} {
				resumed, err := harness.service.Resume(context.Background(), test.actor, result.Track.ID, expectedVersion)
				if resumed != nil || !errors.Is(err, ErrWorkspaceForbidden) {
					t.Fatalf("version %d resume = track:%+v err:%v", expectedVersion, resumed, err)
				}
				messages = append(messages, err.Error())
			}
			if messages[0] != messages[1] {
				t.Fatalf("authorization response varied by expected version: %q != %q", messages[0], messages[1])
			}
		})
	}
	if resumed, err := harness.service.Resume(context.Background(), harness.owner, result.Track.ID, wrongVersion); resumed != nil || !errors.Is(err, ErrTrackVersionConflict) {
		t.Fatalf("creator stale resume = track:%+v err:%v", resumed, err)
	}
	if harness.builder.buildCalls != 1 {
		t.Fatalf("unauthorized or stale resume advanced build %d times", harness.builder.buildCalls)
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

func (*fakeWorkspaceAuthorizer) AuthorizeWorkspaceCapability(actor *models.User, workspaceID int, capability models.WorkspaceCapability) (*models.Workspace, error) {
	if actor == nil || workspaceID != 42 || (actor.ID != 7 && !(actor.ID == 8 && capability == models.WorkspaceCapabilityView) && !(actor.ID == 10 && capability == models.WorkspaceCapabilityBuild)) {
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
	modules := make([]artifacts.ModuleDescriptor, 0, len(request.Modules))
	components := make([]artifacts.ComponentManifest, 0, len(request.Components))
	for _, moduleID := range request.Modules {
		module, _ := artifacts.GetModule(moduleID)
		modules = append(modules, module)
	}
	for _, component := range request.Components {
		documentDigest := sha256.Sum256(component.Data)
		components = append(components, artifacts.ComponentManifest{ID: component.ID, Type: component.Type, Data: component.Data,
			DocumentPath: "components/" + component.ID + ".json", DocumentSHA256: hex.EncodeToString(documentDigest[:])})
	}
	componentAggregate, _ := json.Marshal(request.Components)
	aggregateDigest := sha256.Sum256(componentAggregate)
	files := []artifacts.FileDigest{{Path: "components.json", SHA256: hex.EncodeToString(aggregateDigest[:]), Bytes: len(componentAggregate)}}
	for _, component := range components {
		files = append(files, artifacts.FileDigest{Path: component.DocumentPath, SHA256: component.DocumentSHA256, Bytes: len(component.Data)})
	}
	manifest := artifacts.Manifest{
		ContractVersion: artifacts.ManifestContractVersion, ArtifactID: artifactID, ContentHash: contentHash, WorkspaceID: request.WorkspaceID,
		Template: artifacts.TemplateIdentity{ID: request.TemplateID, Version: artifacts.TemplateCommunityIftarVersion}, Modules: modules, Components: components, Files: files,
		Authorization: artifacts.BundleAuthorization{
			Subject:        artifacts.BundleSubject{ID: request.Subject.ID, UserID: request.Subject.UserID, WorkspaceID: request.WorkspaceID},
			AllowedOrigins: request.AllowedOrigins, ExpiresAt: request.ExpiresAt, Lifecycle: request.Lifecycle,
		},
		Signature: artifacts.BundleSignature{Algorithm: "Ed25519", Value: "verified-test-signature"},
	}
	return artifacts.BuildResult{ArtifactID: artifactID, ContentHash: contentHash, Manifest: manifest}, nil
}

type fakeOriginAuthority struct {
	calls        int
	authorizeErr error
	denyAtCall   int
}

func (a *fakeOriginAuthority) AuthorizeOriginsForLifecycle(_ context.Context, _ *models.User, _ *models.Workspace, lifecycle artifacts.BundleLifecycle, requested artifacts.OriginPolicy) (artifacts.OriginPolicy, error) {
	a.calls++
	if lifecycle != artifacts.BundleLifecyclePreview {
		return artifacts.OriginPolicy{}, domains.ErrOriginNotVerified
	}
	if a.authorizeErr != nil {
		return artifacts.OriginPolicy{}, a.authorizeErr
	}
	if a.denyAtCall > 0 && a.calls == a.denyAtCall {
		return artifacts.OriginPolicy{}, fmt.Errorf("revoked between checks: %w", domains.ErrOriginNotVerified)
	}
	return requested, nil
}

type fakePublicationService struct {
	publishCalls int
	history      []domains.Publication
	claimStatus  domains.Status
	publishErr   error
}

func (p *fakePublicationService) Inspect(_ context.Context, _ *models.User, workspaceID int, claimID string) (domains.Claim, error) {
	status := p.claimStatus
	if status == "" {
		status = domains.StatusVerified
	}
	expiresAt := time.Now().UTC().Add(time.Hour)
	return domains.Claim{ID: claimID, WorkspaceID: workspaceID, Status: status, VerificationExpiresAt: &expiresAt}, nil
}

func (p *fakePublicationService) Publish(_ context.Context, _ *models.User, workspaceID int, claimID, contentHash string) (domains.Publication, error) {
	p.publishCalls++
	if p.publishErr != nil {
		return domains.Publication{}, p.publishErr
	}
	publication := domains.Publication{ID: "publication-1", WorkspaceID: workspaceID, ClaimID: claimID, Origin: "https://community.example", Host: "community.example", ContentHash: contentHash, ArtifactID: "artifact_signed_1", Active: true, ActivatedAt: time.Now().UTC()}
	p.history = []domains.Publication{publication}
	return publication, nil
}

func (p *fakePublicationService) PublicationHistory(context.Context, *models.User, int, string) ([]domains.Publication, error) {
	return append([]domains.Publication(nil), p.history...), nil
}
