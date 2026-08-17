package services

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"taawun/pkg/models"
	"taawun/pkg/repositories"
)

func newSecurityTestRepositories(t *testing.T) (*sql.DB, *repositories.UserRepository, *repositories.WorkspaceRepository) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })

	statements := []string{
		`CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			email TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			role TEXT NOT NULL,
			status TEXT NOT NULL,
			session_version INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE workspaces (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT,
			owner_id INTEGER NOT NULL,
			status TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE workspace_users (
			workspace_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			role TEXT NOT NULL,
			joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (workspace_id, user_id)
		)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	return db, repositories.NewUserRepository(db), repositories.NewWorkspaceRepository(db)
}

func insertSecurityTestUser(t *testing.T, db *sql.DB, user *models.User) {
	t.Helper()
	result, err := db.Exec(`INSERT INTO users (username, email, password, role, status) VALUES (?, ?, ?, ?, ?)`,
		user.Username, user.Email, user.Password, user.Role, user.Status)
	if err != nil {
		t.Fatal(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	user.ID = int(id)
}
