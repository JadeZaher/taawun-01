package handlers

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"taawun/pkg/artifacts"
	"taawun/pkg/ethics"
)

func TestArtifactHTTPHandlerBuildUsesTrustedAuthority(t *testing.T) {
	handler, builder, authority := newArtifactHandler(t)
	request := handlerBuildRequest()
	request.Subject = artifacts.SubjectBinding{}
	request.AllowedOrigins = artifacts.OriginPolicy{}
	body, _ := json.Marshal(request)
	httpRequest := httptest.NewRequest(http.MethodPost, artifactAPIBase, bytes.NewReader(body))
	httpRequest.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httpRequest)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("POST status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response artifactBuildResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode build response: %v", err)
	}
	if response.Artifact.ContentHash == "" || recorder.Header().Get("Location") != response.Artifact.URL {
		t.Fatalf("build response/location = %#v, %q", response, recorder.Header().Get("Location"))
	}
	result, err := builder.Open(context.Background(), response.Artifact.ContentHash)
	if err != nil {
		t.Fatalf("Open() returned an error: %v", err)
	}
	if result.Manifest.Authorization.Subject.ID != authority.Subject.ID || result.Manifest.Authorization.Subject.UserID != authority.Subject.UserID || !reflect.DeepEqual(result.Manifest.Authorization.AllowedOrigins, normalizedAuthorityOrigins(authority.AllowedOrigins)) {
		t.Fatalf("handler did not bind trusted authority: %#v", result.Manifest.Authorization)
	}
	if httpRequest.Header.Get("Authorization") != "" {
		t.Fatal("test unexpectedly supplied credentials; handler should rely on upstream authority")
	}
}

func TestArtifactHTTPHandlerRejectsClientAuthorityAndStrictHTTPViolations(t *testing.T) {
	handler, _, _ := newArtifactHandler(t)
	tests := []struct {
		name        string
		contentType string
		body        string
		wantStatus  int
	}{
		{name: "wrong content type", contentType: "text/plain", body: `{}`, wantStatus: http.StatusUnsupportedMediaType},
		{name: "unknown JSON field", contentType: "application/json", body: `{"unknown":true}`, wantStatus: http.StatusBadRequest},
		{name: "multiple JSON values", contentType: "application/json", body: `{} {}`, wantStatus: http.StatusBadRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, artifactAPIBase, strings.NewReader(test.body))
			request.Header.Set("Content-Type", test.contentType)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, test.wantStatus, recorder.Body.String())
			}
		})
	}

	clientRequest := handlerBuildRequest()
	clientRequest.Subject = artifacts.SubjectBinding{ID: "user:999", UserID: 999}
	body, _ := json.Marshal(clientRequest)
	request := httptest.NewRequest(http.MethodPost, artifactAPIBase, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "subject_mismatch") {
		t.Fatalf("client subject status/body = %d %s", recorder.Code, recorder.Body.String())
	}

	oversized := strings.Repeat("x", defaultArtifactBodyLimit+1)
	request = httptest.NewRequest(http.MethodPost, artifactAPIBase, strings.NewReader(oversized))
	request.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized status = %d, want 413", recorder.Code)
	}
}

func TestArtifactHTTPHandlerServesScopedVerifiedImmutableFiles(t *testing.T) {
	handler, builder, authority := newArtifactHandler(t)
	buildRequest := handlerBuildRequest()
	buildRequest.Subject = authority.Subject
	buildRequest.AllowedOrigins = authority.AllowedOrigins
	result, err := builder.Build(context.Background(), buildRequest)
	if err != nil {
		t.Fatalf("Build() returned an error: %v", err)
	}

	metadataURL := artifactAPIBase + "/" + result.ContentHash
	recorder := serveArtifactRequest(handler, http.MethodGet, metadataURL, nil)
	if recorder.Code != http.StatusOK || recorder.Header().Get("Cache-Control") != "private, max-age=31536000, immutable" {
		t.Fatalf("metadata response = %d %#v %s", recorder.Code, recorder.Header(), recorder.Body.String())
	}

	manifestRecorder := serveArtifactRequest(handler, http.MethodGet, metadataURL+"/manifest.json", nil)
	if manifestRecorder.Code != http.StatusOK || !strings.Contains(manifestRecorder.Body.String(), `"signature"`) {
		t.Fatalf("manifest response = %d %s", manifestRecorder.Code, manifestRecorder.Body.String())
	}

	fileURL := metadataURL + "/files/index.html"
	fileRecorder := serveArtifactRequest(handler, http.MethodGet, fileURL, nil)
	if fileRecorder.Code != http.StatusOK || !strings.HasPrefix(fileRecorder.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("file response = %d %#v %s", fileRecorder.Code, fileRecorder.Header(), fileRecorder.Body.String())
	}
	if csp := fileRecorder.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self' 'unsafe-eval'") || !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Fatalf("standalone CSP header = %q", csp)
	}
	etag := fileRecorder.Header().Get("ETag")
	conditional := serveArtifactRequest(handler, http.MethodGet, fileURL, map[string]string{"If-None-Match": etag})
	if conditional.Code != http.StatusNotModified || conditional.Body.Len() != 0 {
		t.Fatalf("conditional response = %d %q", conditional.Code, conditional.Body.String())
	}
	head := serveArtifactRequest(handler, http.MethodHead, fileURL, nil)
	if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("Content-Length") == "" {
		t.Fatalf("HEAD response = %d %#v %q", head.Code, head.Header(), head.Body.String())
	}

	traversal := serveArtifactRequest(handler, http.MethodGet, metadataURL+"/files/%2e%2e/manifest.json", nil)
	if traversal.Code != http.StatusNotFound {
		t.Fatalf("traversal status = %d, body=%s", traversal.Code, traversal.Body.String())
	}

	wrongScope := *handler
	wrongScope.authority = func(*http.Request) (ArtifactAuthority, bool) {
		other := authority
		other.Subject = artifacts.SubjectBinding{ID: "user:8", UserID: 8}
		return other, true
	}
	recorder = serveArtifactRequest(&wrongScope, http.MethodGet, metadataURL, nil)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("cross-subject read status = %d, want 404", recorder.Code)
	}

	if err := os.WriteFile(filepath.Join(result.Directory, "index.html"), []byte("tampered"), 0o644); err != nil {
		t.Fatalf("tamper fixture: %v", err)
	}
	tampered := serveArtifactRequest(handler, http.MethodGet, fileURL, nil)
	if tampered.Code != http.StatusConflict || !strings.Contains(tampered.Body.String(), "artifact_integrity_error") {
		t.Fatalf("tampered response = %d %s", tampered.Code, tampered.Body.String())
	}
}

func TestArtifactHTTPHandlerRequiresStableSignerAndTrustedAuthority(t *testing.T) {
	ephemeral, err := artifacts.NewBuilder(filepath.Join(t.TempDir(), "ephemeral"))
	if err != nil {
		t.Fatalf("NewBuilder() returned an error: %v", err)
	}
	resolver := func(*http.Request) (ArtifactAuthority, bool) { return ArtifactAuthority{}, false }
	if _, err := NewArtifactHTTPHandler(ephemeral, resolver); err == nil {
		t.Fatal("NewArtifactHTTPHandler() accepted an ephemeral signer")
	}
	stable := newSignedArtifactBuilder(t)
	if _, err := NewArtifactHTTPHandler(stable, nil); err == nil {
		t.Fatal("NewArtifactHTTPHandler() accepted a nil authority resolver")
	}
}

func newArtifactHandler(t *testing.T) (*ArtifactHTTPHandler, *artifacts.Builder, ArtifactAuthority) {
	t.Helper()
	builder := newSignedArtifactBuilder(t)
	authority := ArtifactAuthority{
		WorkspaceID: 42,
		Subject:     artifacts.SubjectBinding{ID: "user:7", UserID: 7},
		AllowedOrigins: artifacts.OriginPolicy{
			Surfaces: []string{"https://cards.example"}, Embedders: []string{"https://portal.example"},
			Connections: []string{"https://api.example"}, Resources: []string{"https://assets.example"},
		},
	}
	handler, err := NewArtifactHTTPHandler(builder, func(*http.Request) (ArtifactAuthority, bool) {
		return authority, true
	})
	if err != nil {
		t.Fatalf("NewArtifactHTTPHandler() returned an error: %v", err)
	}
	return handler, builder, authority
}

func newSignedArtifactBuilder(t *testing.T) *artifacts.Builder {
	t.Helper()
	seed := sha256.Sum256([]byte("handler artifact signing key"))
	builder, err := artifacts.NewSignedBuilder(
		filepath.Join(t.TempDir(), "artifacts"),
		artifacts.SigningConfig{KeyID: "handler-key-1", PrivateKey: ed25519.NewKeyFromSeed(seed[:])},
	)
	if err != nil {
		t.Fatalf("NewSignedBuilder() returned an error: %v", err)
	}
	return builder
}

func handlerBuildRequest() artifacts.BuildRequest {
	return artifacts.BuildRequest{
		WorkspaceID: 42, AppName: "Community Cards", OrganizationName: "Mercy Center", City: "Denver",
		Madhhab: ethics.MadhhabHanafi, TemplateID: artifacts.TemplateCommunityIftar,
		Theme:     artifacts.ThemeRequest{AccentColor: "#166534"},
		Modules:   []string{artifacts.ModuleRegistration, artifacts.ModuleAnnouncements, artifacts.ModuleDonationCampaign},
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour), Lifecycle: artifacts.BundleLifecyclePreview,
	}
}

func serveArtifactRequest(handler http.Handler, method, target string, headers map[string]string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, nil)
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}
