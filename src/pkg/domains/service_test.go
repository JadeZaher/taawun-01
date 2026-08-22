package domains

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"taawun/pkg/artifacts"
	"taawun/pkg/models"
)

type fakeWorkspaceAuthorizer struct {
	roles map[int]string
}

func (a fakeWorkspaceAuthorizer) AuthorizeWorkspaceCapability(actor *models.User, workspaceID int, capability models.WorkspaceCapability) (*models.Workspace, error) {
	if actor == nil || workspaceID < 1 {
		return nil, errors.New("forbidden")
	}
	if actor.Role == models.RoleAdmin {
		return &models.Workspace{ID: workspaceID, OwnerID: actor.ID, Status: models.WorkspaceStatusActive}, nil
	}
	role := a.roles[actor.ID]
	allowed := role == models.WorkspaceRoleOwner || role == models.WorkspaceRoleAdmin
	if capability == models.WorkspaceCapabilityBuild {
		allowed = allowed || role == models.WorkspaceRoleMember
	}
	if !allowed {
		return nil, errors.New("forbidden")
	}
	return &models.Workspace{ID: workspaceID, OwnerID: 1, Status: models.WorkspaceStatusActive}, nil
}

type fakeResolver struct {
	values map[string][]string
	err    error
}

func (r *fakeResolver) LookupTXT(_ context.Context, name string) ([]string, error) {
	if r.err != nil {
		return nil, r.err
	}
	return append([]string(nil), r.values[name]...), nil
}

func TestVerifyTreatsMissingDNSNameAsMissingProof(t *testing.T) {
	db := openDomainTestDB(t)
	service, err := NewService(db, fakeWorkspaceAuthorizer{roles: map[int]string{1: models.WorkspaceRoleOwner}}, Options{
		Artifacts:      &fakeArtifacts{results: map[string]artifacts.BuildResult{}, files: map[string]map[string]artifacts.ArtifactFile{}},
		PreviewOrigins: []string{"https://preview.taawun.example"},
		Resolver:       &fakeResolver{err: &net.DNSError{Err: "name does not exist", Name: "_taawun.missing.example"}},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	owner := &models.User{ID: 1, Role: models.RoleUser}
	claimed, err := service.ClaimOrigin(context.Background(), owner, 1, "https://missing.example")
	if err != nil {
		t.Fatalf("ClaimOrigin() error = %v", err)
	}
	if _, err := service.Verify(context.Background(), owner, 1, claimed.Claim.ID); !errors.Is(err, ErrDNSProofNotFound) {
		t.Fatalf("Verify() missing DNS name error = %v, want ErrDNSProofNotFound", err)
	}
}

type fakeArtifacts struct {
	results   map[string]artifacts.BuildResult
	files     map[string]map[string]artifacts.ArtifactFile
	openCalls int
	readCalls int
}

func TestConfiguredPreviewOriginDoesNotGrantPublication(t *testing.T) {
	db := openDomainTestDB(t)
	service, err := NewService(db, fakeWorkspaceAuthorizer{roles: map[int]string{3: models.WorkspaceRoleMember}}, Options{
		Artifacts:      &fakeArtifacts{results: map[string]artifacts.BuildResult{}, files: map[string]map[string]artifacts.ArtifactFile{}},
		PreviewOrigins: []string{"http://localhost:8080", "https://preview.taawun.example"},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	member := &models.User{ID: 3, Role: models.RoleUser}
	workspace := &models.Workspace{ID: 1}
	requested := artifacts.OriginPolicy{Surfaces: []string{"http://localhost:8080"}}

	preview, err := service.AuthorizeOriginsForLifecycle(context.Background(), member, workspace, artifacts.BundleLifecyclePreview, requested)
	if err != nil || len(preview.Surfaces) != 1 || preview.Surfaces[0] != "http://localhost:8080" {
		t.Fatalf("preview authorization = %#v, %v", preview, err)
	}
	if _, err := service.AuthorizeOrigins(context.Background(), member, workspace, requested); !errors.Is(err, ErrInvalidOrigin) && !errors.Is(err, ErrOriginNotVerified) {
		t.Fatalf("published authorization error = %v, want no publication grant", err)
	}
	if _, err := service.AuthorizeOriginsForLifecycle(context.Background(), member, workspace, artifacts.BundleLifecyclePreview,
		artifacts.OriginPolicy{Surfaces: []string{"https://attacker.example"}}); !errors.Is(err, ErrOriginNotVerified) {
		t.Fatalf("unconfigured preview origin error = %v", err)
	}
	for _, test := range []struct {
		name   string
		policy artifacts.OriginPolicy
	}{
		{name: "surface", policy: artifacts.OriginPolicy{Surfaces: []string{"https://unverified-surface.example"}}},
		{name: "embedder", policy: artifacts.OriginPolicy{Surfaces: []string{"https://preview.taawun.example"}, Embedders: []string{"https://unverified-embedder.example"}}},
		{name: "connection", policy: artifacts.OriginPolicy{Surfaces: []string{"https://preview.taawun.example"}, Connections: []string{"https://unverified-connection.example"}}},
		{name: "resource", policy: artifacts.OriginPolicy{Surfaces: []string{"https://preview.taawun.example"}, Resources: []string{"https://unverified-resource.example"}}},
	} {
		t.Run("rejects unverified "+test.name, func(t *testing.T) {
			if _, err := service.AuthorizeOriginsForLifecycle(context.Background(), member, workspace, artifacts.BundleLifecyclePreview, test.policy); !errors.Is(err, ErrOriginNotVerified) {
				t.Fatalf("unverified %s error = %v", test.name, err)
			}
		})
	}
}

func (s *fakeArtifacts) Open(_ context.Context, contentHash string) (artifacts.BuildResult, error) {
	s.openCalls++
	result, ok := s.results[contentHash]
	if !ok {
		return artifacts.BuildResult{}, artifacts.ErrArtifactNotFound
	}
	return result, nil
}

func (s *fakeArtifacts) ReadFile(_ context.Context, contentHash, filePath string) (artifacts.ArtifactFile, error) {
	s.readCalls++
	file, ok := s.files[contentHash][filePath]
	if !ok {
		return artifacts.ArtifactFile{}, artifacts.ErrArtifactFileNotFound
	}
	return file, nil
}

func (s *fakeArtifacts) ReadVerifiedFile(ctx context.Context, result artifacts.BuildResult, filePath string) (artifacts.ArtifactFile, error) {
	return s.ReadFile(ctx, result.ContentHash, filePath)
}

func TestDomainLifecycleAuthorizationAndPublicRollback(t *testing.T) {
	db := openDomainTestDB(t)
	clock := time.Date(2026, 8, 17, 4, 0, 0, 0, time.UTC)
	resolver := &fakeResolver{values: map[string][]string{}}
	store := &fakeArtifacts{results: map[string]artifacts.BuildResult{}, files: map[string]map[string]artifacts.ArtifactFile{}}
	authorizer := fakeWorkspaceAuthorizer{roles: map[int]string{
		1: models.WorkspaceRoleOwner,
		2: models.WorkspaceRoleAdmin,
		3: models.WorkspaceRoleMember,
		4: models.WorkspaceRoleViewer,
	}}
	service, err := NewService(db, authorizer, Options{
		Resolver: resolver, Artifacts: store, Now: func() time.Time { return clock },
		ChallengeTTL: time.Hour, VerificationTTL: 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	owner := &models.User{ID: 1, Role: models.RoleUser}
	member := &models.User{ID: 3, Role: models.RoleUser}
	viewer := &models.User{ID: 4, Role: models.RoleUser}

	if _, err := service.ClaimOrigin(context.Background(), viewer, 1, "https://app.example.com"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("viewer claim error = %v, want forbidden", err)
	}
	claimed, err := service.ClaimOrigin(context.Background(), owner, 1, "APP.EXAMPLE.COM.")
	if err != nil {
		t.Fatalf("ClaimOrigin() error = %v", err)
	}
	if claimed.Claim.Origin != "https://app.example.com" || claimed.Verification.RecordName != "_taawun.app.example.com" {
		t.Fatalf("claim was not normalized exactly: %#v", claimed)
	}
	var storedDigest []byte
	if err := db.QueryRow(`SELECT challenge_hash FROM workspace_domain_claims WHERE id = ?`, claimed.Claim.ID).Scan(&storedDigest); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(storedDigest), claimed.Verification.Value) || len(storedDigest) != 32 {
		t.Fatal("database did not retain only the challenge digest")
	}
	if _, err := service.ClaimOrigin(context.Background(), owner, 2, "https://app.example.com"); !errors.Is(err, ErrOriginClaimed) {
		t.Fatalf("cross-workspace claim error = %v, want active conflict", err)
	}
	if _, err := service.Verify(context.Background(), owner, 1, claimed.Claim.ID); !errors.Is(err, ErrDNSProofNotFound) {
		t.Fatalf("Verify() without TXT error = %v", err)
	}
	resolver.values[claimed.Verification.RecordName] = []string{claimed.Verification.Value}
	verified, err := service.Verify(context.Background(), owner, 1, claimed.Claim.ID)
	if err != nil || verified.Status != StatusVerified {
		t.Fatalf("Verify() = %#v, %v", verified, err)
	}

	policy, err := service.AuthorizeOrigins(context.Background(), member, &models.Workspace{ID: 1}, artifacts.OriginPolicy{
		Surfaces: []string{"https://APP.EXAMPLE.COM/"},
	})
	if err != nil || len(policy.Surfaces) != 1 || policy.Surfaces[0] != "https://app.example.com" {
		t.Fatalf("AuthorizeOrigins() = %#v, %v", policy, err)
	}
	for _, unverified := range []string{"https://child.app.example.com", "https://example.com", "https://*.example.com"} {
		_, err := service.AuthorizeOrigins(context.Background(), member, &models.Workspace{ID: 1}, artifacts.OriginPolicy{Surfaces: []string{unverified}})
		if err == nil {
			t.Fatalf("AuthorizeOrigins(%q) unexpectedly succeeded", unverified)
		}
	}

	firstHash := strings.Repeat("a", 64)
	secondHash := strings.Repeat("b", 64)
	storeArtifact(store, firstHash, "artifact-one", 1, "app.example.com", clock.Add(12*time.Hour), []byte("first"))
	storeArtifact(store, secondHash, "artifact-two", 1, "app.example.com", clock.Add(12*time.Hour), []byte("second"))
	first, err := service.Publish(context.Background(), owner, 1, claimed.Claim.ID, firstHash)
	if err != nil || !first.Active {
		t.Fatalf("Publish(first) = %#v, %v", first, err)
	}
	second, err := service.Publish(context.Background(), owner, 1, claimed.Claim.ID, secondHash)
	if err != nil || !second.Active {
		t.Fatalf("Publish(second) = %#v, %v", second, err)
	}
	rolledBack, err := service.Activate(context.Background(), owner, 1, claimed.Claim.ID, first.ID)
	if err != nil || rolledBack.SourcePublicationID != first.ID || rolledBack.ContentHash != firstHash {
		t.Fatalf("Activate(rollback) = %#v, %v", rolledBack, err)
	}
	history, err := service.PublicationHistory(context.Background(), owner, 1, claimed.Claim.ID)
	if err != nil || len(history) != 3 {
		t.Fatalf("PublicationHistory() len = %d, err = %v", len(history), err)
	}
	activeCount := 0
	for _, publication := range history {
		if publication.Active {
			activeCount++
		}
	}
	if activeCount != 1 {
		t.Fatalf("active publication count = %d", activeCount)
	}

	fallback := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	public, err := NewPublicHandler(service, []string{"http://localhost:8080"}, fallback)
	if err != nil {
		t.Fatalf("NewPublicHandler() error = %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "http://app.example.com/", nil)
	request.Host = "app.example.com"
	response := httptest.NewRecorder()
	public.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != "first" || store.openCalls < 4 || store.readCalls != 1 {
		t.Fatalf("public response = %d %q; calls open=%d read=%d", response.Code, response.Body.String(), store.openCalls, store.readCalls)
	}
	runtimeRequest := httptest.NewRequest(http.MethodGet, "http://app.example.com/assets/datastar-v1.0.2.js", nil)
	runtimeRequest.Host = "app.example.com"
	runtimeResponse := httptest.NewRecorder()
	public.ServeHTTP(runtimeResponse, runtimeRequest)
	if runtimeResponse.Code != http.StatusOK || runtimeResponse.Body.String() != "pinned runtime" {
		t.Fatalf("published runtime response = %d %q", runtimeResponse.Code, runtimeResponse.Body.String())
	}
	accountRequest := httptest.NewRequest(http.MethodGet, "http://app.example.com/account", nil)
	accountRequest.Host = "app.example.com"
	accountResponse := httptest.NewRecorder()
	public.ServeHTTP(accountResponse, accountRequest)
	if accountResponse.Code != http.StatusNotFound {
		t.Fatalf("published host account path = %d, want 404 without control-site fallback", accountResponse.Code)
	}
	controlRequest := httptest.NewRequest(http.MethodGet, "http://localhost:8080/", nil)
	controlRequest.Host = "localhost:8080"
	controlResponse := httptest.NewRecorder()
	public.ServeHTTP(controlResponse, controlRequest)
	if controlResponse.Code != http.StatusTeapot {
		t.Fatalf("control host status = %d", controlResponse.Code)
	}

	if _, err := service.Revoke(context.Background(), owner, 1, claimed.Claim.ID); err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}
	revokedResponse := httptest.NewRecorder()
	public.ServeHTTP(revokedResponse, request)
	if revokedResponse.Code != http.StatusNotFound {
		t.Fatalf("revoked domain public status = %d", revokedResponse.Code)
	}

	pending, err := service.ClaimOrigin(context.Background(), owner, 1, "pending-expiry.example.com")
	if err != nil {
		t.Fatalf("ClaimOrigin(pending expiry) error = %v", err)
	}
	clock = clock.Add(2 * time.Hour)
	expiredPending, err := service.Inspect(context.Background(), owner, 1, pending.Claim.ID)
	if err != nil || expiredPending.Status != StatusRevoked || expiredPending.RevocationReason != "expired" || expiredPending.RevokedAt == nil {
		t.Fatalf("expired pending claim = %#v, %v", expiredPending, err)
	}
	grant, err := service.ClaimOrigin(context.Background(), owner, 1, "grant-expiry.example.com")
	if err != nil {
		t.Fatalf("ClaimOrigin(grant expiry) error = %v", err)
	}
	resolver.values[grant.Verification.RecordName] = []string{grant.Verification.Value}
	if _, err := service.Verify(context.Background(), owner, 1, grant.Claim.ID); err != nil {
		t.Fatalf("Verify(grant expiry) error = %v", err)
	}
	clock = clock.Add(25 * time.Hour)
	expiredGrant, err := service.Inspect(context.Background(), owner, 1, grant.Claim.ID)
	if err != nil || expiredGrant.Status != StatusRevoked || expiredGrant.RevocationReason != "expired" {
		t.Fatalf("expired verified grant = %#v, %v", expiredGrant, err)
	}
}

func TestNormalizeOriginRejectsInheritanceAndUnsafeHosts(t *testing.T) {
	tests := []string{
		"http://example.com", "https://example.com/path", "https://example.com:8443",
		"https://*.example.com", "https://127.0.0.1", "https://localhost", "https://éxample.com",
	}
	for _, value := range tests {
		if _, _, err := NormalizeOrigin(value); !errors.Is(err, ErrInvalidOrigin) {
			t.Fatalf("NormalizeOrigin(%q) error = %v", value, err)
		}
	}
	origin, host, err := NormalizeOrigin("https://Example.COM:443/")
	if err != nil || origin != "https://example.com" || host != "example.com" {
		t.Fatalf("NormalizeOrigin canonical = %q, %q, %v", origin, host, err)
	}
}

func openDomainTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", "file:"+strings.ReplaceAll(t.Name(), "/", "-")+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func storeArtifact(store *fakeArtifacts, hash, artifactID string, workspaceID int, host string, expires time.Time, body []byte) {
	manifest := artifacts.Manifest{
		ArtifactID: artifactID, ContentHash: hash, WorkspaceID: workspaceID,
		Authorization: artifacts.BundleAuthorization{
			Subject:         artifacts.BundleSubject{WorkspaceID: workspaceID},
			AllowedOrigins:  artifacts.OriginPolicy{Surfaces: []string{"https://" + host}},
			ApprovedDomains: []string{host}, ExpiresAt: expires,
		},
		Security: artifacts.SecurityPolicy{CSP: []artifacts.RenderModeCSP{{
			RenderMode: artifacts.RenderModeStandalone,
			Policy:     artifacts.CSPPolicy{DefaultSrc: []string{"'self'"}, ObjectSrc: []string{"'none'"}},
		}}},
	}
	store.results[hash] = artifacts.BuildResult{ArtifactID: artifactID, ContentHash: hash, Manifest: manifest}
	store.files[hash] = map[string]artifacts.ArtifactFile{
		"index.html":                {Path: "index.html", Contents: body, SHA256: hash},
		"assets/datastar-v1.0.2.js": {Path: "assets/datastar-v1.0.2.js", Contents: []byte("pinned runtime"), SHA256: hash},
	}
}
