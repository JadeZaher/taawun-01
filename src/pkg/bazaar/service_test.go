package bazaar

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"taawun/pkg/artifacts"
	"taawun/pkg/ethics"
	"taawun/pkg/financial"
	"taawun/pkg/models"
)

type testWorkspaces struct{}

func (testWorkspaces) AuthorizeWorkspaceCapability(actor *models.User, workspaceID int, capability models.WorkspaceCapability) (*models.Workspace, error) {
	if actor == nil || actor.ID <= 0 || workspaceID <= 0 {
		return nil, errors.New("forbidden")
	}
	if actor.Role != models.RoleAdmin {
		if workspaceID == 1 && actor.ID != 10 || workspaceID != 1 && actor.ID != 20 {
			return nil, errors.New("forbidden")
		}
	}
	return &models.Workspace{ID: workspaceID, Name: fmt.Sprintf("Workspace %d", workspaceID), OwnerID: actor.ID, Status: models.WorkspaceStatusActive}, nil
}

type testArtifacts struct{ result artifacts.BuildResult }

func (a testArtifacts) Open(_ context.Context, hash string) (artifacts.BuildResult, error) {
	if hash != a.result.ContentHash {
		return artifacts.BuildResult{}, artifacts.ErrArtifactNotFound
	}
	return a.result, nil
}

type testPublications struct{ active bool }

func (p *testPublications) VerifyActivePublication(_ context.Context, workspaceID int, origin, hash string) error {
	if !p.active || workspaceID != 1 || origin != "https://creator.example" || hash != testContentHash {
		return errors.New("not published")
	}
	return nil
}

type testCompliance struct{}

func (testCompliance) ReviewListing(_ context.Context, revision ListingRevision) (ComplianceDisclosure, error) {
	return ComplianceDisclosure{Madhhab: revision.Compliance.Madhhab, Status: "passed", Citations: []string{"review:corpus-1"}}, nil
}

type testShura struct{}

func (testShura) ResolveDecision(_ context.Context, workspaceID int, reference string) (ShuraDecision, error) {
	return ShuraDecision{Reference: reference, WorkspaceID: workspaceID, Approved: reference == "decision-1"}, nil
}

type testFinancial struct {
	created int
	quest   *financial.Quest
}

func (f *testFinancial) CreateQuest(_ context.Context, request financial.CreateQuestRequest) (*financial.CreateQuestResult, error) {
	f.created++
	f.quest = &financial.Quest{ID: "quest-1", WorkspaceID: request.WorkspaceID, FlowID: request.FlowID,
		FlowVersion: 1, Status: financial.QuestPending, Version: 1,
		Intent: financial.QuestIntent{ID: "intent-1", QuestID: "quest-1", Currency: request.Currency,
			AmountMinor: request.AmountMinor, Parties: append([]string(nil), request.Parties...), Status: financial.QuestPending, Version: 1}}
	return &financial.CreateQuestResult{Quest: f.quest, Created: true}, nil
}

func (f *testFinancial) GetQuest(_ context.Context, id string) (*financial.Quest, error) {
	if f.quest == nil || f.quest.ID != id {
		return nil, financial.ErrQuestNotFound
	}
	copy := *f.quest
	return &copy, nil
}

type testGharar struct{ calls int }

func (v *testGharar) ValidateForCheckout(spec *ethics.DeploymentSpec) error {
	v.calls++
	if spec == nil || !spec.IsStagingVetted || spec.PreviewURL == "" || spec.MemoryLimitMB <= 0 || spec.StorageMB <= 0 {
		return errors.New("uncertain terms")
	}
	return nil
}

const testContentHash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

type bazaarFixture struct {
	db           *sql.DB
	service      *Service
	publications *testPublications
	financial    *testFinancial
	gharar       *testGharar
	creator      *models.User
	admin        *models.User
	buyer        *models.User
	now          time.Time
}

func newBazaarFixture(t *testing.T) *bazaarFixture {
	t.Helper()
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:bazaar-%s?mode=memory&cache=shared&_foreign_keys=on", t.Name()))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	financialGateway := &testFinancial{}
	publications := &testPublications{}
	gharar := &testGharar{}
	artifact := artifacts.BuildResult{ArtifactID: "artifact-1", ContentHash: testContentHash,
		Manifest: artifacts.Manifest{ArtifactID: "artifact-1", ContentHash: testContentHash, WorkspaceID: 1,
			Template: artifacts.TemplateIdentity{ID: "bazaar-cooperative", Version: "1.0.0"},
			Modules:  []artifacts.ModuleDescriptor{{ID: "bazaar-template-lifecycle", Version: "1.0.0"}}}}
	service, err := NewService(db, Dependencies{Workspaces: testWorkspaces{}, Artifacts: testArtifacts{result: artifact},
		Publications: publications, Compliance: testCompliance{}, Shura: testShura{}, Financial: financialGateway,
		Gharar: gharar, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return &bazaarFixture{db: db, service: service, publications: publications, financial: financialGateway,
		gharar: gharar, creator: &models.User{ID: 10, Username: "creator", Role: models.RoleUser},
		admin: &models.User{ID: 99, Username: "reviewer", Role: models.RoleAdmin},
		buyer: &models.User{ID: 20, Username: "buyer", Role: models.RoleUser}, now: now}
}

func (f *bazaarFixture) input() DraftInput {
	return DraftInput{CreatorWorkspaceID: 1, Title: "Community mutual-aid workspace",
		Summary: "A disclosed, reviewable cooperative template.", ArtifactID: "artifact-1", ContentHash: testContentHash,
		PublicTestDriveURL: "https://creator.example/test-drive", TemplateID: "bazaar-cooperative", TemplateVersion: "1.0.0",
		Primitives: []PrimitiveVersion{{ID: "bazaar-template-lifecycle", Version: "1.0.0"}},
		PriceMinor: 2500, Currency: "USD", License: "single-workspace commercial",
		StaticHosting: StaticHostingTerms{Included: true, CPULimit: "0.5 vCPU", MemoryLimitMB: 512, StorageMB: 2048,
			UptimeGuarantee: "99.9% monthly", PreviewExpiresAt: f.now.Add(24 * time.Hour)},
		SSOSeats: 5, RelayTier: "community-small", RelayBandwidthBytes: 10 << 30, AZOASandboxCapacity: 100,
		DataCustodyStatement: "Customer workspace data remains under customer control; Taawun does not custody funds.",
		Limitations:          []string{"No custom server code", "Financial settlement requires an external provider"}, Madhhab: ethics.MadhhabHanafi,
		RevenueSplit: RevenueSplit{CreatorBPS: 8000, PlatformBPS: 1000, CommunityBPS: 1000}}
}

func (f *bazaarFixture) publish(t *testing.T) Listing {
	t.Helper()
	listing, err := f.service.CreateDraft(context.Background(), f.creator, f.input())
	if err != nil {
		t.Fatalf("CreateDraft() error = %v", err)
	}
	steps := []struct {
		actor *models.User
		to    State
		ref   string
	}{
		{f.creator, StateSubmitted, ""},
		{f.admin, StateComplianceReview, ""},
		{f.admin, StateShuraReview, ""},
		{f.admin, StateApproved, "decision-1"},
	}
	for _, step := range steps {
		listing, err = f.service.Transition(context.Background(), step.actor, listing.ID,
			TransitionInput{ExpectedVersion: listing.Version, To: step.to, DecisionRef: step.ref})
		if err != nil {
			t.Fatalf("Transition(%s) error = %v", step.to, err)
		}
	}
	f.publications.active = true
	listing, err = f.service.Transition(context.Background(), f.creator, listing.ID,
		TransitionInput{ExpectedVersion: listing.Version, To: StatePublished})
	if err != nil {
		t.Fatalf("Transition(published) error = %v", err)
	}
	return listing
}

func TestBazaarLifecyclePurchaseSettlementAndInstall(t *testing.T) {
	f := newBazaarFixture(t)
	listing, err := f.service.CreateDraft(context.Background(), f.creator, f.input())
	if err != nil {
		t.Fatalf("CreateDraft() error = %v", err)
	}
	if _, err := f.service.PublicDetail(context.Background(), listing.ID); !errors.Is(err, ErrNotPublished) {
		t.Fatalf("draft PublicDetail() error = %v", err)
	}
	if _, err := f.service.Transition(context.Background(), f.creator, listing.ID,
		TransitionInput{ExpectedVersion: listing.Version, To: StateComplianceReview}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("creator review transition error = %v, want forbidden", err)
	}
	listing = f.publishExisting(t, listing)
	if listing.State != StatePublished || listing.CurrentRevision != 3 || listing.Revision.Compliance.Status != "passed" || listing.Revision.ShuraDecisionRef != "decision-1" {
		t.Fatalf("published listing lost review evidence: %#v", listing)
	}
	public, err := f.service.PublicList(context.Background())
	if err != nil || len(public) != 1 || public[0].ID != listing.ID {
		t.Fatalf("PublicList() = %#v, %v", public, err)
	}

	purchase, err := f.service.Purchase(context.Background(), f.buyer, listing.ID, 2, "checkout-1")
	if err != nil || purchase.QuestStatus != financial.QuestPending || f.financial.created != 1 {
		t.Fatalf("Purchase() = %#v, err=%v, createCalls=%d", purchase, err, f.financial.created)
	}
	repeated, err := f.service.Purchase(context.Background(), f.buyer, listing.ID, 2, "checkout-1")
	if err != nil || repeated.ID != purchase.ID || f.financial.created != 1 {
		t.Fatalf("idempotent Purchase() = %#v, err=%v, createCalls=%d", repeated, err, f.financial.created)
	}
	if _, err := f.service.Purchase(context.Background(), f.buyer, listing.ID, 3, "checkout-1"); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("conflicting Purchase() error = %v", err)
	}
	pending, err := f.service.RefreshPurchase(context.Background(), f.buyer, purchase.ID)
	if !errors.Is(err, ErrSettlementPending) || pending.EntitlementID != "" {
		t.Fatalf("pending RefreshPurchase() = %#v, %v", pending, err)
	}
	f.financial.quest.Status = financial.QuestSettled
	f.financial.quest.Version = 2
	f.financial.quest.Intent.Status = financial.QuestSettled
	f.financial.quest.ReconciliationReference = "provider-settlement-1"
	settled, err := f.service.RefreshPurchase(context.Background(), f.buyer, purchase.ID)
	if err != nil || settled.EntitlementID == "" || settled.ReconciliationReference == "" {
		t.Fatalf("settled RefreshPurchase() = %#v, %v", settled, err)
	}
	installed, err := f.service.Install(context.Background(), f.buyer, settled.EntitlementID, 0)
	if err != nil || installed.Revision != purchase.Revision || installed.ContentHash != testContentHash || installed.TargetWorkspaceID != 2 {
		t.Fatalf("Install() = %#v, %v", installed, err)
	}
	if _, err := f.service.Install(context.Background(), f.buyer, settled.EntitlementID, 0); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale Install() error = %v", err)
	}

	events, err := f.service.Events(context.Background(), f.creator, listing.ID)
	if err != nil || len(events) != 6 {
		t.Fatalf("Events() len=%d err=%v", len(events), err)
	}
	if _, err := f.db.Exec(`UPDATE bazaar_listing_revisions SET snapshot_json = '{}' WHERE listing_id = ?`, listing.ID); err == nil {
		t.Fatal("immutable listing revision update unexpectedly succeeded")
	}
	if _, err := f.db.Exec(`DELETE FROM bazaar_events WHERE entity_id = ?`, listing.ID); err == nil {
		t.Fatal("append-only event deletion unexpectedly succeeded")
	}
}

func (f *bazaarFixture) publishExisting(t *testing.T, listing Listing) Listing {
	t.Helper()
	var err error
	steps := []struct {
		actor *models.User
		to    State
		ref   string
	}{
		{f.creator, StateSubmitted, ""}, {f.admin, StateComplianceReview, ""},
		{f.admin, StateShuraReview, ""}, {f.admin, StateApproved, "decision-1"},
	}
	for _, step := range steps {
		listing, err = f.service.Transition(context.Background(), step.actor, listing.ID,
			TransitionInput{ExpectedVersion: listing.Version, To: step.to, DecisionRef: step.ref})
		if err != nil {
			t.Fatalf("Transition(%s) error = %v", step.to, err)
		}
	}
	f.publications.active = true
	listing, err = f.service.Transition(context.Background(), f.creator, listing.ID,
		TransitionInput{ExpectedVersion: listing.Version, To: StatePublished})
	if err != nil {
		t.Fatalf("Transition(published) error = %v", err)
	}
	return listing
}

func TestBazaarRejectsIncompleteCommercialContract(t *testing.T) {
	f := newBazaarFixture(t)
	input := f.input()
	input.RevenueSplit.CommunityBPS--
	if _, err := f.service.CreateDraft(context.Background(), f.creator, input); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid revenue split error = %v", err)
	}
	input = f.input()
	input.Primitives[0].Version = "9.9.9"
	if _, err := f.service.CreateDraft(context.Background(), f.creator, input); !errors.Is(err, ErrInvalid) {
		t.Fatalf("artifact mismatch error = %v", err)
	}
}
