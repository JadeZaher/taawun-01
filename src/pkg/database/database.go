package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

var DB *sql.DB

func InitDB() (*sql.DB, error) {
	var err error

	dbPath := strings.TrimSpace(os.Getenv("APP_DB_PATH"))
	if dbPath == "" {
		dbPath = filepath.Join("data", "user_auth.db")
	}
	absPath, err := filepath.Abs(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve database path: %w", err)
	}

	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	DB, err = sql.Open("sqlite3", sqliteDSN(absPath))
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	DB.SetMaxOpenConns(8)
	DB.SetMaxIdleConns(8)
	DB.SetConnMaxLifetime(0)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = DB.PingContext(ctx); err != nil {
		_ = DB.Close()
		DB = nil
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Create tables
	if err = createTables(); err != nil {
		_ = DB.Close()
		DB = nil
		return nil, fmt.Errorf("failed to create tables: %v", err)
	}

	log.Printf("Database initialized successfully at: %s", absPath)
	return DB, nil
}

func sqliteDSN(absPath string) string {
	uriPath := filepath.ToSlash(absPath)
	if filepath.VolumeName(absPath) != "" && !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	uri := url.URL{Scheme: "file", Path: uriPath}
	query := uri.Query()
	query.Set("_busy_timeout", "5000")
	query.Set("_foreign_keys", "on")
	query.Set("_journal_mode", "WAL")
	query.Set("_synchronous", "NORMAL")
	query.Set("_txlock", "immediate")
	uri.RawQuery = query.Encode()
	return uri.String()
}

func createTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
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
		`CREATE TABLE IF NOT EXISTS workspaces (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT,
			owner_id INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (owner_id) REFERENCES users (id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS workspace_users (
			workspace_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			role TEXT NOT NULL DEFAULT 'member',
			joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (workspace_id, user_id),
			FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS notifications (
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
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,
		`CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)`,
		`CREATE INDEX IF NOT EXISTS idx_workspaces_owner ON workspaces(owner_id)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_user ON notifications(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_read ON notifications(read)`,
	}

	for _, query := range queries {
		if _, err := DB.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query: %v", err)
		}
	}
	if err := ensureUserSessionVersionColumn(); err != nil {
		return err
	}

	return bootstrapAdmin()
}

// ensureUserSessionVersionColumn upgrades databases created before session invalidation existed.
func ensureUserSessionVersionColumn() error {
	rows, err := DB.Query(`PRAGMA table_info(users)`)
	if err != nil {
		return fmt.Errorf("inspect users schema: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, typeName string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &typeName, &notNull, &defaultValue, &primaryKey); err != nil {
			return fmt.Errorf("scan users schema: %w", err)
		}
		if name == "session_version" {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read users schema: %w", err)
	}
	if _, err := DB.Exec(`ALTER TABLE users ADD COLUMN session_version INTEGER NOT NULL DEFAULT 1`); err != nil {
		return fmt.Errorf("add users session version: %w", err)
	}
	return nil
}

func bootstrapAdmin() error {
	username := strings.TrimSpace(os.Getenv("TAWUN_BOOTSTRAP_ADMIN_USERNAME"))
	email := strings.TrimSpace(os.Getenv("TAWUN_BOOTSTRAP_ADMIN_EMAIL"))
	password := os.Getenv("TAWUN_BOOTSTRAP_ADMIN_PASSWORD")
	if username == "" && email == "" && password == "" {
		return nil
	}
	if username == "" || email == "" || len(password) < 12 {
		return fmt.Errorf("bootstrap admin requires username, email, and a password of at least 12 characters")
	}

	var count int
	if err := DB.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&count); err != nil {
		return fmt.Errorf("failed to check admin user: %v", err)
	}
	if count > 0 {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash bootstrap admin password: %v", err)
	}
	if _, err := DB.Exec(`INSERT INTO users (username, email, password, role, status)
		VALUES (?, ?, ?, 'admin', 'active')`, username, email, string(hash)); err != nil {
		return fmt.Errorf("failed to create bootstrap admin: %v", err)
	}
	log.Printf("Bootstrap admin created for %s", email)
	return nil
}

func GetDB() *sql.DB {
	return DB
}
