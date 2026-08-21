package domains

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/mux"

	"taawun/pkg/artifacts"
	"taawun/pkg/models"
)

type publicationLifetimeArtifacts struct {
	mu        sync.Mutex
	results   map[string]artifacts.BuildResult
	files     map[string]map[string]artifacts.ArtifactFile
	openError map[string]error
	openHook  func()
	openCalls int
	readCalls int
}

func (s *publicationLifetimeArtifacts) Open(_ context.Context, contentHash string) (artifacts.BuildResult, error) {
	s.mu.Lock()
	s.openCalls++
	hook := s.openHook
	s.openHook = nil
	err := s.openError[contentHash]
	result, ok := s.results[contentHash]
	s.mu.Unlock()
	if hook != nil {
		hook()
	}
	if err != nil {
		return artifacts.BuildResult{}, err
	}
	if !ok {
		return artifacts.BuildResult{}, artifacts.ErrArtifactNotFound
	}
	return result, nil
}

func (s *publicationLifetimeArtifacts) ReadFile(_ context.Context, contentHash, filePath string) (artifacts.ArtifactFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.readCalls++
	file, ok := s.files[contentHash][filePath]
	if !ok {
		return artifacts.ArtifactFile{}, artifacts.ErrArtifactFileNotFound
	}
	return file, nil
}

func (s *publicationLifetimeArtifacts) ReadVerifiedFile(_ context.Context, built artifacts.BuildResult, filePath string) (artifacts.ArtifactFile, error) {
	return s.ReadFile(context.Background(), built.ContentHash, filePath)
}

func (s *publicationLifetimeArtifacts) counts() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.openCalls, s.readCalls
}

func (s *publicationLifetimeArtifacts) resetCounts() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.openCalls, s.readCalls = 0, 0
}

type publicationBindingResolverStub struct {
	calls    int
	bindings map[string][]PublicationTrackBinding
}

func (s *publicationBindingResolverStub) ResolvePublicationTrackBindings(_ context.Context, _ int, _ string, publications []Publication) (map[string][]PublicationTrackBinding, error) {
	s.calls++
	result := make(map[string][]PublicationTrackBinding, len(publications))
	for _, publication := range publications {
		result[publication.ID] = append([]PublicationTrackBinding(nil), s.bindings[publication.ID]...)
	}
	return result, nil
}

func TestPublicationContextHTTPIsBoundedOpaqueRedactedAndProofEfficient(t *testing.T) {
	db := openDomainTestDB(t)
	now := time.Date(2026, 8, 21, 13, 0, 0, 0, time.UTC)
	store := &publicationLifetimeArtifacts{results: map[string]artifacts.BuildResult{}, files: map[string]map[string]artifacts.ArtifactFile{}}
	service := newPublicationLifetimeService(t, db, store, &now)
	insertPublicationLifetimeClaim(t, db, "claim-context", now.Add(90*24*time.Hour))

	for index := 0; index < 55; index++ {
		publicationID := fmt.Sprintf("publication-%03d", index)
		contentHash := fmt.Sprintf("%064x", index+1)
		artifactID := fmt.Sprintf("artifact-%03d", index)
		var deactivated any = now.Add(time.Duration(index+1) * time.Second).Unix()
		if index == 54 {
			deactivated = nil
		}
		activatedAt := now.Add(time.Duration(index) * time.Second)
		if index == 34 {
			activatedAt = now.Add(35 * time.Second)
		}
		insertPublicationLifetimeRow(t, db, publicationID, "claim-context", contentHash, artifactID, activatedAt, deactivated, "")
		if index == 10 || index == 54 {
			rawManifest := []byte(fmt.Sprintf("{\n  \"publication\": %d,\n  \"serialization\": \"stored\"\n}\n", index))
			store.results[contentHash] = publicationLifetimeBuild(contentHash, artifactID, now.Add(24*time.Hour), rawManifest)
		}
	}
	bindings := &publicationBindingResolverStub{bindings: map[string][]PublicationTrackBinding{
		"publication-054": {{TrackID: "track-active", Status: "PUBLISHED", Version: 9}},
		"publication-010": {
			{TrackID: "track-ambiguous-a", Status: "PUBLISHED", Version: 4},
			{TrackID: "track-ambiguous-b", Status: "PUBLISHED", Version: 7},
		},
	}}
	contexts, err := NewPublicationContextService(service, bindings)
	if err != nil {
		t.Fatalf("NewPublicationContextService() error = %v", err)
	}
	owner := &models.User{ID: 1, Role: models.RoleUser}
	handler, err := NewHTTPHandlerWithPublicationContext(service, contexts, func(context.Context) (*models.User, bool) { return owner, true })
	if err != nil {
		t.Fatalf("NewHTTPHandlerWithPublicationContext() error = %v", err)
	}

	firstResponse := publicationHistoryRequest(handler, owner, "")
	if firstResponse.Code != http.StatusOK {
		t.Fatalf("default publication context status = %d body=%s", firstResponse.Code, firstResponse.Body.String())
	}
	var firstPage PublicationContextPage
	if err := json.Unmarshal(firstResponse.Body.Bytes(), &firstPage); err != nil {
		t.Fatalf("decode first publication context page: %v", err)
	}
	if len(firstPage.Publications) != DefaultPublicationContextLimit || firstPage.NextCursor == "" {
		t.Fatalf("default page = %d records cursor=%q", len(firstPage.Publications), firstPage.NextCursor)
	}
	if !firstPage.ServerTime.Equal(now) {
		t.Fatalf("default page serverTime = %s, want %s", firstPage.ServerTime, now)
	}
	if firstPage.Publications[0].ID != "publication-054" || firstPage.Publications[0].ServingState != ServingStateServing || firstPage.Publications[0].TrackBinding == nil {
		t.Fatalf("active publication context = %+v", firstPage.Publications[0])
	}
	for _, publication := range firstPage.Publications[1:] {
		if publication.AuthorizationExpiresAt != nil || publication.ManifestDigest != nil || publication.ServingState != ServingStateInactive {
			t.Fatalf("ordinary inactive history opened proof for %+v", publication)
		}
	}
	openCalls, _ := store.counts()
	if openCalls != 1 {
		t.Fatalf("default page artifact opens = %d, want exactly one active proof", openCalls)
	}
	body := firstResponse.Body.String()
	if !strings.Contains(body, `"authorizationExpiresAt":null`) || !strings.Contains(body, `"manifestDigest":null`) || !strings.Contains(body, `"trackBinding":null`) || strings.Contains(firstPage.NextCursor, "publication-") {
		t.Fatalf("inactive proof/cursor contract = cursor:%q body:%s", firstPage.NextCursor, body)
	}
	for _, forbidden := range []string{"activatedBy", "\"host\"", "manifestJson", "signature"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("publication context leaked %q: %s", forbidden, body)
		}
	}

	store.resetCounts()
	secondResponse := publicationHistoryRequest(handler, owner, "?cursor="+firstPage.NextCursor)
	var secondPage PublicationContextPage
	if secondResponse.Code != http.StatusOK || json.Unmarshal(secondResponse.Body.Bytes(), &secondPage) != nil || len(secondPage.Publications) != 20 {
		t.Fatalf("cursor page status=%d body=%s", secondResponse.Code, secondResponse.Body.String())
	}
	seen := make(map[string]bool, len(firstPage.Publications))
	for _, publication := range firstPage.Publications {
		seen[publication.ID] = true
	}
	for _, publication := range secondPage.Publications {
		if seen[publication.ID] {
			t.Fatalf("opaque cursor repeated publication %q", publication.ID)
		}
	}
	if opens, _ := store.counts(); opens != 0 {
		t.Fatalf("inactive-only cursor page artifact opens = %d, want zero", opens)
	}

	maximumResponse := publicationHistoryRequest(handler, owner, "?limit=50")
	var maximumPage PublicationContextPage
	if maximumResponse.Code != http.StatusOK || json.Unmarshal(maximumResponse.Body.Bytes(), &maximumPage) != nil || len(maximumPage.Publications) != MaximumPublicationContextLimit || maximumPage.NextCursor == "" {
		t.Fatalf("maximum page status=%d body=%s", maximumResponse.Code, maximumResponse.Body.String())
	}

	store.resetCounts()
	exactResponse := publicationHistoryRequest(handler, owner, "?publicationId=publication-010")
	var exactPage PublicationContextPage
	if exactResponse.Code != http.StatusOK || json.Unmarshal(exactResponse.Body.Bytes(), &exactPage) != nil || len(exactPage.Publications) != 1 {
		t.Fatalf("exact publication status=%d body=%s", exactResponse.Code, exactResponse.Body.String())
	}
	exact := exactPage.Publications[0]
	if !exactPage.ServerTime.Equal(now) {
		t.Fatalf("exact page serverTime = %s, want %s", exactPage.ServerTime, now)
	}
	wantDigest := sha256.Sum256(store.results[exact.ContentHash].ManifestJSON)
	if exact.ServingState != ServingStateInactive || exact.ManifestDigest == nil || *exact.ManifestDigest != hex.EncodeToString(wantDigest[:]) || exact.AuthorizationExpiresAt == nil || exact.TrackBinding != nil {
		t.Fatalf("exact inactive proof/binding = %+v", exact)
	}
	if opens, _ := store.counts(); opens != 1 {
		t.Fatalf("exact selection artifact opens = %d, want one", opens)
	}

	for _, rawQuery := range []string{
		"?limit=51", "?limit=0", "?limit=20&limit=21", "?cursor=%25%25%25", "?publicationId=publication-010&limit=20",
		"?publicationId=publication-010&cursor=opaque", "?unknown=value", "?publicationId=bad%2Fid",
	} {
		response := publicationHistoryRequest(handler, owner, rawQuery)
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "invalid_publication_query") {
			t.Fatalf("query %q status=%d body=%s", rawQuery, response.Code, response.Body.String())
		}
	}

	resolverCalls := bindings.calls
	store.resetCounts()
	viewer := &models.User{ID: 99, Role: models.RoleUser}
	forbidden := publicationHistoryRequest(handler, viewer, "?publicationId=publication-does-not-exist")
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("unauthorized exact lookup status=%d body=%s", forbidden.Code, forbidden.Body.String())
	}
	if bindings.calls != resolverCalls {
		t.Fatalf("unauthorized exact lookup reached binding resolver: calls %d -> %d", resolverCalls, bindings.calls)
	}
	if opens, _ := store.counts(); opens != 0 {
		t.Fatalf("unauthorized exact lookup opened %d artifacts", opens)
	}
}

func TestPublicationExpiryBoundaryIsFailClosedForContextAndHost(t *testing.T) {
	expiresAt := time.Date(2026, 8, 22, 15, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name          string
		now           time.Time
		state         ServingState
		hostStatus    int
		servesOldBody bool
	}{
		{name: "one millisecond before", now: expiresAt.Add(-time.Millisecond), state: ServingStateServing, hostStatus: http.StatusOK, servesOldBody: true},
		{name: "exact boundary", now: expiresAt, state: ServingStateExpired, hostStatus: http.StatusServiceUnavailable},
		{name: "one millisecond after", now: expiresAt.Add(time.Millisecond), state: ServingStateExpired, hostStatus: http.StatusServiceUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := openDomainTestDB(t)
			now := test.now
			store := &publicationLifetimeArtifacts{results: map[string]artifacts.BuildResult{}, files: map[string]map[string]artifacts.ArtifactFile{}}
			service := newPublicationLifetimeService(t, db, store, &now)
			insertPublicationLifetimeClaim(t, db, "claim-boundary", expiresAt.Add(90*24*time.Hour))
			hash := strings.Repeat("a", 64)
			store.results[hash] = publicationLifetimeBuild(hash, "artifact-boundary", expiresAt, []byte("{\"exact\":\"stored manifest bytes\"}\n"))
			store.files[hash] = publicationLifetimeFiles([]byte("old publication body"))
			insertPublicationLifetimeRow(t, db, "publication-boundary", "claim-boundary", hash, "artifact-boundary", expiresAt.Add(-time.Hour), nil, "")

			contexts, err := NewPublicationContextService(service, &publicationBindingResolverStub{bindings: map[string][]PublicationTrackBinding{}})
			if err != nil {
				t.Fatal(err)
			}
			page, err := contexts.Page(context.Background(), &models.User{ID: 1}, 1, "claim-boundary", PublicationContextQuery{PublicationID: "publication-boundary"})
			if err != nil || len(page.Publications) != 1 || page.Publications[0].ServingState != test.state || !page.Publications[0].Active {
				t.Fatalf("context at boundary = %+v err=%v", page, err)
			}

			public, err := NewPublicHandler(service, []string{"http://localhost:8080"}, http.NotFoundHandler())
			if err != nil {
				t.Fatal(err)
			}
			response := publicationHostRequest(public, http.MethodGet, "/")
			if response.Code != test.hostStatus {
				t.Fatalf("Host status = %d body=%q, want %d", response.Code, response.Body.String(), test.hostStatus)
			}
			if strings.Contains(response.Body.String(), "old publication body") != test.servesOldBody {
				t.Fatalf("Host old-body serving = %t, want %t; body=%q", strings.Contains(response.Body.String(), "old publication body"), test.servesOldBody, response.Body.String())
			}
		})
	}
}

func TestServingStateRetainsActivationFactAcrossClaimAndArtifactFailures(t *testing.T) {
	now := time.Date(2026, 8, 21, 15, 30, 0, 0, time.UTC)
	db := openDomainTestDB(t)
	store := &publicationLifetimeArtifacts{results: map[string]artifacts.BuildResult{}, files: map[string]map[string]artifacts.ArtifactFile{}, openError: map[string]error{}}
	service := newPublicationLifetimeService(t, db, store, &now)
	insertPublicationLifetimeClaim(t, db, "claim-serving-state", now.Add(90*24*time.Hour))
	hash := strings.Repeat("9", 64)
	valid := publicationLifetimeBuild(hash, "artifact-serving-state", now.Add(24*time.Hour), []byte("{\"servingState\":true}\n"))
	store.results[hash] = valid
	store.files[hash] = publicationLifetimeFiles([]byte("serving-state body"))
	insertPublicationLifetimeRow(t, db, "publication-serving-state", "claim-serving-state", hash, "artifact-serving-state", now.Add(-time.Hour), nil, "")
	contexts, err := NewPublicationContextService(service, &publicationBindingResolverStub{bindings: map[string][]PublicationTrackBinding{}})
	if err != nil {
		t.Fatal(err)
	}
	public, err := NewPublicHandler(service, []string{"http://localhost:8080"}, http.NotFoundHandler())
	if err != nil {
		t.Fatal(err)
	}
	owner := &models.User{ID: 1}
	assertClaimUnavailable := func(t *testing.T) {
		t.Helper()
		store.resetCounts()
		page, err := contexts.Page(context.Background(), owner, 1, "claim-serving-state", PublicationContextQuery{PublicationID: "publication-serving-state"})
		if err != nil || len(page.Publications) != 1 || page.Publications[0].ServingState != ServingStateClaimUnavailable || !page.Publications[0].Active {
			t.Fatalf("claim-unavailable context = %+v err=%v", page, err)
		}
		if opens, _ := store.counts(); opens != 0 {
			t.Fatalf("claim-unavailable proof opened %d artifacts", opens)
		}
		host := publicationHostRequest(public, http.MethodGet, "/")
		if host.Code != http.StatusNotFound || strings.Contains(host.Body.String(), "serving-state body") {
			t.Fatalf("claim-unavailable Host status=%d body=%q", host.Code, host.Body.String())
		}
		if opens, _ := store.counts(); opens != 0 {
			t.Fatalf("claim-unavailable Host opened %d artifacts", opens)
		}
	}

	if _, err := db.Exec(`UPDATE workspace_domain_claims SET status = 'revoked', revoked_at = ?, revocation_reason = 'user' WHERE id = 'claim-serving-state'`, now.Unix()); err != nil {
		t.Fatal(err)
	}
	t.Run("revoked claim", assertClaimUnavailable)
	if _, err := db.Exec(`UPDATE workspace_domain_claims SET status = 'verified', revoked_at = NULL, revocation_reason = NULL, verification_expires_at = ? WHERE id = 'claim-serving-state'`, now.Unix()); err != nil {
		t.Fatal(err)
	}
	t.Run("claim exact expiry", assertClaimUnavailable)
	if _, err := db.Exec(`UPDATE workspace_domain_claims SET verification_expires_at = ? WHERE id = 'claim-serving-state'`, now.Add(24*time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name      string
		openError error
		mutate    func(*artifacts.BuildResult)
	}{
		{name: "malformed signature", openError: artifacts.ErrInvalidSignature},
		{name: "malformed manifest", openError: errors.New("decode stored manifest")},
		{name: "invalid lifecycle", openError: errors.New("signed lifecycle is invalid")},
		{name: "workspace mismatch", mutate: func(result *artifacts.BuildResult) { result.Manifest.WorkspaceID = 2 }},
		{name: "origin mismatch", mutate: func(result *artifacts.BuildResult) {
			result.Manifest.Authorization.AllowedOrigins.Surfaces = []string{"https://attacker.example"}
		}},
		{name: "content hash mismatch", mutate: func(result *artifacts.BuildResult) { result.ContentHash = strings.Repeat("8", 64) }},
		{name: "artifact id mismatch", mutate: func(result *artifacts.BuildResult) { result.ArtifactID = "artifact-other" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			if test.mutate != nil {
				test.mutate(&candidate)
			}
			store.results[hash] = candidate
			store.openError[hash] = test.openError
			store.resetCounts()
			page, err := contexts.Page(context.Background(), owner, 1, "claim-serving-state", PublicationContextQuery{PublicationID: "publication-serving-state"})
			if err != nil || len(page.Publications) != 1 || page.Publications[0].ServingState != ServingStateArtifactInvalid || !page.Publications[0].Active ||
				page.Publications[0].AuthorizationExpiresAt != nil || page.Publications[0].ManifestDigest != nil {
				t.Fatalf("artifact-invalid context = %+v err=%v", page, err)
			}
			if opens, _ := store.counts(); opens != 1 {
				t.Fatalf("artifact-invalid proof opened %d artifacts, want one", opens)
			}
			host := publicationHostRequest(public, http.MethodGet, "/")
			if host.Code != http.StatusServiceUnavailable || strings.Contains(host.Body.String(), "serving-state body") {
				t.Fatalf("artifact-invalid Host status=%d body=%q", host.Code, host.Body.String())
			}
			delete(store.openError, hash)
		})
	}
}

func TestClaimRevocationBetweenArtifactPreflightAndActivationCommitsNothing(t *testing.T) {
	now := time.Date(2026, 8, 21, 15, 45, 0, 0, time.UTC)
	db := openDomainTestDB(t)
	store := &publicationLifetimeArtifacts{results: map[string]artifacts.BuildResult{}, files: map[string]map[string]artifacts.ArtifactFile{}, openError: map[string]error{}}
	service := newPublicationLifetimeService(t, db, store, &now)
	insertPublicationLifetimeClaim(t, db, "claim-race", now.Add(90*24*time.Hour))
	predecessorHash, replacementHash := strings.Repeat("6", 64), strings.Repeat("7", 64)
	store.results[predecessorHash] = publicationLifetimeBuild(predecessorHash, "artifact-race-old", now.Add(24*time.Hour), []byte("{\"race\":\"old\"}\n"))
	store.results[replacementHash] = publicationLifetimeBuild(replacementHash, "artifact-race-new", now.Add(24*time.Hour), []byte("{\"race\":\"new\"}\n"))
	insertPublicationLifetimeRow(t, db, "publication-race-old", "claim-race", predecessorHash, "artifact-race-old", now.Add(-time.Hour), nil, "")
	var revokeErr error
	store.openHook = func() {
		_, revokeErr = db.Exec(`UPDATE workspace_domain_claims SET status = 'revoked', revoked_at = ?, revocation_reason = 'user' WHERE id = 'claim-race'`, now.Unix())
	}
	publication, err := service.Publish(context.Background(), &models.User{ID: 1}, 1, "claim-race", replacementHash)
	if revokeErr != nil || !errors.Is(err, ErrOriginNotVerified) || publication.ID != "" {
		t.Fatalf("Publish() claim race = %+v err=%v revokeErr=%v", publication, err, revokeErr)
	}
	var count int
	var deactivated sql.NullInt64
	if err := db.QueryRow(`SELECT COUNT(*), MAX(deactivated_at) FROM workspace_domain_publications WHERE claim_id = 'claim-race'`).Scan(&count, &deactivated); err != nil {
		t.Fatal(err)
	}
	if count != 1 || deactivated.Valid {
		t.Fatalf("claim race committed publication/deactivation: count=%d deactivated=%v", count, deactivated)
	}
}

func TestPublicationHostGETAndHEADUseOneVerifiedOpenAndExactHeaders(t *testing.T) {
	now := time.Date(2026, 8, 21, 16, 0, 0, 0, time.UTC)
	db := openDomainTestDB(t)
	store := &publicationLifetimeArtifacts{results: map[string]artifacts.BuildResult{}, files: map[string]map[string]artifacts.ArtifactFile{}}
	service := newPublicationLifetimeService(t, db, store, &now)
	insertPublicationLifetimeClaim(t, db, "claim-host", now.Add(90*24*time.Hour))
	hash := strings.Repeat("b", 64)
	store.results[hash] = publicationLifetimeBuild(hash, "artifact-host", now.Add(24*time.Hour), []byte("{\"host\":true}\n"))
	store.files[hash] = publicationLifetimeFiles([]byte("index body"))
	insertPublicationLifetimeRow(t, db, "publication-host", "claim-host", hash, "artifact-host", now, nil, "")
	public, err := NewPublicHandler(service, []string{"http://localhost:8080"}, http.NotFoundHandler())
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		method      string
		path        string
		filePath    string
		wantBody    string
		contentType string
		cspPart     string
	}{
		{method: http.MethodGet, path: "/", filePath: "index.html", wantBody: "index body", contentType: "text/html; charset=utf-8", cspPart: "frame-ancestors 'none'"},
		{method: http.MethodHead, path: "/", filePath: "index.html", contentType: "text/html; charset=utf-8", cspPart: "frame-ancestors 'none'"},
		{method: http.MethodGet, path: "/embed", filePath: "embed.html", wantBody: "embed body", contentType: "text/html; charset=utf-8", cspPart: "frame-ancestors https://app.example"},
		{method: http.MethodHead, path: "/assets/app.js", filePath: "assets/app.js", contentType: "text/javascript; charset=utf-8"},
	} {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			store.resetCounts()
			response := publicationHostRequest(public, test.method, test.path)
			if response.Code != http.StatusOK || response.Body.String() != test.wantBody {
				t.Fatalf("response status=%d body=%q", response.Code, response.Body.String())
			}
			if response.Header().Get("Content-Type") != test.contentType || response.Header().Get("Cache-Control") != "no-store" ||
				response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Header().Get("Referrer-Policy") != "no-referrer" ||
				response.Header().Get("ETag") != `"`+store.files[hash][test.filePath].SHA256+`"` || response.Header().Get("Content-Length") == "" {
				t.Fatalf("Host headers = %v", response.Header())
			}
			if test.cspPart != "" && !strings.Contains(response.Header().Get("Content-Security-Policy"), test.cspPart) {
				t.Fatalf("Host CSP = %q, want %q", response.Header().Get("Content-Security-Policy"), test.cspPart)
			}
			if test.cspPart == "" && response.Header().Get("Content-Security-Policy") != "" {
				t.Fatalf("non-HTML response had CSP %q", response.Header().Get("Content-Security-Policy"))
			}
			if opens, reads := store.counts(); opens != 1 || reads != 1 {
				t.Fatalf("Host artifact operations open=%d read=%d, want 1/1", opens, reads)
			}
		})
	}
}

func TestExpiredPredecessorIsNotServedUntilAtomicSuccessorActivation(t *testing.T) {
	expiresAt := time.Date(2026, 8, 22, 17, 0, 0, 0, time.UTC)
	now := expiresAt
	db := openDomainTestDB(t)
	store := &publicationLifetimeArtifacts{results: map[string]artifacts.BuildResult{}, files: map[string]map[string]artifacts.ArtifactFile{}}
	service := newPublicationLifetimeService(t, db, store, &now)
	insertPublicationLifetimeClaim(t, db, "claim-successor", now.Add(90*24*time.Hour))
	oldHash, newHash := strings.Repeat("c", 64), strings.Repeat("d", 64)
	store.results[oldHash] = publicationLifetimeBuild(oldHash, "artifact-old", expiresAt, []byte("{\"generation\":\"old\"}\n"))
	store.files[oldHash] = publicationLifetimeFiles([]byte("old immutable bytes"))
	store.results[newHash] = publicationLifetimeBuild(newHash, "artifact-new", now.Add(90*24*time.Hour), []byte("{\"generation\":\"new\"}\n"))
	store.files[newHash] = publicationLifetimeFiles([]byte("successor bytes"))
	insertPublicationLifetimeRow(t, db, "publication-old", "claim-successor", oldHash, "artifact-old", expiresAt.Add(-24*time.Hour), nil, "")
	public, err := NewPublicHandler(service, []string{"http://localhost:8080"}, http.NotFoundHandler())
	if err != nil {
		t.Fatal(err)
	}
	before := publicationHostRequest(public, http.MethodGet, "/")
	if before.Code == http.StatusOK || strings.Contains(before.Body.String(), "old immutable bytes") || strings.Contains(before.Body.String(), "successor bytes") {
		t.Fatalf("expired predecessor served before replacement: status=%d body=%q", before.Code, before.Body.String())
	}

	replacement, err := service.Publish(context.Background(), &models.User{ID: 1}, 1, "claim-successor", newHash)
	if err != nil || replacement.ContentHash != newHash || replacement.SourcePublicationID != "publication-old" || !replacement.Active {
		t.Fatalf("Publish(successor) = %+v err=%v", replacement, err)
	}
	replayed, err := service.Publish(context.Background(), &models.User{ID: 1}, 1, "claim-successor", newHash)
	if err != nil || replayed.ID != replacement.ID || replayed.SourcePublicationID != replacement.SourcePublicationID {
		t.Fatalf("idempotent Publish(successor) = %+v err=%v, want publication %s", replayed, err, replacement.ID)
	}
	after := publicationHostRequest(public, http.MethodGet, "/")
	if after.Code != http.StatusOK || after.Body.String() != "successor bytes" {
		t.Fatalf("successor Host response status=%d body=%q", after.Code, after.Body.String())
	}
	var oldDeactivated sql.NullInt64
	var oldArtifactID, oldContentHash string
	if err := db.QueryRow(`SELECT artifact_id, content_hash, deactivated_at FROM workspace_domain_publications WHERE id = 'publication-old'`).Scan(&oldArtifactID, &oldContentHash, &oldDeactivated); err != nil {
		t.Fatal(err)
	}
	if oldArtifactID != "artifact-old" || oldContentHash != oldHash || !oldDeactivated.Valid {
		t.Fatalf("predecessor was rewritten instead of retained: artifact=%q hash=%q deactivated=%v", oldArtifactID, oldContentHash, oldDeactivated)
	}
	var publicationCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM workspace_domain_publications WHERE claim_id = 'claim-successor'`).Scan(&publicationCount); err != nil || publicationCount != 2 {
		t.Fatalf("idempotent successor publication count=%d err=%v, want 2", publicationCount, err)
	}
}

func TestReplacementAndRollbackAppendImmutableLineage(t *testing.T) {
	now := time.Date(2026, 8, 21, 17, 30, 0, 0, time.UTC)
	db := openDomainTestDB(t)
	store := &publicationLifetimeArtifacts{results: map[string]artifacts.BuildResult{}, files: map[string]map[string]artifacts.ArtifactFile{}}
	service := newPublicationLifetimeService(t, db, store, &now)
	insertPublicationLifetimeClaim(t, db, "claim-lineage", now.Add(90*24*time.Hour))
	firstHash, secondHash := strings.Repeat("1", 64), strings.Repeat("2", 64)
	store.results[firstHash] = publicationLifetimeBuild(firstHash, "artifact-lineage-one", now.Add(90*24*time.Hour), []byte("{\"lineage\":1}\n"))
	store.results[secondHash] = publicationLifetimeBuild(secondHash, "artifact-lineage-two", now.Add(90*24*time.Hour), []byte("{\"lineage\":2}\n"))
	store.files[firstHash] = publicationLifetimeFiles([]byte("lineage one"))
	store.files[secondHash] = publicationLifetimeFiles([]byte("lineage two"))
	owner := &models.User{ID: 1}
	first, err := service.Publish(context.Background(), owner, 1, "claim-lineage", firstHash)
	if err != nil {
		t.Fatalf("Publish(first) error = %v", err)
	}
	now = now.Add(time.Second)
	second, err := service.Publish(context.Background(), owner, 1, "claim-lineage", secondHash)
	if err != nil || second.SourcePublicationID != first.ID {
		t.Fatalf("Publish(replacement) = %+v err=%v", second, err)
	}
	now = now.Add(time.Second)
	rolledBack, err := service.Activate(context.Background(), owner, 1, "claim-lineage", first.ID)
	if err != nil || rolledBack.ID == first.ID || rolledBack.ID == second.ID || rolledBack.SourcePublicationID != first.ID || rolledBack.ContentHash != firstHash || !rolledBack.Active {
		t.Fatalf("Activate(rollback) = %+v err=%v", rolledBack, err)
	}
	rows, err := db.Query(`SELECT id, content_hash, artifact_id, deactivated_at FROM workspace_domain_publications WHERE claim_id = ? ORDER BY activated_at, id`, "claim-lineage")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	type immutableRow struct {
		id, hash, artifactID string
		deactivated          sql.NullInt64
	}
	var history []immutableRow
	for rows.Next() {
		var row immutableRow
		if err := rows.Scan(&row.id, &row.hash, &row.artifactID, &row.deactivated); err != nil {
			t.Fatal(err)
		}
		history = append(history, row)
	}
	if len(history) != 3 {
		t.Fatalf("immutable activation facts = %+v", history)
	}
	want := map[string][2]string{
		first.ID:      {firstHash, "artifact-lineage-one"},
		second.ID:     {secondHash, "artifact-lineage-two"},
		rolledBack.ID: {firstHash, "artifact-lineage-one"},
	}
	activeCount := 0
	for _, row := range history {
		if [2]string{row.hash, row.artifactID} != want[row.id] {
			t.Fatalf("publication %s bytes linkage changed: hash=%s artifact=%s", row.id, row.hash, row.artifactID)
		}
		if !row.deactivated.Valid {
			activeCount++
		}
	}
	if activeCount != 1 {
		t.Fatalf("active activation facts = %d, want one", activeCount)
	}
}

func newPublicationLifetimeService(t *testing.T, db *sql.DB, store ArtifactStore, now *time.Time) *Service {
	t.Helper()
	service, err := NewService(db, fakeWorkspaceAuthorizer{roles: map[int]string{1: models.WorkspaceRoleOwner}}, Options{
		Artifacts: store, Resolver: &fakeResolver{values: map[string][]string{}},
		Random: bytes.NewReader(publicationLifetimeEntropy()), Now: func() time.Time { return *now },
		ChallengeTTL: time.Hour, VerificationTTL: 90 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

func publicationLifetimeEntropy() []byte {
	value := make([]byte, 4096)
	for index := range value {
		value[index] = byte(index)
	}
	return value
}

func insertPublicationLifetimeClaim(t *testing.T, db *sql.DB, claimID string, verificationExpiresAt time.Time) {
	t.Helper()
	createdAt := verificationExpiresAt.Add(-24 * time.Hour).Unix()
	_, err := db.Exec(`INSERT INTO workspace_domain_claims
		(id, workspace_id, origin, host, status, challenge_hash, challenge_expires_at,
		 verified_at, verification_expires_at, claimed_by, verified_by, created_at, updated_at)
		VALUES (?, 1, 'https://publication.example', 'publication.example', 'verified', ?, ?, ?, ?, 1, 1, ?, ?)`,
		claimID, []byte{1}, verificationExpiresAt.Unix(), createdAt, verificationExpiresAt.Unix(), createdAt, createdAt)
	if err != nil {
		t.Fatalf("insert verified claim: %v", err)
	}
}

func insertPublicationLifetimeRow(t *testing.T, db *sql.DB, publicationID, claimID, contentHash, artifactID string, activatedAt time.Time, deactivatedAt any, sourcePublicationID string) {
	t.Helper()
	var source any
	if sourcePublicationID != "" {
		source = sourcePublicationID
	}
	_, err := db.Exec(`INSERT INTO workspace_domain_publications
		(id, workspace_id, claim_id, origin, host, content_hash, artifact_id, source_publication_id, activated_by, activated_at, deactivated_at)
		VALUES (?, 1, ?, 'https://publication.example', 'publication.example', ?, ?, ?, 1, ?, ?)`,
		publicationID, claimID, contentHash, artifactID, source, activatedAt.Unix(), deactivatedAt)
	if err != nil {
		t.Fatalf("insert publication %s: %v", publicationID, err)
	}
}

func publicationLifetimeBuild(contentHash, artifactID string, expiresAt time.Time, manifestJSON []byte) artifacts.BuildResult {
	return artifacts.BuildResult{
		ArtifactID: artifactID, ContentHash: contentHash, ManifestJSON: append([]byte(nil), manifestJSON...),
		Manifest: artifacts.Manifest{
			ArtifactID: artifactID, ContentHash: contentHash, WorkspaceID: 1,
			Authorization: artifacts.BundleAuthorization{
				Subject: artifacts.BundleSubject{WorkspaceID: 1}, AllowedOrigins: artifacts.OriginPolicy{Surfaces: []string{"https://publication.example"}},
				ApprovedDomains: []string{"publication.example"}, ExpiresAt: expiresAt, Lifecycle: artifacts.BundleLifecyclePublished,
			},
			Security: artifacts.SecurityPolicy{CSP: []artifacts.RenderModeCSP{
				{RenderMode: artifacts.RenderModeStandalone, Policy: artifacts.CSPPolicy{DefaultSrc: []string{"'self'"}, ObjectSrc: []string{"'none'"}, BaseURI: []string{"'none'"}, FrameAncestors: []string{"'none'"}}},
				{RenderMode: artifacts.RenderModeEmbed, Policy: artifacts.CSPPolicy{DefaultSrc: []string{"'self'"}, ObjectSrc: []string{"'none'"}, BaseURI: []string{"'none'"}, FrameAncestors: []string{"https://app.example"}}},
			}},
		},
	}
}

func publicationLifetimeFiles(index []byte) map[string]artifacts.ArtifactFile {
	files := map[string][]byte{
		"index.html":    index,
		"embed.html":    []byte("embed body"),
		"assets/app.js": []byte("application javascript body"),
	}
	result := make(map[string]artifacts.ArtifactFile, len(files))
	for filePath, contents := range files {
		digest := sha256.Sum256(contents)
		result[filePath] = artifacts.ArtifactFile{Path: filePath, Contents: contents, SHA256: hex.EncodeToString(digest[:])}
	}
	return result
}

func publicationHistoryRequest(handler *HTTPHandler, actor *models.User, rawQuery string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/api/workspaces/1/domains/claim-context/publications"+rawQuery, nil)
	request = mux.SetURLVars(request, map[string]string{"id": "1", "claim_id": "claim-context"})
	request = request.WithContext(context.WithValue(request.Context(), publicationLifetimeActorKey{}, actor))
	original := handler.currentUser
	handler.currentUser = func(ctx context.Context) (*models.User, bool) {
		resolved, ok := ctx.Value(publicationLifetimeActorKey{}).(*models.User)
		return resolved, ok
	}
	response := httptest.NewRecorder()
	handler.PublicationHistory(response, request)
	handler.currentUser = original
	return response
}

type publicationLifetimeActorKey struct{}

func publicationHostRequest(handler http.Handler, method, requestPath string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, "http://publication.example"+requestPath, nil)
	request.Host = "publication.example"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
