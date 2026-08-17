package conductor

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Repository struct {
	db *sql.DB
}

func OpenRepository(databasePath string) (*Repository, error) {
	if databasePath == "" || databasePath == ":memory:" {
		return nil, fmt.Errorf("%w: durable SQLite path required", ErrInvalidComposition)
	}
	absPath, err := filepath.Abs(databasePath)
	if err != nil {
		return nil, err
	}
	if info, err := os.Lstat(absPath); err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("%w: database path must be a regular file", ErrInvalidComposition)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", absPath+"?_foreign_keys=on&_busy_timeout=5000&_journal_mode=WAL&_synchronous=FULL")
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

func NewRepository(db *sql.DB) (*Repository, error) {
	if db == nil {
		return nil, ErrInvalidComposition
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
CREATE TABLE IF NOT EXISTS conductor_tracks (
    id TEXT PRIMARY KEY,
    workspace_id INTEGER NOT NULL,
    idempotency_key TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    request_json TEXT NOT NULL,
    build_request_json TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('DRAFT','STAGED','VALIDATED','COMPLIANCE_AUDITED','ARTIFACT_SIGNED','PREVIEW_READY','PUBLICATION_REQUESTED','PUBLISHED','FAILED','CANCELLED')),
    version INTEGER NOT NULL CHECK (version > 0),
    compliance_json TEXT NOT NULL DEFAULT '',
    artifact_json TEXT NOT NULL DEFAULT '',
    preview_json TEXT NOT NULL DEFAULT '',
    claim_id TEXT NOT NULL DEFAULT '',
    publication_json TEXT NOT NULL DEFAULT '',
    failure_code TEXT NOT NULL DEFAULT '',
    created_by INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE (workspace_id, idempotency_key)
);
CREATE TABLE IF NOT EXISTS conductor_track_events (
    sequence INTEGER PRIMARY KEY AUTOINCREMENT,
    id TEXT NOT NULL UNIQUE,
    track_id TEXT NOT NULL REFERENCES conductor_tracks(id),
    track_version INTEGER NOT NULL,
    from_status TEXT NOT NULL,
    to_status TEXT NOT NULL,
    event_type TEXT NOT NULL,
    actor_id INTEGER NOT NULL,
    detail_json TEXT NOT NULL,
    previous_hash TEXT NOT NULL,
    event_hash TEXT NOT NULL UNIQUE,
    created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_conductor_events_track ON conductor_track_events(track_id, sequence);
CREATE TRIGGER IF NOT EXISTS conductor_events_no_update BEFORE UPDATE ON conductor_track_events BEGIN SELECT RAISE(ABORT, 'Conductor events are append-only'); END;
CREATE TRIGGER IF NOT EXISTS conductor_events_no_delete BEFORE DELETE ON conductor_track_events BEGIN SELECT RAISE(ABORT, 'Conductor events are append-only'); END;`
	if _, err := r.db.Exec(schema); err != nil {
		return fmt.Errorf("migrate Conductor database: %w", err)
	}
	return nil
}

func (r *Repository) createDraft(ctx context.Context, request CompositionRequest, requestHash string, actorID int, now time.Time) (*Track, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	var existingID, existingHash string
	err = tx.QueryRowContext(ctx, `SELECT id, request_hash FROM conductor_tracks WHERE workspace_id = ? AND idempotency_key = ?`, request.WorkspaceID, request.IdempotencyKey).Scan(&existingID, &existingHash)
	if err == nil {
		if existingHash != requestHash {
			return nil, false, ErrIdempotencyConflict
		}
		existing, err := getTrackTx(ctx, tx, existingID)
		return existing, false, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, err
	}
	trackID, err := randomID("track")
	if err != nil {
		return nil, false, err
	}
	requestJSON, _ := json.Marshal(request)
	if _, err := tx.ExecContext(ctx, `INSERT INTO conductor_tracks
        (id, workspace_id, idempotency_key, request_hash, request_json, status, version, created_by, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, 'DRAFT', 1, ?, ?, ?)`, trackID, request.WorkspaceID, request.IdempotencyKey, requestHash, requestJSON, actorID, now.UnixMilli(), now.UnixMilli()); err != nil {
		return nil, false, err
	}
	if err := appendTrackEvent(ctx, tx, trackID, 1, "", TrackDraft, "TRACK_DRAFTED", actorID, map[string]any{"templateId": request.TemplateID}, now); err != nil {
		return nil, false, err
	}
	track, err := getTrackTx(ctx, tx, trackID)
	if err != nil {
		return nil, false, err
	}
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	return track, true, nil
}

func (r *Repository) transition(ctx context.Context, current, updated *Track, to TrackStatus, eventType string, actorID int, detail any, now time.Time) (*Track, error) {
	if current == nil || updated == nil || current.ID != updated.ID || current.Version != updated.Version || current.Status != updated.Status || !allowedTransition(current.Status, to) {
		return nil, ErrTrackTransition
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	updated.Status = to
	updated.Version = current.Version + 1
	updated.UpdatedAt = now.UTC().Truncate(time.Millisecond)
	buildJSON, err := optionalJSON(updated.BuildRequest)
	if err != nil {
		return nil, err
	}
	complianceJSON, err := optionalJSON(updated.Compliance)
	if err != nil {
		return nil, err
	}
	artifactJSON, err := optionalJSON(updated.Artifact)
	if err != nil {
		return nil, err
	}
	previewJSON, err := optionalJSON(updated.Preview)
	if err != nil {
		return nil, err
	}
	publicationJSON, err := optionalJSON(updated.Publication)
	if err != nil {
		return nil, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE conductor_tracks SET
        build_request_json = ?, status = ?, version = ?, compliance_json = ?, artifact_json = ?, preview_json = ?,
        claim_id = ?, publication_json = ?, failure_code = ?, updated_at = ?
        WHERE id = ? AND status = ? AND version = ?`,
		buildJSON, updated.Status, updated.Version, complianceJSON, artifactJSON, previewJSON,
		updated.ClaimID, publicationJSON, updated.FailureCode, updated.UpdatedAt.UnixMilli(), current.ID, current.Status, current.Version)
	if err != nil {
		return nil, err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return nil, ErrTrackVersionConflict
	}
	if err := appendTrackEvent(ctx, tx, current.ID, updated.Version, current.Status, updated.Status, eventType, actorID, detail, updated.UpdatedAt); err != nil {
		return nil, err
	}
	track, err := getTrackTx(ctx, tx, current.ID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return track, nil
}

const trackSelect = `SELECT id, workspace_id, request_json, build_request_json, status, version,
    compliance_json, artifact_json, preview_json, claim_id, publication_json, failure_code,
    created_by, created_at, updated_at FROM conductor_tracks`

func (r *Repository) getTrack(ctx context.Context, trackID string) (*Track, error) {
	if !validIdentifier(trackID) {
		return nil, ErrTrackNotFound
	}
	return scanTrack(r.db.QueryRowContext(ctx, trackSelect+` WHERE id = ?`, trackID))
}

func getTrackTx(ctx context.Context, tx *sql.Tx, trackID string) (*Track, error) {
	return scanTrack(tx.QueryRowContext(ctx, trackSelect+` WHERE id = ?`, trackID))
}

func scanTrack(row interface{ Scan(...any) error }) (*Track, error) {
	var track Track
	var requestJSON, buildJSON, complianceJSON, artifactJSON, previewJSON, publicationJSON []byte
	var status string
	var createdAt, updatedAt int64
	err := row.Scan(&track.ID, &track.WorkspaceID, &requestJSON, &buildJSON, &status, &track.Version,
		&complianceJSON, &artifactJSON, &previewJSON, &track.ClaimID, &publicationJSON, &track.FailureCode,
		&track.CreatedBy, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrTrackNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(requestJSON, &track.Request); err != nil {
		return nil, err
	}
	if err := decodeOptional(buildJSON, &track.BuildRequest); err != nil {
		return nil, err
	}
	if err := decodeOptional(complianceJSON, &track.Compliance); err != nil {
		return nil, err
	}
	if err := decodeOptional(artifactJSON, &track.Artifact); err != nil {
		return nil, err
	}
	if err := decodeOptional(previewJSON, &track.Preview); err != nil {
		return nil, err
	}
	if err := decodeOptional(publicationJSON, &track.Publication); err != nil {
		return nil, err
	}
	track.Status = TrackStatus(status)
	track.CreatedAt, track.UpdatedAt = time.UnixMilli(createdAt).UTC(), time.UnixMilli(updatedAt).UTC()
	return &track, nil
}

func (r *Repository) events(ctx context.Context, trackID string) ([]TrackEvent, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT sequence, id, track_id, track_version, from_status, to_status, event_type, actor_id, detail_json, previous_hash, event_hash, created_at
        FROM conductor_track_events WHERE track_id = ? ORDER BY sequence`, trackID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []TrackEvent
	for rows.Next() {
		var event TrackEvent
		var from, to string
		var createdAt int64
		if err := rows.Scan(&event.Sequence, &event.ID, &event.TrackID, &event.TrackVersion, &from, &to, &event.Type, &event.ActorID, &event.Detail, &event.PreviousHash, &event.Hash, &createdAt); err != nil {
			return nil, err
		}
		event.FromStatus, event.ToStatus, event.CreatedAt = TrackStatus(from), TrackStatus(to), time.UnixMilli(createdAt).UTC()
		events = append(events, event)
	}
	return events, rows.Err()
}

func appendTrackEvent(ctx context.Context, tx *sql.Tx, trackID string, version int64, from, to TrackStatus, eventType string, actorID int, detail any, now time.Time) error {
	detailJSON, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	var previousHash string
	err = tx.QueryRowContext(ctx, `SELECT event_hash FROM conductor_track_events WHERE track_id = ? ORDER BY sequence DESC LIMIT 1`, trackID).Scan(&previousHash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	eventID, err := randomID("track-event")
	if err != nil {
		return err
	}
	canonical, _ := json.Marshal(struct {
		ID           string          `json:"id"`
		TrackID      string          `json:"trackId"`
		Version      int64           `json:"version"`
		From         TrackStatus     `json:"from"`
		To           TrackStatus     `json:"to"`
		Type         string          `json:"type"`
		ActorID      int             `json:"actorId"`
		Detail       json.RawMessage `json:"detail"`
		PreviousHash string          `json:"previousHash"`
		CreatedAt    int64           `json:"createdAt"`
	}{eventID, trackID, version, from, to, eventType, actorID, detailJSON, previousHash, now.UnixMilli()})
	digest := sha256.Sum256(canonical)
	eventHash := hex.EncodeToString(digest[:])
	_, err = tx.ExecContext(ctx, `INSERT INTO conductor_track_events
        (id, track_id, track_version, from_status, to_status, event_type, actor_id, detail_json, previous_hash, event_hash, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, eventID, trackID, version, from, to, eventType, actorID, detailJSON, previousHash, eventHash, now.UnixMilli())
	return err
}

func optionalJSON(value any) (string, error) {
	if value == nil || reflect.ValueOf(value).Kind() == reflect.Ptr && reflect.ValueOf(value).IsNil() {
		return "", nil
	}
	encoded, err := json.Marshal(value)
	return string(encoded), err
}

func decodeOptional(data []byte, target any) error {
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, target)
}

func randomID(prefix string) (string, error) {
	bytes := make([]byte, 18)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return prefix + "_" + base64.RawURLEncoding.EncodeToString(bytes), nil
}
