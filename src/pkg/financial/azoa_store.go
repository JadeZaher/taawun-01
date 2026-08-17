package financial

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

	_ "github.com/mattn/go-sqlite3"
)

const questSelect = `SELECT
    q.id, q.workspace_id, q.flow_id, q.flow_version, q.status, q.version,
    q.approvals_required, q.approvals_received, q.provider_reference,
    q.reconciliation_reference, q.failure_code, q.created_at, q.updated_at,
    i.id, i.currency, i.amount_minor, i.fee_minor, i.parties_json,
    i.allocations_json, i.memo, i.status, i.version
FROM quests q JOIN intents i ON i.quest_id = q.id`

func openAzoaDatabase(databasePath string) (*sql.DB, error) {
	absPath, err := filepath.Abs(databasePath)
	if err != nil {
		return nil, fmt.Errorf("resolve AZOA database path: %w", err)
	}
	if info, err := os.Lstat(absPath); err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("%w: database path must be a regular file", ErrInvalidQuest)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o700); err != nil {
		return nil, fmt.Errorf("create AZOA database directory: %w", err)
	}
	dsn := absPath + "?_foreign_keys=on&_busy_timeout=5000&_journal_mode=WAL&_synchronous=FULL"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open AZOA database: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping AZOA database: %w", err)
	}
	return db, nil
}

func (o *AzoaOrchestrator) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS quests (
    id TEXT PRIMARY KEY,
    workspace_id INTEGER NOT NULL CHECK (workspace_id > 0),
    flow_id TEXT NOT NULL,
    flow_version INTEGER NOT NULL CHECK (flow_version > 0),
    idempotency_key TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('PENDING','APPROVED','EXECUTING','SETTLED','FAILED','CANCELLED')),
    version INTEGER NOT NULL CHECK (version > 0),
    approvals_required INTEGER NOT NULL CHECK (approvals_required > 0),
    approvals_received INTEGER NOT NULL CHECK (approvals_received >= 0),
    provider_reference TEXT NOT NULL DEFAULT '',
    reconciliation_reference TEXT NOT NULL DEFAULT '',
    failure_code TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE (workspace_id, idempotency_key)
);
CREATE TABLE IF NOT EXISTS intents (
    id TEXT PRIMARY KEY,
    quest_id TEXT NOT NULL UNIQUE REFERENCES quests(id),
    currency TEXT NOT NULL CHECK (length(currency) = 3),
    amount_minor INTEGER NOT NULL CHECK (amount_minor > 0),
    fee_minor INTEGER NOT NULL CHECK (fee_minor >= 0),
    parties_json TEXT NOT NULL,
    allocations_json TEXT NOT NULL,
    memo TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('PENDING','APPROVED','EXECUTING','SETTLED','FAILED','CANCELLED')),
    version INTEGER NOT NULL CHECK (version > 0)
);
CREATE TABLE IF NOT EXISTS approvals (
    quest_id TEXT NOT NULL REFERENCES quests(id),
    actor_id TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (quest_id, actor_id)
);
CREATE TABLE IF NOT EXISTS quest_events (
    sequence INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id TEXT NOT NULL UNIQUE,
    quest_id TEXT NOT NULL REFERENCES quests(id),
    quest_version INTEGER NOT NULL,
    event_type TEXT NOT NULL,
    from_status TEXT NOT NULL,
    to_status TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    detail_json TEXT NOT NULL,
    previous_hash TEXT NOT NULL,
    event_hash TEXT NOT NULL UNIQUE,
    created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_quest_events_quest_sequence ON quest_events(quest_id, sequence);
CREATE TRIGGER IF NOT EXISTS quest_events_no_update
BEFORE UPDATE ON quest_events BEGIN SELECT RAISE(ABORT, 'quest events are append-only'); END;
CREATE TRIGGER IF NOT EXISTS quest_events_no_delete
BEFORE DELETE ON quest_events BEGIN SELECT RAISE(ABORT, 'quest events are append-only'); END;`
	if _, err := o.db.Exec(schema); err != nil {
		return fmt.Errorf("migrate AZOA database: %w", err)
	}
	return nil
}

type rowQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func getQuestDB(ctx context.Context, db *sql.DB, questID string) (*Quest, error) {
	return scanQuest(db.QueryRowContext(ctx, questSelect+` WHERE q.id = ?`, questID))
}

func getQuestTx(ctx context.Context, tx *sql.Tx, questID string) (*Quest, error) {
	return scanQuest(tx.QueryRowContext(ctx, questSelect+` WHERE q.id = ?`, questID))
}

func getQuestByIdempotencyTx(ctx context.Context, tx *sql.Tx, workspaceID int64, key string) (*Quest, string, error) {
	var questID, requestHash string
	err := tx.QueryRowContext(ctx, `SELECT id, request_hash FROM quests WHERE workspace_id = ? AND idempotency_key = ?`, workspaceID, key).Scan(&questID, &requestHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", ErrQuestNotFound
	}
	if err != nil {
		return nil, "", err
	}
	quest, err := getQuestTx(ctx, tx, questID)
	return quest, requestHash, err
}

type scanner interface{ Scan(...any) error }

func scanQuest(row scanner) (*Quest, error) {
	var quest Quest
	var flowID, questStatus, intentStatus string
	var providerReference, reconciliationReference, failureCode string
	var partiesJSON, allocationsJSON []byte
	var createdAt, updatedAt int64
	err := row.Scan(
		&quest.ID, &quest.WorkspaceID, &flowID, &quest.FlowVersion, &questStatus, &quest.Version,
		&quest.ApprovalsRequired, &quest.ApprovalsReceived, &providerReference,
		&reconciliationReference, &failureCode, &createdAt, &updatedAt,
		&quest.Intent.ID, &quest.Intent.Currency, &quest.Intent.AmountMinor, &quest.Intent.FeeMinor,
		&partiesJSON, &allocationsJSON, &quest.Intent.Memo, &intentStatus, &quest.Intent.Version,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrQuestNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read financial quest: %w", err)
	}
	quest.FlowID = FlowID(flowID)
	quest.Status = QuestStatus(questStatus)
	quest.ProviderReference = providerReference
	quest.ReconciliationReference = reconciliationReference
	quest.FailureCode = failureCode
	quest.CreatedAt = time.UnixMilli(createdAt).UTC()
	quest.UpdatedAt = time.UnixMilli(updatedAt).UTC()
	quest.Intent.QuestID = quest.ID
	quest.Intent.Status = QuestStatus(intentStatus)
	if err := json.Unmarshal(partiesJSON, &quest.Intent.Parties); err != nil {
		return nil, fmt.Errorf("decode quest parties: %w", err)
	}
	if err := json.Unmarshal(allocationsJSON, &quest.Intent.Allocations); err != nil {
		return nil, fmt.Errorf("decode quest allocations: %w", err)
	}
	return &quest, nil
}

func updateQuestState(ctx context.Context, tx *sql.Tx, quest *Quest, to QuestStatus, actorID, eventType string, detail any, providerReference, reconciliationReference, failureCode string, now time.Time, approval bool) (*Quest, error) {
	newVersion := quest.Version + 1
	approvalIncrement := 0
	if approval {
		approvalIncrement = 1
	}
	result, err := tx.ExecContext(ctx, `UPDATE quests SET
        status = ?, version = ?, approvals_received = approvals_received + ?,
        provider_reference = CASE WHEN ? <> '' THEN ? ELSE provider_reference END,
        reconciliation_reference = CASE WHEN ? <> '' THEN ? ELSE reconciliation_reference END,
        failure_code = CASE WHEN ? <> '' THEN ? ELSE failure_code END,
        updated_at = ?
        WHERE id = ? AND version = ? AND status = ?`,
		to, newVersion, approvalIncrement,
		providerReference, providerReference, reconciliationReference, reconciliationReference,
		failureCode, failureCode, now.UnixMilli(), quest.ID, quest.Version, quest.Status)
	if err != nil {
		return nil, fmt.Errorf("update financial quest: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return nil, ErrVersionConflict
	}
	intentResult, err := tx.ExecContext(ctx, `UPDATE intents SET status = ?, version = ? WHERE quest_id = ? AND version = ?`, to, newVersion, quest.ID, quest.Intent.Version)
	if err != nil {
		return nil, fmt.Errorf("update financial intent: %w", err)
	}
	intentRows, _ := intentResult.RowsAffected()
	if intentRows != 1 {
		return nil, ErrVersionConflict
	}
	if detail == nil {
		detail = map[string]any{}
	}
	if err := appendQuestEvent(ctx, tx, quest.ID, newVersion, eventType, quest.Status, to, actorID, detail, now); err != nil {
		return nil, err
	}
	return getQuestTx(ctx, tx, quest.ID)
}

func appendQuestEvent(ctx context.Context, tx *sql.Tx, questID string, questVersion int64, eventType string, from, to QuestStatus, actorID string, detail any, now time.Time) error {
	detailJSON, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("encode financial event: %w", err)
	}
	var previousHash string
	err = tx.QueryRowContext(ctx, `SELECT event_hash FROM quest_events WHERE quest_id = ? ORDER BY sequence DESC LIMIT 1`, questID).Scan(&previousHash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read financial event chain: %w", err)
	}
	eventID, err := financialID("event")
	if err != nil {
		return err
	}
	canonical, _ := json.Marshal(struct {
		ID           string          `json:"id"`
		QuestID      string          `json:"questId"`
		QuestVersion int64           `json:"questVersion"`
		Type         string          `json:"type"`
		From         QuestStatus     `json:"from"`
		To           QuestStatus     `json:"to"`
		ActorID      string          `json:"actorId"`
		Detail       json.RawMessage `json:"detail"`
		PreviousHash string          `json:"previousHash"`
		CreatedAt    int64           `json:"createdAt"`
	}{eventID, questID, questVersion, eventType, from, to, actorID, detailJSON, previousHash, now.UnixMilli()})
	digest := sha256.Sum256(canonical)
	eventHash := hex.EncodeToString(digest[:])
	if _, err := tx.ExecContext(ctx, `INSERT INTO quest_events
        (event_id, quest_id, quest_version, event_type, from_status, to_status, actor_id, detail_json, previous_hash, event_hash, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		eventID, questID, questVersion, eventType, from, to, actorID, detailJSON, previousHash, eventHash, now.UnixMilli()); err != nil {
		return fmt.Errorf("append financial event: %w", err)
	}
	return nil
}

func listQuestEvents(ctx context.Context, db *sql.DB, questID string) ([]QuestEvent, error) {
	rows, err := db.QueryContext(ctx, `SELECT sequence, event_id, quest_id, quest_version, event_type,
        from_status, to_status, actor_id, detail_json, previous_hash, event_hash, created_at
        FROM quest_events WHERE quest_id = ? ORDER BY sequence`, questID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []QuestEvent
	for rows.Next() {
		var event QuestEvent
		var from, to string
		var createdAt int64
		if err := rows.Scan(&event.Sequence, &event.ID, &event.QuestID, &event.QuestVersion, &event.Type, &from, &to, &event.ActorID, &event.Detail, &event.PreviousHash, &event.Hash, &createdAt); err != nil {
			return nil, err
		}
		event.FromStatus = QuestStatus(from)
		event.ToStatus = QuestStatus(to)
		event.CreatedAt = time.UnixMilli(createdAt).UTC()
		events = append(events, event)
	}
	return events, rows.Err()
}
