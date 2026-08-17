package oauth

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"taawun/pkg/models"
)

type fakeIdentity struct{ user *models.User }

func (f fakeIdentity) Authenticate(email, password string) (*models.User, error) {
	if email != f.user.Email || password != "correct horse battery staple" {
		return nil, ErrInvalidGrant
	}
	copy := *f.user
	return &copy, nil
}

type fakeUsers struct{ user *models.User }

func (f fakeUsers) GetByID(id int) (*models.User, error) {
	if id != f.user.ID {
		return nil, nil
	}
	copy := *f.user
	return &copy, nil
}

type fakeWorkspaces struct{ workspace *models.Workspace }

func (f fakeWorkspaces) GetWorkspaces(*models.User) ([]*models.Workspace, error) {
	return []*models.Workspace{f.workspace}, nil
}

func (f fakeWorkspaces) AuthorizeWorkspaceCapability(_ *models.User, id int, _ models.WorkspaceCapability) (*models.Workspace, error) {
	if id != f.workspace.ID {
		return nil, ErrInvalidGrant
	}
	return f.workspace, nil
}

type oauthFixture struct {
	db      *sql.DB
	service *Service
	handler *HTTPHandler
	client  *Client
	user    *models.User
	now     time.Time
}

func newOAuthFixture(t *testing.T) *oauthFixture {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "oauth.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, status TEXT); INSERT INTO users (id, status) VALUES (7, 'active')`); err != nil {
		t.Fatal(err)
	}
	repository, err := NewRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 17, 5, 0, 0, 0, time.UTC)
	user := &models.User{ID: 7, Email: "owner@example.com", Status: models.StatusActive, SessionVersion: 1}
	workspace := &models.Workspace{ID: 42, Name: "Relief Fund", OwnerID: 7, Status: models.WorkspaceStatusActive}
	service, err := NewService(repository, fakeIdentity{user}, fakeUsers{user}, fakeWorkspaces{workspace}, func(ctx context.Context, _ *models.User) context.Context { return ctx }, Config{
		Issuer: "https://taawun.example", Resource: "https://taawun.example/mcp", Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	client, err := service.RegisterClient(Client{Name: "Desktop MCP", RedirectURIs: []string{"http://127.0.0.1:49152/callback"}})
	if err != nil {
		t.Fatal(err)
	}
	return &oauthFixture{db: db, service: service, handler: NewHTTPHandler(service), client: client, user: user, now: now}
}

func (f *oauthFixture) authorize(t *testing.T, scopes string) (code, verifier string) {
	t.Helper()
	verifier = strings.Repeat("v", 64)
	challenge, err := pkceChallenge(verifier)
	if err != nil {
		t.Fatal(err)
	}
	requestID, err := f.service.BeginAuthorization(AuthorizationInput{
		ClientID: f.client.ID, RedirectURI: f.client.RedirectURIs[0], ResponseType: "code", Scope: scopes,
		State: "state-123", Resource: f.service.config.Resource, CodeChallenge: challenge, CodeChallengeMethod: "S256",
	})
	if err != nil {
		t.Fatal(err)
	}
	session, csrf, err := f.service.LoginAuthorization(requestID, "owner@example.com", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	redirect, err := f.service.Approve(requestID, session, csrf, []int{42})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(redirect)
	if parsed.Query().Get("state") != "state-123" || parsed.Query().Get("iss") != f.service.config.Issuer {
		t.Fatalf("authorization redirect = %s", redirect)
	}
	return parsed.Query().Get("code"), verifier
}

func (f *oauthFixture) issue(t *testing.T, scopes string) *TokenResponse {
	t.Helper()
	code, verifier := f.authorize(t, scopes)
	tokens, err := f.service.ExchangeCode(code, f.client.ID, f.client.RedirectURIs[0], f.service.config.Resource, verifier)
	if err != nil {
		t.Fatal(err)
	}
	return tokens
}

func TestDiscoveryRegistrationAndExactRedirectValidation(t *testing.T) {
	fixture := newOAuthFixture(t)
	request := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	response := httptest.NewRecorder()
	fixture.handler.AuthorizationServerMetadata(response, request)
	var metadata map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &metadata); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || metadata["client_id_metadata_document_supported"] != true || metadata["registration_endpoint"] == "" {
		t.Fatalf("authorization metadata = %#v", metadata)
	}

	resourceResponse := httptest.NewRecorder()
	fixture.handler.ProtectedResourceMetadata(resourceResponse, request)
	if !strings.Contains(resourceResponse.Body.String(), `"resource":"https://taawun.example/mcp"`) {
		t.Fatalf("resource metadata = %s", resourceResponse.Body.String())
	}

	if _, err := fixture.service.RegisterClient(Client{Name: "Wildcard", RedirectURIs: []string{"https://*.example/callback"}}); err == nil {
		t.Fatal("wildcard redirect registration succeeded")
	}
	verifier := strings.Repeat("v", 64)
	challenge, _ := pkceChallenge(verifier)
	_, err := fixture.service.BeginAuthorization(AuthorizationInput{ClientID: fixture.client.ID, RedirectURI: "http://127.0.0.1:49153/callback", ResponseType: "code", Scope: ScopeRead, State: "state", Resource: fixture.service.config.Resource, CodeChallenge: challenge, CodeChallengeMethod: "S256"})
	if err == nil {
		t.Fatal("unregistered redirect URI was accepted")
	}
}

func TestLoginConsentUsesSecureCookieAndDisclosesRedirectHost(t *testing.T) {
	fixture := newOAuthFixture(t)
	verifier := strings.Repeat("v", 64)
	challenge, _ := pkceChallenge(verifier)
	requestID, err := fixture.service.BeginAuthorization(AuthorizationInput{
		ClientID: fixture.client.ID, RedirectURI: fixture.client.RedirectURIs[0], ResponseType: "code",
		Scope: ScopeRead, State: "state", Resource: fixture.service.config.Resource,
		CodeChallenge: challenge, CodeChallengeMethod: "S256",
	})
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{"request_id": {requestID}, "email": {"owner@example.com"}, "password": {"correct horse battery staple"}}
	request := httptest.NewRequest(http.MethodPost, "/oauth/login", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	fixture.handler.Login(response, request)
	cookie := response.Header().Get("Set-Cookie")
	if !strings.Contains(cookie, "Secure") || !strings.Contains(cookie, "HttpOnly") || !strings.Contains(cookie, "SameSite=Lax") || !strings.Contains(cookie, "__Host-taawun-oauth-session=") {
		t.Fatalf("session cookie = %q", cookie)
	}
	body := response.Body.String()
	for _, required := range []string{"Desktop MCP", "127.0.0.1:49152", "Loopback warning"} {
		if !strings.Contains(body, required) {
			t.Fatalf("consent page does not contain %q", required)
		}
	}
}

func TestOAuthPublicLoginAndRegistrationAreBounded(t *testing.T) {
	fixture := newOAuthFixture(t)
	limiter := newOAuthPublicRateLimiter(time.Minute, 16)
	limiter.now = func() time.Time { return fixture.now }
	fixture.handler.publicLimiter = limiter

	registration := []byte(`{"client_name":"Bounded desktop","redirect_uris":["http://127.0.0.1:49200/callback"]}`)
	for attempt := 0; attempt < OAuthRegistrationAttemptLimit; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/oauth/register", bytes.NewReader(registration))
		request.Header.Set("Content-Type", "application/json")
		request.RemoteAddr = "203.0.113.10:4444"
		response := httptest.NewRecorder()
		fixture.handler.Register(response, request)
		if response.Code != http.StatusCreated {
			t.Fatalf("registration attempt %d status = %d body=%s", attempt+1, response.Code, response.Body.String())
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/oauth/register", bytes.NewReader(registration))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "203.0.113.10:4444"
	response := httptest.NewRecorder()
	fixture.handler.Register(response, request)
	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "60" {
		t.Fatalf("registration rate limit = %d Retry-After=%q", response.Code, response.Header().Get("Retry-After"))
	}

	requestID, err := fixture.service.BeginAuthorization(AuthorizationInput{
		ClientID: fixture.client.ID, RedirectURI: fixture.client.RedirectURIs[0], ResponseType: "code", Scope: ScopeRead,
		State: "state-login-limit", Resource: fixture.service.config.Resource, CodeChallenge: strings.Repeat("a", 43), CodeChallengeMethod: "S256",
	})
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{"request_id": {requestID}, "email": {fixture.user.Email}, "password": {"not the password"}}
	for attempt := 0; attempt < OAuthLoginAttemptLimit; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/oauth/login", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.RemoteAddr = "203.0.113.11:4444"
		response := httptest.NewRecorder()
		fixture.handler.Login(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("login attempt %d status = %d", attempt+1, response.Code)
		}
	}
	request = httptest.NewRequest(http.MethodPost, "/oauth/login", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.RemoteAddr = "203.0.113.11:4444"
	response = httptest.NewRecorder()
	fixture.handler.Login(response, request)
	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "60" {
		t.Fatalf("login rate limit = %d Retry-After=%q", response.Code, response.Header().Get("Retry-After"))
	}

	overdue := append([]byte(`{"client_name":"`), []byte(strings.Repeat("a", maximumPublicOAuthBodyBytes+1))...)
	overdue = append(overdue, []byte(`","redirect_uris":["http://127.0.0.1:49201/callback"]}`)...)
	request = httptest.NewRequest(http.MethodPost, "/oauth/register", bytes.NewReader(overdue))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "203.0.113.12:4444"
	response = httptest.NewRecorder()
	fixture.handler.Register(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized registration status = %d", response.Code)
	}
}

func TestOAuthDurableCapsAndExpiryCleanup(t *testing.T) {
	t.Run("client registration", func(t *testing.T) {
		fixture := newOAuthFixture(t)
		fixture.service.repository.maximumDynamicClients = 1
		request := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(`{"client_name":"Capacity client","redirect_uris":["http://127.0.0.1:49202/callback"]}`))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		fixture.handler.Register(response, request)
		if response.Code != http.StatusServiceUnavailable || response.Header().Get("Retry-After") != "3600" {
			t.Fatalf("registration capacity status = %d Retry-After=%q", response.Code, response.Header().Get("Retry-After"))
		}
	})

	t.Run("browser session", func(t *testing.T) {
		fixture := newOAuthFixture(t)
		fixture.service.repository.maximumActiveSessionsPerUser = 1
		newRequest := func(state string) string {
			t.Helper()
			requestID, err := fixture.service.BeginAuthorization(AuthorizationInput{
				ClientID: fixture.client.ID, RedirectURI: fixture.client.RedirectURIs[0], ResponseType: "code", Scope: ScopeRead,
				State: state, Resource: fixture.service.config.Resource, CodeChallenge: strings.Repeat("a", 43), CodeChallengeMethod: "S256",
			})
			if err != nil {
				t.Fatal(err)
			}
			return requestID
		}
		if _, _, err := fixture.service.LoginAuthorization(newRequest("first"), fixture.user.Email, "correct horse battery staple"); err != nil {
			t.Fatal(err)
		}
		if _, _, err := fixture.service.LoginAuthorization(newRequest("second"), fixture.user.Email, "correct horse battery staple"); !errors.Is(err, ErrCapacity) {
			t.Fatalf("second browser session error = %v, want ErrCapacity", err)
		}
	})

	t.Run("expiry cleanup", func(t *testing.T) {
		fixture := newOAuthFixture(t)
		newRequest := func(state string) string {
			t.Helper()
			requestID, err := fixture.service.BeginAuthorization(AuthorizationInput{
				ClientID: fixture.client.ID, RedirectURI: fixture.client.RedirectURIs[0], ResponseType: "code", Scope: ScopeRead,
				State: state, Resource: fixture.service.config.Resource, CodeChallenge: strings.Repeat("a", 43), CodeChallengeMethod: "S256",
			})
			if err != nil {
				t.Fatal(err)
			}
			return requestID
		}
		activeRequest := newRequest("active")
		activeSession, _, err := fixture.service.LoginAuthorization(activeRequest, fixture.user.Email, "correct horse battery staple")
		if err != nil {
			t.Fatal(err)
		}
		staleRequest := newRequest("stale")
		staleSession, _, err := fixture.service.LoginAuthorization(staleRequest, fixture.user.Email, "correct horse battery staple")
		if err != nil {
			t.Fatal(err)
		}
		expired := fixture.now.Add(-time.Second).Unix()
		if _, err := fixture.db.Exec(`UPDATE oauth_authorization_requests SET expires_at = ? WHERE request_hash = ?`, expired, credentialHash(staleRequest)); err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.db.Exec(`UPDATE oauth_sessions SET expires_at = ? WHERE session_hash = ?`, expired, credentialHash(staleSession)); err != nil {
			t.Fatal(err)
		}
		if err := fixture.service.repository.cleanupExpired(fixture.now); err != nil {
			t.Fatal(err)
		}
		if request, err := fixture.service.repository.getAuthorizationRequest(credentialHash(staleRequest)); err != nil || request != nil {
			t.Fatalf("stale authorization request = %#v, %v", request, err)
		}
		if session, err := fixture.service.repository.getSession(credentialHash(staleSession)); err != nil || session != nil {
			t.Fatalf("stale browser session = %#v, %v", session, err)
		}
		if _, err := fixture.service.activeRequest(activeRequest); err != nil {
			t.Fatalf("active authorization request was removed: %v", err)
		}
		if _, _, err := fixture.service.sessionUser(activeSession); err != nil {
			t.Fatalf("active browser session was removed: %v", err)
		}
	})
}

func TestAuthorizationCodeOpaqueStorageAndMCPMiddleware(t *testing.T) {
	fixture := newOAuthFixture(t)
	code, verifier := fixture.authorize(t, ScopeRead+" "+ScopeBuild)
	tokens, err := fixture.service.ExchangeCode(code, fixture.client.ID, fixture.client.RedirectURIs[0], fixture.service.config.Resource, verifier)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.ExchangeCode(code, fixture.client.ID, fixture.client.RedirectURIs[0], fixture.service.config.Resource, verifier); err == nil {
		t.Fatal("authorization code was accepted twice")
	}
	var rawStored int
	if err := fixture.db.QueryRow(`SELECT COUNT(*) FROM oauth_access_tokens WHERE token_hash = ?`, tokens.AccessToken).Scan(&rawStored); err != nil || rawStored != 0 {
		t.Fatalf("raw access token persistence count = %d, err = %v", rawStored, err)
	}
	if err := fixture.db.QueryRow(`SELECT COUNT(*) FROM oauth_refresh_tokens WHERE token_hash = ?`, tokens.RefreshToken).Scan(&rawStored); err != nil || rawStored != 0 {
		t.Fatalf("raw refresh token persistence count = %d, err = %v", rawStored, err)
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok || principal.User.ID != 7 || !WorkspaceGranted(r.Context(), 42) {
			t.Error("validated OAuth principal missing")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	recorder := httptest.NewRecorder()
	fixture.service.Middleware(next).ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("valid MCP bearer status = %d", recorder.Code)
	}

	for name, request := range map[string]*http.Request{
		"platform jwt": httptest.NewRequest(http.MethodPost, "/mcp", nil),
		"query token":  httptest.NewRequest(http.MethodPost, "/mcp?access_token="+url.QueryEscape(tokens.AccessToken), nil),
	} {
		if name == "platform jwt" {
			request.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiJ9.platform.jwt")
		}
		recorder := httptest.NewRecorder()
		fixture.service.Middleware(next).ServeHTTP(recorder, request)
		if recorder.Code < 400 {
			t.Fatalf("%s status = %d", name, recorder.Code)
		}
		if name == "platform jwt" {
			challenge := recorder.Header().Get("WWW-Authenticate")
			if !strings.Contains(challenge, "resource_metadata=") || !strings.Contains(challenge, `scope="taawun:read"`) {
				t.Fatalf("challenge = %q", challenge)
			}
		}
	}
}

func TestRefreshRotationReplayRevokesFamily(t *testing.T) {
	fixture := newOAuthFixture(t)
	initial := fixture.issue(t, ScopeRead)
	rotated, err := fixture.service.Refresh(initial.RefreshToken, fixture.client.ID, fixture.service.config.Resource)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.ValidateAccess(rotated.AccessToken); err != nil {
		t.Fatalf("rotated access token invalid before replay: %v", err)
	}
	if _, err := fixture.service.Refresh(initial.RefreshToken, fixture.client.ID, fixture.service.config.Resource); err == nil {
		t.Fatal("rotated refresh token was accepted twice")
	}
	if _, err := fixture.service.ValidateAccess(rotated.AccessToken); err == nil {
		t.Fatal("family access token survived refresh replay")
	}
}

func TestRevocationAndInsufficientScopeChallenges(t *testing.T) {
	fixture := newOAuthFixture(t)
	buildOnly := fixture.issue(t, ScopeBuild)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+buildOnly.AccessToken)
	fixture.service.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(recorder, req)
	if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Header().Get("WWW-Authenticate"), `error="insufficient_scope"`) || !strings.Contains(recorder.Header().Get("WWW-Authenticate"), "resource_metadata=") {
		t.Fatalf("insufficient scope response = %d, %q", recorder.Code, recorder.Header().Get("WWW-Authenticate"))
	}

	fixture = newOAuthFixture(t)
	tokens := fixture.issue(t, ScopeRead)
	if err := fixture.service.Revoke(tokens.RefreshToken, fixture.client.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.ValidateAccess(tokens.AccessToken); err == nil {
		t.Fatal("access token survived refresh-family revocation")
	}
}

func TestGrantUsesPersistedClientSnapshotAfterAuthorizationStarts(t *testing.T) {
	fixture := newOAuthFixture(t)
	verifier := strings.Repeat("v", 64)
	challenge, err := pkceChallenge(verifier)
	if err != nil {
		t.Fatal(err)
	}
	requestID, err := fixture.service.BeginAuthorization(AuthorizationInput{
		ClientID: fixture.client.ID, RedirectURI: fixture.client.RedirectURIs[0], ResponseType: "code", Scope: ScopeRead,
		State: "state-123", Resource: fixture.service.config.Resource, CodeChallenge: challenge, CodeChallengeMethod: "S256",
	})
	if err != nil {
		t.Fatal(err)
	}
	request, err := fixture.service.activeRequest(requestID)
	if err != nil || request == nil || request.ClientFingerprint == "" {
		t.Fatalf("authorization request snapshot = %#v, err = %v", request, err)
	}
	snapshot, err := fixture.service.repository.getClientSnapshot(request.ClientFingerprint)
	if err != nil || snapshot == nil || snapshot.Client.Name != fixture.client.Name {
		t.Fatalf("persisted snapshot = %#v, err = %v", snapshot, err)
	}
	session, csrf, err := fixture.service.LoginAuthorization(requestID, "owner@example.com", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.db.Exec(`UPDATE oauth_clients SET client_name = 'mutated', redirect_uris_json = '["https://example.invalid/callback"]' WHERE client_id = ?`, fixture.client.ID); err != nil {
		t.Fatal(err)
	}
	view, err := fixture.service.ConsentView(requestID, session, csrf)
	if err != nil || view.Client.Name != "Desktop MCP" || view.Client.RedirectURIs[0] != fixture.client.RedirectURIs[0] {
		t.Fatalf("consent view did not use frozen client metadata: %#v, err = %v", view, err)
	}
	redirect, err := fixture.service.Approve(requestID, session, csrf, []int{42})
	if err != nil {
		t.Fatal(err)
	}
	codeURL, _ := url.Parse(redirect)
	tokens, err := fixture.service.ExchangeCode(codeURL.Query().Get("code"), fixture.client.ID, fixture.client.RedirectURIs[0], fixture.service.config.Resource, verifier)
	if err != nil {
		t.Fatalf("code exchange depended on mutable client metadata: %v", err)
	}
	if _, err := fixture.service.ValidateAccess(tokens.AccessToken); err != nil {
		t.Fatalf("access validation depended on mutable client metadata: %v", err)
	}
	if _, err := fixture.service.Refresh(tokens.RefreshToken, fixture.client.ID, fixture.service.config.Resource); err != nil {
		t.Fatalf("refresh depended on mutable client metadata: %v", err)
	}
	if err := fixture.service.Revoke(tokens.RefreshToken, fixture.client.ID); err != nil {
		t.Fatalf("revocation depended on mutable client metadata: %v", err)
	}
}

func TestOAuthSessionVersionInvalidatesBrowserCodeAndTokenFamilies(t *testing.T) {
	t.Run("browser session", func(t *testing.T) {
		fixture := newOAuthFixture(t)
		verifier := strings.Repeat("v", 64)
		challenge, _ := pkceChallenge(verifier)
		requestID, err := fixture.service.BeginAuthorization(AuthorizationInput{
			ClientID: fixture.client.ID, RedirectURI: fixture.client.RedirectURIs[0], ResponseType: "code", Scope: ScopeRead,
			State: "state-123", Resource: fixture.service.config.Resource, CodeChallenge: challenge, CodeChallengeMethod: "S256",
		})
		if err != nil {
			t.Fatal(err)
		}
		session, csrf, err := fixture.service.LoginAuthorization(requestID, "owner@example.com", "correct horse battery staple")
		if err != nil {
			t.Fatal(err)
		}
		fixture.user.SessionVersion++
		if _, err := fixture.service.ConsentView(requestID, session, csrf); err == nil {
			t.Fatal("browser OAuth session survived password-session rotation")
		}
	})

	t.Run("authorization code", func(t *testing.T) {
		fixture := newOAuthFixture(t)
		code, verifier := fixture.authorize(t, ScopeRead)
		fixture.user.SessionVersion++
		if _, err := fixture.service.ExchangeCode(code, fixture.client.ID, fixture.client.RedirectURIs[0], fixture.service.config.Resource, verifier); err == nil {
			t.Fatal("authorization code survived password-session rotation")
		}
	})

	t.Run("access and refresh family", func(t *testing.T) {
		fixture := newOAuthFixture(t)
		tokens := fixture.issue(t, ScopeRead)
		fixture.user.SessionVersion++
		if _, err := fixture.service.ValidateAccess(tokens.AccessToken); err == nil {
			t.Fatal("access token survived password-session rotation")
		}
		if _, err := fixture.service.Refresh(tokens.RefreshToken, fixture.client.ID, fixture.service.config.Resource); err == nil {
			t.Fatal("refresh token survived password-session rotation")
		}
		var revoked int
		if err := fixture.db.QueryRow(`SELECT COUNT(*) FROM oauth_access_tokens WHERE revoked_at IS NOT NULL`).Scan(&revoked); err != nil {
			t.Fatal(err)
		}
		if revoked == 0 {
			t.Fatal("password-session rotation did not revoke the refresh token family")
		}
	})
}

func TestCIMDSSRFPrimitivesFailClosed(t *testing.T) {
	private := []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "::1", "fc00::1"}
	for _, raw := range private {
		if publicIP(netParseIP(t, raw)) {
			t.Fatalf("private address %s accepted", raw)
		}
	}
	if !isMetadataClientID("https://client.example/mcp.json") || isMetadataClientID("https://client.example") || isMetadataClientID("http://client.example/mcp.json") {
		t.Fatal("Client ID Metadata Document URL validation mismatch")
	}
	if _, _, err := fetchClientMetadata(t.Context(), "https://127.0.0.1/client.json"); err == nil {
		t.Fatal("CIMD fetch accepted a loopback destination")
	}
}

func netParseIP(t *testing.T, raw string) net.IP {
	t.Helper()
	ip := net.ParseIP(raw)
	if ip == nil {
		t.Fatalf("parse IP %s", raw)
	}
	return ip
}
