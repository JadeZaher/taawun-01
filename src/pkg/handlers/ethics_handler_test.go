package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"taawun/pkg/ethics"
	"taawun/pkg/models"
)

type ethicsAuditorStub struct {
	calls int
}

func (s *ethicsAuditorStub) AuditPrompt(string) (*ethics.ComplianceResult, error) {
	s.calls++
	return &ethics.ComplianceResult{Passed: true}, nil
}

func TestEthicsAuditRequiresIdentityAndStrictBoundedJSON(t *testing.T) {
	auditor := &ethicsAuditorStub{}
	handler := NewEthicsHTTPHandler(auditor)

	unauthenticated := httptest.NewRequest(http.MethodPost, "/api/ethics/audit", strings.NewReader(`{"prompt":"review"}`))
	unauthenticated.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.Audit(response, unauthenticated)
	if response.Code != http.StatusUnauthorized || auditor.calls != 0 {
		t.Fatalf("unauthenticated audit = status:%d calls:%d", response.Code, auditor.calls)
	}

	tests := []struct {
		name string
		body []byte
	}{
		{name: "empty prompt", body: []byte(`{"prompt":"   "}`)},
		{name: "unknown field", body: []byte(`{"prompt":"review","extra":true}`)},
		{name: "trailing document", body: []byte(`{"prompt":"review"} {}`)},
		{name: "over limit", body: bytes.Repeat([]byte("a"), maximumEthicsAuditBodyBytes+1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/ethics/audit", bytes.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			request = request.WithContext(WithCurrentUser(request.Context(), &models.User{ID: 7}))
			response := httptest.NewRecorder()
			handler.Audit(response, request)
			if response.Code != http.StatusBadRequest && response.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("strict audit status = %d body=%s", response.Code, response.Body.String())
			}
			if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatalf("strict audit headers = %v", response.Header())
			}
		})
	}
}

func TestEthicsAuditThrottlesAuthenticatedPrincipal(t *testing.T) {
	handler := NewEthicsHTTPHandler(&ethicsAuditorStub{})
	for attempt := 0; attempt <= ethicsAuditAttemptLimit; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/api/ethics/audit", strings.NewReader(`{"prompt":"review this composition"}`))
		request.Header.Set("Content-Type", "application/json")
		request = request.WithContext(WithCurrentUser(request.Context(), &models.User{ID: 7}))
		response := httptest.NewRecorder()
		handler.Audit(response, request)
		if attempt < ethicsAuditAttemptLimit && response.Code != http.StatusOK {
			t.Fatalf("audit attempt %d status = %d body=%s", attempt+1, response.Code, response.Body.String())
		}
		if attempt == ethicsAuditAttemptLimit && response.Code != http.StatusTooManyRequests {
			t.Fatalf("audit attempt %d status = %d, want 429", attempt+1, response.Code)
		}
	}
}
