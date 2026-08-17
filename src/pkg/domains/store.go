package domains

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const claimColumns = `id, workspace_id, origin, host, status, challenge_expires_at,
	verified_at, verification_expires_at, revoked_at, revocation_reason,
	claimed_by, verified_by, revoked_by, created_at, updated_at`

func ensureSchema(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS workspace_domain_claims (
			id TEXT PRIMARY KEY,
			workspace_id INTEGER NOT NULL,
			origin TEXT NOT NULL,
			host TEXT NOT NULL,
			status TEXT NOT NULL CHECK (status IN ('pending', 'verified', 'revoked')),
			challenge_hash BLOB NOT NULL,
			challenge_expires_at INTEGER NOT NULL,
			verified_at INTEGER,
			verification_expires_at INTEGER,
			revoked_at INTEGER,
			revocation_reason TEXT,
			claimed_by INTEGER NOT NULL,
			verified_by INTEGER,
			revoked_by INTEGER,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE,
			FOREIGN KEY (claimed_by) REFERENCES users(id),
			FOREIGN KEY (verified_by) REFERENCES users(id),
			FOREIGN KEY (revoked_by) REFERENCES users(id)
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_domain_claims_active_origin
			ON workspace_domain_claims(origin) WHERE status IN ('pending', 'verified')`,
		`CREATE INDEX IF NOT EXISTS idx_domain_claims_workspace
			ON workspace_domain_claims(workspace_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_domain_claims_verified
			ON workspace_domain_claims(workspace_id, status, verification_expires_at)`,
		`CREATE TABLE IF NOT EXISTS workspace_domain_publications (
			id TEXT PRIMARY KEY,
			workspace_id INTEGER NOT NULL,
			claim_id TEXT NOT NULL,
			origin TEXT NOT NULL,
			host TEXT NOT NULL,
			content_hash TEXT NOT NULL,
			artifact_id TEXT NOT NULL,
			source_publication_id TEXT,
			activated_by INTEGER NOT NULL,
			activated_at INTEGER NOT NULL,
			deactivated_at INTEGER,
			FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE,
			FOREIGN KEY (claim_id) REFERENCES workspace_domain_claims(id),
			FOREIGN KEY (source_publication_id) REFERENCES workspace_domain_publications(id),
			FOREIGN KEY (activated_by) REFERENCES users(id)
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_domain_publications_active_origin
			ON workspace_domain_publications(origin) WHERE deactivated_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_domain_publications_history
			ON workspace_domain_publications(workspace_id, claim_id, activated_at DESC)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return fmt.Errorf("initialize domain claims schema: %w", err)
		}
	}
	return nil
}

func (s *Service) expireStale(ctx context.Context, now time.Time) error {
	nowUnix := now.Unix()
	statements := []string{
		`UPDATE workspace_domain_claims SET status = 'revoked', revoked_at = ?,
			revocation_reason = 'expired', updated_at = ?
			WHERE status = 'pending' AND challenge_expires_at <= ?`,
		`UPDATE workspace_domain_claims SET status = 'revoked', revoked_at = ?,
			revocation_reason = 'expired', updated_at = ?
			WHERE status = 'verified' AND verification_expires_at <= ?`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement, nowUnix, nowUnix, nowUnix); err != nil {
			return fmt.Errorf("expire domain claims: %w", err)
		}
	}
	return nil
}

func scanClaim(scanner interface{ Scan(...any) error }) (Claim, error) {
	var claim Claim
	var status string
	var challengeExpires, createdAt, updatedAt int64
	var verifiedAt, verificationExpires, revokedAt sql.NullInt64
	var verifiedBy, revokedBy sql.NullInt64
	var reason sql.NullString
	err := scanner.Scan(
		&claim.ID, &claim.WorkspaceID, &claim.Origin, &claim.Host, &status, &challengeExpires,
		&verifiedAt, &verificationExpires, &revokedAt, &reason,
		&claim.ClaimedBy, &verifiedBy, &revokedBy, &createdAt, &updatedAt,
	)
	if err != nil {
		return Claim{}, err
	}
	claim.Status = Status(status)
	claim.ChallengeExpiresAt = time.Unix(challengeExpires, 0).UTC()
	claim.CreatedAt = time.Unix(createdAt, 0).UTC()
	claim.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	claim.VerifiedAt = nullableTime(verifiedAt)
	claim.VerificationExpiresAt = nullableTime(verificationExpires)
	claim.RevokedAt = nullableTime(revokedAt)
	claim.VerifiedBy = nullableInt(verifiedBy)
	claim.RevokedBy = nullableInt(revokedBy)
	if reason.Valid {
		claim.RevocationReason = reason.String
	}
	return claim, nil
}

func nullableTime(value sql.NullInt64) *time.Time {
	if !value.Valid {
		return nil
	}
	result := time.Unix(value.Int64, 0).UTC()
	return &result
}

func nullableInt(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	result := int(value.Int64)
	return &result
}

func translateClaimError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrClaimNotFound
	}
	return err
}
