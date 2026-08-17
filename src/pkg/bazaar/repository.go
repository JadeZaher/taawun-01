package bazaar

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"taawun/pkg/models"
)

func migrate(db *sql.DB) error {
	const schema = `
CREATE TABLE IF NOT EXISTS bazaar_listings (
    id TEXT PRIMARY KEY,
    creator_user_id INTEGER NOT NULL,
    creator_workspace_id INTEGER NOT NULL,
    state TEXT NOT NULL,
    version INTEGER NOT NULL CHECK (version > 0),
    current_revision INTEGER NOT NULL CHECK (current_revision > 0),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS bazaar_listing_revisions (
    listing_id TEXT NOT NULL REFERENCES bazaar_listings(id),
    revision INTEGER NOT NULL,
    snapshot_json TEXT NOT NULL,
    created_by INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (listing_id, revision)
);
CREATE TABLE IF NOT EXISTS bazaar_events (
    sequence INTEGER PRIMARY KEY AUTOINCREMENT,
    id TEXT NOT NULL UNIQUE,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    entity_version INTEGER NOT NULL,
    revision INTEGER NOT NULL,
    event_type TEXT NOT NULL,
    from_state TEXT NOT NULL,
    to_state TEXT NOT NULL,
    actor_user_id INTEGER NOT NULL,
    detail_json TEXT NOT NULL,
    created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS bazaar_purchases (
    id TEXT PRIMARY KEY,
    listing_id TEXT NOT NULL REFERENCES bazaar_listings(id),
    revision INTEGER NOT NULL,
    buyer_user_id INTEGER NOT NULL,
    target_workspace_id INTEGER NOT NULL,
    idempotency_key TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    quest_id TEXT NOT NULL,
    quest_status TEXT NOT NULL,
    quest_version INTEGER NOT NULL,
    reconciliation_reference TEXT NOT NULL DEFAULT '',
    entitlement_id TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE (buyer_user_id, idempotency_key)
);
CREATE TABLE IF NOT EXISTS bazaar_entitlements (
    id TEXT PRIMARY KEY,
    purchase_id TEXT NOT NULL UNIQUE REFERENCES bazaar_purchases(id),
    listing_id TEXT NOT NULL,
    revision INTEGER NOT NULL,
    content_hash TEXT NOT NULL,
    buyer_user_id INTEGER NOT NULL,
    target_workspace_id INTEGER NOT NULL,
    created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS bazaar_installations (
    id TEXT PRIMARY KEY,
    entitlement_id TEXT NOT NULL REFERENCES bazaar_entitlements(id),
    listing_id TEXT NOT NULL,
    revision INTEGER NOT NULL,
    content_hash TEXT NOT NULL,
    target_workspace_id INTEGER NOT NULL,
    version INTEGER NOT NULL,
    installed_by INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE (target_workspace_id, listing_id)
);
CREATE INDEX IF NOT EXISTS idx_bazaar_public ON bazaar_listings(state, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_bazaar_events_entity ON bazaar_events(entity_type, entity_id, sequence);
CREATE INDEX IF NOT EXISTS idx_bazaar_purchases_buyer ON bazaar_purchases(buyer_user_id, created_at DESC);
CREATE TRIGGER IF NOT EXISTS bazaar_revision_no_update BEFORE UPDATE ON bazaar_listing_revisions BEGIN SELECT RAISE(ABORT, 'Bazaar revisions are immutable'); END;
CREATE TRIGGER IF NOT EXISTS bazaar_revision_no_delete BEFORE DELETE ON bazaar_listing_revisions BEGIN SELECT RAISE(ABORT, 'Bazaar revisions are immutable'); END;
CREATE TRIGGER IF NOT EXISTS bazaar_event_no_update BEFORE UPDATE ON bazaar_events BEGIN SELECT RAISE(ABORT, 'Bazaar events are append-only'); END;
CREATE TRIGGER IF NOT EXISTS bazaar_event_no_delete BEFORE DELETE ON bazaar_events BEGIN SELECT RAISE(ABORT, 'Bazaar events are append-only'); END;
CREATE TRIGGER IF NOT EXISTS bazaar_entitlement_no_update BEFORE UPDATE ON bazaar_entitlements BEGIN SELECT RAISE(ABORT, 'Bazaar entitlements are immutable'); END;
CREATE TRIGGER IF NOT EXISTS bazaar_entitlement_no_delete BEFORE DELETE ON bazaar_entitlements BEGIN SELECT RAISE(ABORT, 'Bazaar entitlements are immutable'); END;`
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("migrate Bazaar database: %w", err)
	}
	return nil
}

func (s *Service) getListing(ctx context.Context, listingID string) (Listing, error) {
	row := s.db.QueryRowContext(ctx, `SELECT l.id, l.state, l.version, l.current_revision,
		l.created_at, l.updated_at, r.snapshot_json
		FROM bazaar_listings l JOIN bazaar_listing_revisions r
		ON r.listing_id = l.id AND r.revision = l.current_revision WHERE l.id = ?`, listingID)
	var listing Listing
	var state string
	var createdAt, updatedAt int64
	var snapshot []byte
	if err := row.Scan(&listing.ID, &state, &listing.Version, &listing.CurrentRevision, &createdAt, &updatedAt, &snapshot); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Listing{}, ErrNotFound
		}
		return Listing{}, fmt.Errorf("load Bazaar listing: %w", err)
	}
	if err := json.Unmarshal(snapshot, &listing.Revision); err != nil {
		return Listing{}, fmt.Errorf("decode Bazaar revision: %w", err)
	}
	listing.State = State(state)
	listing.CreatedAt = time.UnixMilli(createdAt).UTC()
	listing.UpdatedAt = time.UnixMilli(updatedAt).UTC()
	return listing, nil
}

func (s *Service) listPublished(ctx context.Context) ([]Listing, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT l.id, l.state, l.version, l.current_revision,
		l.created_at, l.updated_at, r.snapshot_json
		FROM bazaar_listings l JOIN bazaar_listing_revisions r
		ON r.listing_id = l.id AND r.revision = l.current_revision
		WHERE l.state = 'published' ORDER BY l.updated_at DESC, l.id`)
	if err != nil {
		return nil, fmt.Errorf("list public Bazaar: %w", err)
	}
	defer rows.Close()
	listings := make([]Listing, 0)
	for rows.Next() {
		var listing Listing
		var state string
		var createdAt, updatedAt int64
		var snapshot []byte
		if err := rows.Scan(&listing.ID, &state, &listing.Version, &listing.CurrentRevision, &createdAt, &updatedAt, &snapshot); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(snapshot, &listing.Revision); err != nil {
			return nil, err
		}
		listing.State = State(state)
		listing.CreatedAt = time.UnixMilli(createdAt).UTC()
		listing.UpdatedAt = time.UnixMilli(updatedAt).UTC()
		listings = append(listings, listing)
	}
	return listings, rows.Err()
}

func appendEvent(ctx context.Context, tx *sql.Tx, event Event) error {
	detail := event.Detail
	if len(detail) == 0 {
		detail = json.RawMessage(`{}`)
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO bazaar_events
		(id, entity_type, entity_id, entity_version, revision, event_type, from_state,
		 to_state, actor_user_id, detail_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ID, event.EntityType, event.EntityID, event.EntityVersion, event.Revision, event.Type,
		string(event.FromState), string(event.ToState), event.ActorUserID, string(detail), event.CreatedAt.UnixMilli())
	return err
}

func (s *Service) Events(ctx context.Context, actor *models.User, listingID string) ([]Event, error) {
	listing, err := s.getListing(ctx, listingID)
	if err != nil {
		return nil, err
	}
	if !s.platform(actor) {
		if actor == nil || actor.ID != listing.Revision.CreatorUserID {
			return nil, ErrForbidden
		}
		if _, err := s.workspaces.AuthorizeWorkspaceCapability(actor, listing.Revision.CreatorWorkspaceID, models.WorkspaceCapabilityPublish); err != nil {
			return nil, ErrForbidden
		}
	}
	rows, err := s.db.QueryContext(ctx, `SELECT sequence, id, entity_type, entity_id,
		entity_version, revision, event_type, from_state, to_state, actor_user_id, detail_json, created_at
		FROM bazaar_events WHERE entity_type = 'listing' AND entity_id = ? ORDER BY sequence`, listingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]Event, 0)
	for rows.Next() {
		var event Event
		var from, to string
		var detail string
		var createdAt int64
		if err := rows.Scan(&event.Sequence, &event.ID, &event.EntityType, &event.EntityID,
			&event.EntityVersion, &event.Revision, &event.Type, &from, &to, &event.ActorUserID,
			&detail, &createdAt); err != nil {
			return nil, err
		}
		event.FromState, event.ToState = State(from), State(to)
		event.Detail = json.RawMessage(detail)
		event.CreatedAt = time.UnixMilli(createdAt).UTC()
		events = append(events, event)
	}
	return events, rows.Err()
}
