package bazaar

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"taawun/pkg/artifacts"
	"taawun/pkg/ethics"
	"taawun/pkg/financial"
	"taawun/pkg/models"
)

type State string

const (
	StateDraft            State = "draft"
	StateSubmitted        State = "submitted"
	StateComplianceReview State = "compliance_review"
	StateShuraReview      State = "shura_review"
	StateApproved         State = "approved"
	StatePublished        State = "published"
	StateRejected         State = "rejected"
	StateSuspended        State = "suspended"
	StateRetired          State = "retired"
)

var (
	ErrInvalid             = errors.New("invalid bazaar request")
	ErrForbidden           = errors.New("bazaar operation forbidden")
	ErrNotFound            = errors.New("bazaar record not found")
	ErrVersionConflict     = errors.New("bazaar version conflict")
	ErrInvalidTransition   = errors.New("invalid bazaar transition")
	ErrNotPublished        = errors.New("bazaar listing is not published")
	ErrSettlementPending   = errors.New("marketplace settlement is not reconciled")
	ErrIdempotencyConflict = errors.New("bazaar idempotency conflict")
	ErrNotEntitled         = errors.New("bazaar entitlement not found")
)

type PrimitiveVersion struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

type StaticHostingTerms struct {
	Included         bool      `json:"included"`
	CPULimit         string    `json:"cpuLimit"`
	MemoryLimitMB    int       `json:"memoryLimitMb"`
	StorageMB        int       `json:"storageMb"`
	UptimeGuarantee  string    `json:"uptimeGuarantee"`
	PreviewExpiresAt time.Time `json:"previewExpiresAt"`
}

type RevenueSplit struct {
	CreatorBPS   int `json:"creatorBps"`
	PlatformBPS  int `json:"platformBps"`
	CommunityBPS int `json:"communityBps"`
}

type ComplianceDisclosure struct {
	Madhhab   ethics.Madhhab `json:"madhhab"`
	Status    string         `json:"status"`
	Citations []string       `json:"citations"`
}

type ListingRevision struct {
	ListingID            string               `json:"listingId"`
	Revision             int64                `json:"revision"`
	CreatorUserID        int                  `json:"creatorUserId"`
	CreatorName          string               `json:"creatorName"`
	CreatorWorkspaceID   int                  `json:"creatorWorkspaceId"`
	CreatorWorkspaceName string               `json:"creatorWorkspaceName"`
	Title                string               `json:"title"`
	Summary              string               `json:"summary"`
	ArtifactID           string               `json:"artifactId"`
	ContentHash          string               `json:"contentHash"`
	PublicTestDriveURL   string               `json:"publicTestDriveUrl"`
	TemplateID           string               `json:"templateId"`
	TemplateVersion      string               `json:"templateVersion"`
	Primitives           []PrimitiveVersion   `json:"primitives"`
	PriceMinor           int64                `json:"priceMinor"`
	Currency             string               `json:"currency"`
	License              string               `json:"license"`
	StaticHosting        StaticHostingTerms   `json:"staticHosting"`
	SSOSeats             int                  `json:"ssoSeats"`
	RelayTier            string               `json:"relayTier"`
	RelayBandwidthBytes  int64                `json:"relayBandwidthBytes"`
	AZOASandboxCapacity  int                  `json:"azoaSandboxCapacity"`
	DataCustodyStatement string               `json:"dataCustodyStatement"`
	Limitations          []string             `json:"limitations"`
	Compliance           ComplianceDisclosure `json:"compliance"`
	ShuraDecisionRef     string               `json:"shuraDecisionRef,omitempty"`
	RevenueSplit         RevenueSplit         `json:"revenueSplit"`
	CreatedAt            time.Time            `json:"createdAt"`
}

type Listing struct {
	ID              string          `json:"id"`
	State           State           `json:"state"`
	Version         int64           `json:"version"`
	CurrentRevision int64           `json:"currentRevision"`
	Revision        ListingRevision `json:"revision"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

type DraftInput struct {
	CreatorWorkspaceID   int                `json:"creatorWorkspaceId"`
	Title                string             `json:"title"`
	Summary              string             `json:"summary"`
	ArtifactID           string             `json:"artifactId"`
	ContentHash          string             `json:"contentHash"`
	PublicTestDriveURL   string             `json:"publicTestDriveUrl"`
	TemplateID           string             `json:"templateId"`
	TemplateVersion      string             `json:"templateVersion"`
	Primitives           []PrimitiveVersion `json:"primitives"`
	PriceMinor           int64              `json:"priceMinor"`
	Currency             string             `json:"currency"`
	License              string             `json:"license"`
	StaticHosting        StaticHostingTerms `json:"staticHosting"`
	SSOSeats             int                `json:"ssoSeats"`
	RelayTier            string             `json:"relayTier"`
	RelayBandwidthBytes  int64              `json:"relayBandwidthBytes"`
	AZOASandboxCapacity  int                `json:"azoaSandboxCapacity"`
	DataCustodyStatement string             `json:"dataCustodyStatement"`
	Limitations          []string           `json:"limitations"`
	Madhhab              ethics.Madhhab     `json:"madhhab"`
	RevenueSplit         RevenueSplit       `json:"revenueSplit"`
}

type TransitionInput struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	To              State  `json:"to"`
	DecisionRef     string `json:"decisionRef,omitempty"`
	Reason          string `json:"reason,omitempty"`
}

type Event struct {
	Sequence      int64           `json:"sequence"`
	ID            string          `json:"id"`
	EntityType    string          `json:"entityType"`
	EntityID      string          `json:"entityId"`
	EntityVersion int64           `json:"entityVersion"`
	Revision      int64           `json:"revision"`
	Type          string          `json:"type"`
	FromState     State           `json:"fromState,omitempty"`
	ToState       State           `json:"toState,omitempty"`
	ActorUserID   int             `json:"actorUserId"`
	Detail        json.RawMessage `json:"detail"`
	CreatedAt     time.Time       `json:"createdAt"`
}

type Purchase struct {
	ID                      string                `json:"id"`
	ListingID               string                `json:"listingId"`
	Revision                int64                 `json:"revision"`
	BuyerUserID             int                   `json:"buyerUserId"`
	TargetWorkspaceID       int                   `json:"targetWorkspaceId"`
	IdempotencyKey          string                `json:"idempotencyKey"`
	QuestID                 string                `json:"questId"`
	QuestStatus             financial.QuestStatus `json:"questStatus"`
	QuestVersion            int64                 `json:"questVersion"`
	ReconciliationReference string                `json:"reconciliationReference,omitempty"`
	EntitlementID           string                `json:"entitlementId,omitempty"`
	CreatedAt               time.Time             `json:"createdAt"`
	UpdatedAt               time.Time             `json:"updatedAt"`
}

type Entitlement struct {
	ID                string    `json:"id"`
	PurchaseID        string    `json:"purchaseId"`
	ListingID         string    `json:"listingId"`
	Revision          int64     `json:"revision"`
	ContentHash       string    `json:"contentHash"`
	BuyerUserID       int       `json:"buyerUserId"`
	TargetWorkspaceID int       `json:"targetWorkspaceId"`
	CreatedAt         time.Time `json:"createdAt"`
}

type Installation struct {
	ID                string    `json:"id"`
	EntitlementID     string    `json:"entitlementId"`
	ListingID         string    `json:"listingId"`
	Revision          int64     `json:"revision"`
	ContentHash       string    `json:"contentHash"`
	TargetWorkspaceID int       `json:"targetWorkspaceId"`
	Version           int64     `json:"version"`
	InstalledBy       int       `json:"installedBy"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type WorkspaceAuthorizer interface {
	AuthorizeWorkspaceCapability(*models.User, int, models.WorkspaceCapability) (*models.Workspace, error)
}

type ArtifactInspector interface {
	Open(context.Context, string) (artifacts.BuildResult, error)
}

type PublicationResolver interface {
	VerifyActivePublication(context.Context, int, string, string) error
}

type ComplianceReviewer interface {
	ReviewListing(context.Context, ListingRevision) (ComplianceDisclosure, error)
}

type ShuraDecision struct {
	Reference   string
	WorkspaceID int
	Approved    bool
}

type ShuraDecisionResolver interface {
	ResolveDecision(context.Context, int, string) (ShuraDecision, error)
}

type FinancialGateway interface {
	CreateQuest(context.Context, financial.CreateQuestRequest) (*financial.CreateQuestResult, error)
	GetQuest(context.Context, string) (*financial.Quest, error)
}

type GhararValidator interface {
	ValidateForCheckout(*ethics.DeploymentSpec) error
}

type Dependencies struct {
	Workspaces   WorkspaceAuthorizer
	Artifacts    ArtifactInspector
	Publications PublicationResolver
	Compliance   ComplianceReviewer
	Shura        ShuraDecisionResolver
	Financial    FinancialGateway
	Gharar       GhararValidator
	Now          func() time.Time
}

type Service struct {
	db           *sql.DB
	workspaces   WorkspaceAuthorizer
	artifacts    ArtifactInspector
	publications PublicationResolver
	compliance   ComplianceReviewer
	shura        ShuraDecisionResolver
	financial    FinancialGateway
	gharar       GhararValidator
	now          func() time.Time
}
