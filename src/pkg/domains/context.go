package domains

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"taawun/pkg/artifacts"
	"taawun/pkg/models"
)

type PublicationContextService struct {
	domains *Service
	tracks  PublicationTrackResolver
}

func NewPublicationContextService(service *Service, tracks PublicationTrackResolver) (*PublicationContextService, error) {
	if service == nil || tracks == nil {
		return nil, errors.New("domain service and publication track resolver are required")
	}
	return &PublicationContextService{domains: service, tracks: tracks}, nil
}

func (s *PublicationContextService) Page(ctx context.Context, actor *models.User, workspaceID int, claimID string, query PublicationContextQuery) (PublicationContextPage, error) {
	requestNow := s.domains.now().UTC()
	if _, err := s.domains.authorize(actor, workspaceID, models.WorkspaceCapabilityPublish); err != nil {
		return PublicationContextPage{}, err
	}
	if query.PublicationID != "" {
		if query.Cursor != "" || query.Limit != 0 || !validPublicationToken(query.PublicationID) {
			return PublicationContextPage{}, ErrInvalidPublicationQuery
		}
	} else if query.Limit < 1 || query.Limit > MaximumPublicationContextLimit {
		return PublicationContextPage{}, ErrInvalidPublicationQuery
	}
	claim, err := s.domains.getClaim(ctx, workspaceID, claimID)
	if err != nil {
		return PublicationContextPage{}, err
	}
	publications, nextCursor, exact, err := s.loadPublications(ctx, workspaceID, claimID, query)
	if err != nil {
		return PublicationContextPage{}, err
	}
	bindings, err := s.tracks.ResolvePublicationTrackBindings(ctx, workspaceID, claimID, publications)
	if err != nil {
		return PublicationContextPage{}, fmt.Errorf("resolve publication track bindings: %w", err)
	}
	contexts := make([]PublicationContext, len(publications))
	proofIndexes := make([]int, 0, 1)
	for index, publication := range publications {
		contexts[index] = publicationAuditContext(publication)
		if candidates := bindings[publication.ID]; len(candidates) == 1 {
			binding := candidates[0]
			contexts[index].TrackBinding = &binding
		}
		if exact || publication.Active {
			proofIndexes = append(proofIndexes, index)
		}
	}
	if len(proofIndexes) == 1 {
		s.deriveServingContext(ctx, claim, publications[proofIndexes[0]], &contexts[proofIndexes[0]], requestNow)
	} else if len(proofIndexes) > 1 {
		for _, index := range proofIndexes {
			contexts[index].ServingState = ServingStateArtifactInvalid
		}
	}
	return PublicationContextPage{Publications: contexts, NextCursor: nextCursor, ServerTime: requestNow}, nil
}

func (s *PublicationContextService) loadPublications(ctx context.Context, workspaceID int, claimID string, query PublicationContextQuery) ([]Publication, string, bool, error) {
	if query.PublicationID != "" {
		publication, err := scanPublication(s.domains.db.QueryRowContext(ctx, `SELECT `+publicationColumns+`
			FROM workspace_domain_publications WHERE id = ? AND workspace_id = ? AND claim_id = ?`, query.PublicationID, workspaceID, claimID))
		if errors.Is(err, sql.ErrNoRows) {
			return []Publication{}, "", true, nil
		}
		if err != nil {
			return nil, "", true, fmt.Errorf("load exact publication context: %w", err)
		}
		return []Publication{publication}, "", true, nil
	}
	arguments := []any{workspaceID, claimID}
	statement := `SELECT ` + publicationColumns + ` FROM workspace_domain_publications
		WHERE workspace_id = ? AND claim_id = ?`
	if query.Cursor != "" {
		cursor, err := decodePublicationCursor(query.Cursor)
		if err != nil {
			return nil, "", false, ErrInvalidPublicationQuery
		}
		statement += ` AND (activated_at < ? OR (activated_at = ? AND id < ?))`
		arguments = append(arguments, cursor.ActivatedAt.Unix(), cursor.ActivatedAt.Unix(), cursor.ID)
	}
	statement += ` ORDER BY activated_at DESC, id DESC LIMIT ?`
	arguments = append(arguments, query.Limit+1)
	rows, err := s.domains.db.QueryContext(ctx, statement, arguments...)
	if err != nil {
		return nil, "", false, fmt.Errorf("list publication context: %w", err)
	}
	defer rows.Close()
	publications := make([]Publication, 0, query.Limit+1)
	for rows.Next() {
		publication, err := scanPublication(rows)
		if err != nil {
			return nil, "", false, fmt.Errorf("scan publication context: %w", err)
		}
		publications = append(publications, publication)
	}
	if err := rows.Err(); err != nil {
		return nil, "", false, fmt.Errorf("read publication context: %w", err)
	}
	nextCursor := ""
	if len(publications) > query.Limit {
		publications = publications[:query.Limit]
		last := publications[len(publications)-1]
		nextCursor = encodePublicationCursor(last.ActivatedAt, last.ID)
	}
	return publications, nextCursor, false, nil
}

func publicationAuditContext(publication Publication) PublicationContext {
	state := ServingStateInactive
	if publication.Active {
		state = ServingStateArtifactInvalid
	}
	return PublicationContext{
		ID: publication.ID, WorkspaceID: publication.WorkspaceID, ClaimID: publication.ClaimID,
		Origin: publication.Origin, ContentHash: publication.ContentHash, ArtifactID: publication.ArtifactID,
		SourcePublicationID: publication.SourcePublicationID, ActivatedAt: publication.ActivatedAt,
		DeactivatedAt: publication.DeactivatedAt, Active: publication.Active, ServingState: state,
	}
}

func (s *PublicationContextService) deriveServingContext(ctx context.Context, claim Claim, publication Publication, target *PublicationContext, now time.Time) {
	if claim.Status != StatusVerified || claim.VerificationExpiresAt == nil || !claim.VerificationExpiresAt.After(now) ||
		claim.WorkspaceID != publication.WorkspaceID || claim.ID != publication.ClaimID ||
		claim.Origin != publication.Origin || claim.Host != publication.Host {
		target.ServingState = ServingStateClaimUnavailable
		return
	}
	built, err := s.domains.artifacts.Open(ctx, publication.ContentHash)
	if err != nil || len(built.ManifestJSON) == 0 || validateArtifactBindingForPublication(built, claim, publication) != nil {
		target.ServingState = ServingStateArtifactInvalid
		return
	}
	expiresAt := built.Manifest.Authorization.ExpiresAt.UTC()
	digest := sha256.Sum256(built.ManifestJSON)
	manifestDigest := hex.EncodeToString(digest[:])
	target.AuthorizationExpiresAt = &expiresAt
	target.ManifestDigest = &manifestDigest
	if errors.Is(artifacts.CheckManifestExpiry(built.Manifest, now), artifacts.ErrArtifactExpired) {
		target.ServingState = ServingStateExpired
		return
	}
	if publication.Active {
		target.ServingState = ServingStateServing
		return
	}
	target.ServingState = ServingStateInactive
}

func validPublicationToken(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || strings.ContainsRune("-_", character) {
			continue
		}
		return false
	}
	return true
}

func encodePublicationCursor(activatedAt time.Time, id string) string {
	value := strconv.FormatInt(activatedAt.Unix(), 10) + "\n" + id
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}

func decodePublicationCursor(value string) (struct {
	ActivatedAt time.Time
	ID          string
}, error) {
	var cursor struct {
		ActivatedAt time.Time
		ID          string
	}
	if len(value) == 0 || len(value) > 256 {
		return cursor, ErrInvalidPublicationQuery
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(decoded) == 0 || len(decoded) > 192 {
		return cursor, ErrInvalidPublicationQuery
	}
	parts := strings.Split(string(decoded), "\n")
	if len(parts) != 2 || !validPublicationToken(parts[1]) {
		return cursor, ErrInvalidPublicationQuery
	}
	seconds, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || seconds <= 0 || strconv.FormatInt(seconds, 10) != parts[0] {
		return cursor, ErrInvalidPublicationQuery
	}
	cursor.ActivatedAt = time.Unix(seconds, 0).UTC()
	cursor.ID = parts[1]
	return cursor, nil
}
