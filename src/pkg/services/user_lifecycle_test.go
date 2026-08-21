package services

import (
	"errors"
	"fmt"
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
	user := &models.User{Username: "audit-subject", Email: "audit-subject@example.com", Password: string(hash), Role: models.RoleUser, Status: models.StatusActive}
	owner := &models.User{Username: "workspace-owner", Email: "workspace-owner@example.com", Password: string(hash), Role: models.RoleUser, Status: models.StatusActive}
	insertSecurityTestUser(t, db, user)
	insertSecurityTestUser(t, db, owner)
	userService := NewUserService(userRepo)
	workspaceService := NewWorkspaceService(workspaceRepo, userRepo)
	workspace, err := workspaceService.CreateWorkspace(owner, &models.CreateWorkspaceRequest{Name: "Preserved workspace"})
	if err != nil {
		t.Fatal(err)
	}
	if err := workspaceRepo.AddUser(workspace.ID, user.ID, models.WorkspaceRoleMember); err != nil {
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
	if strings.Contains(deleted.Username, "audit-subject") || strings.Contains(deleted.Email, "audit-subject@example.com") || deleted.Password == string(hash) {
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

func TestDeleteUserRejectsAnyOwnedWorkspaceWithoutMutation(t *testing.T) {
	tests := []struct {
		name     string
		statuses []string
	}{
		{name: "one active", statuses: []string{models.WorkspaceStatusActive}},
		{name: "many mixed lifecycle states", statuses: []string{models.WorkspaceStatusActive, models.WorkspaceStatusInactive, models.WorkspaceStatusArchived}},
		{name: "one archived", statuses: []string{models.WorkspaceStatusArchived}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, userRepo, workspaceRepo := newSecurityTestRepositories(t)
			for _, statement := range []string{
				`CREATE TABLE oauth_sessions (session_hash TEXT PRIMARY KEY, user_id INTEGER NOT NULL, revoked_at INTEGER)`,
				`CREATE TABLE oauth_consents (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL, revoked_at INTEGER)`,
				`CREATE TABLE oauth_authorization_codes (code_hash TEXT PRIMARY KEY, user_id INTEGER NOT NULL, consumed_at INTEGER)`,
				`CREATE TABLE oauth_access_tokens (token_hash TEXT PRIMARY KEY, user_id INTEGER NOT NULL, revoked_at INTEGER)`,
				`CREATE TABLE oauth_refresh_tokens (token_hash TEXT PRIMARY KEY, user_id INTEGER NOT NULL, revoked_at INTEGER)`,
				`CREATE TABLE notifications (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL, type TEXT, title TEXT, message TEXT, read INTEGER)`,
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
			user := &models.User{Username: "owner", Email: "owner@example.com", Password: string(hash), Role: models.RoleUser, Status: models.StatusActive}
			insertSecurityTestUser(t, db, user)
			workspaceService := NewWorkspaceService(workspaceRepo, userRepo)
			for index, status := range test.statuses {
				workspace, err := workspaceService.CreateWorkspace(user, &models.CreateWorkspaceRequest{Name: fmt.Sprintf("Private workspace %d", index+1)})
				if err != nil {
					t.Fatal(err)
				}
				if status != models.WorkspaceStatusActive {
					if _, err := workspaceService.UpdateWorkspace(user, workspace.ID, &models.UpdateWorkspaceRequest{Status: status}); err != nil {
						t.Fatal(err)
					}
				}
			}
			for _, statement := range []string{
				`INSERT INTO oauth_sessions (session_hash, user_id) VALUES ('session-owned', ?)`,
				`INSERT INTO oauth_consents (id, user_id) VALUES (1, ?)`,
				`INSERT INTO oauth_authorization_codes (code_hash, user_id) VALUES ('code-owned', ?)`,
				`INSERT INTO oauth_access_tokens (token_hash, user_id) VALUES ('access-owned', ?)`,
				`INSERT INTO oauth_refresh_tokens (token_hash, user_id) VALUES ('refresh-owned', ?)`,
			} {
				if _, err := db.Exec(statement, user.ID); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := db.Exec(`INSERT INTO notifications (id, user_id, type, title, message, read) VALUES (1, ?, 'system', 'Private', 'Keep me', 0)`, user.ID); err != nil {
				t.Fatal(err)
			}
			authService, err := NewAuthService(userRepo, []byte("0123456789abcdef0123456789abcdef"))
			if err != nil {
				t.Fatal(err)
			}
			login, err := authService.Login(user.Email, password)
			if err != nil {
				t.Fatal(err)
			}

			if err := NewUserService(userRepo).DeleteUser(user.ID); !errors.Is(err, ErrOwnedWorkspacesRemaining) {
				t.Fatalf("DeleteUser() error = %v, want owned-workspace conflict", err)
			}
			unchanged, err := userRepo.GetByID(user.ID)
			if err != nil {
				t.Fatal(err)
			}
			if unchanged == nil || unchanged.Username != user.Username || unchanged.Email != user.Email || unchanged.Password != user.Password || unchanged.Status != models.StatusActive || unchanged.SessionVersion != 1 {
				t.Fatalf("account changed after rejected deletion: %+v", unchanged)
			}
			if _, err := authService.ValidateToken(login.Token); err != nil {
				t.Fatalf("first-party token was revoked after rejected deletion: %v", err)
			}
			if _, err := authService.Login(user.Email, password); err != nil {
				t.Fatalf("current password stopped authenticating after rejected deletion: %v", err)
			}
			for _, table := range []string{"oauth_sessions", "oauth_consents", "oauth_access_tokens", "oauth_refresh_tokens"} {
				var revokedAt any
				if err := db.QueryRow(`SELECT revoked_at FROM `+table+` WHERE user_id = ?`, user.ID).Scan(&revokedAt); err != nil {
					t.Fatal(err)
				}
				if revokedAt != nil {
					t.Fatalf("%s revoked after rejected deletion: %v", table, revokedAt)
				}
			}
			var consumedAt any
			if err := db.QueryRow(`SELECT consumed_at FROM oauth_authorization_codes WHERE user_id = ?`, user.ID).Scan(&consumedAt); err != nil {
				t.Fatal(err)
			}
			if consumedAt != nil {
				t.Fatalf("OAuth authorization code consumed after rejected deletion: %v", consumedAt)
			}
			var membershipCount int
			if err := db.QueryRow(`SELECT COUNT(*) FROM workspace_users WHERE user_id = ?`, user.ID).Scan(&membershipCount); err != nil || membershipCount != len(test.statuses) {
				t.Fatalf("owner memberships = %d, err = %v", membershipCount, err)
			}
			var notificationCount int
			if err := db.QueryRow(`SELECT COUNT(*) FROM notifications WHERE user_id = ?`, user.ID).Scan(&notificationCount); err != nil || notificationCount != 1 {
				t.Fatalf("notifications = %d, err = %v", notificationCount, err)
			}
		})
	}
}

func TestDeleteOwnedWorkspaceThenSelfDeleteSucceeds(t *testing.T) {
	db, userRepo, workspaceRepo := newSecurityTestRepositories(t)
	password := "private-beta-password"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	user := &models.User{Username: "sequence-owner", Email: "sequence-owner@example.com", Password: string(hash), Role: models.RoleUser, Status: models.StatusActive}
	insertSecurityTestUser(t, db, user)
	workspaceService := NewWorkspaceService(workspaceRepo, userRepo)
	workspace, err := workspaceService.CreateWorkspace(user, &models.CreateWorkspaceRequest{Name: "Delete first"})
	if err != nil {
		t.Fatal(err)
	}
	userService := NewUserService(userRepo)
	if err := userService.DeleteUser(user.ID); !errors.Is(err, ErrOwnedWorkspacesRemaining) {
		t.Fatalf("first DeleteUser() error = %v, want owned-workspace conflict", err)
	}
	authService, err := NewAuthService(userRepo, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	login, err := authService.Login(user.Email, password)
	if err != nil {
		t.Fatal(err)
	}
	if err := workspaceService.DeleteWorkspace(user, workspace.ID); err != nil {
		t.Fatal(err)
	}
	if err := userService.DeleteUser(user.ID); err != nil {
		t.Fatalf("DeleteUser() after workspace deletion error = %v", err)
	}
	if _, err := authService.ValidateToken(login.Token); err == nil {
		t.Fatal("first-party token remained valid after successful account deletion")
	}
}
