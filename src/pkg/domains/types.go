package domains

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"time"

	"taawun/pkg/artifacts"
	"taawun/pkg/models"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusVerified Status = "verified"
	StatusRevoked  Status = "revoked"
)

var (
	ErrForbidden               = errors.New("domain management forbidden")
	ErrInvalidOrigin           = errors.New("invalid HTTPS origin")
	ErrOriginClaimed           = errors.New("origin already has an active claim")
	ErrAlreadyVerified         = errors.New("origin is already verified")
	ErrClaimNotFound           = errors.New("domain claim not found")
	ErrInvalidState            = errors.New("domain claim is not pending")
	ErrChallengeExpired        = errors.New("domain challenge expired")
	ErrDNSProofNotFound        = errors.New("DNS TXT proof not found")
	ErrOriginNotVerified       = errors.New("origin is not verified for workspace")
	ErrArtifactInvalid         = errors.New("artifact is not publishable for domain")
	ErrPublicationMissing      = errors.New("publication not found")
	ErrInvalidPublicationQuery = errors.New("publication context query is invalid")
)

const (
	DefaultPublicationContextLimit = 20
	MaximumPublicationContextLimit = 50
)

type Claim struct {
	ID                    string     `json:"id"`
	WorkspaceID           int        `json:"workspaceId"`
	Origin                string     `json:"origin"`
	Host                  string     `json:"host"`
	Status                Status     `json:"status"`
	ChallengeExpiresAt    time.Time  `json:"challengeExpiresAt"`
	VerifiedAt            *time.Time `json:"verifiedAt,omitempty"`
	VerificationExpiresAt *time.Time `json:"verificationExpiresAt,omitempty"`
	RevokedAt             *time.Time `json:"revokedAt,omitempty"`
	RevocationReason      string     `json:"revocationReason,omitempty"`
	ClaimedBy             int        `json:"claimedBy"`
	VerifiedBy            *int       `json:"verifiedBy,omitempty"`
	RevokedBy             *int       `json:"revokedBy,omitempty"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
}

type Verification struct {
	RecordType string    `json:"recordType"`
	RecordName string    `json:"recordName"`
	Value      string    `json:"value"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

type ClaimResult struct {
	Claim        Claim        `json:"claim"`
	Verification Verification `json:"verification"`
}

type TXTResolver interface {
	LookupTXT(context.Context, string) ([]string, error)
}

type WorkspaceAuthorizer interface {
	AuthorizeWorkspaceCapability(*models.User, int, models.WorkspaceCapability) (*models.Workspace, error)
}

type ArtifactStore interface {
	Open(context.Context, string) (artifacts.BuildResult, error)
	ReadFile(context.Context, string, string) (artifacts.ArtifactFile, error)
	ReadVerifiedFile(context.Context, artifacts.BuildResult, string) (artifacts.ArtifactFile, error)
}

type Publication struct {
	ID                  string     `json:"id"`
	WorkspaceID         int        `json:"workspaceId"`
	ClaimID             string     `json:"claimId"`
	Origin              string     `json:"origin"`
	Host                string     `json:"host"`
	ContentHash         string     `json:"contentHash"`
	ArtifactID          string     `json:"artifactId"`
	SourcePublicationID string     `json:"sourcePublicationId,omitempty"`
	ActivatedBy         int        `json:"activatedBy"`
	ActivatedAt         time.Time  `json:"activatedAt"`
	DeactivatedAt       *time.Time `json:"deactivatedAt,omitempty"`
	Active              bool       `json:"active"`
}

type ServingState string

const (
	ServingStateServing          ServingState = "serving"
	ServingStateInactive         ServingState = "inactive"
	ServingStateExpired          ServingState = "expired"
	ServingStateClaimUnavailable ServingState = "claim_unavailable"
	ServingStateArtifactInvalid  ServingState = "artifact_invalid"
)

type PublicationTrackBinding struct {
	TrackID string `json:"trackId"`
	Status  string `json:"status"`
	Version int64  `json:"version"`
}

// PublicationContext is the redacted audit DTO; it is not a mutation response.
type PublicationContext struct {
	ID                     string                   `json:"id"`
	WorkspaceID            int                      `json:"workspaceId"`
	ClaimID                string                   `json:"claimId"`
	Origin                 string                   `json:"origin"`
	ContentHash            string                   `json:"contentHash"`
	ArtifactID             string                   `json:"artifactId"`
	SourcePublicationID    string                   `json:"sourcePublicationId,omitempty"`
	ActivatedAt            time.Time                `json:"activatedAt"`
	DeactivatedAt          *time.Time               `json:"deactivatedAt"`
	Active                 bool                     `json:"active"`
	AuthorizationExpiresAt *time.Time               `json:"authorizationExpiresAt"`
	ManifestDigest         *string                  `json:"manifestDigest"`
	ServingState           ServingState             `json:"servingState"`
	TrackBinding           *PublicationTrackBinding `json:"trackBinding"`
}

type PublicationContextPage struct {
	Publications []PublicationContext `json:"publications"`
	NextCursor   string               `json:"nextCursor,omitempty"`
	ServerTime   time.Time            `json:"serverTime"`
}

type PublicationContextQuery struct {
	Limit         int
	Cursor        string
	PublicationID string
}

type PublicationTrackResolver interface {
	ResolvePublicationTrackBindings(context.Context, int, string, []Publication) (map[string][]PublicationTrackBinding, error)
}

type Options struct {
	Resolver        TXTResolver
	Artifacts       ArtifactStore
	PreviewOrigins  []string
	Random          io.Reader
	Now             func() time.Time
	ChallengeTTL    time.Duration
	VerificationTTL time.Duration
}

type Service struct {
	db              *sql.DB
	workspaces      WorkspaceAuthorizer
	resolver        TXTResolver
	artifacts       ArtifactStore
	previewOrigins  map[string]struct{}
	random          io.Reader
	now             func() time.Time
	challengeTTL    time.Duration
	verificationTTL time.Duration
}
