package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerSeparatesLandingAndAccountDocuments(t *testing.T) {
	handler := Handler()
	tests := []struct {
		path        string
		wantStatus  int
		wantBody    []string
		rejectBody  []string
		wantRobots  string
		contentType string
	}{
		{
			path:        "/",
			wantStatus:  http.StatusOK,
			wantBody:    []string{"id=\"heroTitle\"", "id=\"geometryCanvas\"", "/account#register", "Sūrat an-Nūr"},
			rejectBody:  []string{"id=\"loginForm\"", "id=\"appView\"", "Authorization"},
			contentType: "text/html; charset=utf-8",
		},
		{
			path:        "/account",
			wantStatus:  http.StatusOK,
			wantBody:    []string{"id=\"loginForm\"", "id=\"registerForm\"", "id=\"appView\"", "noindex,nofollow,noarchive"},
			rejectBody:  []string{"id=\"geometryCanvas\""},
			wantRobots:  "noindex, nofollow, noarchive",
			contentType: "text/html; charset=utf-8",
		},
		{
			path:        "/account/",
			wantStatus:  http.StatusOK,
			wantBody:    []string{"Account access | Taawun", "location.hash === '#register'"},
			wantRobots:  "noindex, nofollow, noarchive",
			contentType: "text/html; charset=utf-8",
		},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "http://control.example"+test.path, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			result := response.Result()
			defer result.Body.Close()
			body, err := io.ReadAll(result.Body)
			if err != nil {
				t.Fatalf("ReadAll() error = %v", err)
			}
			if result.StatusCode != test.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", result.StatusCode, test.wantStatus, body)
			}
			if got := result.Header.Get("Content-Type"); got != test.contentType {
				t.Errorf("Content-Type = %q, want %q", got, test.contentType)
			}
			if got := result.Header.Get("X-Robots-Tag"); got != test.wantRobots {
				t.Errorf("X-Robots-Tag = %q, want %q", got, test.wantRobots)
			}
			for _, expected := range test.wantBody {
				if !strings.Contains(string(body), expected) {
					t.Errorf("body missing %q", expected)
				}
			}
			for _, rejected := range test.rejectBody {
				if strings.Contains(string(body), rejected) {
					t.Errorf("body unexpectedly contains %q", rejected)
				}
			}
		})
	}
}

func TestHandlerBoundsMethodsAssetsAndUnknownPaths(t *testing.T) {
	handler := Handler()

	head := httptest.NewRecorder()
	handler.ServeHTTP(head, httptest.NewRequest(http.MethodHead, "http://control.example/", nil))
	if head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Fatalf("HEAD / = %d with %d body bytes, want 200 with no body", head.Code, head.Body.Len())
	}

	asset := httptest.NewRecorder()
	handler.ServeHTTP(asset, httptest.NewRequest(http.MethodGet, "http://control.example/geometric-renderer.js", nil))
	if asset.Code != http.StatusOK || !strings.Contains(asset.Body.String(), "createGeometricRenderer") {
		t.Fatalf("GET renderer = %d body %q", asset.Code, asset.Body.String())
	}

	runtime := httptest.NewRecorder()
	handler.ServeHTTP(runtime, httptest.NewRequest(http.MethodGet, "http://control.example/runtime/index.js", nil))
	if runtime.Code != http.StatusOK || !strings.Contains(runtime.Body.String(), "runtime") {
		t.Fatalf("GET runtime/index.js = %d body %q", runtime.Code, runtime.Body.String())
	}

	for _, legacyPath := range []string{"/index.html", "/landing.html"} {
		redirect := httptest.NewRecorder()
		handler.ServeHTTP(redirect, httptest.NewRequest(http.MethodGet, "http://control.example"+legacyPath, nil))
		if redirect.Code != http.StatusMovedPermanently || redirect.Header().Get("Location") != "/" {
			t.Fatalf("GET %s = %d Location %q, want 301 to /", legacyPath, redirect.Code, redirect.Header().Get("Location"))
		}
	}

	unknown := httptest.NewRecorder()
	handler.ServeHTTP(unknown, httptest.NewRequest(http.MethodGet, "http://control.example/private-source", nil))
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("GET unknown = %d, want 404", unknown.Code)
	}

	method := httptest.NewRecorder()
	handler.ServeHTTP(method, httptest.NewRequest(http.MethodPost, "http://control.example/", nil))
	if method.Code != http.StatusMethodNotAllowed || method.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("POST / = %d Allow %q, want 405 and GET, HEAD", method.Code, method.Header().Get("Allow"))
	}
}
