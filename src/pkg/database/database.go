package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDB() (*sql.DB, error) {
	var err error

	// Central point of truth for database path
	dbPath := os.Getenv("APP_DB_PATH")
	if dbPath == "" {
		dbPath = filepath.Join("data", "user_auth.db")
	}

	// Ensure the directory exists before SQLite tries to write
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %v", err)
	}

	DB, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	if err = DB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	// Create tables
	if err = createTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %v", err)
	}

	log.Printf("Database initialized successfully at: %s", dbPath)
	return DB, nil
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

	// Create default admin user if not exists
	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check admin user: %v", err)
	}

	if count == 0 {
		_, err = DB.Exec(`INSERT INTO users (username, email, password, role, status)
			VALUES ('admin', 'admin@example.com', '$2a$10$X1Y2Z3A4B5C6D7E8F9G0H1I2J3K4L5M6N7O8P9Q0R1S2T3U4V5W6X7Y8Z9A0B', 'admin', 'active')`)
		if err != nil {
			return fmt.Errorf("failed to create default admin: %v", err)
		}
		log.Println("Default admin user created: admin@example.com / password: admin123")
	}

	return nil
}

func GetDB() *sql.DB {
	return DB
}
