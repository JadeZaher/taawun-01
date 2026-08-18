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
	body := `{"workspaceId":7,"appName":"Community Iftar","organizationName":"Northside Mosque","city":"Denver","madhhab":"hanafi","templateId":"community-iftar","theme":{"accentColor":"#166534"},"modules":["iftar-registration"],"components":[{"id":"iftar-registration","type":"iftar-registration","data":{"title":"Register","summary":"Join us","audience":["families"]}}]}`
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
	if len(service.request.Components) != 1 || string(service.request.Components[0].Data) != `{"title":"Register","summary":"Join us","audience":["families"]}` {
		t.Fatalf("preview components = %#v", service.request.Components)
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

func TestCompositionPreviewReturnsSafeComponentValidationDetails(t *testing.T) {
	componentErr := &artifacts.ComponentValidationError{ComponentID: "announcements", Key: "meta.actorId", Reason: "reserved_key"}
	service := &compositionServiceStub{composeErr: errors.Join(conductor.ErrInvalidComposition, componentErr)}
	handler, err := NewCompositionHTTPHandler(service, artifactReaderStub{}, CurrentUser, []string{"http://localhost:8080"})
	if err != nil {
		t.Fatalf("NewCompositionHTTPHandler() error = %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/artifacts/preview", strings.NewReader(`{"workspaceId":7}`))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(WithCurrentUser(request.Context(), &models.User{ID: 7}))
	response := httptest.NewRecorder()
	handler.Preview(response, request)
	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), `"componentId":"announcements"`) || !strings.Contains(response.Body.String(), `"key":""`) || !strings.Contains(response.Body.String(), `"reason":"reserved_key"`) || strings.Contains(response.Body.String(), "actorId") {
		t.Fatalf("component validation envelope = status:%d body:%s", response.Code, response.Body.String())
	}
}

func TestCompositionPreviewRejectsUnpairedUnicodeEscapesSafely(t *testing.T) {
	for _, test := range []struct {
		name     string
		document string
	}{
		{name: "root high surrogate", document: `{"title":"Unicode","summary":"\ud800"}`},
		{name: "nested low surrogate", document: `{"title":"Unicode","summary":"Safe","details":{"note":"\udc00"}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &compositionServiceStub{validateComponents: true}
			handler, err := NewCompositionHTTPHandler(service, artifactReaderStub{}, CurrentUser, []string{"http://localhost:8080"})
			if err != nil {
				t.Fatalf("NewCompositionHTTPHandler() error = %v", err)
			}
			body := `{"idempotencyKey":"unicode-denial","workspaceId":7,"appName":"Preview","organizationName":"Community","city":"Denver","madhhab":"hanafi","templateId":"community-iftar","modules":["announcements"],"components":[{"id":"announcements","type":"announcements","data":DOCUMENT}],"requestedOrigins":{"surfaces":["http://localhost:8080"]}}`
			body = strings.Replace(body, "DOCUMENT", test.document, 1)
			request := httptest.NewRequest(http.MethodPost, "/api/artifacts/preview", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			request = request.WithContext(WithCurrentUser(request.Context(), &models.User{ID: 7}))
			response := httptest.NewRecorder()

			handler.Preview(response, request)

			if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), `"code":"invalid_composition"`) || !strings.Contains(response.Body.String(), `"reason":"invalid_unicode_scalar"`) {
				t.Fatalf("Unicode rejection = status:%d body:%s", response.Code, response.Body.String())
			}
			for _, secret := range []string{"d800", "dc00", "Unicode", "Safe"} {
				if strings.Contains(response.Body.String(), secret) {
					t.Fatalf("Unicode rejection leaked %q: %s", secret, response.Body.String())
				}
			}
			if len(service.request.Components) != 1 || !bytes.Contains(service.request.Components[0].Data, []byte(`\u`)) {
				t.Fatalf("HTTP transport lost raw Unicode escape: %#v", service.request.Components)
			}
		})
	}
}

func TestCompositionPreviewRejectsExplicitEmptyComponents(t *testing.T) {
	service := &compositionServiceStub{track: previewTrack(), rejectEmptyComponents: true}
	handler, err := NewCompositionHTTPHandler(service, artifactReaderStub{}, CurrentUser, []string{"http://localhost:8080"})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/artifacts/preview", strings.NewReader(`{"workspaceId":7,"components":[]}`))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(WithCurrentUser(request.Context(), &models.User{ID: 7}))
	response := httptest.NewRecorder()
	handler.Preview(response, request)
	if response.Code != http.StatusUnprocessableEntity || service.request.Components == nil || !strings.Contains(response.Body.String(), `"reason":"components_required"`) {
		t.Fatalf("explicit empty components = status:%d request:%#v body:%s", response.Code, service.request.Components, response.Body.String())
	}
}

func TestCompositionValidationEnvelopeRedactsUntrustedMetadata(t *testing.T) {
	secretID := strings.Repeat("secret-token-", 40)
	secretKey := strings.Repeat("credential.", 40) + "password"
	response := httptest.NewRecorder()
	writeCompositionServiceError(response, errors.Join(conductor.ErrInvalidComposition, &artifacts.ComponentValidationError{ComponentID: secretID, Key: secretKey, Reason: "reserved_key"}))
	if response.Code != http.StatusUnprocessableEntity || strings.Contains(response.Body.String(), "secret-token") || strings.Contains(response.Body.String(), "credential") || strings.Contains(response.Body.String(), "password") || response.Body.Len() > 400 {
		t.Fatalf("untrusted component metadata leaked: %s", response.Body.String())
	}
}

func TestCompositionGetTrackCanReopenVerifiedPreview(t *testing.T) {
	track := previewTrack()
	opened := verifiedBuildResult(track)
	handler, err := NewCompositionHTTPHandler(&compositionServiceStub{track: track}, artifactReaderStub{open: opened}, CurrentUser, []string{"http://localhost:8080"})
	if err != nil {
		t.Fatalf("NewCompositionHTTPHandler() error = %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/conductor/tracks/track-1?includeVerifiedPreview=true", nil)
	request = mux.SetURLVars(request, map[string]string{"track_id": "track-1"})
	request = request.WithContext(WithCurrentUser(request.Context(), &models.User{ID: 7}))
	response := httptest.NewRecorder()
	handler.GetTrack(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"verified":true`) || !strings.Contains(response.Body.String(), `"manifestDigest"`) || !strings.Contains(response.Body.String(), `"previewUrl":"/api/conductor/tracks/track-1/preview/files/index.html"`) {
		t.Fatalf("verified track response = status:%d body:%s", response.Code, response.Body.String())
	}
}

func TestCompositionGetTrackCanReopenVerifiedLegacyV1Preview(t *testing.T) {
	track := legacyPreviewTrack()
	opened := verifiedBuildResult(track)
	handler, err := NewCompositionHTTPHandler(&compositionServiceStub{track: track}, artifactReaderStub{open: opened}, CurrentUser, []string{"http://localhost:8080"})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/conductor/tracks/legacy-track?includeVerifiedPreview=true", nil)
	request = mux.SetURLVars(request, map[string]string{"track_id": "legacy-track"})
	request = request.WithContext(WithCurrentUser(request.Context(), &models.User{ID: 7}))
	response := httptest.NewRecorder()
	handler.GetTrack(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"verified":true`) || !strings.Contains(response.Body.String(), artifacts.ManifestContractVersionV1) {
		t.Fatalf("legacy verified track = status:%d body:%s", response.Code, response.Body.String())
	}
}

func TestCompositionGetTrackLegacyV1BindingRejectsComponentOrManifestTampering(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*conductor.Track, *artifacts.BuildResult)
	}{
		{name: "explicit request components", mutate: func(track *conductor.Track, _ *artifacts.BuildResult) {
			track.Request.Components = []artifacts.ComponentInstance{}
		}},
		{name: "explicit build components", mutate: func(track *conductor.Track, _ *artifacts.BuildResult) {
			track.BuildRequest.Components = []artifacts.ComponentInstance{}
		}},
		{name: "stored component-bearing v1", mutate: func(track *conductor.Track, _ *artifacts.BuildResult) {
			track.Artifact.Manifest.Components = []artifacts.ComponentManifest{}
		}},
		{name: "reopened component-bearing v1", mutate: func(_ *conductor.Track, opened *artifacts.BuildResult) {
			opened.Manifest.Components = []artifacts.ComponentManifest{}
		}},
		{name: "stored module tamper", mutate: func(track *conductor.Track, _ *artifacts.BuildResult) {
			track.Artifact.Manifest.Modules[0].Title = "Changed"
		}},
		{name: "reopened template tamper", mutate: func(_ *conductor.Track, opened *artifacts.BuildResult) { opened.Manifest.Template.Version = "changed" }},
		{name: "signature contract tamper", mutate: func(_ *conductor.Track, opened *artifacts.BuildResult) {
			opened.Manifest.Signature.ContractVersion = artifacts.SignatureContractVersion
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			track := clonePreviewTrack(t, legacyPreviewTrack())
			opened := cloneBuildResult(t, verifiedBuildResult(track))
			test.mutate(track, &opened)
			handler, err := NewCompositionHTTPHandler(&compositionServiceStub{track: track}, artifactReaderStub{open: opened}, CurrentUser, []string{"http://localhost:8080"})
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodGet, "/api/conductor/tracks/legacy-track?includeVerifiedPreview=true", nil)
			request = mux.SetURLVars(request, map[string]string{"track_id": "legacy-track"})
			request = request.WithContext(WithCurrentUser(request.Context(), &models.User{ID: 7}))
			response := httptest.NewRecorder()
			handler.GetTrack(response, request)
			if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"code":"preview_integrity_error"`) {
				t.Fatalf("legacy tamper = status:%d body:%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestCompositionGetTrackVerifiedPreviewFailsClosedAcrossEveryComponentLayer(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*conductor.Track, *artifacts.BuildResult)
	}{
		{name: "request missing", mutate: func(track *conductor.Track, _ *artifacts.BuildResult) {
			track.Request.Components = []artifacts.ComponentInstance{}
		}},
		{name: "request tampered", mutate: func(track *conductor.Track, _ *artifacts.BuildResult) {
			track.Request.Components[0].Data = json.RawMessage(`{"summary":"changed","title":"Register"}`)
		}},
		{name: "build missing", mutate: func(track *conductor.Track, _ *artifacts.BuildResult) { track.BuildRequest.Components = nil }},
		{name: "build tampered", mutate: func(track *conductor.Track, _ *artifacts.BuildResult) {
			track.BuildRequest.Components[0].Data = json.RawMessage(`{"summary":"changed","title":"Register"}`)
		}},
		{name: "stored manifest missing", mutate: func(track *conductor.Track, _ *artifacts.BuildResult) { track.Artifact.Manifest.Components = nil }},
		{name: "stored manifest tampered", mutate: func(track *conductor.Track, _ *artifacts.BuildResult) {
			track.Artifact.Manifest.Components[0].Data = json.RawMessage(`{"summary":"changed","title":"Register"}`)
		}},
		{name: "stored module tampered", mutate: func(track *conductor.Track, _ *artifacts.BuildResult) {
			track.Artifact.Manifest.Modules[0].Title = "Changed"
		}},
		{name: "stored template tampered", mutate: func(track *conductor.Track, _ *artifacts.BuildResult) {
			track.Artifact.Manifest.Template.Version = "tampered"
		}},
		{name: "reopened manifest missing", mutate: func(_ *conductor.Track, opened *artifacts.BuildResult) { opened.Manifest.Components = nil }},
		{name: "reopened manifest tampered", mutate: func(_ *conductor.Track, opened *artifacts.BuildResult) {
			opened.Manifest.Components[0].Data = json.RawMessage(`{"summary":"changed","title":"Register"}`)
		}},
		{name: "reopened module tampered", mutate: func(_ *conductor.Track, opened *artifacts.BuildResult) { opened.Manifest.Modules[0].Title = "Changed" }},
		{name: "reopened template tampered", mutate: func(_ *conductor.Track, opened *artifacts.BuildResult) {
			opened.Manifest.Template.ID = artifacts.TemplateCommunityWorkspace
		}},
		{name: "matching module descriptor tamper", mutate: func(track *conductor.Track, opened *artifacts.BuildResult) {
			track.Artifact.Manifest.Modules[0].Title = "Changed"
			opened.Manifest.Modules[0].Title = "Changed"
		}},
		{name: "matching template identity tamper", mutate: func(track *conductor.Track, opened *artifacts.BuildResult) {
			track.Artifact.Manifest.Template.Version = "tampered"
			opened.Manifest.Template.Version = "tampered"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			track := clonePreviewTrack(t, previewTrack())
			opened := cloneBuildResult(t, verifiedBuildResult(track))
			test.mutate(track, &opened)
			handler, err := NewCompositionHTTPHandler(&compositionServiceStub{track: track}, artifactReaderStub{open: opened}, CurrentUser, []string{"http://localhost:8080"})
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodGet, "/api/conductor/tracks/track-1?includeVerifiedPreview=true", nil)
			request = mux.SetURLVars(request, map[string]string{"track_id": "track-1"})
			request = request.WithContext(WithCurrentUser(request.Context(), &models.User{ID: 7}))
			response := httptest.NewRecorder()
			handler.GetTrack(response, request)
			if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"code":"preview_integrity_error"`) {
				t.Fatalf("mismatch response = status:%d body:%s", response.Code, response.Body.String())
			}
		})
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
	track                 *conductor.Track
	actor                 *models.User
	request               conductor.CompositionRequest
	composeErr            error
	rejectEmptyComponents bool
	validateComponents    bool
}

func (s *compositionServiceStub) Compose(_ context.Context, actor *models.User, request conductor.CompositionRequest) (*conductor.ComposeResult, error) {
	s.actor, s.request = actor, request
	if s.rejectEmptyComponents && request.Components != nil && len(request.Components) == 0 {
		return nil, errors.Join(conductor.ErrInvalidComposition, &artifacts.ComponentValidationError{Reason: "components_required"})
	}
	if s.validateComponents {
		if _, err := artifacts.CanonicalizeSuppliedComponents(request.TemplateID, request.Modules, request.Components); err != nil {
			return nil, errors.Join(conductor.ErrInvalidComposition, err)
		}
	}
	if s.composeErr != nil {
		return nil, s.composeErr
	}
	return &conductor.ComposeResult{Track: s.track, Created: true}, nil
}

func clonePreviewTrack(t *testing.T, track *conductor.Track) *conductor.Track {
	t.Helper()
	encoded, _ := json.Marshal(track)
	var cloned conductor.Track
	if err := json.Unmarshal(encoded, &cloned); err != nil {
		t.Fatal(err)
	}
	return &cloned
}

func cloneBuildResult(t *testing.T, result artifacts.BuildResult) artifacts.BuildResult {
	t.Helper()
	encoded, _ := json.Marshal(result)
	var cloned artifacts.BuildResult
	if err := json.Unmarshal(encoded, &cloned); err != nil {
		t.Fatal(err)
	}
	return cloned
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
	componentData := json.RawMessage(`{"summary":"Join the gathering","title":"Register"}`)
	module, _ := artifacts.GetModule(artifacts.ModuleRegistration)
	component := artifacts.ComponentManifest{ID: artifacts.ModuleRegistration, Type: artifacts.ModuleRegistration, Data: componentData,
		DocumentPath: "components/iftar-registration.json", DocumentSHA256: sha256Hex(componentData)}
	aggregate, _ := json.Marshal([]artifacts.ComponentInstance{{ID: artifacts.ModuleRegistration, Type: artifacts.ModuleRegistration, Data: componentData}})
	manifest := artifacts.Manifest{
		ContractVersion: artifacts.ManifestContractVersion,
		ArtifactID:      "art-test", ContentHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", WorkspaceID: 7,
		Template:   artifacts.TemplateIdentity{ID: artifacts.TemplateCommunityIftar, Version: artifacts.TemplateCommunityIftarVersion},
		Modules:    []artifacts.ModuleDescriptor{module},
		Components: []artifacts.ComponentManifest{component},
		Files: []artifacts.FileDigest{
			{Path: "components.json", SHA256: sha256Hex(aggregate), Bytes: len(aggregate)},
			{Path: component.DocumentPath, SHA256: component.DocumentSHA256, Bytes: len(componentData)},
		},
		Authorization: artifacts.BundleAuthorization{
			Subject: artifacts.BundleSubject{WorkspaceID: 7}, ExpiresAt: time.Now().Add(time.Hour), SignerKeyID: "test-key-1", Lifecycle: artifacts.BundleLifecyclePreview,
		},
		Signature: artifacts.BundleSignature{Algorithm: "Ed25519", KeyID: "test-key-1", Value: "signed-test-value"},
	}
	return &conductor.Track{
		ID: "track-1", WorkspaceID: 7,
		Request: conductor.CompositionRequest{TemplateID: artifacts.TemplateCommunityIftar, Modules: []string{artifacts.ModuleRegistration},
			Components: []artifacts.ComponentInstance{{ID: artifacts.ModuleRegistration, Type: artifacts.ModuleRegistration, Data: componentData}}},
		BuildRequest: &artifacts.BuildRequest{TemplateID: artifacts.TemplateCommunityIftar, Modules: []string{artifacts.ModuleRegistration},
			Components: []artifacts.ComponentInstance{{ID: artifacts.ModuleRegistration, Type: artifacts.ModuleRegistration, Data: componentData}}},
		Artifact: &conductor.ArtifactReference{ArtifactID: manifest.ArtifactID, ContentHash: manifest.ContentHash, Manifest: manifest},
		Preview:  &conductor.PreviewMetadata{ContentHash: manifest.ContentHash},
	}
}

func sha256Hex(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func verifiedBuildResult(track *conductor.Track) artifacts.BuildResult {
	return artifacts.BuildResult{ArtifactID: track.Artifact.ArtifactID, ContentHash: track.Artifact.ContentHash, Manifest: track.Artifact.Manifest}
}

func legacyPreviewTrack() *conductor.Track {
	track := previewTrack()
	track.ID = "legacy-track"
	track.Request.Components = nil
	track.BuildRequest.Components = nil
	track.Artifact.Manifest.ContractVersion = artifacts.ManifestContractVersionV1
	track.Artifact.Manifest.Template.Version = "1.0.0"
	track.Artifact.Manifest.Components = nil
	track.Artifact.Manifest.Files = nil
	track.Artifact.Manifest.Authorization.Version = artifacts.SignatureContractVersionV1
	track.Artifact.Manifest.Signature.ContractVersion = artifacts.SignatureContractVersionV1
	return track
}
