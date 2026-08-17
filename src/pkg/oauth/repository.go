package oauth

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type Repository struct {
	db                           *sql.DB
	maximumDynamicClients        int
	maximumActiveSessions        int
	maximumActiveSessionsPerUser int
	maximumActiveRequests        int
}

const (
	defaultMaximumDynamicClients        = 4096
	defaultMaximumActiveSessions        = 8192
	defaultMaximumActiveSessionsPerUser = 8
	defaultMaximumActiveRequests        = 8192
	oauthClientRetention                = 90 * 24 * time.Hour
)

type authorizationRequest struct {
	Hash              string
	ClientID          string
	ClientFingerprint string
	RedirectURI       string
	Resource          string
	Scopes            []string
	State             string
	CodeChallenge     string
	ExpiresAt         time.Time
	Consumed          bool
}

type browserSession struct {
	Hash               string
	UserID             int
	UserSessionVersion int64
	CSRFHash           string
	ExpiresAt          time.Time
	Revoked            bool
}

type consent struct {
	ID                 int64
	UserID             int
	UserSessionVersion int64
	ClientID           string
	ClientFingerprint  string
	Resource           string
	Scopes             []string
	WorkspaceIDs       []int
	Revoked            bool
}

type authorizationCode struct {
	Hash          string
	ConsentID     int64
	UserID        int
	ClientID      string
	RedirectURI   string
	Resource      string
	Scopes        []string
	WorkspaceIDs  []int
	CodeChallenge string
	ExpiresAt     time.Time
	Consumed      bool
}

type accessTokenRecord struct {
	ID           int64
	Hash         string
	FamilyID     string
	ConsentID    int64
	UserID       int
	ClientID     string
	Resource     string
	Scopes       []string
	WorkspaceIDs []int
	ExpiresAt    time.Time
	Revoked      bool
}

type refreshTokenRecord struct {
	Hash         string
	FamilyID     string
	ConsentID    int64
	UserID       int
	ClientID     string
	Resource     string
	Scopes       []string
	WorkspaceIDs []int
	ExpiresAt    time.Time
	Used         bool
	Revoked      bool
}

type issuedTokens struct {
	AccessHash         string
	RefreshHash        string
	FamilyID           string
	ConsentID          int64
	UserID             int
	UserSessionVersion int64
	ClientID           string
	ClientFingerprint  string
	Resource           string
	Scopes             []string
	Workspaces         []int
	AccessExp          time.Time
	RefreshExp         time.Time
}

type tokenFamily struct {
	ID                 string
	ConsentID          int64
	UserID             int
	UserSessionVersion int64
	ClientID           string
	ClientFingerprint  string
}

func NewRepository(db *sql.DB) (*Repository, error) {
	if db == nil {
		return nil, errors.New("OAuth database is required")
	}
	repository := &Repository{
		db:                           db,
		maximumDynamicClients:        defaultMaximumDynamicClients,
		maximumActiveSessions:        defaultMaximumActiveSessions,
		maximumActiveSessionsPerUser: defaultMaximumActiveSessionsPerUser,
		maximumActiveRequests:        defaultMaximumActiveRequests,
	}
	if err := repository.migrate(); err != nil {
		return nil, err
	}
	return repository, nil
}

func (r *Repository) migrate() error {
	queries := []string{
		`PRAGMA foreign_keys = ON`,
		`CREATE TABLE IF NOT EXISTS oauth_clients (
			client_id TEXT PRIMARY KEY,
			client_name TEXT NOT NULL,
			redirect_uris_json TEXT NOT NULL,
			grant_types_json TEXT NOT NULL,
			response_types_json TEXT NOT NULL,
			token_endpoint_auth_method TEXT NOT NULL,
			client_uri TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL,
		registration_kind TEXT NOT NULL DEFAULT 'dcr',
			disabled_at INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS oauth_authorization_requests (
			request_hash TEXT PRIMARY KEY,
			client_id TEXT NOT NULL,
			client_fingerprint TEXT NOT NULL DEFAULT '',
			redirect_uri TEXT NOT NULL,
			resource TEXT NOT NULL,
			scopes_json TEXT NOT NULL,
			state TEXT NOT NULL,
			code_challenge TEXT NOT NULL,
			expires_at INTEGER NOT NULL,
			consumed_at INTEGER,
			FOREIGN KEY (client_id) REFERENCES oauth_clients(client_id)
		)`,
		`CREATE TABLE IF NOT EXISTS oauth_sessions (
			session_hash TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			user_session_version INTEGER NOT NULL DEFAULT 0,
			csrf_hash TEXT NOT NULL,
			expires_at INTEGER NOT NULL,
			revoked_at INTEGER,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS oauth_consents (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			user_session_version INTEGER NOT NULL DEFAULT 0,
			client_id TEXT NOT NULL,
			client_fingerprint TEXT NOT NULL DEFAULT '',
			resource TEXT NOT NULL,
			scopes_json TEXT NOT NULL,
			workspace_ids_json TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			revoked_at INTEGER,
			FOREIGN KEY (user_id) REFERENCES users(id),
			FOREIGN KEY (client_id) REFERENCES oauth_clients(client_id)
		)`,
		`CREATE TABLE IF NOT EXISTS oauth_authorization_codes (
			code_hash TEXT PRIMARY KEY,
			consent_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			client_id TEXT NOT NULL,
			redirect_uri TEXT NOT NULL,
			resource TEXT NOT NULL,
			scopes_json TEXT NOT NULL,
			workspace_ids_json TEXT NOT NULL,
			code_challenge TEXT NOT NULL,
			expires_at INTEGER NOT NULL,
			consumed_at INTEGER,
			FOREIGN KEY (consent_id) REFERENCES oauth_consents(id)
		)`,
		`CREATE TABLE IF NOT EXISTS oauth_access_tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token_hash TEXT NOT NULL UNIQUE,
			family_id TEXT NOT NULL,
			consent_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			client_id TEXT NOT NULL,
			resource TEXT NOT NULL,
			scopes_json TEXT NOT NULL,
			workspace_ids_json TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL,
			revoked_at INTEGER,
			FOREIGN KEY (consent_id) REFERENCES oauth_consents(id)
		)`,
		`CREATE TABLE IF NOT EXISTS oauth_refresh_tokens (
			token_hash TEXT PRIMARY KEY,
			family_id TEXT NOT NULL,
			consent_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			client_id TEXT NOT NULL,
			resource TEXT NOT NULL,
			scopes_json TEXT NOT NULL,
			workspace_ids_json TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL,
			used_at INTEGER,
			revoked_at INTEGER,
			FOREIGN KEY (consent_id) REFERENCES oauth_consents(id)
		)`,
		`CREATE TABLE IF NOT EXISTS oauth_client_snapshots (
			fingerprint TEXT PRIMARY KEY,
			client_id TEXT NOT NULL,
			metadata_json TEXT NOT NULL,
			created_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS oauth_token_families (
			family_id TEXT PRIMARY KEY,
			consent_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			user_session_version INTEGER NOT NULL,
			client_id TEXT NOT NULL,
			client_fingerprint TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			FOREIGN KEY (consent_id) REFERENCES oauth_consents(id),
			FOREIGN KEY (client_fingerprint) REFERENCES oauth_client_snapshots(fingerprint)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_oauth_access_hash ON oauth_access_tokens(token_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_oauth_access_family ON oauth_access_tokens(family_id)`,
		`CREATE INDEX IF NOT EXISTS idx_oauth_refresh_family ON oauth_refresh_tokens(family_id)`,
		`CREATE INDEX IF NOT EXISTS idx_oauth_consent_subject ON oauth_consents(user_id, client_id, resource)`,
		`CREATE INDEX IF NOT EXISTS idx_oauth_snapshot_client ON oauth_client_snapshots(client_id)`,
	}
	for _, query := range queries {
		if _, err := r.db.Exec(query); err != nil {
			return fmt.Errorf("OAuth migration failed: %w", err)
		}
	}
	for _, migration := range []struct{ table, column, definition string }{
		{"oauth_clients", "registration_kind", "TEXT NOT NULL DEFAULT 'dcr'"},
		{"oauth_authorization_requests", "client_fingerprint", "TEXT NOT NULL DEFAULT ''"},
		{"oauth_sessions", "user_session_version", "INTEGER NOT NULL DEFAULT 0"},
		{"oauth_consents", "user_session_version", "INTEGER NOT NULL DEFAULT 0"},
		{"oauth_consents", "client_fingerprint", "TEXT NOT NULL DEFAULT ''"},
	} {
		if err := r.ensureColumn(migration.table, migration.column, migration.definition); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) ensureColumn(table, column, definition string) error {
	rows, err := r.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return fmt.Errorf("inspect OAuth table %s: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var position, notNull, primaryKey int
		var name, typeName string
		var defaultValue sql.NullString
		if err := rows.Scan(&position, &name, &typeName, &notNull, &defaultValue, &primaryKey); err != nil {
			return fmt.Errorf("scan OAuth table %s: %w", table, err)
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read OAuth table %s: %w", table, err)
	}
	if _, err := r.db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + column + ` ` + definition); err != nil {
		return fmt.Errorf("add OAuth column %s.%s: %w", table, column, err)
	}
	return nil
}

// cleanupExpired removes only credentials and metadata that can no longer authorize a request.
func (r *Repository) cleanupExpired(now time.Time) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := r.cleanupExpiredTx(tx, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) cleanupExpiredTx(tx *sql.Tx, now time.Time) error {
	current := now.UTC().Unix()
	for _, statement := range []struct {
		query string
		args  []any
	}{
		{`DELETE FROM oauth_authorization_requests WHERE expires_at <= ? OR consumed_at IS NOT NULL`, []any{current}},
		{`DELETE FROM oauth_sessions WHERE expires_at <= ? OR revoked_at IS NOT NULL`, []any{current}},
		{`DELETE FROM oauth_authorization_codes WHERE expires_at <= ? OR consumed_at IS NOT NULL`, []any{current}},
		{`DELETE FROM oauth_access_tokens WHERE expires_at <= ?`, []any{current}},
		{`DELETE FROM oauth_refresh_tokens WHERE expires_at <= ?`, []any{current}},
		{`DELETE FROM oauth_token_families
			WHERE NOT EXISTS (SELECT 1 FROM oauth_access_tokens WHERE family_id = oauth_token_families.family_id)
			  AND NOT EXISTS (SELECT 1 FROM oauth_refresh_tokens WHERE family_id = oauth_token_families.family_id)`, nil},
		{`DELETE FROM oauth_consents
			WHERE NOT EXISTS (SELECT 1 FROM oauth_authorization_codes WHERE consent_id = oauth_consents.id)
			  AND NOT EXISTS (SELECT 1 FROM oauth_access_tokens WHERE consent_id = oauth_consents.id)
			  AND NOT EXISTS (SELECT 1 FROM oauth_refresh_tokens WHERE consent_id = oauth_consents.id)
			  AND NOT EXISTS (SELECT 1 FROM oauth_token_families WHERE consent_id = oauth_consents.id)`, nil},
		{`DELETE FROM oauth_client_snapshots
			WHERE created_at <= ?
			  AND NOT EXISTS (SELECT 1 FROM oauth_authorization_requests WHERE client_fingerprint = oauth_client_snapshots.fingerprint)
			  AND NOT EXISTS (SELECT 1 FROM oauth_consents WHERE client_fingerprint = oauth_client_snapshots.fingerprint)
			  AND NOT EXISTS (SELECT 1 FROM oauth_token_families WHERE client_fingerprint = oauth_client_snapshots.fingerprint)`, []any{now.UTC().Add(-oauthClientRetention).Unix()}},
		{`DELETE FROM oauth_clients
			WHERE created_at <= ?
			  AND NOT EXISTS (SELECT 1 FROM oauth_authorization_requests WHERE client_id = oauth_clients.client_id)
			  AND NOT EXISTS (SELECT 1 FROM oauth_consents WHERE client_id = oauth_clients.client_id)`, []any{now.UTC().Add(-oauthClientRetention).Unix()}},
	} {
		if _, err := tx.Exec(statement.query, statement.args...); err != nil {
			return err
		}
	}
	return nil
}

func countBelow(tx *sql.Tx, query string, limit int, args ...any) error {
	if limit <= 0 {
		return ErrCapacity
	}
	var count int
	if err := tx.QueryRow(query, args...).Scan(&count); err != nil {
		return err
	}
	if count >= limit {
		return ErrCapacity
	}
	return nil
}

func encode(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func decode(raw string, target any) error {
	return json.Unmarshal([]byte(raw), target)
}

func (r *Repository) createClient(client Client, now time.Time) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := r.cleanupExpiredTx(tx, now); err != nil {
		return err
	}
	if err := countBelow(tx, `SELECT COUNT(*) FROM oauth_clients WHERE registration_kind = 'dcr'`, r.maximumDynamicClients); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO oauth_clients
		(client_id, client_name, redirect_uris_json, grant_types_json, response_types_json, token_endpoint_auth_method, client_uri, created_at, registration_kind)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'dcr')`, client.ID, client.Name, encode(client.RedirectURIs), encode(client.GrantTypes), encode(client.ResponseTypes), client.TokenEndpointAuthMethod, client.ClientURI, now.Unix()); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) upsertMetadataClient(client Client, now time.Time) error {
	_, err := r.db.Exec(`INSERT INTO oauth_clients
		(client_id, client_name, redirect_uris_json, grant_types_json, response_types_json, token_endpoint_auth_method, client_uri, created_at, registration_kind)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'metadata')
		ON CONFLICT(client_id) DO UPDATE SET
		client_name = excluded.client_name,
		redirect_uris_json = excluded.redirect_uris_json,
		grant_types_json = excluded.grant_types_json,
		response_types_json = excluded.response_types_json,
		token_endpoint_auth_method = excluded.token_endpoint_auth_method,
		client_uri = excluded.client_uri,
		registration_kind = 'metadata'`, client.ID, client.Name, encode(client.RedirectURIs), encode(client.GrantTypes), encode(client.ResponseTypes), client.TokenEndpointAuthMethod, client.ClientURI, now.Unix())
	return err
}

func (r *Repository) getClient(id string) (*Client, error) {
	var client Client
	var redirects, grants, responses string
	var disabled sql.NullInt64
	err := r.db.QueryRow(`SELECT client_id, client_name, redirect_uris_json, grant_types_json,
		response_types_json, token_endpoint_auth_method, client_uri, disabled_at
		FROM oauth_clients WHERE client_id = ?`, id).Scan(&client.ID, &client.Name, &redirects, &grants, &responses, &client.TokenEndpointAuthMethod, &client.ClientURI, &disabled)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := decode(redirects, &client.RedirectURIs); err != nil {
		return nil, err
	}
	if err := decode(grants, &client.GrantTypes); err != nil {
		return nil, err
	}
	if err := decode(responses, &client.ResponseTypes); err != nil {
		return nil, err
	}
	client.Disabled = disabled.Valid
	return &client, nil
}

func (r *Repository) createClientSnapshot(client Client, now time.Time) (*clientSnapshot, error) {
	snapshot, err := newClientSnapshot(client)
	if err != nil {
		return nil, err
	}
	metadata, err := encodeClientSnapshot(snapshot)
	if err != nil {
		return nil, err
	}
	_, err = r.db.Exec(`INSERT INTO oauth_client_snapshots (fingerprint, client_id, metadata_json, created_at)
		VALUES (?, ?, ?, ?) ON CONFLICT(fingerprint) DO NOTHING`, snapshot.Fingerprint, snapshot.Client.ID, metadata, now.Unix())
	if err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (r *Repository) getClientSnapshot(fingerprint string) (*clientSnapshot, error) {
	if fingerprint == "" {
		return nil, nil
	}
	var raw string
	err := r.db.QueryRow(`SELECT metadata_json FROM oauth_client_snapshots WHERE fingerprint = ?`, fingerprint).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return decodeClientSnapshot(fingerprint, raw)
}

func (r *Repository) clientDisabled(clientID string) (bool, error) {
	var disabled sql.NullInt64
	err := r.db.QueryRow(`SELECT disabled_at FROM oauth_clients WHERE client_id = ?`, clientID).Scan(&disabled)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return disabled.Valid, nil
}

func (r *Repository) createAuthorizationRequest(request authorizationRequest, now time.Time) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := r.cleanupExpiredTx(tx, now); err != nil {
		return err
	}
	if err := countBelow(tx, `SELECT COUNT(*) FROM oauth_authorization_requests WHERE expires_at > ? AND consumed_at IS NULL`, r.maximumActiveRequests, now.Unix()); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO oauth_authorization_requests
		(request_hash, client_id, client_fingerprint, redirect_uri, resource, scopes_json, state, code_challenge, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, request.Hash, request.ClientID, request.ClientFingerprint, request.RedirectURI, request.Resource, encode(request.Scopes), request.State, request.CodeChallenge, request.ExpiresAt.Unix()); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) consumeAuthorizationRequest(hash string, now time.Time) error {
	result, err := r.db.Exec(`UPDATE oauth_authorization_requests SET consumed_at = ? WHERE request_hash = ? AND consumed_at IS NULL AND expires_at > ?`, now.Unix(), hash, now.Unix())
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows != 1 {
		return ErrInvalidGrant
	}
	return nil
}

func (r *Repository) getAuthorizationRequest(hash string) (*authorizationRequest, error) {
	var request authorizationRequest
	var scopes string
	var expires int64
	var consumed sql.NullInt64
	err := r.db.QueryRow(`SELECT request_hash, client_id, client_fingerprint, redirect_uri, resource, scopes_json, state,
		code_challenge, expires_at, consumed_at FROM oauth_authorization_requests WHERE request_hash = ?`, hash).
		Scan(&request.Hash, &request.ClientID, &request.ClientFingerprint, &request.RedirectURI, &request.Resource, &scopes, &request.State, &request.CodeChallenge, &expires, &consumed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := decode(scopes, &request.Scopes); err != nil {
		return nil, err
	}
	request.ExpiresAt = time.Unix(expires, 0).UTC()
	request.Consumed = consumed.Valid
	return &request, nil
}

func (r *Repository) createSession(session browserSession, now time.Time) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := r.cleanupExpiredTx(tx, now); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM oauth_sessions WHERE user_id = ? AND user_session_version <> ?`, session.UserID, session.UserSessionVersion); err != nil {
		return err
	}
	if err := countBelow(tx, `SELECT COUNT(*) FROM oauth_sessions WHERE expires_at > ? AND revoked_at IS NULL`, r.maximumActiveSessions, now.Unix()); err != nil {
		return err
	}
	if err := countBelow(tx, `SELECT COUNT(*) FROM oauth_sessions WHERE user_id = ? AND expires_at > ? AND revoked_at IS NULL`, r.maximumActiveSessionsPerUser, session.UserID, now.Unix()); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO oauth_sessions (session_hash, user_id, user_session_version, csrf_hash, expires_at) VALUES (?, ?, ?, ?, ?)`, session.Hash, session.UserID, session.UserSessionVersion, session.CSRFHash, session.ExpiresAt.Unix()); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) getSession(hash string) (*browserSession, error) {
	var session browserSession
	var expires int64
	var revoked sql.NullInt64
	err := r.db.QueryRow(`SELECT session_hash, user_id, user_session_version, csrf_hash, expires_at, revoked_at FROM oauth_sessions WHERE session_hash = ?`, hash).
		Scan(&session.Hash, &session.UserID, &session.UserSessionVersion, &session.CSRFHash, &expires, &revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	session.ExpiresAt = time.Unix(expires, 0).UTC()
	session.Revoked = revoked.Valid
	return &session, nil
}

func (r *Repository) updateSessionCSRF(sessionHash, csrfHash string, now time.Time) error {
	result, err := r.db.Exec(`UPDATE oauth_sessions SET csrf_hash = ? WHERE session_hash = ? AND revoked_at IS NULL AND expires_at > ?`, csrfHash, sessionHash, now.Unix())
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows != 1 {
		return ErrInvalidGrant
	}
	return nil
}

func (r *Repository) approve(requestHash string, record consent, code authorizationCode, now time.Time) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE oauth_authorization_requests SET consumed_at = ? WHERE request_hash = ? AND consumed_at IS NULL AND expires_at > ?`, now.Unix(), requestHash, now.Unix())
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows != 1 {
		return ErrInvalidGrant
	}
	if _, err := tx.Exec(`UPDATE oauth_consents SET revoked_at = ? WHERE user_id = ? AND client_id = ? AND resource = ? AND revoked_at IS NULL`, now.Unix(), record.UserID, record.ClientID, record.Resource); err != nil {
		return err
	}
	result, err = tx.Exec(`INSERT INTO oauth_consents
		(user_id, user_session_version, client_id, client_fingerprint, resource, scopes_json, workspace_ids_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, record.UserID, record.UserSessionVersion, record.ClientID, record.ClientFingerprint, record.Resource, encode(record.Scopes), encode(record.WorkspaceIDs), now.Unix())
	if err != nil {
		return err
	}
	consentID, err := result.LastInsertId()
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO oauth_authorization_codes
		(code_hash, consent_id, user_id, client_id, redirect_uri, resource, scopes_json, workspace_ids_json, code_challenge, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, code.Hash, consentID, code.UserID, code.ClientID, code.RedirectURI, code.Resource, encode(code.Scopes), encode(code.WorkspaceIDs), code.CodeChallenge, code.ExpiresAt.Unix())
	if err != nil {
		return err
	}
	return tx.Commit()
}

func scanAuthorizationCode(row interface{ Scan(...any) error }) (*authorizationCode, error) {
	var code authorizationCode
	var scopes, workspaces string
	var expires int64
	var consumed sql.NullInt64
	err := row.Scan(&code.Hash, &code.ConsentID, &code.UserID, &code.ClientID, &code.RedirectURI, &code.Resource, &scopes, &workspaces, &code.CodeChallenge, &expires, &consumed)
	if err != nil {
		return nil, err
	}
	if err := decode(scopes, &code.Scopes); err != nil {
		return nil, err
	}
	if err := decode(workspaces, &code.WorkspaceIDs); err != nil {
		return nil, err
	}
	code.ExpiresAt = time.Unix(expires, 0).UTC()
	code.Consumed = consumed.Valid
	return &code, nil
}

func (r *Repository) getAuthorizationCode(hash string) (*authorizationCode, error) {
	row := r.db.QueryRow(`SELECT code_hash, consent_id, user_id, client_id, redirect_uri, resource,
		scopes_json, workspace_ids_json, code_challenge, expires_at, consumed_at
		FROM oauth_authorization_codes WHERE code_hash = ?`, hash)
	code, err := scanAuthorizationCode(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return code, err
}

func insertTokens(tx *sql.Tx, tokens issuedTokens, now time.Time) error {
	_, err := tx.Exec(`INSERT INTO oauth_token_families
		(family_id, consent_id, user_id, user_session_version, client_id, client_fingerprint, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT(family_id) DO NOTHING`,
		tokens.FamilyID, tokens.ConsentID, tokens.UserID, tokens.UserSessionVersion, tokens.ClientID, tokens.ClientFingerprint, now.Unix())
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO oauth_access_tokens
		(token_hash, family_id, consent_id, user_id, client_id, resource, scopes_json, workspace_ids_json, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, tokens.AccessHash, tokens.FamilyID, tokens.ConsentID, tokens.UserID, tokens.ClientID, tokens.Resource, encode(tokens.Scopes), encode(tokens.Workspaces), now.Unix(), tokens.AccessExp.Unix())
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO oauth_refresh_tokens
		(token_hash, family_id, consent_id, user_id, client_id, resource, scopes_json, workspace_ids_json, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, tokens.RefreshHash, tokens.FamilyID, tokens.ConsentID, tokens.UserID, tokens.ClientID, tokens.Resource, encode(tokens.Scopes), encode(tokens.Workspaces), now.Unix(), tokens.RefreshExp.Unix())
	return err
}

func (r *Repository) exchangeCode(codeHash string, expected authorizationCode, tokens issuedTokens, now time.Time) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	row := tx.QueryRow(`SELECT code_hash, consent_id, user_id, client_id, redirect_uri, resource,
		scopes_json, workspace_ids_json, code_challenge, expires_at, consumed_at
		FROM oauth_authorization_codes WHERE code_hash = ?`, codeHash)
	code, err := scanAuthorizationCode(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidGrant
		}
		return err
	}
	if code.Consumed || !code.ExpiresAt.After(now) || code.ClientID != expected.ClientID || code.RedirectURI != expected.RedirectURI || code.Resource != expected.Resource || code.CodeChallenge != expected.CodeChallenge {
		return ErrInvalidGrant
	}
	result, err := tx.Exec(`UPDATE oauth_authorization_codes SET consumed_at = ? WHERE code_hash = ? AND consumed_at IS NULL`, now.Unix(), codeHash)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows != 1 {
		return ErrInvalidGrant
	}
	tokens.ConsentID = code.ConsentID
	tokens.UserID = code.UserID
	tokens.Scopes = code.Scopes
	tokens.Workspaces = code.WorkspaceIDs
	if err := insertTokens(tx, tokens, now); err != nil {
		return err
	}
	return tx.Commit()
}

func scanAccessToken(row interface{ Scan(...any) error }) (*accessTokenRecord, error) {
	var token accessTokenRecord
	var scopes, workspaces string
	var expires int64
	var revoked sql.NullInt64
	err := row.Scan(&token.ID, &token.Hash, &token.FamilyID, &token.ConsentID, &token.UserID, &token.ClientID, &token.Resource, &scopes, &workspaces, &expires, &revoked)
	if err != nil {
		return nil, err
	}
	if err := decode(scopes, &token.Scopes); err != nil {
		return nil, err
	}
	if err := decode(workspaces, &token.WorkspaceIDs); err != nil {
		return nil, err
	}
	token.ExpiresAt = time.Unix(expires, 0).UTC()
	token.Revoked = revoked.Valid
	return &token, nil
}

func (r *Repository) getAccessToken(hash string) (*accessTokenRecord, error) {
	row := r.db.QueryRow(`SELECT id, token_hash, family_id, consent_id, user_id, client_id, resource,
		scopes_json, workspace_ids_json, expires_at, revoked_at FROM oauth_access_tokens WHERE token_hash = ?`, hash)
	token, err := scanAccessToken(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return token, err
}

func (r *Repository) consentActive(id int64) (bool, error) {
	var one int
	err := r.db.QueryRow(`SELECT 1 FROM oauth_consents WHERE id = ? AND revoked_at IS NULL`, id).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (r *Repository) getConsent(id int64) (*consent, error) {
	var record consent
	var scopes, workspaces string
	var revoked sql.NullInt64
	err := r.db.QueryRow(`SELECT id, user_id, user_session_version, client_id, client_fingerprint, resource,
		scopes_json, workspace_ids_json, revoked_at FROM oauth_consents WHERE id = ?`, id).
		Scan(&record.ID, &record.UserID, &record.UserSessionVersion, &record.ClientID, &record.ClientFingerprint,
			&record.Resource, &scopes, &workspaces, &revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := decode(scopes, &record.Scopes); err != nil {
		return nil, err
	}
	if err := decode(workspaces, &record.WorkspaceIDs); err != nil {
		return nil, err
	}
	record.Revoked = revoked.Valid
	return &record, nil
}

func scanRefreshToken(row interface{ Scan(...any) error }) (*refreshTokenRecord, error) {
	var token refreshTokenRecord
	var scopes, workspaces string
	var expires int64
	var used, revoked sql.NullInt64
	err := row.Scan(&token.Hash, &token.FamilyID, &token.ConsentID, &token.UserID, &token.ClientID, &token.Resource, &scopes, &workspaces, &expires, &used, &revoked)
	if err != nil {
		return nil, err
	}
	if err := decode(scopes, &token.Scopes); err != nil {
		return nil, err
	}
	if err := decode(workspaces, &token.WorkspaceIDs); err != nil {
		return nil, err
	}
	token.ExpiresAt = time.Unix(expires, 0).UTC()
	token.Used = used.Valid
	token.Revoked = revoked.Valid
	return &token, nil
}

func (r *Repository) getRefreshToken(hash string) (*refreshTokenRecord, error) {
	row := r.db.QueryRow(`SELECT token_hash, family_id, consent_id, user_id, client_id, resource,
		scopes_json, workspace_ids_json, expires_at, used_at, revoked_at
		FROM oauth_refresh_tokens WHERE token_hash = ?`, hash)
	token, err := scanRefreshToken(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return token, err
}

func (r *Repository) getTokenFamily(id string) (*tokenFamily, error) {
	var family tokenFamily
	err := r.db.QueryRow(`SELECT family_id, consent_id, user_id, user_session_version, client_id, client_fingerprint
		FROM oauth_token_families WHERE family_id = ?`, id).
		Scan(&family.ID, &family.ConsentID, &family.UserID, &family.UserSessionVersion, &family.ClientID, &family.ClientFingerprint)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &family, nil
}

func revokeFamily(tx *sql.Tx, familyID string, now time.Time) error {
	if _, err := tx.Exec(`UPDATE oauth_refresh_tokens SET revoked_at = COALESCE(revoked_at, ?) WHERE family_id = ?`, now.Unix(), familyID); err != nil {
		return err
	}
	_, err := tx.Exec(`UPDATE oauth_access_tokens SET revoked_at = COALESCE(revoked_at, ?) WHERE family_id = ?`, now.Unix(), familyID)
	return err
}

func (r *Repository) rotateRefresh(oldHash, clientID, resource string, next issuedTokens, now time.Time) (*refreshTokenRecord, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	row := tx.QueryRow(`SELECT token_hash, family_id, consent_id, user_id, client_id, resource,
		scopes_json, workspace_ids_json, expires_at, used_at, revoked_at
		FROM oauth_refresh_tokens WHERE token_hash = ?`, oldHash)
	old, err := scanRefreshToken(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrInvalidGrant
	}
	if err != nil {
		return nil, err
	}
	if old.ClientID != clientID || old.Resource != resource {
		return nil, ErrInvalidGrant
	}
	if old.Used || old.Revoked {
		if err := revokeFamily(tx, old.FamilyID, now); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return nil, ErrReplay
	}
	if !old.ExpiresAt.After(now) {
		if err := revokeFamily(tx, old.FamilyID, now); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return nil, ErrInvalidGrant
	}
	var active int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM oauth_consents WHERE id = ? AND revoked_at IS NULL`, old.ConsentID).Scan(&active); err != nil || active != 1 {
		return nil, ErrInvalidGrant
	}
	result, err := tx.Exec(`UPDATE oauth_refresh_tokens SET used_at = ? WHERE token_hash = ? AND used_at IS NULL AND revoked_at IS NULL`, now.Unix(), oldHash)
	if err != nil {
		return nil, err
	}
	rows, _ := result.RowsAffected()
	if rows != 1 {
		return nil, ErrInvalidGrant
	}
	next.FamilyID = old.FamilyID
	next.ConsentID = old.ConsentID
	next.UserID = old.UserID
	next.ClientID = old.ClientID
	next.Resource = old.Resource
	next.Scopes = old.Scopes
	next.Workspaces = old.WorkspaceIDs
	if err := insertTokens(tx, next, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return old, nil
}

func (r *Repository) revokeToken(hash, clientID string, now time.Time) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var family, owner string
	err = tx.QueryRow(`SELECT family_id, client_id FROM oauth_refresh_tokens WHERE token_hash = ?`, hash).Scan(&family, &owner)
	if err == nil && owner == clientID {
		if err := revokeFamily(tx, family, now); err != nil {
			return err
		}
		return tx.Commit()
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = tx.Exec(`UPDATE oauth_access_tokens SET revoked_at = COALESCE(revoked_at, ?) WHERE token_hash = ? AND client_id = ?`, now.Unix(), hash, clientID)
	if err != nil {
		return err
	}
	return tx.Commit()
}
