package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"taawun/pkg/database"
	"taawun/pkg/models"
	"taawun/pkg/repositories"
	"taawun/pkg/services"
)

func TestDashboardEvidenceHasHonestEmptyState(t *testing.T) {
	db := newDashboardTestDB(t)
	actor := &models.User{ID: 1, Username: "empty-operator", Email: "empty@example.com", Password: "unused", Role: models.RoleUser, Status: models.StatusActive}
	if err := db.Create(actor).Error; err != nil {
		t.Fatal(err)
	}
	handler := newDashboardTestHandler(db)

	statsResponse := httptest.NewRecorder()
	handler.GetDashboardStats(statsResponse, dashboardRequest(actor, "/api/dashboard/stats"))
	if statsResponse.Code != http.StatusOK {
		t.Fatalf("stats status = %d, body = %s", statsResponse.Code, statsResponse.Body.String())
	}
	var stats models.DashboardStats
	if err := json.NewDecoder(statsResponse.Body).Decode(&stats); err != nil {
		t.Fatal(err)
	}
	if stats.Scope != "accessible_workspaces" || stats.EvidenceState != "empty" || stats.EmptyState == "" {
		t.Fatalf("empty stats evidence = %#v", stats)
	}
	if stats.TotalUsers != 0 || stats.ActiveUsers != 0 || stats.TotalWorkspaces != 0 || stats.ActiveWorkspaces != 0 {
		t.Fatalf("empty stats contain fabricated counts: %#v", stats)
	}

	activityResponse := httptest.NewRecorder()
	handler.GetRecentActivity(activityResponse, dashboardRequest(actor, "/api/dashboard/recent"))
	if activityResponse.Code != http.StatusOK {
		t.Fatalf("activity status = %d, body = %s", activityResponse.Code, activityResponse.Body.String())
	}
	activityBody := activityResponse.Body.Bytes()
	var feed models.RecentActivityFeed
	if err := json.Unmarshal(activityBody, &feed); err != nil {
		t.Fatal(err)
	}
	if feed.Scope != "accessible_workspaces" || feed.EmptyState == "" || len(feed.Activities) != 0 {
		t.Fatalf("empty activity feed = %#v", feed)
	}
	if strings.Contains(string(activityBody), "2024-01-01") || strings.Contains(string(activityBody), "Logged in") {
		t.Fatalf("empty activity contains fabricated evidence: %s", activityBody)
	}
}

func TestDashboardEvidenceIsWorkspaceScopedAndDeterministic(t *testing.T) {
	db := newDashboardTestDB(t)
	users := []*models.User{
		{ID: 10, Username: "operator", Email: "operator@example.com", Password: "unused", Role: models.RoleUser, Status: models.StatusActive},
		{ID: 11, Username: "active-member", Email: "active@example.com", Password: "unused", Role: models.RoleUser, Status: models.StatusActive},
		{ID: 12, Username: "inactive-member", Email: "inactive@example.com", Password: "unused", Role: models.RoleUser, Status: models.StatusInactive},
		{ID: 20, Username: "outside-owner", Email: "outside@example.com", Password: "unused", Role: models.RoleUser, Status: models.StatusActive},
		{ID: 21, Username: "outside-member", Email: "outside-member@example.com", Password: "unused", Role: models.RoleUser, Status: models.StatusActive},
	}
	for _, user := range users {
		if err := db.Create(user).Error; err != nil {
			t.Fatal(err)
		}
	}
	createdAt := time.Date(2026, time.August, 18, 5, 30, 0, 0, time.FixedZone("MDT", -6*60*60))
	workspaces := []*models.Workspace{
		{ID: 100, Name: "Visible One", OwnerID: users[0].ID, Status: models.WorkspaceStatusActive, CreatedAt: createdAt},
		{ID: 101, Name: "Visible Two", OwnerID: users[0].ID, Status: models.WorkspaceStatusInactive, CreatedAt: createdAt},
		{ID: 999, Name: "Hidden Workspace", OwnerID: users[3].ID, Status: models.WorkspaceStatusActive, CreatedAt: createdAt.Add(time.Hour)},
	}
	for _, workspace := range workspaces {
		if err := db.Create(workspace).Error; err != nil {
			t.Fatal(err)
		}
	}
	memberships := []*models.WorkspaceUser{
		{WorkspaceID: 100, UserID: 10, Role: models.WorkspaceRoleOwner},
		{WorkspaceID: 100, UserID: 11, Role: models.WorkspaceRoleMember},
		{WorkspaceID: 101, UserID: 10, Role: models.WorkspaceRoleOwner},
		{WorkspaceID: 101, UserID: 12, Role: models.WorkspaceRoleMember},
		{WorkspaceID: 999, UserID: 20, Role: models.WorkspaceRoleOwner},
		{WorkspaceID: 999, UserID: 21, Role: models.WorkspaceRoleMember},
	}
	for _, membership := range memberships {
		if err := db.Create(membership).Error; err != nil {
			t.Fatal(err)
		}
	}

	handler := newDashboardTestHandler(db)
	statsResponse := httptest.NewRecorder()
	handler.GetDashboardStats(statsResponse, dashboardRequest(users[0], "/api/dashboard/stats"))
	var stats models.DashboardStats
	if err := json.NewDecoder(statsResponse.Body).Decode(&stats); err != nil {
		t.Fatal(err)
	}
	if stats.TotalUsers != 3 || stats.ActiveUsers != 2 || stats.TotalWorkspaces != 2 || stats.ActiveWorkspaces != 1 {
		t.Fatalf("scoped stats = %#v", stats)
	}
	if stats.EvidenceState != "ready" || stats.EmptyState != "" {
		t.Fatalf("non-empty stats evidence = %#v", stats)
	}

	activityResponse := httptest.NewRecorder()
	handler.GetRecentActivity(activityResponse, dashboardRequest(users[0], "/api/dashboard/recent"))
	activityBody := activityResponse.Body.Bytes()
	var feed models.RecentActivityFeed
	if err := json.Unmarshal(activityBody, &feed); err != nil {
		t.Fatal(err)
	}
	if len(feed.Activities) != 2 {
		t.Fatalf("activity count = %d, feed = %#v", len(feed.Activities), feed)
	}
	if feed.Activities[0].WorkspaceID != 101 || feed.Activities[1].WorkspaceID != 100 {
		t.Fatalf("activity order is not deterministic: %#v", feed.Activities)
	}
	for _, activity := range feed.Activities {
		if activity.WorkspaceID == 999 || activity.Workspace == "Hidden Workspace" {
			t.Fatalf("activity leaked inaccessible workspace: %#v", activity)
		}
		if activity.Type != "workspace_created" || activity.Action != "Workspace created" || activity.Timestamp != "2026-08-18T11:30:00Z" {
			t.Fatalf("activity is not sourced from workspace data: %#v", activity)
		}
	}
	if strings.Contains(string(activityBody), "2024-01-01") || strings.Contains(string(activityBody), "Logged in") {
		t.Fatalf("activity contains fixed placeholder evidence: %s", activityBody)
	}
}

func newDashboardTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	t.Setenv("APP_DB_PATH", filepath.Join(t.TempDir(), "dashboard.db"))
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

func newDashboardTestHandler(db *gorm.DB) *DashboardHandler {
	userRepo := repositories.NewUserRepository(db)
	workspaceService := services.NewWorkspaceService(repositories.NewWorkspaceRepository(db), userRepo)
	notificationService := services.NewNotificationService(repositories.NewNotificationRepository(db), userRepo)
	return NewDashboardHandler(services.NewUserService(userRepo), workspaceService, notificationService)
}

func dashboardRequest(user *models.User, path string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	return request.WithContext(WithCurrentUser(request.Context(), user))
}
