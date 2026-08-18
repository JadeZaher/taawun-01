package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"taawun/pkg/models"
	"taawun/pkg/repositories"
	"taawun/pkg/services"
)

func TestWorkspacePeopleIsScopedAndNonSensitive(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	for _, statement := range []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT NOT NULL UNIQUE, email TEXT NOT NULL UNIQUE, password TEXT NOT NULL, role TEXT NOT NULL, status TEXT NOT NULL, session_version INTEGER NOT NULL DEFAULT 1, created_at DATETIME DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE workspaces (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, description TEXT, owner_id INTEGER NOT NULL, status TEXT NOT NULL, created_at DATETIME DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE workspace_users (workspace_id INTEGER NOT NULL, user_id INTEGER NOT NULL, role TEXT NOT NULL, joined_at DATETIME DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (workspace_id, user_id))`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	orm, err := gorm.Open(sqlite.Dialector{Conn: db}, &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	userRepo := repositories.NewUserRepository(orm)
	workspaceRepo := repositories.NewWorkspaceRepository(orm)
	service := services.NewWorkspaceService(workspaceRepo, userRepo)
	owner := &models.User{Username: "owner", Email: "private-owner@example.test", Password: "x", Role: models.RoleUser, Status: models.StatusActive}
	viewer := &models.User{Username: "viewer", Email: "private-viewer@example.test", Password: "x", Role: models.RoleUser, Status: models.StatusActive}
	outsider := &models.User{Username: "outsider", Email: "private-outsider@example.test", Password: "x", Role: models.RoleUser, Status: models.StatusActive}
	for _, user := range []*models.User{owner, viewer, outsider} {
		if err := userRepo.Create(user); err != nil {
			t.Fatal(err)
		}
	}
	workspace, err := service.CreateWorkspace(owner, &models.CreateWorkspaceRequest{Name: "Private workspace"})
	if err != nil {
		t.Fatal(err)
	}
	if err := workspaceRepo.AddUser(workspace.ID, viewer.ID, models.WorkspaceRoleViewer); err != nil {
		t.Fatal(err)
	}
	handler := NewWorkspaceHandler(service)

	request := httptest.NewRequest(http.MethodGet, "/api/workspaces/"+strconv.Itoa(workspace.ID)+"/people", nil)
	request = mux.SetURLVars(request, map[string]string{"id": strconv.Itoa(workspace.ID)})
	request = request.WithContext(WithCurrentUser(request.Context(), viewer))
	response := httptest.NewRecorder()
	handler.GetWorkspacePeople(response, request)
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("authorized people response = status %d headers %+v body %s", response.Code, response.Header(), response.Body.String())
	}
	var payload struct {
		Members []models.WorkspaceMember `json:"members"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Members) != 2 || payload.Members[1].UserID != viewer.ID || payload.Members[1].Role != models.WorkspaceRoleViewer {
		t.Fatalf("people projection = %+v", payload.Members)
	}
	if strings.Contains(response.Body.String(), "private-") || strings.Contains(response.Body.String(), "password") {
		t.Fatalf("people response leaked sensitive identity fields: %s", response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/api/workspaces/"+strconv.Itoa(workspace.ID)+"/people", nil)
	request = mux.SetURLVars(request, map[string]string{"id": strconv.Itoa(workspace.ID)})
	request = request.WithContext(WithCurrentUser(request.Context(), outsider))
	response = httptest.NewRecorder()
	handler.GetWorkspacePeople(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-workspace people status = %d body %s", response.Code, response.Body.String())
	}

	if _, err := db.Exec(`DROP TABLE workspace_users`); err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodGet, "/api/workspaces/"+strconv.Itoa(workspace.ID)+"/people", nil)
	request = mux.SetURLVars(request, map[string]string{"id": strconv.Itoa(workspace.ID)})
	request = request.WithContext(WithCurrentUser(request.Context(), owner))
	response = httptest.NewRecorder()
	handler.GetWorkspacePeople(response, request)
	if response.Code != http.StatusInternalServerError || response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("failed people response = status %d headers %+v body %s", response.Code, response.Header(), response.Body.String())
	}
	if response.Body.String() != "{\"error\":{\"code\":\"workspace_people_unavailable\",\"message\":\"Workspace people could not be loaded.\"}}\n" {
		t.Fatalf("failed people response exposed internal detail: %s", response.Body.String())
	}
}
