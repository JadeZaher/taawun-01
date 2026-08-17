package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"taawun/pkg/models"
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
			if decodeBoundedJSON(response, request, &target, maximumPublicAuthBodyBytes) {
				t.Fatal("decodeBoundedJSON unexpectedly accepted the request")
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
