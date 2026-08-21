package domains

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"taawun/pkg/artifacts"
	"taawun/pkg/models"
)

const publicationColumns = `id, workspace_id, claim_id, origin, host, content_hash, artifact_id,
	source_publication_id, activated_by, activated_at, deactivated_at`

// Publish validates an immutable signed bundle and makes it the explicit active binding.
func (s *Service) Publish(ctx context.Context, actor *models.User, workspaceID int, claimID, contentHash string) (Publication, error) {
	if _, err := s.authorize(actor, workspaceID, models.WorkspaceCapabilityPublish); err != nil {
		return Publication{}, err
	}
	claim, err := s.publishableClaim(ctx, workspaceID, claimID)
	if err != nil {
		return Publication{}, err
	}
	return s.activateContent(ctx, actor, claim, contentHash, nil)
}

// Activate creates a new activation event from a prior publication for rollback.
func (s *Service) Activate(ctx context.Context, actor *models.User, workspaceID int, claimID, publicationID string) (Publication, error) {
	if _, err := s.authorize(actor, workspaceID, models.WorkspaceCapabilityPublish); err != nil {
		return Publication{}, err
	}
	claim, err := s.publishableClaim(ctx, workspaceID, claimID)
	if err != nil {
		return Publication{}, err
	}
	prior, err := scanPublication(s.db.QueryRowContext(ctx, `SELECT `+publicationColumns+`
		FROM workspace_domain_publications WHERE id = ? AND workspace_id = ? AND claim_id = ?`,
		publicationID, workspaceID, claimID))
	if errors.Is(err, sql.ErrNoRows) {
		return Publication{}, ErrPublicationMissing
	}
	if err != nil {
		return Publication{}, fmt.Errorf("load publication rollback target: %w", err)
	}
	return s.activateContent(ctx, actor, claim, prior.ContentHash, &prior)
}

func (s *Service) PublicationHistory(ctx context.Context, actor *models.User, workspaceID int, claimID string) ([]Publication, error) {
	if _, err := s.authorize(actor, workspaceID, models.WorkspaceCapabilityPublish); err != nil {
		return nil, err
	}
	if _, err := s.getClaim(ctx, workspaceID, claimID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+publicationColumns+`
		FROM workspace_domain_publications WHERE workspace_id = ? AND claim_id = ?
		ORDER BY activated_at DESC, id DESC`, workspaceID, claimID)
	if err != nil {
		return nil, fmt.Errorf("list publication history: %w", err)
	}
	defer rows.Close()
	publications := make([]Publication, 0)
	for rows.Next() {
		publication, scanErr := scanPublication(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan publication: %w", scanErr)
		}
		publications = append(publications, publication)
	}
	return publications, rows.Err()
}

// ActivePublication returns the exact active activation fact for one authorized claim.
func (s *Service) ActivePublication(ctx context.Context, actor *models.User, workspaceID int, claimID string) (Publication, error) {
	if _, err := s.authorize(actor, workspaceID, models.WorkspaceCapabilityPublish); err != nil {
		return Publication{}, err
	}
	publication, err := scanPublication(s.db.QueryRowContext(ctx, `SELECT `+publicationColumns+`
		FROM workspace_domain_publications WHERE workspace_id = ? AND claim_id = ? AND deactivated_at IS NULL`, workspaceID, claimID))
	if errors.Is(err, sql.ErrNoRows) {
		return Publication{}, ErrPublicationMissing
	}
	if err != nil {
		return Publication{}, fmt.Errorf("load active publication: %w", err)
	}
	return publication, nil
}

// VerifyActivePublication confirms one exact origin currently serves one immutable artifact.
func (s *Service) VerifyActivePublication(ctx context.Context, workspaceID int, origin, contentHash string) error {
	now := s.now().UTC()
	normalized, host, err := NormalizeOrigin(origin)
	if err != nil {
		return err
	}
	publication, claim, err := s.activePublicationForHostAt(ctx, host, now)
	if err != nil {
		return ErrPublicationMissing
	}
	if publication.WorkspaceID != workspaceID || publication.Origin != normalized || publication.ContentHash != contentHash {
		return ErrPublicationMissing
	}
	built, err := s.artifacts.Open(ctx, contentHash)
	if err != nil {
		return ErrArtifactInvalid
	}
	if err := validateArtifactBindingForPublication(built, claim, publication); err != nil {
		return err
	}
	return validateArtifactForClaim(built, claim, now)
}

func (s *Service) activateContent(ctx context.Context, actor *models.User, claim Claim, contentHash string, rollbackSource *Publication) (Publication, error) {
	operationNow := s.now().UTC()
	built, err := s.artifacts.Open(ctx, contentHash)
	if err != nil {
		return Publication{}, fmt.Errorf("%w: %v", ErrArtifactInvalid, err)
	}
	if err := validateArtifactForClaim(built, claim, operationNow); err != nil {
		return Publication{}, err
	}
	if rollbackSource != nil {
		if err := validateArtifactBindingForPublication(built, claim, *rollbackSource); err != nil {
			return Publication{}, err
		}
	}
	now := operationNow.Unix()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Publication{}, fmt.Errorf("begin publication activation: %w", err)
	}
	defer tx.Rollback()
	currentClaim, err := scanClaim(tx.QueryRowContext(ctx, `SELECT `+claimColumns+` FROM workspace_domain_claims
		WHERE workspace_id = ? AND id = ?`, claim.WorkspaceID, claim.ID))
	if errors.Is(err, sql.ErrNoRows) {
		return Publication{}, ErrClaimNotFound
	}
	if err != nil {
		return Publication{}, fmt.Errorf("recheck publication claim: %w", err)
	}
	if currentClaim.Status != StatusVerified || currentClaim.VerificationExpiresAt == nil ||
		!currentClaim.VerificationExpiresAt.After(operationNow) || currentClaim.Origin != claim.Origin || currentClaim.Host != claim.Host {
		return Publication{}, ErrOriginNotVerified
	}
	if err := validateArtifactForClaim(built, currentClaim, operationNow); err != nil {
		return Publication{}, err
	}
	if rollbackSource != nil {
		if err := validateArtifactBindingForPublication(built, currentClaim, *rollbackSource); err != nil {
			return Publication{}, err
		}
	}
	var predecessor Publication
	predecessor, err = scanPublication(tx.QueryRowContext(ctx, `SELECT `+publicationColumns+` FROM workspace_domain_publications
		WHERE origin = ? AND deactivated_at IS NULL`, currentClaim.Origin))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Publication{}, fmt.Errorf("load active publication predecessor: %w", err)
	}
	if err == nil && rollbackSource == nil && publicationMatchesActivation(predecessor, currentClaim, built) {
		return predecessor, nil
	}
	publicationID, err := randomToken(s.random, 18)
	if err != nil {
		return Publication{}, fmt.Errorf("create publication identity: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE workspace_domain_publications SET deactivated_at = ?
		WHERE origin = ? AND deactivated_at IS NULL`, now, currentClaim.Origin); err != nil {
		return Publication{}, fmt.Errorf("deactivate prior publication: %w", err)
	}
	var source any
	if rollbackSource != nil {
		source = rollbackSource.ID
	} else if predecessor.ID != "" {
		source = predecessor.ID
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO workspace_domain_publications
		(id, workspace_id, claim_id, origin, host, content_hash, artifact_id,
		 source_publication_id, activated_by, activated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, publicationID, currentClaim.WorkspaceID, currentClaim.ID,
		currentClaim.Origin, currentClaim.Host, built.ContentHash, built.ArtifactID, source, actor.ID, now); err != nil {
		return Publication{}, fmt.Errorf("activate publication: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Publication{}, fmt.Errorf("commit publication activation: %w", err)
	}
	return s.getPublication(ctx, publicationID)
}

func publicationMatchesActivation(publication Publication, claim Claim, built artifacts.BuildResult) bool {
	return publication.Active && publication.WorkspaceID == claim.WorkspaceID && publication.ClaimID == claim.ID &&
		publication.Origin == claim.Origin && publication.Host == claim.Host &&
		publication.ContentHash == built.ContentHash && publication.ArtifactID == built.ArtifactID
}

func (s *Service) publishableClaim(ctx context.Context, workspaceID int, claimID string) (Claim, error) {
	now := s.now().UTC()
	if err := s.expireStale(ctx, now); err != nil {
		return Claim{}, err
	}
	claim, err := s.getClaim(ctx, workspaceID, claimID)
	if err != nil {
		return Claim{}, err
	}
	if claim.Status != StatusVerified || claim.VerificationExpiresAt == nil || !claim.VerificationExpiresAt.After(now) {
		return Claim{}, ErrOriginNotVerified
	}
	return claim, nil
}

func validateArtifactForClaim(built artifacts.BuildResult, claim Claim, now time.Time) error {
	if err := validateArtifactBindingForClaim(built, claim); err != nil {
		return err
	}
	if err := artifacts.CheckManifestExpiry(built.Manifest, now); err != nil {
		return fmt.Errorf("%w: authorization expired", ErrArtifactInvalid)
	}
	return nil
}

func validateArtifactBindingForClaim(built artifacts.BuildResult, claim Claim) error {
	manifest := built.Manifest
	if built.ContentHash == "" || built.ContentHash != manifest.ContentHash || built.ArtifactID == "" || built.ArtifactID != manifest.ArtifactID {
		return ErrArtifactInvalid
	}
	if manifest.WorkspaceID != claim.WorkspaceID || manifest.Authorization.Subject.WorkspaceID != claim.WorkspaceID {
		return ErrArtifactInvalid
	}
	domainApproved := false
	for _, domain := range manifest.Authorization.ApprovedDomains {
		if domain == claim.Host {
			domainApproved = true
			break
		}
	}
	originApproved := false
	for _, origin := range manifest.Authorization.AllowedOrigins.Surfaces {
		if origin == claim.Origin {
			originApproved = true
			break
		}
	}
	if domainApproved && originApproved {
		return nil
	}
	return fmt.Errorf("%w: manifest does not approve exact host", ErrArtifactInvalid)
}

func validateArtifactBindingForPublication(built artifacts.BuildResult, claim Claim, publication Publication) error {
	if publication.WorkspaceID != claim.WorkspaceID || publication.ClaimID != claim.ID ||
		publication.Origin != claim.Origin || publication.Host != claim.Host ||
		publication.ContentHash != built.ContentHash || publication.ArtifactID != built.ArtifactID {
		return ErrArtifactInvalid
	}
	return validateArtifactBindingForClaim(built, claim)
}

func (s *Service) getPublication(ctx context.Context, publicationID string) (Publication, error) {
	publication, err := scanPublication(s.db.QueryRowContext(ctx, `SELECT `+publicationColumns+`
		FROM workspace_domain_publications WHERE id = ?`, publicationID))
	if errors.Is(err, sql.ErrNoRows) {
		return Publication{}, ErrPublicationMissing
	}
	return publication, err
}

func (s *Service) activePublicationForHostAt(ctx context.Context, host string, now time.Time) (Publication, Claim, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+"p."+publicationColumnsWithPrefix()+`, `+claimColumnsWithPrefix()+`
		FROM workspace_domain_publications p
		JOIN workspace_domain_claims c ON c.id = p.claim_id AND c.workspace_id = p.workspace_id
		WHERE p.host = ? AND p.deactivated_at IS NULL AND c.status = 'verified'
		AND c.verification_expires_at > ?`, host, now.UTC().Unix())
	var publication Publication
	var source sql.NullString
	var publicationActivated, publicationDeactivated sql.NullInt64
	var claim Claim
	var claimStatus string
	var challengeExpires, claimCreated, claimUpdated int64
	var verifiedAt, verificationExpires, revokedAt sql.NullInt64
	var reason sql.NullString
	var verifiedBy, revokedBy sql.NullInt64
	err := row.Scan(
		&publication.ID, &publication.WorkspaceID, &publication.ClaimID, &publication.Origin, &publication.Host,
		&publication.ContentHash, &publication.ArtifactID, &source, &publication.ActivatedBy,
		&publicationActivated, &publicationDeactivated,
		&claim.ID, &claim.WorkspaceID, &claim.Origin, &claim.Host, &claimStatus, &challengeExpires,
		&verifiedAt, &verificationExpires, &revokedAt, &reason, &claim.ClaimedBy, &verifiedBy,
		&revokedBy, &claimCreated, &claimUpdated,
	)
	if err != nil {
		return Publication{}, Claim{}, err
	}
	publication.SourcePublicationID = source.String
	publication.ActivatedAt = time.Unix(publicationActivated.Int64, 0).UTC()
	publication.DeactivatedAt = nullableTime(publicationDeactivated)
	publication.Active = !publicationDeactivated.Valid
	claim.Status = Status(claimStatus)
	claim.ChallengeExpiresAt = time.Unix(challengeExpires, 0).UTC()
	claim.VerifiedAt = nullableTime(verifiedAt)
	claim.VerificationExpiresAt = nullableTime(verificationExpires)
	claim.RevokedAt = nullableTime(revokedAt)
	claim.RevocationReason = reason.String
	claim.VerifiedBy = nullableInt(verifiedBy)
	claim.RevokedBy = nullableInt(revokedBy)
	claim.CreatedAt = time.Unix(claimCreated, 0).UTC()
	claim.UpdatedAt = time.Unix(claimUpdated, 0).UTC()
	return publication, claim, nil
}

func scanPublication(scanner interface{ Scan(...any) error }) (Publication, error) {
	var publication Publication
	var source sql.NullString
	var activatedAt int64
	var deactivatedAt sql.NullInt64
	err := scanner.Scan(&publication.ID, &publication.WorkspaceID, &publication.ClaimID,
		&publication.Origin, &publication.Host, &publication.ContentHash, &publication.ArtifactID,
		&source, &publication.ActivatedBy, &activatedAt, &deactivatedAt)
	if err != nil {
		return Publication{}, err
	}
	publication.SourcePublicationID = source.String
	publication.ActivatedAt = time.Unix(activatedAt, 0).UTC()
	publication.DeactivatedAt = nullableTime(deactivatedAt)
	publication.Active = !deactivatedAt.Valid
	return publication, nil
}

func publicationColumnsWithPrefix() string {
	return `id, p.workspace_id, claim_id, p.origin, p.host, content_hash, artifact_id,
		source_publication_id, activated_by, activated_at, deactivated_at`
}

func claimColumnsWithPrefix() string {
	return `c.id, c.workspace_id, c.origin, c.host, c.status, c.challenge_expires_at,
		c.verified_at, c.verification_expires_at, c.revoked_at, c.revocation_reason,
		c.claimed_by, c.verified_by, c.revoked_by, c.created_at, c.updated_at`
}
