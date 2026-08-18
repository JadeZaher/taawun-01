package primitives

import (
	"fmt"
	"sync"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Document represents an LTAP (Hybrid Transactional/Analytical Processing) record.
type Document struct {
	ID        string                 `json:"$id"`
	CreatedAt string                 `json:"$createdAt"`
	UpdatedAt string                 `json:"$updatedAt"`
	Data      map[string]interface{} `json:"data"`
}

// LTAPStorageService manages isolated lightweight SQLite/DuckDB instances per artifact/app.
type LTAPStorageService struct {
	mu        sync.RWMutex
	dataDir   string
	databases map[string]*gorm.DB
}

func NewLTAPStorageService(dataDir string) *LTAPStorageService {
	return &LTAPStorageService{
		dataDir:   dataDir,
		databases: make(map[string]*gorm.DB),
	}
}

// CreateCollection initializes a new collection/table schema for a generated app artifact.
func (s *LTAPStorageService) CreateCollection(artifactID, collectionName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensures database file exists for artifact
	db, err := s.getOrCreateDB(artifactID)
	if err != nil {
		return fmt.Errorf("failed to open database for artifact %s: %w", artifactID, err)
	}

	query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id TEXT PRIMARY KEY,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		payload JSON NOT NULL
	);`, collectionName)

	if err := db.Exec(query).Error; err != nil {
		return fmt.Errorf("failed to create collection %s: %w", collectionName, err)
	}

	return nil
}

func (s *LTAPStorageService) getOrCreateDB(artifactID string) (*gorm.DB, error) {
	if db, ok := s.databases[artifactID]; ok {
		return db, nil
	}

	dbPath := fmt.Sprintf("%s/%s_ltap.db", s.dataDir, artifactID)
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, err
	}

	s.databases[artifactID] = db
	return db, nil
}
