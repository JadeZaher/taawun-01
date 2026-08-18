package shura

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"taawun/pkg/database"
)

type Repository struct {
	db *sql.DB
}

func OpenRepository(databasePath string) (*Repository, error) {
	if databasePath == "" || databasePath == ":memory:" {
		return nil, fmt.Errorf("%w: durable SQLite path required", ErrGovernanceInvalid)
	}
	absPath, err := filepath.Abs(databasePath)
	if err != nil {
		return nil, err
	}
	if info, err := os.Lstat(absPath); err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("%w: database path must be a regular file", ErrGovernanceInvalid)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o700); err != nil {
		return nil, err
	}
	orm, err := gorm.Open(sqlite.Open(absPath+"?_foreign_keys=on&_busy_timeout=5000&_journal_mode=WAL&_synchronous=FULL"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, err
	}
	db, err := database.SQLDB(orm)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	repository, err := NewRepository(db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return repository, nil
}

// NewRepository adopts a caller-owned SQLite database and applies Shura migrations.
func NewRepository(db *sql.DB) (*Repository, error) {
	if db == nil {
		return nil, ErrGovernanceInvalid
	}
	repository := &Repository{db: db}
	if err := repository.migrate(); err != nil {
		return nil, err
	}
	return repository, nil
}

func (r *Repository) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

func (r *Repository) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS shura_invitations (
    id TEXT PRIMARY KEY,
    workspace_id INTEGER NOT NULL,
    invitee TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('Architect','Maintainer','Viewer')),
    token_hash TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL CHECK (status IN ('PENDING','ACCEPTED','EXPIRED','REVOKED')),
    version INTEGER NOT NULL CHECK (version > 0),
    created_by TEXT NOT NULL,
    accepted_by TEXT NOT NULL DEFAULT '',
    expires_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS shura_capabilities (
    jti TEXT PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    issuer TEXT NOT NULL,
    kid TEXT NOT NULL,
    subject TEXT NOT NULL,
    workspace_id INTEGER NOT NULL,
    role TEXT NOT NULL,
    audience TEXT NOT NULL,
    scopes_json TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('ACTIVE','REVOKED')),
    version INTEGER NOT NULL CHECK (version > 0),
    issued_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS shura_revocations (
    jti TEXT PRIMARY KEY REFERENCES shura_capabilities(jti),
    reason TEXT NOT NULL,
    revoked_by TEXT NOT NULL,
    revoked_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS shura_proposals (
    id TEXT PRIMARY KEY,
    workspace_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    policy_json TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('OPEN','DECIDED','CANCELLED')),
    version INTEGER NOT NULL CHECK (version > 0),
    created_by TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS shura_deliberation_entries (
    id TEXT PRIMARY KEY,
    proposal_id TEXT NOT NULL REFERENCES shura_proposals(id),
    author TEXT NOT NULL,
    body TEXT NOT NULL,
    proposal_version INTEGER NOT NULL,
    created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS shura_votes (
    id TEXT PRIMARY KEY,
    proposal_id TEXT NOT NULL REFERENCES shura_proposals(id),
    voter TEXT NOT NULL,
    choice TEXT NOT NULL CHECK (choice IN ('APPROVE','REJECT','ABSTAIN')),
    rationale TEXT NOT NULL,
    role TEXT NOT NULL,
    proposal_version INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    UNIQUE (proposal_id, voter)
);
CREATE TABLE IF NOT EXISTS shura_decisions (
    id TEXT PRIMARY KEY,
    proposal_id TEXT NOT NULL UNIQUE REFERENCES shura_proposals(id),
    outcome TEXT NOT NULL CHECK (outcome IN ('APPROVED','REJECTED')),
    rationale TEXT NOT NULL,
    decided_by TEXT NOT NULL,
    vote_count INTEGER NOT NULL,
    approval_count INTEGER NOT NULL,
    rejection_count INTEGER NOT NULL,
    proposal_version INTEGER NOT NULL,
    created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS shura_audit_events (
    sequence INTEGER PRIMARY KEY AUTOINCREMENT,
    id TEXT NOT NULL UNIQUE,
    workspace_id INTEGER NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    entity_version INTEGER NOT NULL,
    event_type TEXT NOT NULL,
    actor TEXT NOT NULL,
    detail_json TEXT NOT NULL,
    previous_hash TEXT NOT NULL,
    event_hash TEXT NOT NULL UNIQUE,
    created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_shura_audit_entity ON shura_audit_events(entity_type, entity_id, sequence);
CREATE INDEX IF NOT EXISTS idx_shura_proposals_workspace ON shura_proposals(workspace_id, created_at);
CREATE TRIGGER IF NOT EXISTS shura_audit_no_update BEFORE UPDATE ON shura_audit_events BEGIN SELECT RAISE(ABORT, 'Shura audit is append-only'); END;
CREATE TRIGGER IF NOT EXISTS shura_audit_no_delete BEFORE DELETE ON shura_audit_events BEGIN SELECT RAISE(ABORT, 'Shura audit is append-only'); END;
CREATE TRIGGER IF NOT EXISTS shura_deliberation_no_update BEFORE UPDATE ON shura_deliberation_entries BEGIN SELECT RAISE(ABORT, 'Shura deliberation is append-only'); END;
CREATE TRIGGER IF NOT EXISTS shura_deliberation_no_delete BEFORE DELETE ON shura_deliberation_entries BEGIN SELECT RAISE(ABORT, 'Shura deliberation is append-only'); END;
CREATE TRIGGER IF NOT EXISTS shura_votes_no_update BEFORE UPDATE ON shura_votes BEGIN SELECT RAISE(ABORT, 'Shura votes are append-only'); END;
CREATE TRIGGER IF NOT EXISTS shura_votes_no_delete BEFORE DELETE ON shura_votes BEGIN SELECT RAISE(ABORT, 'Shura votes are append-only'); END;
CREATE TRIGGER IF NOT EXISTS shura_decisions_no_update BEFORE UPDATE ON shura_decisions BEGIN SELECT RAISE(ABORT, 'Shura decisions are append-only'); END;
CREATE TRIGGER IF NOT EXISTS shura_decisions_no_delete BEFORE DELETE ON shura_decisions BEGIN SELECT RAISE(ABORT, 'Shura decisions are append-only'); END;`
	if _, err := r.db.Exec(schema); err != nil {
		return fmt.Errorf("migrate Shura database: %w", err)
	}
	return nil
}

func (r *Repository) capabilityActive(ctx context.Context, tokenID, tokenHash string) (bool, error) {
	var status string
	var revocations int
	err := r.db.QueryRowContext(ctx, `SELECT c.status, COUNT(r.jti)
        FROM shura_capabilities c LEFT JOIN shura_revocations r ON r.jti = c.jti
        WHERE c.jti = ? AND c.token_hash = ? GROUP BY c.jti`, tokenID, tokenHash).Scan(&status, &revocations)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return status == "ACTIVE" && revocations == 0, nil
}

func appendAudit(ctx context.Context, tx *sql.Tx, workspaceID int, entityType, entityID string, version int64, eventType, actor string, detail any, now time.Time) error {
	detailJSON, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	var previousHash string
	err = tx.QueryRowContext(ctx, `SELECT event_hash FROM shura_audit_events WHERE entity_type = ? AND entity_id = ? ORDER BY sequence DESC LIMIT 1`, entityType, entityID).Scan(&previousHash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	eventID, err := secureID("shura-event")
	if err != nil {
		return err
	}
	canonical, _ := json.Marshal(struct {
		ID            string          `json:"id"`
		WorkspaceID   int             `json:"workspace_id"`
		EntityType    string          `json:"entity_type"`
		EntityID      string          `json:"entity_id"`
		EntityVersion int64           `json:"entity_version"`
		EventType     string          `json:"event_type"`
		Actor         string          `json:"actor"`
		Detail        json.RawMessage `json:"detail"`
		PreviousHash  string          `json:"previous_hash"`
		CreatedAt     int64           `json:"created_at"`
	}{eventID, workspaceID, entityType, entityID, version, eventType, actor, detailJSON, previousHash, now.UnixMilli()})
	digest := sha256.Sum256(canonical)
	eventHash := hex.EncodeToString(digest[:])
	_, err = tx.ExecContext(ctx, `INSERT INTO shura_audit_events
        (id, workspace_id, entity_type, entity_id, entity_version, event_type, actor, detail_json, previous_hash, event_hash, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		eventID, workspaceID, entityType, entityID, version, eventType, actor, detailJSON, previousHash, eventHash, now.UnixMilli())
	return err
}
