package handlers

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"taawun/pkg/database"
	"taawun/pkg/models"
	"taawun/pkg/repositories"
	"taawun/pkg/services"
)

func TestDecodeBoundedJSONRejectsUnsafePublicAuthBodies(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        []byte
		wantStatus  int
	}{
		{name: "content type", contentType: "text/plain", body: []byte(`{}`), wantStatus: http.StatusUnsupportedMediaType},
		{name: "unknown field", contentType: "application/json", body: []byte(`{"email":"member@example.com","password":"correct-password","unexpected":true}`), wantStatus: http.StatusBadRequest},
		{name: "trailing document", contentType: "application/json", body: []byte(`{} {}`), wantStatus: http.StatusBadRequest},
		{name: "over limit", contentType: "application/json", body: bytes.Repeat([]byte("a"), maximumPublicAuthBodyBytes+1), wantStatus: http.StatusRequestEntityTooLarge},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(test.body))
			request.Header.Set("Content-Type", test.contentType)
			response := httptest.NewRecorder()
			var target models.LoginRequest
			accepted, status := decodeBoundedJSON(response, request, &target, maximumPublicAuthBodyBytes)
			if accepted {
				t.Fatal("decodeBoundedJSON unexpectedly accepted the request")
			}
			if status != test.wantStatus {
				t.Fatalf("decode status = %d, want %d", status, test.wantStatus)
			}
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
		})
	}
}

func TestAuthRateLimiterBoundsAttemptsAndStorage(t *testing.T) {
	now := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	limiter := newAuthRateLimiter(time.Minute, 2)
	limiter.now = func() time.Time { return now }
	if allowed, _ := limiter.Allow("login", "203.0.113.10", 2); !allowed {
		t.Fatal("first request should be allowed")
	}
	if allowed, _ := limiter.Allow("login", "203.0.113.10", 2); !allowed {
		t.Fatal("second request should be allowed")
	}
	if allowed, retryAfter := limiter.Allow("login", "203.0.113.10", 2); allowed || retryAfter != time.Minute {
		t.Fatalf("third request = allowed:%t retry:%s, want denied for one minute", allowed, retryAfter)
	}
	if allowed, _ := limiter.Allow("register", "203.0.113.11", 1); !allowed {
		t.Fatal("second tracked client should be allowed")
	}
	if allowed, _ := limiter.Allow("register", "203.0.113.12", 1); allowed {
		t.Fatal("bounded rate limiter should reject a third concurrent client")
	}
	now = now.Add(time.Minute)
	if allowed, _ := limiter.Allow("login", "203.0.113.10", 2); !allowed {
		t.Fatal("expired client entry should be cleaned up and allowed")
	}
}

func TestPublicAttemptRejectionSetsRetryAfter(t *testing.T) {
	now := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	limiter := newAuthRateLimiter(time.Minute, 8)
	limiter.now = func() time.Time { return now }
	handler := &AuthHandler{publicLimiter: limiter}
	request := httptest.NewRequest(http.MethodPost, "/api/login", nil)
	request.RemoteAddr = "203.0.113.10:12345"
	if !handler.allowPublicAttempt(httptest.NewRecorder(), request, "login", 1) {
		t.Fatal("first request should be allowed")
	}
	response := httptest.NewRecorder()
	if handler.allowPublicAttempt(response, request, "login", 1) {
		t.Fatal("second request should be rejected")
	}
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusTooManyRequests)
	}
	if response.Header().Get("Retry-After") != "60" {
		t.Fatalf("Retry-After = %q, want 60", response.Header().Get("Retry-After"))
	}
}

func TestRegisterThrottleUsesRailwayClientAcrossEdgePeers(t *testing.T) {
	t.Setenv("RAILWAY_ENVIRONMENT_ID", "production-environment")
	handler := &AuthHandler{publicLimiter: newAuthRateLimiter(time.Minute, 8)}
	peers := []string{
		"100.64.0.11:4100",
		"100.65.0.12:4101",
		"100.66.0.13:4102",
		"100.67.0.14:4103",
		"100.68.0.15:4104",
		"100.69.0.16:4105",
	}
	for index, peer := range peers {
		request := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader(`{"unexpected":true}`))
		request.RemoteAddr = peer
		request.Header.Set("X-Real-IP", "198.51.100.77")
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.Register(response, request)
		if index < registrationAttemptLimit && response.Code != http.StatusBadRequest {
			t.Fatalf("Railway registration attempt %d status = %d, want strict-body 400", index+1, response.Code)
		}
		if index == registrationAttemptLimit {
			if response.Code != http.StatusTooManyRequests {
				t.Fatalf("Railway registration attempt %d status:%d, want 429", index+1, response.Code)
			}
		}
	}
}

func TestRegisterThenImmediateLoginThroughHTTP(t *testing.T) {
	var logOutput bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logOutput, nil))
	handler := newHTTPAuthTestHandler(t, logger)
	router := http.NewServeMux()
	router.HandleFunc("/api/register", handler.Register)
	router.HandleFunc("/api/login", handler.Login)

	email := "private-beta-member@example.com"
	password := "one-consistent-private-beta-password"
	registration := postAuthJSON(t, router, "/api/register", map[string]string{
		"username": "private-beta-member",
		"email":    email,
		"password": password,
	})
	if registration.Code != http.StatusCreated {
		t.Fatalf("registration status = %d body=%s", registration.Code, registration.Body.String())
	}

	login := postAuthJSON(t, router, "/api/login", map[string]string{
		"email":    email,
		"password": password,
	})
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d body=%s", login.Code, login.Body.String())
	}
	var response models.LoginResponse
	if err := json.NewDecoder(login.Body).Decode(&response); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if response.Token == "" || response.User.Email != email {
		t.Fatalf("login response missing authenticated principal: %#v", response.User)
	}

	rejected := postAuthJSON(t, router, "/api/login", map[string]string{
		"email":    email,
		"password": "a-different-private-beta-password",
	})
	if rejected.Code != http.StatusUnauthorized {
		t.Fatalf("rejected login status = %d, want %d", rejected.Code, http.StatusUnauthorized)
	}
	logs := logOutput.String()
	for _, expected := range []string{
		"operation=register outcome=accepted status=201",
		"operation=login outcome=accepted status=200",
		"operation=login outcome=rejected status=401",
	} {
		if !strings.Contains(logs, expected) {
			t.Fatalf("auth log missing %q: %s", expected, logs)
		}
	}
	for _, secret := range []string{email, password, "a-different-private-beta-password"} {
		if strings.Contains(logs, secret) {
			t.Fatalf("auth log leaked credential material %q", secret)
		}
	}
}

func TestRequestSourceUsesOnlyRailwayRealIPFromTrustedPeer(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/login", nil)
	request.RemoteAddr = "10.20.30.40:4567"
	request.Header.Set("X-Real-IP", "203.0.113.55")
	request.Header.Set("X-Forwarded-For", "198.51.100.10")

	t.Setenv("RAILWAY_ENVIRONMENT_ID", "")
	if got := requestSource(request); got != "10.20.30.40" {
		t.Fatalf("non-Railway source = %q, want direct peer", got)
	}

	t.Setenv("RAILWAY_ENVIRONMENT_ID", "production-environment")
	if got := requestSource(request); got != "203.0.113.55" {
		t.Fatalf("Railway real IP source = %q, want documented client header", got)
	}

	request.RemoteAddr = "100.64.12.8:4567"
	request.Header.Set("X-Real-IP", "203.0.113.56")
	if got := requestSource(request); got != "203.0.113.56" {
		t.Fatalf("Railway 100/8 proxy source = %q, want documented client header", got)
	}

	request.Header.Del("X-Real-IP")
	if got := requestSource(request); got != "100.64.12.8" {
		t.Fatalf("spoofed X-Forwarded-For source = %q, want direct peer", got)
	}

	request.RemoteAddr = "192.0.2.44:4567"
	request.Header.Set("X-Real-IP", "203.0.113.99")
	if got := requestSource(request); got != "192.0.2.44" {
		t.Fatalf("public direct peer source = %q, want direct peer", got)
	}

	request.RemoteAddr = "[fd12::8]:4567"
	request.Header.Set("X-Real-IP", "not-an-address")
	if got := requestSource(request); got != "fd12::8" {
		t.Fatalf("invalid real IP source = %q, want direct peer", got)
	}
}

func newHTTPAuthTestHandler(t *testing.T, logger *slog.Logger) *AuthHandler {
	t.Helper()
	t.Setenv("APP_DB_PATH", filepath.Join(t.TempDir(), "auth-handler.db"))
	t.Setenv("TAWUN_BOOTSTRAP_ADMIN_USERNAME", "")
	t.Setenv("TAWUN_BOOTSTRAP_ADMIN_EMAIL", "")
	t.Setenv("TAWUN_BOOTSTRAP_ADMIN_PASSWORD", "")
	db, err := database.InitDB()
	if err != nil {
		t.Fatalf("initialize auth database: %v", err)
	}
	sqlDB, err := database.SQLDB(db)
	if err != nil {
		t.Fatalf("access auth database connection: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	repository := repositories.NewUserRepository(db)
	userService := services.NewUserService(repository)
	authService, err := services.NewAuthService(repository, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("initialize auth service: %v", err)
	}
	handler := NewAuthHandler(authService, userService)
	handler.logger = logger
	return handler
}

func postAuthJSON(t *testing.T, handler http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
