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

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// Migration-only records preserve the deployed SQLite table and index names.
type userSchema struct {
	ID             int       `gorm:"primaryKey;autoIncrement"`
	Username       string    `gorm:"not null;uniqueIndex:idx_users_username"`
	Email          string    `gorm:"not null;uniqueIndex:idx_users_email"`
	Password       string    `gorm:"not null"`
	Role           string    `gorm:"not null;default:user"`
	Status         string    `gorm:"not null;default:active"`
	SessionVersion int64     `gorm:"not null;default:1"`
	CreatedAt      time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt      time.Time `gorm:"default:CURRENT_TIMESTAMP"`
}

func (userSchema) TableName() string { return "users" }

type workspaceSchema struct {
	ID          int    `gorm:"primaryKey;autoIncrement"`
	Name        string `gorm:"not null"`
	Description string
	OwnerID     int        `gorm:"not null;index:idx_workspaces_owner"`
	Status      string     `gorm:"not null;default:active"`
	CreatedAt   time.Time  `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt   time.Time  `gorm:"default:CURRENT_TIMESTAMP"`
	Owner       userSchema `gorm:"foreignKey:OwnerID;references:ID;constraint:OnDelete:CASCADE"`
}

func (workspaceSchema) TableName() string { return "workspaces" }

type workspaceUserSchema struct {
	WorkspaceID int             `gorm:"primaryKey"`
	UserID      int             `gorm:"primaryKey"`
	Role        string          `gorm:"not null;default:member"`
	JoinedAt    time.Time       `gorm:"default:CURRENT_TIMESTAMP"`
	Workspace   workspaceSchema `gorm:"foreignKey:WorkspaceID;references:ID;constraint:OnDelete:CASCADE"`
	User        userSchema      `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
}

func (workspaceUserSchema) TableName() string { return "workspace_users" }

type notificationSchema struct {
	ID        int        `gorm:"primaryKey;autoIncrement"`
	UserID    int        `gorm:"not null;index:idx_notifications_user"`
	Type      string     `gorm:"not null"`
	Title     string     `gorm:"not null"`
	Message   string     `gorm:"not null"`
	Read      bool       `gorm:"default:false;index:idx_notifications_read"`
	CreatedAt time.Time  `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time  `gorm:"default:CURRENT_TIMESTAMP"`
	User      userSchema `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
}

func (notificationSchema) TableName() string { return "notifications" }

func InitDB() (*gorm.DB, error) {
	dbPath := strings.TrimSpace(os.Getenv("APP_DB_PATH"))
	if dbPath == "" {
		dbPath = filepath.Join("data", "user_auth.db")
	}
	absPath, err := filepath.Abs(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve database path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0700); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	db, err := gorm.Open(sqlite.Open(sqliteDSN(absPath)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	sqlDB, err := SQLDB(db)
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(8)
	sqlDB.SetMaxIdleConns(8)
	sqlDB.SetConnMaxLifetime(0)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	if err := migrateSharedSchema(db); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}
	if err := bootstrapAdmin(db); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	DB = db
	log.Printf("Database initialized successfully at: %s", absPath)
	return db, nil
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

func migrateSharedSchema(db *gorm.DB) error {
	migrator := db.Migrator()
	for _, model := range []any{&userSchema{}, &workspaceSchema{}, &workspaceUserSchema{}, &notificationSchema{}} {
		if migrator.HasTable(model) {
			continue
		}
		if err := migrator.CreateTable(model); err != nil {
			return fmt.Errorf("create %T: %w", model, err)
		}
	}
	if !migrator.HasColumn(&userSchema{}, "SessionVersion") {
		if err := migrator.AddColumn(&userSchema{}, "SessionVersion"); err != nil {
			return fmt.Errorf("add users session version: %w", err)
		}
	}
	indexes := []struct {
		model any
		name  string
	}{
		{&userSchema{}, "idx_users_email"},
		{&userSchema{}, "idx_users_username"},
		{&workspaceSchema{}, "idx_workspaces_owner"},
		{&notificationSchema{}, "idx_notifications_user"},
		{&notificationSchema{}, "idx_notifications_read"},
	}
	for _, index := range indexes {
		if migrator.HasIndex(index.model, index.name) {
			continue
		}
		if err := migrator.CreateIndex(index.model, index.name); err != nil {
			return fmt.Errorf("create index %s: %w", index.name, err)
		}
	}
	return nil
}

func bootstrapAdmin(db *gorm.DB) error {
	username := strings.TrimSpace(os.Getenv("TAWUN_BOOTSTRAP_ADMIN_USERNAME"))
	email := strings.TrimSpace(os.Getenv("TAWUN_BOOTSTRAP_ADMIN_EMAIL"))
	password := os.Getenv("TAWUN_BOOTSTRAP_ADMIN_PASSWORD")
	if username == "" && email == "" && password == "" {
		return nil
	}
	if username == "" || email == "" || len(password) < 12 {
		return fmt.Errorf("bootstrap admin requires username, email, and a password of at least 12 characters")
	}
	var count int64
	if err := db.Model(&userSchema{}).Where("role = ?", "admin").Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check admin user: %w", err)
	}
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash bootstrap admin password: %w", err)
	}
	admin := userSchema{Username: username, Email: email, Password: string(hash), Role: "admin", Status: "active", SessionVersion: 1}
	if err := db.Create(&admin).Error; err != nil {
		return fmt.Errorf("failed to create bootstrap admin: %w", err)
	}
	log.Printf("Bootstrap admin created for %s", email)
	return nil
}

func GetDB() *gorm.DB { return DB }

// SQLDB exposes GORM's pooled connection to append-only stores with hand-tuned SQL.
func SQLDB(db *gorm.DB) (*sql.DB, error) {
	if db == nil {
		return nil, fmt.Errorf("database is required")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("open SQL connection pool: %w", err)
	}
	return sqlDB, nil
}
