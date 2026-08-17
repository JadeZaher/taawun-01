package database

import (
	"database/sql"
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
		defer db.Close()
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
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
		defer db.Close()
		var storedPassword, role string
		if err := db.QueryRow("SELECT password, role FROM users WHERE email = ?", "admin@example.com").Scan(&storedPassword, &role); err != nil {
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
	defer db.Close()
	var version int64
	if err := db.QueryRow(`SELECT session_version FROM users WHERE email = 'member@example.com'`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 1 {
		t.Fatalf("legacy user session version = %d, want 1", version)
	}
}
