package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"

	"taawun/pkg/models"
)

func TestRequireAdminDistinguishesAuthenticationAndAuthorization(t *testing.T) {
	handler := (&AuthHandler{}).RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status = %d, want %d", response.Code, http.StatusUnauthorized)
	}

	request = withCurrentUser(request, &models.User{ID: 1, Role: models.RoleUser})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("member status = %d, want %d", response.Code, http.StatusForbidden)
	}

	request = withCurrentUser(request, &models.User{ID: 2, Role: models.RoleAdmin})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("admin status = %d, want %d", response.Code, http.StatusNoContent)
	}
}

func TestUserEndpointsRejectCrossAccountAccessBeforeServiceCall(t *testing.T) {
	handler := NewUserHandler(nil)
	request := httptest.NewRequest(http.MethodGet, "/api/users/2", nil)
	request = mux.SetURLVars(request, map[string]string{"id": "2"})
	request = withCurrentUser(request, &models.User{ID: 1, Role: models.RoleUser})
	response := httptest.NewRecorder()

	handler.GetUser(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-account status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestUserListingRequiresPlatformAdmin(t *testing.T) {
	handler := NewUserHandler(nil)
	request := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	request = withCurrentUser(request, &models.User{ID: 1, Role: models.RoleUser})
	response := httptest.NewRecorder()

	handler.GetUsers(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("member listing status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func withCurrentUser(request *http.Request, user *models.User) *http.Request {
	ctx := context.WithValue(request.Context(), userContextKey, user)
	return request.WithContext(ctx)
}
