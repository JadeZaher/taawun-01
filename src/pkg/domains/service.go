package domains

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"sort"
	"strings"
	"time"

	"taawun/pkg/artifacts"
	"taawun/pkg/models"
)

const challengePrefix = "taawun-domain-verification="

type netTXTResolver struct{}

func (netTXTResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	return net.DefaultResolver.LookupTXT(ctx, name)
}

// NewService creates the durable domain authority and applies its idempotent schema.
func NewService(db *sql.DB, workspaces WorkspaceAuthorizer, options Options) (*Service, error) {
	if db == nil || workspaces == nil || options.Artifacts == nil {
		return nil, errors.New("domain database, workspace authorizer, and artifact store are required")
	}
	if options.Resolver == nil {
		options.Resolver = netTXTResolver{}
	}
	if options.Random == nil {
		options.Random = rand.Reader
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.ChallengeTTL == 0 {
		options.ChallengeTTL = 24 * time.Hour
	}
	if options.VerificationTTL == 0 {
		options.VerificationTTL = 90 * 24 * time.Hour
	}
	if options.ChallengeTTL <= 0 || options.VerificationTTL <= 0 {
		return nil, errors.New("domain lifetimes must be positive")
	}
	previewOrigins := make(map[string]struct{}, len(options.PreviewOrigins))
	for _, value := range options.PreviewOrigins {
		origin, err := normalizePreviewOrigin(value)
		if err != nil {
			return nil, err
		}
		if _, duplicate := previewOrigins[origin]; duplicate {
			return nil, fmt.Errorf("duplicate preview origin %q", origin)
		}
		previewOrigins[origin] = struct{}{}
	}
	if err := ensureSchema(db); err != nil {
		return nil, err
	}
	return &Service{
		db: db, workspaces: workspaces, resolver: options.Resolver, artifacts: options.Artifacts,
		previewOrigins: previewOrigins, random: options.Random,
		now: options.Now, challengeTTL: options.ChallengeTTL, verificationTTL: options.VerificationTTL,
	}, nil
}

// ClaimOrigin creates or rotates a one-time DNS ownership proof.
func (s *Service) ClaimOrigin(ctx context.Context, actor *models.User, workspaceID int, value string) (ClaimResult, error) {
	if _, err := s.authorize(actor, workspaceID, models.WorkspaceCapabilityPublish); err != nil {
		return ClaimResult{}, err
	}
	origin, host, err := NormalizeOrigin(value)
	if err != nil {
		return ClaimResult{}, err
	}
	now := s.now().UTC()
	if err := s.expireStale(ctx, now); err != nil {
		return ClaimResult{}, err
	}
	claimID, err := randomToken(s.random, 18)
	if err != nil {
		return ClaimResult{}, fmt.Errorf("create claim identity: %w", err)
	}
	secret, err := randomToken(s.random, 32)
	if err != nil {
		return ClaimResult{}, fmt.Errorf("create DNS challenge: %w", err)
	}
	challenge := challengePrefix + secret
	expires := now.Add(s.challengeTTL)
	digest := challengeDigest(workspaceID, origin, challenge)

	var activeID string
	var activeWorkspace int
	var activeStatus string
	lookupErr := s.db.QueryRowContext(ctx, `SELECT id, workspace_id, status FROM workspace_domain_claims
		WHERE origin = ? AND status IN ('pending', 'verified')`, origin).Scan(&activeID, &activeWorkspace, &activeStatus)
	switch {
	case lookupErr == nil && activeWorkspace != workspaceID:
		return ClaimResult{}, ErrOriginClaimed
	case lookupErr == nil && activeStatus == string(StatusVerified):
		return ClaimResult{}, ErrAlreadyVerified
	case lookupErr == nil:
		_, err = s.db.ExecContext(ctx, `UPDATE workspace_domain_claims SET challenge_hash = ?,
			challenge_expires_at = ?, claimed_by = ?, updated_at = ? WHERE id = ? AND status = 'pending'`,
			digest[:], expires.Unix(), actor.ID, now.Unix(), activeID)
		claimID = activeID
	case !errors.Is(lookupErr, sql.ErrNoRows):
		return ClaimResult{}, fmt.Errorf("inspect active domain claim: %w", lookupErr)
	default:
		_, err = s.db.ExecContext(ctx, `INSERT INTO workspace_domain_claims
			(id, workspace_id, origin, host, status, challenge_hash, challenge_expires_at,
			 claimed_by, created_at, updated_at) VALUES (?, ?, ?, ?, 'pending', ?, ?, ?, ?, ?)`,
			claimID, workspaceID, origin, host, digest[:], expires.Unix(), actor.ID, now.Unix(), now.Unix())
	}
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return ClaimResult{}, ErrOriginClaimed
		}
		return ClaimResult{}, fmt.Errorf("persist domain claim: %w", err)
	}
	claim, err := s.getClaim(ctx, workspaceID, claimID)
	if err != nil {
		return ClaimResult{}, err
	}
	return ClaimResult{Claim: claim, Verification: Verification{
		RecordType: "TXT", RecordName: challengeRecord(host), Value: challenge, ExpiresAt: expires,
	}}, nil
}

// Verify resolves the exact TXT record and activates a time-bounded origin.
func (s *Service) Verify(ctx context.Context, actor *models.User, workspaceID int, claimID string) (Claim, error) {
	if _, err := s.authorize(actor, workspaceID, models.WorkspaceCapabilityPublish); err != nil {
		return Claim{}, err
	}
	claim, digest, err := s.getClaimWithDigest(ctx, workspaceID, claimID)
	if err != nil {
		return Claim{}, err
	}
	if claim.Status != StatusPending {
		return Claim{}, ErrInvalidState
	}
	now := s.now().UTC()
	if !claim.ChallengeExpiresAt.After(now) {
		_, _ = s.db.ExecContext(ctx, `UPDATE workspace_domain_claims SET status = 'revoked', revoked_at = ?,
			revocation_reason = 'expired', updated_at = ? WHERE id = ? AND status = 'pending'`, now.Unix(), now.Unix(), claimID)
		return Claim{}, ErrChallengeExpired
	}
	values, err := s.resolver.LookupTXT(ctx, challengeRecord(claim.Host))
	if err != nil {
		var dnsError *net.DNSError
		if errors.As(err, &dnsError) && (dnsError.IsNotFound || (!dnsError.IsTimeout && !dnsError.IsTemporary)) {
			return Claim{}, ErrDNSProofNotFound
		}
		return Claim{}, fmt.Errorf("resolve DNS TXT proof: %w", err)
	}
	matched := false
	for _, value := range values {
		candidate := challengeDigest(workspaceID, claim.Origin, strings.TrimSpace(value))
		if len(digest) == len(candidate) && subtle.ConstantTimeCompare(digest, candidate[:]) == 1 {
			matched = true
			break
		}
	}
	if !matched {
		return Claim{}, ErrDNSProofNotFound
	}
	verificationExpires := now.Add(s.verificationTTL)
	result, err := s.db.ExecContext(ctx, `UPDATE workspace_domain_claims SET status = 'verified',
		verified_at = ?, verification_expires_at = ?, verified_by = ?, updated_at = ?
		WHERE id = ? AND workspace_id = ? AND status = 'pending' AND challenge_expires_at > ?`,
		now.Unix(), verificationExpires.Unix(), actor.ID, now.Unix(), claimID, workspaceID, now.Unix())
	if err != nil {
		return Claim{}, fmt.Errorf("verify domain claim: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return Claim{}, ErrInvalidState
	}
	return s.getClaim(ctx, workspaceID, claimID)
}

func (s *Service) List(ctx context.Context, actor *models.User, workspaceID int) ([]Claim, error) {
	if _, err := s.authorize(actor, workspaceID, models.WorkspaceCapabilityPublish); err != nil {
		return nil, err
	}
	if err := s.expireStale(ctx, s.now().UTC()); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+claimColumns+` FROM workspace_domain_claims
		WHERE workspace_id = ? ORDER BY created_at DESC, id`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list domain claims: %w", err)
	}
	defer rows.Close()
	claims := make([]Claim, 0)
	for rows.Next() {
		claim, scanErr := scanClaim(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan domain claim: %w", scanErr)
		}
		claims = append(claims, claim)
	}
	return claims, rows.Err()
}

func (s *Service) Inspect(ctx context.Context, actor *models.User, workspaceID int, claimID string) (Claim, error) {
	if _, err := s.authorize(actor, workspaceID, models.WorkspaceCapabilityPublish); err != nil {
		return Claim{}, err
	}
	if err := s.expireStale(ctx, s.now().UTC()); err != nil {
		return Claim{}, err
	}
	return s.getClaim(ctx, workspaceID, claimID)
}

func (s *Service) Revoke(ctx context.Context, actor *models.User, workspaceID int, claimID string) (Claim, error) {
	if _, err := s.authorize(actor, workspaceID, models.WorkspaceCapabilityPublish); err != nil {
		return Claim{}, err
	}
	now := s.now().UTC()
	result, err := s.db.ExecContext(ctx, `UPDATE workspace_domain_claims SET status = 'revoked',
		revoked_at = ?, revoked_by = ?, revocation_reason = 'user', updated_at = ?
		WHERE id = ? AND workspace_id = ? AND status IN ('pending', 'verified')`,
		now.Unix(), actor.ID, now.Unix(), claimID, workspaceID)
	if err != nil {
		return Claim{}, fmt.Errorf("revoke domain claim: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return Claim{}, ErrClaimNotFound
	}
	return s.getClaim(ctx, workspaceID, claimID)
}

// VerifiedOrigins resolves the exact active origin set owned by one workspace.
func (s *Service) VerifiedOrigins(ctx context.Context, workspaceID int) ([]string, error) {
	if workspaceID <= 0 {
		return nil, ErrOriginNotVerified
	}
	now := s.now().UTC()
	if err := s.expireStale(ctx, now); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT origin FROM workspace_domain_claims
		WHERE workspace_id = ? AND status = 'verified' AND verification_expires_at > ? ORDER BY origin`, workspaceID, now.Unix())
	if err != nil {
		return nil, fmt.Errorf("resolve verified origins: %w", err)
	}
	defer rows.Close()
	origins := make([]string, 0)
	for rows.Next() {
		var origin string
		if err := rows.Scan(&origin); err != nil {
			return nil, fmt.Errorf("scan verified origin: %w", err)
		}
		origins = append(origins, origin)
	}
	return origins, rows.Err()
}

// ResolveVerifiedOrigin returns only an exact, active workspace origin.
func (s *Service) ResolveVerifiedOrigin(ctx context.Context, workspaceID int, value string) (string, error) {
	origin, _, err := NormalizeOrigin(value)
	if err != nil {
		return "", err
	}
	now := s.now().UTC().Unix()
	var resolved string
	err = s.db.QueryRowContext(ctx, `SELECT origin FROM workspace_domain_claims
		WHERE workspace_id = ? AND origin = ? AND status = 'verified' AND verification_expires_at > ?`,
		workspaceID, origin, now).Scan(&resolved)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrOriginNotVerified
	}
	if err != nil {
		return "", fmt.Errorf("resolve verified origin: %w", err)
	}
	return resolved, nil
}

// AuthorizeOrigins enforces an exact verified subset for MCP artifact composition.
func (s *Service) AuthorizeOrigins(ctx context.Context, actor *models.User, workspace *models.Workspace, requested artifacts.OriginPolicy) (artifacts.OriginPolicy, error) {
	return s.AuthorizeOriginsForLifecycle(ctx, actor, workspace, artifacts.BundleLifecyclePublished, requested)
}

// AuthorizeOriginsForLifecycle permits configured platform origins only for authenticated previews.
func (s *Service) AuthorizeOriginsForLifecycle(ctx context.Context, actor *models.User, workspace *models.Workspace, lifecycle artifacts.BundleLifecycle, requested artifacts.OriginPolicy) (artifacts.OriginPolicy, error) {
	if workspace == nil {
		return artifacts.OriginPolicy{}, ErrForbidden
	}
	if lifecycle != artifacts.BundleLifecyclePreview && lifecycle != artifacts.BundleLifecyclePublished {
		return artifacts.OriginPolicy{}, ErrOriginNotVerified
	}
	if _, err := s.authorize(actor, workspace.ID, models.WorkspaceCapabilityBuild); err != nil {
		return artifacts.OriginPolicy{}, err
	}
	verified, err := s.VerifiedOrigins(ctx, workspace.ID)
	if err != nil {
		return artifacts.OriginPolicy{}, err
	}
	allowed := make(map[string]struct{}, len(verified))
	for _, origin := range verified {
		allowed[origin] = struct{}{}
	}
	if lifecycle == artifacts.BundleLifecyclePreview {
		for origin := range s.previewOrigins {
			allowed[origin] = struct{}{}
		}
	}
	authorized := artifacts.OriginPolicy{}
	checks := []struct {
		name   string
		input  []string
		output *[]string
	}{
		{name: "surfaces", input: requested.Surfaces, output: &authorized.Surfaces},
		{name: "embedders", input: requested.Embedders, output: &authorized.Embedders},
		{name: "connections", input: requested.Connections, output: &authorized.Connections},
		{name: "resources", input: requested.Resources, output: &authorized.Resources},
	}
	for _, check := range checks {
		seen := make(map[string]struct{}, len(check.input))
		for _, value := range check.input {
			var origin string
			var normalizeErr error
			if lifecycle == artifacts.BundleLifecyclePreview {
				origin, normalizeErr = normalizePreviewOrigin(value)
			} else {
				origin, _, normalizeErr = NormalizeOrigin(value)
			}
			if normalizeErr != nil {
				return artifacts.OriginPolicy{}, fmt.Errorf("%s origin: %w", check.name, normalizeErr)
			}
			if _, ok := allowed[origin]; !ok {
				return artifacts.OriginPolicy{}, fmt.Errorf("%s origin %q: %w", check.name, origin, ErrOriginNotVerified)
			}
			if _, duplicate := seen[origin]; duplicate {
				return artifacts.OriginPolicy{}, fmt.Errorf("duplicate %s origin %q", check.name, origin)
			}
			seen[origin] = struct{}{}
			*check.output = append(*check.output, origin)
		}
		sort.Strings(*check.output)
	}
	return authorized, nil
}

func normalizePreviewOrigin(value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || strings.Contains(parsed.Hostname(), "*") {
		return "", fmt.Errorf("invalid configured preview origin %q", value)
	}
	scheme := strings.ToLower(parsed.Scheme)
	hostname := strings.ToLower(parsed.Hostname())
	loopback := hostname == "localhost" || net.ParseIP(hostname) != nil && net.ParseIP(hostname).IsLoopback()
	if scheme != "https" && !(scheme == "http" && loopback) {
		return "", fmt.Errorf("configured preview origin %q must use HTTPS outside loopback development", value)
	}
	return scheme + "://" + strings.ToLower(parsed.Host), nil
}

func (s *Service) authorize(actor *models.User, workspaceID int, capability models.WorkspaceCapability) (*models.Workspace, error) {
	if actor == nil || actor.ID <= 0 || workspaceID <= 0 {
		return nil, ErrForbidden
	}
	workspace, err := s.workspaces.AuthorizeWorkspaceCapability(actor, workspaceID, capability)
	if err != nil || workspace == nil || workspace.ID != workspaceID {
		return nil, ErrForbidden
	}
	return workspace, nil
}

func (s *Service) getClaim(ctx context.Context, workspaceID int, claimID string) (Claim, error) {
	claim, err := scanClaim(s.db.QueryRowContext(ctx, `SELECT `+claimColumns+` FROM workspace_domain_claims
		WHERE workspace_id = ? AND id = ?`, workspaceID, claimID))
	return claim, translateClaimError(err)
}

func (s *Service) getClaimWithDigest(ctx context.Context, workspaceID int, claimID string) (Claim, []byte, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+claimColumns+`, challenge_hash FROM workspace_domain_claims
		WHERE workspace_id = ? AND id = ?`, workspaceID, claimID)
	var claim Claim
	var status string
	var challengeExpires, createdAt, updatedAt int64
	var verifiedAt, verificationExpires, revokedAt sql.NullInt64
	var verifiedBy, revokedBy sql.NullInt64
	var reason sql.NullString
	var digest []byte
	err := row.Scan(&claim.ID, &claim.WorkspaceID, &claim.Origin, &claim.Host, &status, &challengeExpires,
		&verifiedAt, &verificationExpires, &revokedAt, &reason, &claim.ClaimedBy, &verifiedBy, &revokedBy,
		&createdAt, &updatedAt, &digest)
	if err != nil {
		return Claim{}, nil, translateClaimError(err)
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
	return claim, digest, nil
}

func randomToken(source io.Reader, size int) (string, error) {
	value := make([]byte, size)
	if _, err := io.ReadFull(source, value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func challengeDigest(workspaceID int, origin, challenge string) [sha256.Size]byte {
	return sha256.Sum256([]byte(fmt.Sprintf("taawun-domain-v1\n%d\n%s\n%s", workspaceID, origin, challenge)))
}
