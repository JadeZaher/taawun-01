package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"gorm.io/gorm"

	"taawun/pkg/database"
	"taawun/pkg/models"
	"taawun/pkg/repositories"
	"taawun/pkg/services"
)

func TestDeleteUserDoesNotExposePersistenceFailure(t *testing.T) {
	db := newUserHandlerLifecycleDB(t)
	sqlDB, err := database.SQLDB(db)
	if err != nil {
		t.Fatal(err)
	}

	user := &models.User{Username: "delete-failure", Email: "delete-failure@example.com", Password: "unused", Role: models.RoleUser, Status: models.StatusActive}
	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.Exec(`CREATE TRIGGER fail_account_anonymization BEFORE UPDATE ON users BEGIN SELECT RAISE(ABORT, 'raw FOREIGN KEY constraint detail'); END`); err != nil {
		t.Fatal(err)
	}

	userRepo := repositories.NewUserRepository(db)
	handler := NewUserHandler(services.NewUserService(userRepo))
	request := httptest.NewRequest(http.MethodDelete, "/api/users/"+strconv.Itoa(user.ID), nil)
	request = mux.SetURLVars(request, map[string]string{"id": strconv.Itoa(user.ID)})
	request = withCurrentUser(request, user)
	response := httptest.NewRecorder()
	handler.DeleteUser(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	body := response.Body.String()
	if !strings.Contains(body, `"code":"account_deletion_failed"`) {
		t.Fatalf("response body = %s", body)
	}
	if strings.Contains(body, "FOREIGN KEY") || strings.Contains(body, "constraint") {
		t.Fatalf("response exposed persistence details: %s", body)
	}
	assertUserLifecycleHeaders(t, response)
	unchanged, err := userRepo.GetByID(user.ID)
	if err != nil || unchanged == nil || unchanged.Status != models.StatusActive || unchanged.Username != user.Username || unchanged.Email != user.Email || unchanged.SessionVersion != 1 {
		t.Fatalf("user changed after persistence failure: %+v, %v", unchanged, err)
	}
}

func TestDeleteUserRejectsOwnedWorkspaceWithBoundedConflict(t *testing.T) {
	db := newUserHandlerLifecycleDB(t)
	userRepo := repositories.NewUserRepository(db)
	workspaceRepo := repositories.NewWorkspaceRepository(db)
	owner := &models.User{Username: "conflict-owner", Email: "conflict-owner@example.com", Password: "unchanged", Role: models.RoleUser, Status: models.StatusActive}
	admin := &models.User{Username: "platform-admin", Email: "platform-admin@example.com", Password: "unchanged", Role: models.RoleAdmin, Status: models.StatusActive}
	if err := db.Create(owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(admin).Error; err != nil {
		t.Fatal(err)
	}
	workspace, err := services.NewWorkspaceService(workspaceRepo, userRepo).CreateWorkspace(owner, &models.CreateWorkspaceRequest{Name: "Private Relief Workspace"})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewUserHandler(services.NewUserService(userRepo))

	for _, actor := range []*models.User{owner, admin} {
		request := httptest.NewRequest(http.MethodDelete, "/api/users/"+strconv.Itoa(owner.ID), nil)
		request = mux.SetURLVars(request, map[string]string{"id": strconv.Itoa(owner.ID)})
		request = withCurrentUser(request, actor)
		response := httptest.NewRecorder()
		handler.DeleteUser(response, request)

		if response.Code != http.StatusConflict {
			t.Fatalf("actor %d status = %d, want %d", actor.ID, response.Code, http.StatusConflict)
		}
		assertUserLifecycleHeaders(t, response)
		if response.Header().Get("Content-Type") != "application/json; charset=utf-8" {
			t.Fatalf("conflict content type = %q", response.Header().Get("Content-Type"))
		}
		if response.Body.Len() > 256 {
			t.Fatalf("conflict body length = %d, want <= 256", response.Body.Len())
		}
		body := response.Body.String()
		if strings.Contains(body, workspace.Name) || strings.Contains(body, strconv.Itoa(workspace.ID)) {
			t.Fatalf("conflict exposed workspace identifier: %s", body)
		}
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if len(envelope) != 1 || envelope["error"] == nil {
			t.Fatalf("conflict envelope has extra members: %s", body)
		}
		var errorBody map[string]string
		if err := json.Unmarshal(envelope["error"], &errorBody); err != nil {
			t.Fatal(err)
		}
		if len(errorBody) != 2 || errorBody["code"] != "owned_workspaces_remaining" || errorBody["message"] != "Owned workspaces must be deleted before this account can be deleted." {
			t.Fatalf("conflict error envelope = %+v", errorBody)
		}
	}
	unchanged, err := userRepo.GetByID(owner.ID)
	if err != nil || unchanged == nil || unchanged.Status != models.StatusActive || unchanged.Username != owner.Username || unchanged.Email != owner.Email || unchanged.SessionVersion != 1 {
		t.Fatalf("owner changed after rejected deletion: %+v, %v", unchanged, err)
	}
}

func TestDeleteUserLookupFailureIsRedactedAndNonDestructive(t *testing.T) {
	db := newUserHandlerLifecycleDB(t)
	sqlDB, err := database.SQLDB(db)
	if err != nil {
		t.Fatal(err)
	}
	userRepo := repositories.NewUserRepository(db)
	workspaceRepo := repositories.NewWorkspaceRepository(db)
	user := &models.User{Username: "lookup-failure", Email: "lookup-failure@example.com", Password: "unchanged", Role: models.RoleUser, Status: models.StatusActive}
	owner := &models.User{Username: "lookup-owner", Email: "lookup-owner@example.com", Password: "unchanged", Role: models.RoleUser, Status: models.StatusActive}
	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(owner).Error; err != nil {
		t.Fatal(err)
	}
	workspace, err := services.NewWorkspaceService(workspaceRepo, userRepo).CreateWorkspace(owner, &models.CreateWorkspaceRequest{Name: "Lookup dependency"})
	if err != nil {
		t.Fatal(err)
	}
	if err := workspaceRepo.AddUser(workspace.ID, user.ID, models.WorkspaceRoleMember); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE oauth_sessions (session_hash TEXT PRIMARY KEY, user_id INTEGER NOT NULL, revoked_at INTEGER)`,
		`CREATE TABLE oauth_consents (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL, revoked_at INTEGER)`,
		`CREATE TABLE oauth_authorization_codes (code_hash TEXT PRIMARY KEY, user_id INTEGER NOT NULL, consumed_at INTEGER)`,
		`CREATE TABLE oauth_access_tokens (token_hash TEXT PRIMARY KEY, user_id INTEGER NOT NULL, revoked_at INTEGER)`,
		`CREATE TABLE oauth_refresh_tokens (token_hash TEXT PRIMARY KEY, user_id INTEGER NOT NULL, revoked_at INTEGER)`,
		`CREATE TABLE lifecycle_lookup_refs (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id))`,
		`INSERT INTO notifications (id, user_id, type, title, message, read) VALUES (1, ` + strconv.Itoa(user.ID) + `, 'system', 'Private', 'Keep', 0)`,
		`INSERT INTO oauth_sessions (session_hash, user_id) VALUES ('session', ` + strconv.Itoa(user.ID) + `)`,
		`INSERT INTO oauth_consents (id, user_id) VALUES (1, ` + strconv.Itoa(user.ID) + `)`,
		`INSERT INTO oauth_authorization_codes (code_hash, user_id) VALUES ('code', ` + strconv.Itoa(user.ID) + `)`,
		`INSERT INTO oauth_access_tokens (token_hash, user_id) VALUES ('access', ` + strconv.Itoa(user.ID) + `)`,
		`INSERT INTO oauth_refresh_tokens (token_hash, user_id) VALUES ('refresh', ` + strconv.Itoa(user.ID) + `)`,
		`INSERT INTO lifecycle_lookup_refs (id, user_id) VALUES (1, ` + strconv.Itoa(user.ID) + `)`,
	} {
		if _, err := sqlDB.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := sqlDB.Exec(`ALTER TABLE workspaces RENAME TO workspaces_unavailable`); err != nil {
		t.Fatal(err)
	}
	handler := NewUserHandler(services.NewUserService(userRepo))
	request := httptest.NewRequest(http.MethodDelete, "/api/users/"+strconv.Itoa(user.ID), nil)
	request = mux.SetURLVars(request, map[string]string{"id": strconv.Itoa(user.ID)})
	request = withCurrentUser(request, user)
	response := httptest.NewRecorder()
	handler.DeleteUser(response, request)
	if _, err := sqlDB.Exec(`ALTER TABLE workspaces_unavailable RENAME TO workspaces`); err != nil {
		t.Fatal(err)
	}

	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), `"code":"account_deletion_failed"`) {
		t.Fatalf("lookup failure response = %d %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "lookup detail") || strings.Contains(strings.ToLower(response.Body.String()), "no such table") {
		t.Fatalf("lookup failure response exposed persistence details: %s", response.Body.String())
	}
	assertUserLifecycleHeaders(t, response)
	unchanged, err := userRepo.GetByID(user.ID)
	if err != nil || unchanged == nil || unchanged.Status != models.StatusActive || unchanged.Username != user.Username || unchanged.Email != user.Email || unchanged.SessionVersion != 1 {
		t.Fatalf("user changed after lookup failure: %+v, %v", unchanged, err)
	}
	for _, table := range []string{"workspace_users", "notifications", "lifecycle_lookup_refs"} {
		var count int
		if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE user_id = ?`, user.ID).Scan(&count); err != nil || count != 1 {
			t.Fatalf("%s rows after lookup failure = %d, %v", table, count, err)
		}
	}
	for _, table := range []string{"oauth_sessions", "oauth_consents", "oauth_access_tokens", "oauth_refresh_tokens"} {
		var revoked any
		if err := sqlDB.QueryRow(`SELECT revoked_at FROM `+table+` WHERE user_id = ?`, user.ID).Scan(&revoked); err != nil || revoked != nil {
			t.Fatalf("%s revoked_at after lookup failure = %v, %v", table, revoked, err)
		}
	}
	var consumed any
	if err := sqlDB.QueryRow(`SELECT consumed_at FROM oauth_authorization_codes WHERE user_id = ?`, user.ID).Scan(&consumed); err != nil || consumed != nil {
		t.Fatalf("authorization code after lookup failure = %v, %v", consumed, err)
	}
}

func TestDeleteUserSuccessIsBodylessAndNonCacheable(t *testing.T) {
	db := newUserHandlerLifecycleDB(t)
	userRepo := repositories.NewUserRepository(db)
	user := &models.User{Username: "zero-owned", Email: "zero-owned@example.com", Password: "unchanged", Role: models.RoleUser, Status: models.StatusActive}
	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	handler := NewUserHandler(services.NewUserService(userRepo))
	request := httptest.NewRequest(http.MethodDelete, "/api/users/"+strconv.Itoa(user.ID), nil)
	request = mux.SetURLVars(request, map[string]string{"id": strconv.Itoa(user.ID)})
	request = withCurrentUser(request, user)
	response := httptest.NewRecorder()
	handler.DeleteUser(response, request)

	if response.Code != http.StatusNoContent || response.Body.Len() != 0 {
		t.Fatalf("success response = %d %q", response.Code, response.Body.String())
	}
	assertUserLifecycleHeaders(t, response)
}

func TestDeleteUserPropagatesCancellationWithoutMutationOrRawCause(t *testing.T) {
	db := newUserHandlerLifecycleDB(t)
	userRepo := repositories.NewUserRepository(db)
	user := &models.User{Username: "cancelled-delete", Email: "cancelled-delete@example.com", Password: "unchanged", Role: models.RoleUser, Status: models.StatusActive}
	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	handler := NewUserHandler(services.NewUserService(userRepo))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest(http.MethodDelete, "/api/users/"+strconv.Itoa(user.ID), nil).WithContext(ctx)
	request = mux.SetURLVars(request, map[string]string{"id": strconv.Itoa(user.ID)})
	request = withCurrentUser(request, user)
	response := httptest.NewRecorder()
	handler.DeleteUser(response, request)

	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), `"code":"account_deletion_failed"`) {
		t.Fatalf("cancelled delete response = %d %s", response.Code, response.Body.String())
	}
	if strings.Contains(strings.ToLower(response.Body.String()), "context canceled") || strings.Contains(strings.ToLower(response.Body.String()), "database is locked") {
		t.Fatalf("cancelled delete leaked raw cause: %s", response.Body.String())
	}
	assertUserLifecycleHeaders(t, response)
	unchanged, err := userRepo.GetByID(user.ID)
	if err != nil || unchanged == nil || unchanged.Status != models.StatusActive || unchanged.SessionVersion != 1 {
		t.Fatalf("cancelled deletion changed user: %+v, %v", unchanged, err)
	}
}

func TestWorkspaceDeleteThenAccountDeleteHTTPSequence(t *testing.T) {
	db := newUserHandlerLifecycleDB(t)
	userRepo := repositories.NewUserRepository(db)
	workspaceRepo := repositories.NewWorkspaceRepository(db)
	owner := &models.User{Username: "http-sequence", Email: "http-sequence@example.com", Password: "unchanged", Role: models.RoleUser, Status: models.StatusActive}
	if err := db.Create(owner).Error; err != nil {
		t.Fatal(err)
	}
	workspaceService := services.NewWorkspaceService(workspaceRepo, userRepo)
	workspace, err := workspaceService.CreateWorkspace(owner, &models.CreateWorkspaceRequest{Name: "Delete through HTTP"})
	if err != nil {
		t.Fatal(err)
	}
	userHandler := NewUserHandler(services.NewUserService(userRepo))
	workspaceHandler := NewWorkspaceHandler(workspaceService)

	accountRequest := httptest.NewRequest(http.MethodDelete, "/api/users/"+strconv.Itoa(owner.ID), nil)
	accountRequest = mux.SetURLVars(accountRequest, map[string]string{"id": strconv.Itoa(owner.ID)})
	accountRequest = withCurrentUser(accountRequest, owner)
	accountResponse := httptest.NewRecorder()
	userHandler.DeleteUser(accountResponse, accountRequest)
	if accountResponse.Code != http.StatusConflict {
		t.Fatalf("initial account delete status = %d", accountResponse.Code)
	}

	workspaceRequest := httptest.NewRequest(http.MethodDelete, "/api/workspaces/"+strconv.Itoa(workspace.ID), nil)
	workspaceRequest = mux.SetURLVars(workspaceRequest, map[string]string{"id": strconv.Itoa(workspace.ID)})
	workspaceRequest = withCurrentUser(workspaceRequest, owner)
	workspaceResponse := httptest.NewRecorder()
	workspaceHandler.DeleteWorkspace(workspaceResponse, workspaceRequest)
	if workspaceResponse.Code != http.StatusNoContent {
		t.Fatalf("workspace delete status = %d, body = %s", workspaceResponse.Code, workspaceResponse.Body.String())
	}

	accountRequest = httptest.NewRequest(http.MethodDelete, "/api/users/"+strconv.Itoa(owner.ID), nil)
	accountRequest = mux.SetURLVars(accountRequest, map[string]string{"id": strconv.Itoa(owner.ID)})
	accountRequest = withCurrentUser(accountRequest, owner)
	accountResponse = httptest.NewRecorder()
	userHandler.DeleteUser(accountResponse, accountRequest)
	if accountResponse.Code != http.StatusNoContent || accountResponse.Body.Len() != 0 {
		t.Fatalf("final account delete response = %d %q", accountResponse.Code, accountResponse.Body.String())
	}
	assertUserLifecycleHeaders(t, accountResponse)
}

func newUserHandlerLifecycleDB(t *testing.T) *gorm.DB {
	t.Helper()
	t.Setenv("APP_DB_PATH", filepath.Join(t.TempDir(), "user-delete.db"))
	t.Setenv("TAWUN_BOOTSTRAP_ADMIN_USERNAME", "")
	t.Setenv("TAWUN_BOOTSTRAP_ADMIN_EMAIL", "")
	t.Setenv("TAWUN_BOOTSTRAP_ADMIN_PASSWORD", "")
	db, err := database.InitDB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := database.SQLDB(db)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func assertUserLifecycleHeaders(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("lifecycle headers = %+v", response.Header())
	}
}
