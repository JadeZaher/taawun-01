package database

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestInitDBBootstrapsAdminOnlyFromCompleteEnvironment(t *testing.T) {
	t.Run("disabled by default", func(t *testing.T) {
		t.Setenv("APP_DB_PATH", filepath.Join(t.TempDir(), "default.db"))
		t.Setenv("TAWUN_BOOTSTRAP_ADMIN_USERNAME", "")
		t.Setenv("TAWUN_BOOTSTRAP_ADMIN_EMAIL", "")
		t.Setenv("TAWUN_BOOTSTRAP_ADMIN_PASSWORD", "")
		db, err := InitDB()
		if err != nil {
			t.Fatal(err)
		}
		sqlDB, err := SQLDB(db)
		if err != nil {
			t.Fatal(err)
		}
		defer sqlDB.Close()
		var count int
		if err := sqlDB.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("user count = %d, want 0", count)
		}
	})

	t.Run("rejects partial configuration", func(t *testing.T) {
		t.Setenv("APP_DB_PATH", filepath.Join(t.TempDir(), "partial.db"))
		t.Setenv("TAWUN_BOOTSTRAP_ADMIN_USERNAME", "admin")
		t.Setenv("TAWUN_BOOTSTRAP_ADMIN_EMAIL", "")
		t.Setenv("TAWUN_BOOTSTRAP_ADMIN_PASSWORD", "short")
		if db, err := InitDB(); err == nil || db != nil {
			t.Fatalf("InitDB() = (%v, %v), want nil database and error", db, err)
		}
	})

	t.Run("hashes configured bootstrap password", func(t *testing.T) {
		password := "correct horse battery staple"
		t.Setenv("APP_DB_PATH", filepath.Join(t.TempDir(), "configured.db"))
		t.Setenv("TAWUN_BOOTSTRAP_ADMIN_USERNAME", "bootstrap-admin")
		t.Setenv("TAWUN_BOOTSTRAP_ADMIN_EMAIL", "admin@example.com")
		t.Setenv("TAWUN_BOOTSTRAP_ADMIN_PASSWORD", password)
		db, err := InitDB()
		if err != nil {
			t.Fatal(err)
		}
		sqlDB, err := SQLDB(db)
		if err != nil {
			t.Fatal(err)
		}
		defer sqlDB.Close()
		var storedPassword, role string
		if err := sqlDB.QueryRow("SELECT password, role FROM users WHERE email = ?", "admin@example.com").Scan(&storedPassword, &role); err != nil {
			t.Fatal(err)
		}
		if storedPassword == password {
			t.Fatal("bootstrap password was stored in plaintext")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(password)); err != nil {
			t.Fatalf("stored password is not the expected bcrypt hash: %v", err)
		}
		if role != "admin" {
			t.Fatalf("role = %q, want admin", role)
		}
	})
}

func TestInitDBAddsSessionVersionToExistingIdentityDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	legacy, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Exec(`CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		email TEXT NOT NULL UNIQUE,
		password TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'user',
		status TEXT NOT NULL DEFAULT 'active',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		_ = legacy.Close()
		t.Fatal(err)
	}
	if _, err := legacy.Exec(`INSERT INTO users (username, email, password, role, status) VALUES ('member', 'member@example.com', 'hash', 'user', 'active')`); err != nil {
		_ = legacy.Close()
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APP_DB_PATH", path)
	t.Setenv("TAWUN_BOOTSTRAP_ADMIN_USERNAME", "")
	t.Setenv("TAWUN_BOOTSTRAP_ADMIN_EMAIL", "")
	t.Setenv("TAWUN_BOOTSTRAP_ADMIN_PASSWORD", "")
	db, err := InitDB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := SQLDB(db)
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	var version int64
	if err := sqlDB.QueryRow(`SELECT session_version FROM users WHERE email = 'member@example.com'`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 1 {
		t.Fatalf("legacy user session version = %d, want 1", version)
	}
}

func TestInitDBAdoptsDeployedSchemaWithoutRebuildingTables(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deployed.db")
	legacy, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`PRAGMA foreign_keys = ON`,
		`CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			email TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'user',
			status TEXT NOT NULL DEFAULT 'active',
			session_version INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE workspaces (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT,
			owner_id INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (owner_id) REFERENCES users (id) ON DELETE CASCADE
		)`,
		`CREATE TABLE workspace_users (
			workspace_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			role TEXT NOT NULL DEFAULT 'member',
			joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (workspace_id, user_id),
			FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
		)`,
		`CREATE TABLE notifications (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			type TEXT NOT NULL,
			title TEXT NOT NULL,
			message TEXT NOT NULL,
			read BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
		)`,
		`CREATE INDEX idx_users_email ON users(email)`,
		`CREATE INDEX idx_users_username ON users(username)`,
		`CREATE INDEX idx_workspaces_owner ON workspaces(owner_id)`,
		`CREATE INDEX idx_notifications_user ON notifications(user_id)`,
		`CREATE INDEX idx_notifications_read ON notifications(read)`,
		`INSERT INTO users (id, username, email, password) VALUES (7, 'member', 'member@example.com', 'hash')`,
		`INSERT INTO workspaces (id, name, description, owner_id) VALUES (9, 'Community', 'Existing data', 7)`,
		`INSERT INTO workspace_users (workspace_id, user_id, role) VALUES (9, 7, 'owner')`,
		`INSERT INTO notifications (id, user_id, type, title, message) VALUES (11, 7, 'system', 'Existing', 'Preserve me')`,
	}
	for _, statement := range statements {
		if _, err := legacy.Exec(statement); err != nil {
			_ = legacy.Close()
			t.Fatalf("prepare deployed database: %v", err)
		}
	}
	tables := []string{"users", "workspaces", "workspace_users", "notifications"}
	originalDefinitions := make(map[string]string, len(tables))
	for _, table := range tables {
		var definition string
		if err := legacy.QueryRow(`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&definition); err != nil {
			_ = legacy.Close()
			t.Fatal(err)
		}
		originalDefinitions[table] = definition
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	t.Setenv("APP_DB_PATH", path)
	t.Setenv("TAWUN_BOOTSTRAP_ADMIN_USERNAME", "")
	t.Setenv("TAWUN_BOOTSTRAP_ADMIN_EMAIL", "")
	t.Setenv("TAWUN_BOOTSTRAP_ADMIN_PASSWORD", "")
	db, err := InitDB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := SQLDB(db)
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	for _, table := range tables {
		var definition string
		if err := sqlDB.QueryRow(`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&definition); err != nil {
			t.Fatal(err)
		}
		if definition != originalDefinitions[table] {
			t.Fatalf("%s table was rebuilt\nbefore: %s\nafter:  %s", table, originalDefinitions[table], definition)
		}
	}
	var workspaceName, membershipRole, notificationMessage string
	if err := sqlDB.QueryRow(`SELECT w.name, wu.role, n.message
		FROM workspaces w
		JOIN workspace_users wu ON wu.workspace_id = w.id
		JOIN notifications n ON n.user_id = wu.user_id
		WHERE w.id = 9 AND wu.user_id = 7 AND n.id = 11`).Scan(&workspaceName, &membershipRole, &notificationMessage); err != nil {
		t.Fatal(err)
	}
	if workspaceName != "Community" || membershipRole != "owner" || notificationMessage != "Preserve me" {
		t.Fatalf("deployed rows changed: %q %q %q", workspaceName, membershipRole, notificationMessage)
	}
}

func TestInitDBCreatesFreshForeignKeysAndIndexes(t *testing.T) {
	t.Setenv("APP_DB_PATH", filepath.Join(t.TempDir(), "fresh.db"))
	t.Setenv("TAWUN_BOOTSTRAP_ADMIN_USERNAME", "")
	t.Setenv("TAWUN_BOOTSTRAP_ADMIN_EMAIL", "")
	t.Setenv("TAWUN_BOOTSTRAP_ADMIN_PASSWORD", "")
	db, err := InitDB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := SQLDB(db)
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	for _, index := range []string{"idx_users_email", "idx_users_username", "idx_workspaces_owner", "idx_notifications_user", "idx_notifications_read"} {
		var count int
		if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?`, index).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("fresh index %s count = %d, want 1", index, count)
		}
	}
	if _, err := sqlDB.Exec(`INSERT INTO workspaces (name, owner_id) VALUES ('orphan', 999)`); err == nil {
		t.Fatal("fresh workspace foreign key accepted a missing owner")
	}
	for _, table := range []string{"workspaces", "workspace_users", "notifications"} {
		rows, err := sqlDB.Query(fmt.Sprintf("PRAGMA foreign_key_list(%s)", table))
		if err != nil {
			t.Fatal(err)
		}
		if !rows.Next() {
			_ = rows.Close()
			t.Fatalf("fresh table %s has no foreign keys", table)
		}
		if err := rows.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
