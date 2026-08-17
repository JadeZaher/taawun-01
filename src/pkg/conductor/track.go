package conductor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"taawun/pkg/artifacts"
	"taawun/pkg/domains"
	"taawun/pkg/ethics"
	"taawun/pkg/models"
)

var (
	ErrInvalidComposition     = errors.New("invalid Conductor composition")
	ErrTrackNotFound          = errors.New("Conductor track not found")
	ErrTrackVersionConflict   = errors.New("Conductor track version conflict")
	ErrTrackTransition        = errors.New("invalid Conductor track transition")
	ErrIdempotencyConflict    = errors.New("Conductor idempotency key conflict")
	ErrWorkspaceForbidden     = errors.New("Conductor workspace authorization denied")
	ErrDependencyUnavailable  = errors.New("Conductor dependency unavailable")
	ErrLegacyWorkflowDisabled = errors.New("legacy Docker/LTAP Conductor workflow is disabled")
)

type TrackStatus string

const (
	TrackDraft                TrackStatus = "DRAFT"
	TrackStaged               TrackStatus = "STAGED"
	TrackValidated            TrackStatus = "VALIDATED"
	TrackComplianceAudited    TrackStatus = "COMPLIANCE_AUDITED"
	TrackArtifactSigned       TrackStatus = "ARTIFACT_SIGNED"
	TrackPreviewReady         TrackStatus = "PREVIEW_READY"
	TrackPublicationRequested TrackStatus = "PUBLICATION_REQUESTED"
	TrackPublished            TrackStatus = "PUBLISHED"
	TrackFailed               TrackStatus = "FAILED"
	TrackCancelled            TrackStatus = "CANCELLED"
)

// TrackStepState is retained as a non-deployment compatibility alias.
type TrackStepState = TrackStatus

type ComplianceDisposition string

const (
	CompliancePass   ComplianceDisposition = "PASS"
	ComplianceReject ComplianceDisposition = "REJECT"
)

type ComplianceReviewStatus string

const (
	ReviewPendingQualified ComplianceReviewStatus = "pending-qualified-review"
	ReviewCompleted        ComplianceReviewStatus = "reviewed"
)

type CompositionRequest struct {
	IdempotencyKey   string                 `json:"idempotencyKey"`
	WorkspaceID      int                    `json:"workspaceId"`
	AppName          string                 `json:"appName"`
	OrganizationName string                 `json:"organizationName"`
	City             string                 `json:"city"`
	Madhhab          ethics.Madhhab         `json:"madhhab"`
	TemplateID       string                 `json:"templateId"`
	Theme            artifacts.ThemeRequest `json:"theme"`
	Modules          []string               `json:"modules"`
	RequestedOrigins artifacts.OriginPolicy `json:"requestedOrigins"`
	TTLHours         int                    `json:"ttlHours"`
}

// TrackRequest is the safe declarative compatibility name used by older routing code.
type TrackRequest = CompositionRequest

type ComplianceEvidence struct {
	ReferenceID  string                    `json:"referenceId"`
	Madhhab      ethics.Madhhab            `json:"madhhab"`
	Disposition  ComplianceDisposition     `json:"disposition"`
	ReviewStatus ComplianceReviewStatus    `json:"reviewStatus"`
	Audit        *ethics.ComplianceResult  `json:"audit"`
	References   []ethics.ComplianceRecord `json:"references"`
	Disclaimer   string                    `json:"disclaimer"`
}

type ArtifactReference struct {
	ArtifactID  string             `json:"artifactId"`
	ContentHash string             `json:"contentHash"`
	Manifest    artifacts.Manifest `json:"manifest"`
}

// PreviewMetadata is authorization metadata, not proof of public deployment.
type PreviewMetadata struct {
	ArtifactID             string                   `json:"artifactId"`
	ContentHash            string                   `json:"contentHash"`
	WorkspaceID            int                      `json:"workspaceId"`
	Subject                artifacts.SubjectBinding `json:"subject"`
	AllowedOrigins         artifacts.OriginPolicy   `json:"allowedOrigins"`
	AuthorizationExpiresAt time.Time                `json:"authorizationExpiresAt"`
	AuthenticationRequired bool                     `json:"authenticationRequired"`
}

type Track struct {
	ID           string                  `json:"id"`
	WorkspaceID  int                     `json:"workspaceId"`
	Request      CompositionRequest      `json:"request"`
	BuildRequest *artifacts.BuildRequest `json:"buildRequest,omitempty"`
	Status       TrackStatus             `json:"status"`
	Version      int64                   `json:"version"`
	Compliance   *ComplianceEvidence     `json:"compliance,omitempty"`
	Artifact     *ArtifactReference      `json:"artifact,omitempty"`
	Preview      *PreviewMetadata        `json:"preview,omitempty"`
	ClaimID      string                  `json:"claimId,omitempty"`
	Publication  *domains.Publication    `json:"publication,omitempty"`
	FailureCode  string                  `json:"failureCode,omitempty"`
	CreatedBy    int                     `json:"createdBy"`
	CreatedAt    time.Time               `json:"createdAt"`
	UpdatedAt    time.Time               `json:"updatedAt"`
}

type TrackResult = Track

type TrackEvent struct {
	Sequence     int64           `json:"sequence"`
	ID           string          `json:"id"`
	TrackID      string          `json:"trackId"`
	TrackVersion int64           `json:"trackVersion"`
	FromStatus   TrackStatus     `json:"fromStatus,omitempty"`
	ToStatus     TrackStatus     `json:"toStatus"`
	Type         string          `json:"type"`
	ActorID      int             `json:"actorId"`
	Detail       json.RawMessage `json:"detail"`
	PreviousHash string          `json:"previousHash,omitempty"`
	Hash         string          `json:"hash"`
	CreatedAt    time.Time       `json:"createdAt"`
}

type ComposeResult struct {
	Track   *Track `json:"track"`
	Created bool   `json:"created"`
}

type WorkspaceAuthorizer interface {
	AuthorizeWorkspaceCapability(*models.User, int, models.WorkspaceCapability) (*models.Workspace, error)
}

type SubjectResolver interface {
	ResolveArtifactSubject(context.Context, *models.User, *models.Workspace) (artifacts.SubjectBinding, error)
}

type CompositionValidator interface {
	ValidateComposition(context.Context, artifacts.BuildRequest) error
}

type ComplianceAuditor interface {
	AuditComposition(context.Context, artifacts.BuildRequest) (ComplianceEvidence, error)
}

type SignedArtifactBuilder interface {
	Build(context.Context, artifacts.BuildRequest) (artifacts.BuildResult, error)
	ProductionReady() bool
}

type StagingOriginAuthority interface {
	AuthorizeOriginsForLifecycle(context.Context, *models.User, *models.Workspace, artifacts.BundleLifecycle, artifacts.OriginPolicy) (artifacts.OriginPolicy, error)
}

type DomainPublicationService interface {
	Publish(context.Context, *models.User, int, string, string) (domains.Publication, error)
	PublicationHistory(context.Context, *models.User, int, string) ([]domains.Publication, error)
}

type ActorSubjectResolver struct{}

func (ActorSubjectResolver) ResolveArtifactSubject(_ context.Context, actor *models.User, workspace *models.Workspace) (artifacts.SubjectBinding, error) {
	if actor == nil || actor.ID <= 0 || workspace == nil || workspace.ID <= 0 {
		return artifacts.SubjectBinding{}, ErrWorkspaceForbidden
	}
	return artifacts.SubjectBinding{ID: "taawun:user:" + strconv.Itoa(actor.ID), UserID: actor.ID}, nil
}

type CuratedCompositionValidator struct{}

func (CuratedCompositionValidator) ValidateComposition(_ context.Context, request artifacts.BuildRequest) error {
	return artifacts.ValidateRequest(request)
}

type Service struct {
	repository   *Repository
	workspaces   WorkspaceAuthorizer
	subjects     SubjectResolver
	validator    CompositionValidator
	compliance   ComplianceAuditor
	builder      SignedArtifactBuilder
	origins      StagingOriginAuthority
	publications DomainPublicationService
	now          func() time.Time
}

func NewService(repository *Repository, workspaces WorkspaceAuthorizer, subjects SubjectResolver, validator CompositionValidator, compliance ComplianceAuditor, builder SignedArtifactBuilder, origins StagingOriginAuthority, publications DomainPublicationService) (*Service, error) {
	if repository == nil || workspaces == nil || subjects == nil || validator == nil || compliance == nil || builder == nil || !builder.ProductionReady() || origins == nil || publications == nil {
		return nil, ErrInvalidComposition
	}
	return &Service{repository: repository, workspaces: workspaces, subjects: subjects, validator: validator, compliance: compliance, builder: builder, origins: origins, publications: publications, now: time.Now}, nil
}

func (s *Service) Compose(ctx context.Context, actor *models.User, request CompositionRequest) (*ComposeResult, error) {
	workspace, err := s.authorize(actor, request.WorkspaceID, models.WorkspaceCapabilityBuild)
	if err != nil {
		return nil, err
	}
	if err := validateCompositionRequest(request); err != nil {
		return nil, err
	}
	request = normalizeCompositionRequest(request)
	requestHash, err := compositionHash(request, actor.ID)
	if err != nil {
		return nil, err
	}
	track, created, err := s.repository.createDraft(ctx, request, requestHash, actor.ID, s.now().UTC().Truncate(time.Millisecond))
	if err != nil {
		return nil, err
	}
	if track.CreatedBy != actor.ID {
		return nil, ErrIdempotencyConflict
	}
	if !created && !resumableStatus(track.Status) {
		return &ComposeResult{Track: track, Created: false}, nil
	}
	track, err = s.advance(ctx, actor, workspace, track)
	return &ComposeResult{Track: track, Created: created}, err
}

func (s *Service) Resume(ctx context.Context, actor *models.User, trackID string, expectedVersion int64) (*Track, error) {
	track, err := s.repository.getTrack(ctx, trackID)
	if err != nil {
		return nil, err
	}
	if track.Version != expectedVersion {
		return nil, ErrTrackVersionConflict
	}
	if actor == nil || actor.ID != track.CreatedBy {
		return nil, ErrWorkspaceForbidden
	}
	workspace, err := s.authorize(actor, track.WorkspaceID, models.WorkspaceCapabilityBuild)
	if err != nil {
		return nil, err
	}
	if !resumableStatus(track.Status) {
		return nil, ErrTrackTransition
	}
	return s.advance(ctx, actor, workspace, track)
}

func (s *Service) advance(ctx context.Context, actor *models.User, workspace *models.Workspace, track *Track) (*Track, error) {
	for {
		if err := contextError(ctx); err != nil {
			return track, err
		}
		switch track.Status {
		case TrackDraft:
			updated := cloneTrack(track)
			var err error
			track, err = s.repository.transition(ctx, track, updated, TrackStaged, "COMPOSITION_STAGED", actor.ID, map[string]any{"templateId": track.Request.TemplateID}, s.now())
			if err != nil {
				return track, err
			}
		case TrackStaged:
			subject, err := s.subjects.ResolveArtifactSubject(ctx, actor, workspace)
			if err != nil || subject.UserID != actor.ID || subject.ID == "" {
				return s.fail(ctx, track, actor.ID, "subject_resolution_failed", errors.Join(ErrWorkspaceForbidden, err))
			}
			authorizedOrigins, err := s.origins.AuthorizeOriginsForLifecycle(ctx, actor, workspace, artifacts.BundleLifecyclePreview, cloneOrigins(track.Request.RequestedOrigins))
			if err != nil {
				return s.fail(ctx, track, actor.ID, "preview_origin_denied", err)
			}
			buildRequest := artifacts.BuildRequest{
				WorkspaceID: track.WorkspaceID, AppName: track.Request.AppName, OrganizationName: track.Request.OrganizationName,
				City: track.Request.City, Madhhab: track.Request.Madhhab, TemplateID: track.Request.TemplateID,
				Theme: track.Request.Theme, Modules: append([]string(nil), track.Request.Modules...), AllowedOrigins: cloneOrigins(authorizedOrigins),
				Subject: subject, ExpiresAt: s.now().UTC().Add(time.Duration(track.Request.TTLHours) * time.Hour), Lifecycle: artifacts.BundleLifecyclePreview,
			}
			if err := s.validator.ValidateComposition(ctx, buildRequest); err != nil {
				return s.fail(ctx, track, actor.ID, "composition_validation_failed", err)
			}
			updated := cloneTrack(track)
			updated.BuildRequest = &buildRequest
			track, err = s.repository.transition(ctx, track, updated, TrackValidated, "COMPOSITION_VALIDATED", actor.ID, map[string]any{"lifecycle": artifacts.BundleLifecyclePreview, "authorizedOrigins": authorizedOrigins}, s.now())
			if err != nil {
				return track, err
			}
		case TrackValidated:
			if track.BuildRequest == nil {
				return s.fail(ctx, track, actor.ID, "validated_request_missing", ErrInvalidComposition)
			}
			evidence, err := s.compliance.AuditComposition(ctx, *track.BuildRequest)
			if err != nil {
				return track, fmt.Errorf("%w: compliance audit: %v", ErrDependencyUnavailable, err)
			}
			if err := validateComplianceEvidence(evidence, track.BuildRequest.Madhhab); err != nil {
				return s.fail(ctx, track, actor.ID, "compliance_evidence_invalid", err)
			}
			if evidence.Disposition != CompliancePass {
				updated := cloneTrack(track)
				updated.Compliance = &evidence
				return s.failUpdated(ctx, track, updated, actor.ID, "compliance_rejected", ErrInvalidComposition)
			}
			updated := cloneTrack(track)
			updated.Compliance = &evidence
			track, err = s.repository.transition(ctx, track, updated, TrackComplianceAudited, "COMPLIANCE_AUDITED", actor.ID, map[string]any{"referenceId": evidence.ReferenceID, "reviewStatus": evidence.ReviewStatus, "disposition": evidence.Disposition}, s.now())
			if err != nil {
				return track, err
			}
		case TrackComplianceAudited:
			if track.BuildRequest == nil {
				return s.fail(ctx, track, actor.ID, "audited_request_missing", ErrInvalidComposition)
			}
			built, err := s.builder.Build(ctx, *track.BuildRequest)
			if err != nil {
				return track, fmt.Errorf("%w: artifact build: %v", ErrDependencyUnavailable, err)
			}
			artifact := ArtifactReference{ArtifactID: built.ArtifactID, ContentHash: built.ContentHash, Manifest: built.Manifest}
			if err := validateSignedArtifact(artifact, *track.BuildRequest); err != nil {
				return s.fail(ctx, track, actor.ID, "signed_artifact_invalid", err)
			}
			updated := cloneTrack(track)
			updated.Artifact = &artifact
			track, err = s.repository.transition(ctx, track, updated, TrackArtifactSigned, "ARTIFACT_SIGNED", actor.ID, map[string]any{"artifactId": artifact.ArtifactID, "contentHash": artifact.ContentHash}, s.now())
			if err != nil {
				return track, err
			}
		case TrackArtifactSigned:
			if track.Artifact == nil || track.BuildRequest == nil {
				return s.fail(ctx, track, actor.ID, "artifact_metadata_missing", ErrInvalidComposition)
			}
			preview := PreviewMetadata{
				ArtifactID: track.Artifact.ArtifactID, ContentHash: track.Artifact.ContentHash, WorkspaceID: track.WorkspaceID,
				Subject: track.BuildRequest.Subject, AllowedOrigins: cloneOrigins(track.BuildRequest.AllowedOrigins),
				AuthorizationExpiresAt: track.Artifact.Manifest.Authorization.ExpiresAt, AuthenticationRequired: true,
			}
			updated := cloneTrack(track)
			updated.Preview = &preview
			next, err := s.repository.transition(ctx, track, updated, TrackPreviewReady, "PREVIEW_READY", actor.ID, map[string]any{"contentHash": preview.ContentHash, "authenticationRequired": true}, s.now())
			if err != nil {
				return track, err
			}
			track = next
			return track, nil
		default:
			return track, nil
		}
	}
}

func (s *Service) RequestPublication(ctx context.Context, actor *models.User, trackID string, expectedVersion int64, claimID string) (*Track, error) {
	track, err := s.repository.getTrack(ctx, trackID)
	if err != nil {
		return nil, err
	}
	if _, err := s.authorize(actor, track.WorkspaceID, models.WorkspaceCapabilityPublish); err != nil {
		return nil, err
	}
	if track.Version != expectedVersion {
		return nil, ErrTrackVersionConflict
	}
	if !validIdentifier(claimID) {
		return nil, ErrInvalidComposition
	}
	if track.Status == TrackPublicationRequested && track.ClaimID == claimID {
		return track, nil
	}
	if track.Status != TrackPreviewReady || track.Artifact == nil {
		return nil, ErrTrackTransition
	}
	updated := cloneTrack(track)
	updated.ClaimID = claimID
	return s.repository.transition(ctx, track, updated, TrackPublicationRequested, "PUBLICATION_REQUESTED", actor.ID, map[string]any{"claimId": claimID, "contentHash": track.Artifact.ContentHash}, s.now())
}

func (s *Service) ActivatePublication(ctx context.Context, actor *models.User, trackID string, expectedVersion int64) (*Track, error) {
	track, err := s.repository.getTrack(ctx, trackID)
	if err != nil {
		return nil, err
	}
	if _, err := s.authorize(actor, track.WorkspaceID, models.WorkspaceCapabilityPublish); err != nil {
		return nil, err
	}
	if track.Version != expectedVersion {
		return nil, ErrTrackVersionConflict
	}
	if track.Status != TrackPublicationRequested || track.Artifact == nil || !validIdentifier(track.ClaimID) {
		return nil, ErrTrackTransition
	}
	reserved := cloneTrack(track)
	track, err = s.repository.transition(ctx, track, reserved, TrackPublicationRequested, "PUBLICATION_ACTIVATION_STARTED", actor.ID, map[string]any{"claimId": track.ClaimID}, s.now())
	if err != nil {
		return nil, err
	}
	publication, err := s.recoverOrPublish(ctx, actor, track)
	if err != nil {
		updated := cloneTrack(track)
		blocked, transitionErr := s.repository.transition(ctx, track, updated, TrackPublicationRequested, "PUBLICATION_ACTIVATION_BLOCKED", actor.ID, map[string]any{"code": "domain_publication_unavailable"}, s.now())
		if transitionErr != nil {
			return track, errors.Join(err, transitionErr)
		}
		return blocked, fmt.Errorf("%w: %v", ErrDependencyUnavailable, err)
	}
	if publication.WorkspaceID != track.WorkspaceID || publication.ClaimID != track.ClaimID || publication.ContentHash != track.Artifact.ContentHash || publication.ArtifactID != track.Artifact.ArtifactID || !publication.Active {
		return s.fail(ctx, track, actor.ID, "publication_invariant_failed", ErrInvalidComposition)
	}
	updated := cloneTrack(track)
	updated.Publication = &publication
	return s.repository.transition(ctx, track, updated, TrackPublished, "PUBLICATION_ACTIVATED", actor.ID, map[string]any{"publicationId": publication.ID, "origin": publication.Origin}, s.now())
}

func (s *Service) recoverOrPublish(ctx context.Context, actor *models.User, track *Track) (domains.Publication, error) {
	history, err := s.publications.PublicationHistory(ctx, actor, track.WorkspaceID, track.ClaimID)
	if err == nil {
		for _, publication := range history {
			if publication.Active && publication.ContentHash == track.Artifact.ContentHash && publication.ArtifactID == track.Artifact.ArtifactID {
				return publication, nil
			}
		}
	}
	return s.publications.Publish(ctx, actor, track.WorkspaceID, track.ClaimID, track.Artifact.ContentHash)
}

func (s *Service) Cancel(ctx context.Context, actor *models.User, trackID string, expectedVersion int64, reason string) (*Track, error) {
	track, err := s.repository.getTrack(ctx, trackID)
	if err != nil {
		return nil, err
	}
	if _, err := s.authorize(actor, track.WorkspaceID, models.WorkspaceCapabilityBuild); err != nil {
		return nil, err
	}
	if track.Version != expectedVersion {
		return nil, ErrTrackVersionConflict
	}
	reason = strings.TrimSpace(reason)
	if !validText(reason, 500) || !cancellableStatus(track.Status) {
		return nil, ErrTrackTransition
	}
	updated := cloneTrack(track)
	return s.repository.transition(ctx, track, updated, TrackCancelled, "TRACK_CANCELLED", actor.ID, map[string]any{"reason": reason}, s.now())
}

func (s *Service) GetTrack(ctx context.Context, actor *models.User, trackID string) (*Track, error) {
	track, err := s.repository.getTrack(ctx, trackID)
	if err != nil {
		return nil, err
	}
	if _, err := s.authorize(actor, track.WorkspaceID, models.WorkspaceCapabilityView); err != nil {
		return nil, err
	}
	return track, nil
}

func (s *Service) Events(ctx context.Context, actor *models.User, trackID string) ([]TrackEvent, error) {
	if _, err := s.GetTrack(ctx, actor, trackID); err != nil {
		return nil, err
	}
	return s.repository.events(ctx, trackID)
}

func (s *Service) fail(ctx context.Context, track *Track, actorID int, code string, cause error) (*Track, error) {
	return s.failUpdated(ctx, track, cloneTrack(track), actorID, code, cause)
}

func (s *Service) failUpdated(ctx context.Context, track, updated *Track, actorID int, code string, cause error) (*Track, error) {
	updated.FailureCode = code
	failed, err := s.repository.transition(ctx, track, updated, TrackFailed, "TRACK_FAILED", actorID, map[string]any{"code": code}, s.now())
	if err != nil {
		return track, errors.Join(cause, err)
	}
	return failed, cause
}

func (s *Service) authorize(actor *models.User, workspaceID int, capability models.WorkspaceCapability) (*models.Workspace, error) {
	if actor == nil || actor.ID <= 0 || workspaceID <= 0 {
		return nil, ErrWorkspaceForbidden
	}
	workspace, err := s.workspaces.AuthorizeWorkspaceCapability(actor, workspaceID, capability)
	if err != nil || workspace == nil || workspace.ID != workspaceID || workspace.Status != models.WorkspaceStatusActive {
		return nil, ErrWorkspaceForbidden
	}
	return workspace, nil
}

func validateCompositionRequest(request CompositionRequest) error {
	if request.WorkspaceID <= 0 || !validIdentifier(request.IdempotencyKey) || request.TTLHours < 1 || request.TTLHours > 90*24 {
		return ErrInvalidComposition
	}
	return nil
}

func normalizeCompositionRequest(request CompositionRequest) CompositionRequest {
	request.Modules = append([]string(nil), request.Modules...)
	sort.Strings(request.Modules)
	request.RequestedOrigins = cloneOrigins(request.RequestedOrigins)
	return request
}

func compositionHash(request CompositionRequest, actorID int) (string, error) {
	canonical, err := json.Marshal(struct {
		ActorID int                `json:"actorId"`
		Request CompositionRequest `json:"request"`
	}{actorID, request})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}

func validateComplianceEvidence(evidence ComplianceEvidence, madhhab ethics.Madhhab) error {
	if !validIdentifier(evidence.ReferenceID) || evidence.Madhhab != madhhab || evidence.Audit == nil || evidence.Disclaimer == "" {
		return ErrInvalidComposition
	}
	if evidence.Disposition != CompliancePass && evidence.Disposition != ComplianceReject {
		return ErrInvalidComposition
	}
	if evidence.ReviewStatus != ReviewPendingQualified && evidence.ReviewStatus != ReviewCompleted {
		return ErrInvalidComposition
	}
	if evidence.Disposition == CompliancePass && !evidence.Audit.Passed || evidence.Disposition == ComplianceReject && evidence.Audit.Passed {
		return ErrInvalidComposition
	}
	return nil
}

func validateSignedArtifact(artifact ArtifactReference, request artifacts.BuildRequest) error {
	if !validIdentifier(artifact.ArtifactID) || len(artifact.ContentHash) != sha256.Size*2 {
		return ErrInvalidComposition
	}
	if _, err := hex.DecodeString(artifact.ContentHash); err != nil {
		return ErrInvalidComposition
	}
	manifest := artifact.Manifest
	if manifest.ArtifactID != artifact.ArtifactID || manifest.ContentHash != artifact.ContentHash || manifest.WorkspaceID != request.WorkspaceID || manifest.Authorization.Subject.WorkspaceID != request.WorkspaceID || manifest.Authorization.Subject.UserID != request.Subject.UserID || manifest.Authorization.Subject.ID != request.Subject.ID || manifest.Authorization.Lifecycle != artifacts.BundleLifecyclePreview || manifest.Signature.Algorithm != "Ed25519" || manifest.Signature.Value == "" || !reflect.DeepEqual(manifest.Authorization.AllowedOrigins, request.AllowedOrigins) {
		return ErrInvalidComposition
	}
	return nil
}

func cloneTrack(track *Track) *Track {
	if track == nil {
		return nil
	}
	copy := *track
	copy.Request = normalizeCompositionRequest(track.Request)
	if track.BuildRequest != nil {
		build := *track.BuildRequest
		build.Modules = append([]string(nil), track.BuildRequest.Modules...)
		build.AllowedOrigins = cloneOrigins(track.BuildRequest.AllowedOrigins)
		copy.BuildRequest = &build
	}
	if track.Compliance != nil {
		compliance := *track.Compliance
		compliance.References = append([]ethics.ComplianceRecord(nil), track.Compliance.References...)
		copy.Compliance = &compliance
	}
	if track.Artifact != nil {
		artifact := *track.Artifact
		copy.Artifact = &artifact
	}
	if track.Preview != nil {
		preview := *track.Preview
		preview.AllowedOrigins = cloneOrigins(track.Preview.AllowedOrigins)
		copy.Preview = &preview
	}
	if track.Publication != nil {
		publication := *track.Publication
		copy.Publication = &publication
	}
	return &copy
}

func cloneOrigins(policy artifacts.OriginPolicy) artifacts.OriginPolicy {
	return artifacts.OriginPolicy{
		Surfaces: append([]string(nil), policy.Surfaces...), Embedders: append([]string(nil), policy.Embedders...),
		Connections: append([]string(nil), policy.Connections...), Resources: append([]string(nil), policy.Resources...),
	}
}

func resumableStatus(status TrackStatus) bool {
	return status == TrackDraft || status == TrackStaged || status == TrackValidated || status == TrackComplianceAudited || status == TrackArtifactSigned
}

func cancellableStatus(status TrackStatus) bool {
	return resumableStatus(status) || status == TrackPreviewReady || status == TrackPublicationRequested
}

func allowedTransition(from, to TrackStatus) bool {
	if from == to {
		return from == TrackPublicationRequested
	}
	if to == TrackFailed {
		return resumableStatus(from) || from == TrackPublicationRequested
	}
	if to == TrackCancelled {
		return cancellableStatus(from)
	}
	switch from {
	case TrackDraft:
		return to == TrackStaged
	case TrackStaged:
		return to == TrackValidated
	case TrackValidated:
		return to == TrackComplianceAudited
	case TrackComplianceAudited:
		return to == TrackArtifactSigned
	case TrackArtifactSigned:
		return to == TrackPreviewReady
	case TrackPreviewReady:
		return to == TrackPublicationRequested
	case TrackPublicationRequested:
		return to == TrackPublished
	default:
		return false
	}
}

func validIdentifier(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || strings.ContainsRune("._:-", character) {
			continue
		}
		return false
	}
	return true
}

func validText(value string, maximum int) bool {
	return len(value) > 0 && len(value) <= maximum && !strings.ContainsAny(value, "\x00\r\n")
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return errors.New("Conductor context is required")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// ConductorTrack keeps old main wiring compilable while refusing unsafe execution.
type ConductorTrack struct{}

func NewConductorTrack(_, _ any) *ConductorTrack { return &ConductorTrack{} }

func (*ConductorTrack) ExecuteTrack(context.Context, *TrackRequest) (*TrackResult, error) {
	return nil, ErrLegacyWorkflowDisabled
}

func (*ConductorTrack) GetTrackResult(string) (*TrackResult, bool) { return nil, false }
