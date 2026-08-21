package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"

	"taawun/pkg/artifacts"
	"taawun/pkg/conductor"
	"taawun/pkg/models"
)

func (s artifactReaderStub) ReadVerifiedFile(_ context.Context, _ artifacts.BuildResult, _ string) (artifacts.ArtifactFile, error) {
	return s.file, nil
}

func TestPublicationTTLHTTPDefaultAndBoundsReachConductorExactly(t *testing.T) {
	serverNow := time.Date(2026, 8, 21, 18, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name       string
		ttlJSON    string
		wantTTL    int
		serviceErr error
		wantStatus int
	}{
		{name: "omitted defaults to 24 hours", wantTTL: 24, wantStatus: http.StatusCreated},
		{name: "minimum one hour", ttlJSON: `,"ttlHours":1`, wantTTL: 1, wantStatus: http.StatusCreated},
		{name: "maximum 2160 hours", ttlJSON: `,"ttlHours":2160`, wantTTL: 2160, wantStatus: http.StatusCreated},
		{name: "2161 rejected", ttlJSON: `,"ttlHours":2161`, wantTTL: 2161, serviceErr: conductor.ErrInvalidComposition, wantStatus: http.StatusUnprocessableEntity},
	} {
		t.Run(test.name, func(t *testing.T) {
			track := previewTrack()
			opened := verifiedBuildResult(track)
			opened.ManifestJSON, _ = json.Marshal(opened.Manifest)
			service := &compositionServiceStub{track: track, composeErr: test.serviceErr}
			handler, err := NewCompositionHTTPHandlerWithClock(service, artifactReaderStub{open: opened}, CurrentUser, []string{"http://localhost:8080"}, func() time.Time { return serverNow })
			if err != nil {
				t.Fatalf("NewCompositionHTTPHandlerWithClock() error = %v", err)
			}
			body := fmt.Sprintf(`{"idempotencyKey":"publication-ttl-test","workspaceId":7,"appName":"Community Iftar","organizationName":"Northside Mosque","city":"Denver","madhhab":"hanafi","templateId":"community-iftar","theme":{"accentColor":"#166534"},"modules":["iftar-registration"]%s}`, test.ttlJSON)
			request := httptest.NewRequest(http.MethodPost, "/api/artifacts/preview", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			request = request.WithContext(WithCurrentUser(request.Context(), &models.User{ID: 7}))
			response := httptest.NewRecorder()
			handler.Preview(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("Preview() status=%d body=%s, want %d", response.Code, response.Body.String(), test.wantStatus)
			}
			if service.request.TTLHours != test.wantTTL {
				t.Fatalf("Conductor ttlHours=%d, want %d", service.request.TTLHours, test.wantTTL)
			}
		})
	}
}

func TestCompositionHTTPHandlerRejectsMissingPublicationClock(t *testing.T) {
	track := previewTrack()
	opened := verifiedBuildResult(track)
	opened.ManifestJSON, _ = json.Marshal(opened.Manifest)
	if _, err := NewCompositionHTTPHandlerWithClock(&compositionServiceStub{track: track}, artifactReaderStub{open: opened}, CurrentUser, []string{"http://localhost:8080"}, nil); err == nil {
		t.Fatalf("NewCompositionHTTPHandlerWithClock(nil) error = %v", err)
	}
}

type publicationClockArtifactReader struct {
	opened            artifacts.BuildResult
	file              artifacts.ArtifactFile
	openCalls         int
	verifiedReadCalls int
}

func (r *publicationClockArtifactReader) Open(context.Context, string) (artifacts.BuildResult, error) {
	r.openCalls++
	return r.opened, nil
}

func (r *publicationClockArtifactReader) ReadFile(context.Context, string, string) (artifacts.ArtifactFile, error) {
	return r.file, nil
}

func (r *publicationClockArtifactReader) ReadVerifiedFile(context.Context, artifacts.BuildResult, string) (artifacts.ArtifactFile, error) {
	r.verifiedReadCalls++
	return r.file, nil
}

func TestVerifiedTrackAndFileExpiryUseTheSameInjectedClockBoundary(t *testing.T) {
	expiresAt := time.Date(2026, 8, 22, 18, 0, 0, 500_000_000, time.UTC)
	for _, test := range []struct {
		name               string
		now                time.Time
		authorizationState string
		fileStatus         int
		fileReads          int
	}{
		{name: "one millisecond before", now: expiresAt.Add(-time.Millisecond), authorizationState: "active", fileStatus: http.StatusOK, fileReads: 1},
		{name: "exact boundary", now: expiresAt, authorizationState: "expired", fileStatus: http.StatusGone},
		{name: "one millisecond after", now: expiresAt.Add(time.Millisecond), authorizationState: "expired", fileStatus: http.StatusGone},
	} {
		t.Run(test.name, func(t *testing.T) {
			track := previewTrack()
			track.Artifact.Manifest.Authorization.ExpiresAt = expiresAt
			track.Preview.AuthorizationExpiresAt = expiresAt
			opened := verifiedBuildResult(track)
			reader := &publicationClockArtifactReader{
				opened: opened,
				file:   artifacts.ArtifactFile{Path: "index.html", Contents: []byte("verified preview file"), SHA256: "digest"},
			}
			handler, err := NewCompositionHTTPHandlerWithClock(&compositionServiceStub{track: track}, reader, CurrentUser, []string{"http://localhost:8080"}, func() time.Time { return test.now })
			if err != nil {
				t.Fatal(err)
			}

			getRequest := httptest.NewRequest(http.MethodGet, "/api/conductor/tracks/track-1?includeVerifiedPreview=true", nil)
			getRequest = mux.SetURLVars(getRequest, map[string]string{"track_id": "track-1"})
			getRequest = getRequest.WithContext(WithCurrentUser(getRequest.Context(), &models.User{ID: 7}))
			getResponse := httptest.NewRecorder()
			handler.GetTrack(getResponse, getRequest)
			var payload struct {
				Verification struct {
					AuthorizationState string    `json:"authorizationState"`
					ServerTime         time.Time `json:"serverTime"`
				} `json:"verification"`
			}
			if getResponse.Code != http.StatusOK || json.Unmarshal(getResponse.Body.Bytes(), &payload) != nil ||
				payload.Verification.AuthorizationState != test.authorizationState || !payload.Verification.ServerTime.Equal(test.now) {
				t.Fatalf("verified track status=%d body=%s payload=%+v", getResponse.Code, getResponse.Body.String(), payload)
			}

			reader.openCalls, reader.verifiedReadCalls = 0, 0
			fileRequest := httptest.NewRequest(http.MethodGet, "/api/conductor/tracks/track-1/preview/files/index.html", nil)
			fileRequest = mux.SetURLVars(fileRequest, map[string]string{"track_id": "track-1", "path": "index.html"})
			fileRequest = fileRequest.WithContext(WithCurrentUser(fileRequest.Context(), &models.User{ID: 7}))
			fileResponse := httptest.NewRecorder()
			handler.PreviewFile(fileResponse, fileRequest)
			if fileResponse.Code != test.fileStatus || reader.openCalls != 1 || reader.verifiedReadCalls != test.fileReads {
				t.Fatalf("verified file status=%d body=%q opens=%d reads=%d", fileResponse.Code, fileResponse.Body.String(), reader.openCalls, reader.verifiedReadCalls)
			}
			if test.fileStatus == http.StatusOK && fileResponse.Body.String() != "verified preview file" {
				t.Fatalf("verified file body=%q", fileResponse.Body.String())
			}
			if test.fileStatus == http.StatusGone && !strings.Contains(fileResponse.Body.String(), "preview_authorization_expired") {
				t.Fatalf("expired file response=%s", fileResponse.Body.String())
			}
		})
	}
}
