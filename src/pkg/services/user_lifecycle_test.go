package services

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"taawun/pkg/models"
)

func TestDeleteUserAnonymizesAndRevokesAuthorityWithoutDeletingAuditRows(t *testing.T) {
	db, userRepo, workspaceRepo := newSecurityTestRepositories(t)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE notifications (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id), type TEXT, title TEXT, message TEXT, read INTEGER, created_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE workspace_domain_claims (id TEXT PRIMARY KEY, workspace_id INTEGER NOT NULL REFERENCES workspaces(id), claimed_by INTEGER NOT NULL REFERENCES users(id))`,
		`CREATE TABLE oauth_sessions (session_hash TEXT PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id), revoked_at INTEGER)`,
		`CREATE TABLE oauth_consents (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id), revoked_at INTEGER)`,
		`CREATE TABLE oauth_authorization_codes (code_hash TEXT PRIMARY KEY, user_id INTEGER NOT NULL, consumed_at INTEGER)`,
		`CREATE TABLE oauth_access_tokens (token_hash TEXT PRIMARY KEY, user_id INTEGER NOT NULL, revoked_at INTEGER)`,
		`CREATE TABLE oauth_refresh_tokens (token_hash TEXT PRIMARY KEY, user_id INTEGER NOT NULL, revoked_at INTEGER)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}

	password := "private-beta-password"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	user := &models.User{Username: "domain-owner", Email: "domain-owner@example.com", Password: string(hash), Role: models.RoleUser, Status: models.StatusActive}
	insertSecurityTestUser(t, db, user)
	userService := NewUserService(userRepo)
	workspaceService := NewWorkspaceService(workspaceRepo, userRepo)
	workspace, err := workspaceService.CreateWorkspace(user, &models.CreateWorkspaceRequest{Name: "Preserved workspace"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO notifications (id, user_id, type, title, message, read) VALUES (1, ?, 'system', 'Private', 'Remove me', 0)`, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO workspace_domain_claims (id, workspace_id, claimed_by) VALUES ('claim-1', ?, ?)`, workspace.ID, user.ID); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`INSERT INTO oauth_sessions (session_hash, user_id) VALUES ('session-1', ?)`,
		`INSERT INTO oauth_consents (id, user_id) VALUES (1, ?)`,
		`INSERT INTO oauth_authorization_codes (code_hash, user_id) VALUES ('code-1', ?)`,
		`INSERT INTO oauth_access_tokens (token_hash, user_id) VALUES ('access-1', ?)`,
		`INSERT INTO oauth_refresh_tokens (token_hash, user_id) VALUES ('refresh-1', ?)`,
	} {
		if _, err := db.Exec(statement, user.ID); err != nil {
			t.Fatal(err)
		}
	}

	authService, err := NewAuthService(userRepo, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	login, err := authService.Login(user.Email, password)
	if err != nil {
		t.Fatal(err)
	}
	originalSessionVersion := user.SessionVersion
	if originalSessionVersion == 0 {
		originalSessionVersion = 1
	}

	if err := userService.DeleteUser(user.ID); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}
	deleted, err := userRepo.GetByID(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if deleted == nil || deleted.Status != models.StatusDeleted {
		t.Fatalf("deleted account = %+v", deleted)
	}
	if deleted.SessionVersion != originalSessionVersion+1 {
		t.Fatalf("session version = %d, want %d", deleted.SessionVersion, originalSessionVersion+1)
	}
	if strings.Contains(deleted.Username, "domain-owner") || strings.Contains(deleted.Email, "domain-owner@example.com") || deleted.Password == string(hash) {
		t.Fatalf("account was not anonymized: %+v", deleted)
	}
	if _, err := authService.ValidateToken(login.Token); err == nil {
		t.Fatal("first-party token remained valid after account deletion")
	}
	if _, err := workspaceService.AuthorizeWorkspaceCapability(user, workspace.ID, models.WorkspaceCapabilityPublish); !errors.Is(err, ErrWorkspaceForbidden) {
		t.Fatalf("deleted owner workspace authorization error = %v, want forbidden", err)
	}
	if _, err := userService.UpdateUser(user.ID, &models.UpdateUserRequest{Email: user.Email, Password: password}); err == nil {
		t.Fatal("deleted account was restored through the profile update path")
	}
	if err := userRepo.UpdateStatus(user.ID, models.StatusActive); err == nil {
		t.Fatal("deleted account was restored through the administrative status path")
	}

	var workspaceCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM workspaces WHERE id = ?`, workspace.ID).Scan(&workspaceCount); err != nil || workspaceCount != 1 {
		t.Fatalf("preserved workspace rows = %d, err = %v", workspaceCount, err)
	}
	var claimCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM workspace_domain_claims WHERE id = 'claim-1' AND claimed_by = ?`, user.ID).Scan(&claimCount); err != nil || claimCount != 1 {
		t.Fatalf("preserved domain claim rows = %d, err = %v", claimCount, err)
	}
	for _, table := range []string{"workspace_users", "notifications"} {
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE user_id = ?`, user.ID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("remaining %s grants/data = %d, err = %v", table, count, err)
		}
	}
	for _, table := range []string{"oauth_sessions", "oauth_consents", "oauth_access_tokens", "oauth_refresh_tokens"} {
		var revokedAt int64
		if err := db.QueryRow(`SELECT revoked_at FROM `+table+` WHERE user_id = ?`, user.ID).Scan(&revokedAt); err != nil || revokedAt <= 0 {
			t.Fatalf("%s revoked_at = %d, err = %v", table, revokedAt, err)
		}
	}
	var consumedAt int64
	if err := db.QueryRow(`SELECT consumed_at FROM oauth_authorization_codes WHERE user_id = ?`, user.ID).Scan(&consumedAt); err != nil || consumedAt <= 0 {
		t.Fatalf("authorization code consumed_at = %d, err = %v", consumedAt, err)
	}

	username, email := deleted.Username, deleted.Email
	if err := userService.DeleteUser(user.ID); err != nil {
		t.Fatalf("idempotent DeleteUser() error = %v", err)
	}
	deleted, err = userRepo.GetByID(user.ID)
	if err != nil || deleted.Username != username || deleted.Email != email {
		t.Fatalf("idempotent tombstone changed identity: %+v, %v", deleted, err)
	}
}
