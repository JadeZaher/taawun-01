package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"

	"taawun/pkg/artifacts"
	"taawun/pkg/conductor"
	"taawun/pkg/domains"
	"taawun/pkg/models"
)

func TestCompositionPreviewUsesAuthenticatedPrincipalAndServerPreviewOrigin(t *testing.T) {
	service := &compositionServiceStub{track: previewTrack()}
	handler, err := NewCompositionHTTPHandler(service, artifactReaderStub{open: verifiedBuildResult(service.track)}, CurrentUser, []string{"http://localhost:8080"})
	if err != nil {
		t.Fatalf("NewCompositionHTTPHandler() error = %v", err)
	}
	body := `{"workspaceId":7,"appName":"Community Iftar","organizationName":"Northside Mosque","city":"Denver","madhhab":"hanafi","templateId":"community-iftar","theme":{"accentColor":"#166534"},"modules":["iftar-registration"]}`
	request := httptest.NewRequest(http.MethodPost, "/api/artifacts/preview", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(WithCurrentUser(request.Context(), &models.User{ID: 7}))
	response := httptest.NewRecorder()

	handler.Preview(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("preview status = %d body=%s", response.Code, response.Body.String())
	}
	if service.actor == nil || service.actor.ID != 7 || service.request.WorkspaceID != 7 || service.request.TTLHours != 24 {
		t.Fatalf("trusted composition call = actor=%+v request=%+v", service.actor, service.request)
	}
	if len(service.request.RequestedOrigins.Surfaces) != 1 || service.request.RequestedOrigins.Surfaces[0] != "http://localhost:8080" {
		t.Fatalf("preview origins = %#v", service.request.RequestedOrigins)
	}
	var payload struct {
		Manifest     artifacts.Manifest `json:"manifest"`
		Preview      previewURLs        `json:"preview"`
		Verification struct {
			Verified           bool   `json:"verified"`
			ContentHash        string `json:"contentHash"`
			WorkspaceID        int    `json:"workspaceId"`
			SignatureAlgorithm string `json:"signatureAlgorithm"`
			SignerKeyID        string `json:"signerKeyId"`
			SignatureValue     string `json:"signatureValue"`
			ManifestDigest     string `json:"manifestDigest"`
			ManifestJSON       string `json:"manifestJson"`
		} `json:"verification"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode preview response: %v", err)
	}
	if payload.Manifest.ContentHash != service.track.Artifact.ContentHash || payload.Preview.DocumentURL != "/api/conductor/tracks/track-1/preview/files/index.html" {
		t.Fatalf("preview response = %+v", payload)
	}
	if !payload.Verification.Verified || payload.Verification.ContentHash != payload.Manifest.ContentHash || payload.Verification.WorkspaceID != 7 || payload.Verification.SignatureAlgorithm != "Ed25519" || payload.Verification.SignerKeyID != "test-key-1" || payload.Verification.SignatureValue != "signed-test-value" {
		t.Fatalf("preview verification = %+v", payload.Verification)
	}
	digest := sha256.Sum256([]byte(payload.Verification.ManifestJSON))
	if payload.Verification.ManifestDigest != hex.EncodeToString(digest[:]) {
		t.Fatalf("manifest digest = %q, want digest of exact payload", payload.Verification.ManifestDigest)
	}
	var attested artifacts.Manifest
	if err := json.Unmarshal([]byte(payload.Verification.ManifestJSON), &attested); err != nil || !reflect.DeepEqual(attested, payload.Manifest) {
		t.Fatalf("attested manifest does not match rendered manifest: error=%v attested=%+v rendered=%+v", err, attested, payload.Manifest)
	}
	exactManifestJSON, err := json.Marshal(payload.Manifest)
	if err != nil || payload.Verification.ManifestJSON != string(exactManifestJSON) {
		t.Fatalf("manifest JSON is not the exact canonical serialization: error=%v", err)
	}
}

func TestCompositionPreviewRejectsUnverifiedArtifact(t *testing.T) {
	service := &compositionServiceStub{track: previewTrack()}
	handler, err := NewCompositionHTTPHandler(service, artifactReaderStub{openErr: artifacts.ErrInvalidSignature}, CurrentUser, []string{"http://localhost:8080"})
	if err != nil {
		t.Fatalf("NewCompositionHTTPHandler() error = %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/artifacts/preview", strings.NewReader(`{"workspaceId":7}`))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(WithCurrentUser(request.Context(), &models.User{ID: 7}))
	response := httptest.NewRecorder()

	handler.Preview(response, request)

	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"code":"preview_integrity_error"`) || strings.Contains(response.Body.String(), artifacts.ErrInvalidSignature.Error()) {
		t.Fatalf("unverified preview = status:%d body:%s", response.Code, response.Body.String())
	}
}

func TestCompositionPreviewFilesAreTrackScoped(t *testing.T) {
	service := &compositionServiceStub{track: previewTrack()}
	store := artifactReaderStub{file: artifacts.ArtifactFile{Path: "index.html", Contents: []byte("<!doctype html>"), SHA256: "digest"}}
	handler, err := NewCompositionHTTPHandler(service, store, CurrentUser, []string{"http://localhost:8080"})
	if err != nil {
		t.Fatalf("NewCompositionHTTPHandler() error = %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/conductor/tracks/track-1/preview/files/index.html", nil)
	request = mux.SetURLVars(request, map[string]string{"track_id": "track-1", "path": "index.html"})
	request = request.WithContext(WithCurrentUser(request.Context(), &models.User{ID: 7}))
	response := httptest.NewRecorder()

	handler.PreviewFile(response, request)

	if response.Code != http.StatusOK || response.Body.String() != "<!doctype html>" || response.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("preview file = status=%d headers=%v body=%q", response.Code, response.Header(), response.Body.String())
	}
}

func TestCompositionPreviewRejectsUnknownRequestFields(t *testing.T) {
	handler, err := NewCompositionHTTPHandler(&compositionServiceStub{track: previewTrack()}, artifactReaderStub{}, CurrentUser, []string{"http://localhost:8080"})
	if err != nil {
		t.Fatalf("NewCompositionHTTPHandler() error = %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/artifacts/preview", strings.NewReader(`{"unknown":true}`))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(WithCurrentUser(request.Context(), &models.User{ID: 7}))
	response := httptest.NewRecorder()

	handler.Preview(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("unknown field status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestCompositionPublicationClaimStateIsActionable(t *testing.T) {
	response := httptest.NewRecorder()
	writeCompositionServiceError(response, conductor.ErrPublicationClaimState)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"code":"publication_claim_unavailable"`) {
		t.Fatalf("publication claim error = status:%d body:%s", response.Code, response.Body.String())
	}
}

func TestCompositionPreviewReturnsInvalidCompositionEnvelope(t *testing.T) {
	service := &compositionServiceStub{composeErr: conductor.ErrInvalidComposition}
	handler, err := NewCompositionHTTPHandler(service, artifactReaderStub{}, CurrentUser, []string{"http://localhost:8080"})
	if err != nil {
		t.Fatalf("NewCompositionHTTPHandler() error = %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/artifacts/preview", strings.NewReader(`{"workspaceId":7}`))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(WithCurrentUser(request.Context(), &models.User{ID: 7}))
	response := httptest.NewRecorder()
	handler.Preview(response, request)
	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), `"code":"invalid_composition"`) {
		t.Fatalf("invalid composition = status:%d body:%s", response.Code, response.Body.String())
	}
}

func TestCompositionOperationalFailuresDoNotUseClientDenialEnvelope(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "dependency unavailable", err: conductor.ErrDependencyUnavailable, wantStatus: http.StatusBadGateway, wantCode: "composition_dependency_unavailable"},
		{name: "request canceled", err: context.Canceled, wantStatus: http.StatusInternalServerError, wantCode: "composition_operation_failed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			writeCompositionServiceError(response, test.err)
			if response.Code != test.wantStatus || !strings.Contains(response.Body.String(), `"code":"`+test.wantCode+`"`) || strings.Contains(response.Body.String(), `"code":"invalid_composition"`) {
				t.Fatalf("operational error = status:%d body:%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestCompositionPreviewOriginDenialIsSafeCorrelatedAndNever500(t *testing.T) {
	t.Setenv("RAILWAY_ENVIRONMENT_ID", "production-environment")
	for _, category := range []string{"surfaces", "embedders", "connections", "resources"} {
		t.Run(category, func(t *testing.T) {
			var logs bytes.Buffer
			service := &compositionServiceStub{composeErr: errors.Join(conductor.ErrInvalidComposition, conductor.ErrPreviewOriginDenied, domains.ErrOriginNotVerified)}
			handler, err := NewCompositionHTTPHandler(service, artifactReaderStub{}, CurrentUser, []string{"http://localhost:8080"})
			if err != nil {
				t.Fatalf("NewCompositionHTTPHandler() error = %v", err)
			}
			handler.logger = slog.New(slog.NewTextHandler(&logs, nil))
			canaryOrigin := "https://private-" + category + ".example"
			body, _ := json.Marshal(map[string]any{
				"idempotencyKey": "denied-" + category, "workspaceId": 7,
				"appName": "private-email@example.test", "organizationName": "Private organization", "city": "private-body-token",
				"madhhab": "hanafi", "templateId": "community-iftar", "modules": []string{"iftar-registration"},
				"requestedOrigins": map[string]any{category: []string{canaryOrigin}},
			})
			requestID := "railway-request-" + category
			request := httptest.NewRequest(http.MethodPost, "/api/artifacts/preview", bytes.NewReader(body))
			request.RemoteAddr = "100.64.0.8:4321"
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer private-bearer-token")
			request.Header.Set("X-Railway-Request-Id", requestID)
			request = request.WithContext(WithCurrentUser(request.Context(), &models.User{ID: 7, Email: "principal@example.test"}))
			response := httptest.NewRecorder()

			handler.Preview(response, request)

			if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), `"code":"invalid_composition"`) || strings.Contains(response.Body.String(), "composition_operation_failed") {
				t.Fatalf("origin denial = status:%d body:%s", response.Code, response.Body.String())
			}
			if response.Header().Get("X-Request-ID") != requestID {
				t.Fatalf("request correlation = %q, want %q", response.Header().Get("X-Request-ID"), requestID)
			}
			output := logs.String()
			for _, expected := range []string{"composition_preview_outcome", "request_id=" + requestID, "outcome=denied", "reason=origin_not_verified", "status=422"} {
				if !strings.Contains(output, expected) {
					t.Fatalf("telemetry missing %q: %s", expected, output)
				}
			}
			for _, secret := range []string{canaryOrigin, "private-email@example.test", "private-body-token", "private-bearer-token", "principal@example.test"} {
				if strings.Contains(output, secret) {
					t.Fatalf("telemetry leaked %q: %s", secret, output)
				}
			}
		})
	}
}

func TestCompositionRequestIDRejectsUntrustedRailwayHeader(t *testing.T) {
	t.Setenv("RAILWAY_ENVIRONMENT_ID", "production-environment")
	request := httptest.NewRequest(http.MethodPost, "/api/artifacts/preview", nil)
	request.RemoteAddr = "203.0.113.8:4321"
	request.Header.Set("X-Railway-Request-Id", "forged-request-id")
	if got := compositionRequestID(request); got == "forged-request-id" || !strings.HasPrefix(got, "request-") {
		t.Fatalf("untrusted Railway request ID = %q", got)
	}
}

type compositionServiceStub struct {
	track      *conductor.Track
	actor      *models.User
	request    conductor.CompositionRequest
	composeErr error
}

func (s *compositionServiceStub) Compose(_ context.Context, actor *models.User, request conductor.CompositionRequest) (*conductor.ComposeResult, error) {
	s.actor, s.request = actor, request
	if s.composeErr != nil {
		return nil, s.composeErr
	}
	return &conductor.ComposeResult{Track: s.track, Created: true}, nil
}

func (s *compositionServiceStub) Resume(context.Context, *models.User, string, int64) (*conductor.Track, error) {
	return s.track, nil
}

func (s *compositionServiceStub) RequestPublication(context.Context, *models.User, string, int64, string) (*conductor.Track, error) {
	return s.track, nil
}

func (s *compositionServiceStub) ActivatePublication(context.Context, *models.User, string, int64) (*conductor.Track, error) {
	return s.track, nil
}

func (s *compositionServiceStub) GetTrack(context.Context, *models.User, string) (*conductor.Track, error) {
	return s.track, nil
}

func (s *compositionServiceStub) Events(context.Context, *models.User, string) ([]conductor.TrackEvent, error) {
	return nil, nil
}

type artifactReaderStub struct {
	file    artifacts.ArtifactFile
	open    artifacts.BuildResult
	openErr error
}

func (s artifactReaderStub) Open(context.Context, string) (artifacts.BuildResult, error) {
	return s.open, s.openErr
}

func (s artifactReaderStub) ReadFile(context.Context, string, string) (artifacts.ArtifactFile, error) {
	return s.file, nil
}

func previewTrack() *conductor.Track {
	manifest := artifacts.Manifest{
		ArtifactID: "art-test", ContentHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", WorkspaceID: 7,
		Authorization: artifacts.BundleAuthorization{
			Subject: artifacts.BundleSubject{WorkspaceID: 7}, ExpiresAt: time.Now().Add(time.Hour), SignerKeyID: "test-key-1", Lifecycle: artifacts.BundleLifecyclePreview,
		},
		Signature: artifacts.BundleSignature{Algorithm: "Ed25519", KeyID: "test-key-1", Value: "signed-test-value"},
	}
	return &conductor.Track{
		ID: "track-1", WorkspaceID: 7,
		Artifact: &conductor.ArtifactReference{ArtifactID: manifest.ArtifactID, ContentHash: manifest.ContentHash, Manifest: manifest},
		Preview:  &conductor.PreviewMetadata{ContentHash: manifest.ContentHash},
	}
}

func verifiedBuildResult(track *conductor.Track) artifacts.BuildResult {
	return artifacts.BuildResult{ArtifactID: track.Artifact.ArtifactID, ContentHash: track.Artifact.ContentHash, Manifest: track.Artifact.Manifest}
}
