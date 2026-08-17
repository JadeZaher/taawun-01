package database

import (
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
