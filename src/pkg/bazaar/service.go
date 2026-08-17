package bazaar

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
	"io"
	"net/url"
	"sort"
	"strings"
	"time"

	"taawun/pkg/ethics"
	"taawun/pkg/financial"
	"taawun/pkg/models"
)

// NewService applies Bazaar migrations to the existing application database.
func NewService(db *sql.DB, dependencies Dependencies) (*Service, error) {
	if db == nil || dependencies.Workspaces == nil || dependencies.Artifacts == nil ||
		dependencies.Publications == nil || dependencies.Compliance == nil || dependencies.Shura == nil ||
		dependencies.Financial == nil || dependencies.Gharar == nil {
		return nil, errors.New("Bazaar dependencies are required")
	}
	if dependencies.Now == nil {
		dependencies.Now = time.Now
	}
	if err := migrate(db); err != nil {
		return nil, err
	}
	return &Service{db: db, workspaces: dependencies.Workspaces, artifacts: dependencies.Artifacts,
		publications: dependencies.Publications, compliance: dependencies.Compliance,
		shura: dependencies.Shura, financial: dependencies.Financial, gharar: dependencies.Gharar,
		now: dependencies.Now}, nil
}

func (s *Service) CreateDraft(ctx context.Context, actor *models.User, input DraftInput) (Listing, error) {
	workspace, err := s.authorize(actor, input.CreatorWorkspaceID, models.WorkspaceCapabilityPublish)
	if err != nil {
		return Listing{}, err
	}
	now := s.now().UTC().Truncate(time.Millisecond)
	listingID, err := secureID("listing")
	if err != nil {
		return Listing{}, err
	}
	revision := revisionFromInput(listingID, 1, actor, workspace, input, now)
	if err := s.validateRevision(ctx, revision); err != nil {
		return Listing{}, err
	}
	snapshot, err := json.Marshal(revision)
	if err != nil {
		return Listing{}, fmt.Errorf("encode Bazaar revision: %w", err)
	}
	eventID, err := secureID("event")
	if err != nil {
		return Listing{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Listing{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO bazaar_listings
		(id, creator_user_id, creator_workspace_id, state, version, current_revision, created_at, updated_at)
		VALUES (?, ?, ?, 'draft', 1, 1, ?, ?)`, listingID, actor.ID, workspace.ID, now.UnixMilli(), now.UnixMilli()); err != nil {
		return Listing{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO bazaar_listing_revisions
		(listing_id, revision, snapshot_json, created_by, created_at) VALUES (?, 1, ?, ?, ?)`,
		listingID, snapshot, actor.ID, now.UnixMilli()); err != nil {
		return Listing{}, err
	}
	if err := appendEvent(ctx, tx, Event{ID: eventID, EntityType: "listing", EntityID: listingID,
		EntityVersion: 1, Revision: 1, Type: "LISTING_CREATED", ToState: StateDraft,
		ActorUserID: actor.ID, CreatedAt: now}); err != nil {
		return Listing{}, err
	}
	if err := tx.Commit(); err != nil {
		return Listing{}, err
	}
	return s.getListing(ctx, listingID)
}

func (s *Service) UpdateDraft(ctx context.Context, actor *models.User, listingID string, expectedVersion int64, input DraftInput) (Listing, error) {
	current, err := s.getListing(ctx, listingID)
	if err != nil {
		return Listing{}, err
	}
	if current.State != StateDraft || current.Version != expectedVersion || input.CreatorWorkspaceID != current.Revision.CreatorWorkspaceID {
		if current.Version != expectedVersion {
			return Listing{}, ErrVersionConflict
		}
		return Listing{}, ErrInvalidTransition
	}
	workspace, err := s.authorizeCreator(actor, current)
	if err != nil {
		return Listing{}, err
	}
	now := s.now().UTC().Truncate(time.Millisecond)
	revision := revisionFromInput(listingID, current.CurrentRevision+1, actor, workspace, input, now)
	revision.CreatorUserID = current.Revision.CreatorUserID
	revision.CreatorName = current.Revision.CreatorName
	if err := s.validateRevision(ctx, revision); err != nil {
		return Listing{}, err
	}
	snapshot, err := json.Marshal(revision)
	if err != nil {
		return Listing{}, fmt.Errorf("encode Bazaar revision: %w", err)
	}
	eventID, err := secureID("event")
	if err != nil {
		return Listing{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Listing{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE bazaar_listings SET version = version + 1,
		current_revision = ?, updated_at = ? WHERE id = ? AND version = ? AND state = 'draft'`,
		revision.Revision, now.UnixMilli(), listingID, expectedVersion)
	if err != nil {
		return Listing{}, err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return Listing{}, ErrVersionConflict
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO bazaar_listing_revisions
		(listing_id, revision, snapshot_json, created_by, created_at) VALUES (?, ?, ?, ?, ?)`,
		listingID, revision.Revision, snapshot, actor.ID, now.UnixMilli()); err != nil {
		return Listing{}, err
	}
	if err := appendEvent(ctx, tx, Event{ID: eventID, EntityType: "listing", EntityID: listingID,
		EntityVersion: expectedVersion + 1, Revision: revision.Revision, Type: "DRAFT_REVISED",
		FromState: StateDraft, ToState: StateDraft, ActorUserID: actor.ID, CreatedAt: now}); err != nil {
		return Listing{}, err
	}
	if err := tx.Commit(); err != nil {
		return Listing{}, err
	}
	return s.getListing(ctx, listingID)
}

func (s *Service) Transition(ctx context.Context, actor *models.User, listingID string, input TransitionInput) (Listing, error) {
	listing, err := s.getListing(ctx, listingID)
	if err != nil {
		return Listing{}, err
	}
	if listing.Version != input.ExpectedVersion {
		return Listing{}, ErrVersionConflict
	}
	if err := s.authorizeTransition(actor, listing, input.To); err != nil {
		return Listing{}, err
	}
	if !allowedTransition(listing.State, input.To) {
		return Listing{}, ErrInvalidTransition
	}
	if requiresReason(listing.State, input.To) && !validText(input.Reason, 500) {
		return Listing{}, ErrInvalid
	}
	revision := listing.Revision
	revisionChanged := false
	switch {
	case listing.State == StateDraft && input.To == StateSubmitted:
		if err := s.validateCheckoutDisclosure(revision); err != nil {
			return Listing{}, err
		}
	case listing.State == StateComplianceReview && input.To == StateShuraReview:
		review, err := s.compliance.ReviewListing(ctx, revision)
		if err != nil || review.Status != "passed" || review.Madhhab != revision.Compliance.Madhhab || len(review.Citations) == 0 {
			return Listing{}, fmt.Errorf("%w: compliance review did not pass", ErrInvalidTransition)
		}
		revision.Compliance = cloneCompliance(review)
		revisionChanged = true
	case listing.State == StateShuraReview && input.To == StateApproved:
		if !validIdentifier(input.DecisionRef) {
			return Listing{}, ErrInvalid
		}
		decision, err := s.shura.ResolveDecision(ctx, revision.CreatorWorkspaceID, input.DecisionRef)
		if err != nil || !decision.Approved || decision.WorkspaceID != revision.CreatorWorkspaceID || decision.Reference != input.DecisionRef {
			return Listing{}, fmt.Errorf("%w: approved Shura decision required", ErrInvalidTransition)
		}
		revision.ShuraDecisionRef = decision.Reference
		revisionChanged = true
	case input.To == StatePublished:
		if err := s.validateCheckoutDisclosure(revision); err != nil {
			return Listing{}, err
		}
		origin, err := originOf(revision.PublicTestDriveURL)
		if err != nil || s.publications.VerifyActivePublication(ctx, revision.CreatorWorkspaceID, origin, revision.ContentHash) != nil {
			return Listing{}, fmt.Errorf("%w: public test drive is not an active verified publication", ErrInvalidTransition)
		}
	}
	now := s.now().UTC().Truncate(time.Millisecond)
	newRevision := listing.CurrentRevision
	var snapshot []byte
	if revisionChanged {
		newRevision++
		revision.Revision = newRevision
		revision.CreatedAt = now
		snapshot, err = json.Marshal(revision)
		if err != nil {
			return Listing{}, fmt.Errorf("encode Bazaar review revision: %w", err)
		}
	}
	detail, err := json.Marshal(map[string]string{"reason": strings.TrimSpace(input.Reason), "decisionRef": strings.TrimSpace(input.DecisionRef)})
	if err != nil {
		return Listing{}, err
	}
	eventID, err := secureID("event")
	if err != nil {
		return Listing{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Listing{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE bazaar_listings SET state = ?, version = version + 1,
		current_revision = ?, updated_at = ? WHERE id = ? AND version = ? AND state = ?`,
		input.To, newRevision, now.UnixMilli(), listingID, input.ExpectedVersion, listing.State)
	if err != nil {
		return Listing{}, err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return Listing{}, ErrVersionConflict
	}
	if revisionChanged {
		if _, err := tx.ExecContext(ctx, `INSERT INTO bazaar_listing_revisions
			(listing_id, revision, snapshot_json, created_by, created_at) VALUES (?, ?, ?, ?, ?)`,
			listingID, newRevision, snapshot, actor.ID, now.UnixMilli()); err != nil {
			return Listing{}, err
		}
	}
	if err := appendEvent(ctx, tx, Event{ID: eventID, EntityType: "listing", EntityID: listingID,
		EntityVersion: input.ExpectedVersion + 1, Revision: newRevision, Type: "STATE_TRANSITIONED",
		FromState: listing.State, ToState: input.To, ActorUserID: actor.ID, Detail: detail, CreatedAt: now}); err != nil {
		return Listing{}, err
	}
	if err := tx.Commit(); err != nil {
		return Listing{}, err
	}
	return s.getListing(ctx, listingID)
}

func (s *Service) PublicList(ctx context.Context) ([]Listing, error) {
	return s.listPublished(ctx)
}

func (s *Service) PublicDetail(ctx context.Context, listingID string) (Listing, error) {
	listing, err := s.getListing(ctx, listingID)
	if err != nil {
		return Listing{}, err
	}
	if listing.State != StatePublished {
		return Listing{}, ErrNotPublished
	}
	return listing, nil
}

func (s *Service) Purchase(ctx context.Context, actor *models.User, listingID string, targetWorkspaceID int, idempotencyKey string) (Purchase, error) {
	listing, err := s.PublicDetail(ctx, listingID)
	if err != nil {
		return Purchase{}, err
	}
	if _, err := s.authorize(actor, targetWorkspaceID, models.WorkspaceCapabilityBuild); err != nil {
		return Purchase{}, err
	}
	if !validIdentifier(idempotencyKey) {
		return Purchase{}, ErrInvalid
	}
	if err := s.validateCheckoutDisclosure(listing.Revision); err != nil {
		return Purchase{}, err
	}
	origin, _ := originOf(listing.Revision.PublicTestDriveURL)
	if s.publications.VerifyActivePublication(ctx, listing.Revision.CreatorWorkspaceID, origin, listing.Revision.ContentHash) != nil {
		return Purchase{}, ErrNotPublished
	}
	requestHash := purchaseRequestHash(listing, actor.ID, targetWorkspaceID, idempotencyKey)
	existing, err := s.purchaseByIdempotency(ctx, actor.ID, idempotencyKey)
	if err == nil {
		var storedHash string
		if err := s.db.QueryRowContext(ctx, `SELECT request_hash FROM bazaar_purchases WHERE id = ?`, existing.ID).Scan(&storedHash); err != nil {
			return Purchase{}, err
		}
		if storedHash != requestHash {
			return Purchase{}, ErrIdempotencyConflict
		}
		return existing, nil
	} else if !errors.Is(err, ErrNotFound) {
		return Purchase{}, err
	}
	questKeyDigest := sha256.Sum256([]byte(requestHash))
	questRequest := financial.CreateQuestRequest{
		WorkspaceID: int64(targetWorkspaceID), IdempotencyKey: "bazaar:" + hex.EncodeToString(questKeyDigest[:16]),
		FlowID: financial.FlowMarketplaceEscrow, Currency: listing.Revision.Currency,
		AmountMinor: listing.Revision.PriceMinor, ActorID: actorParty(actor.ID),
		Parties: []string{actorParty(actor.ID), creatorParty(listing.Revision.CreatorUserID)},
		Memo:    fmt.Sprintf("Bazaar %s revision %d", listing.ID, listing.CurrentRevision),
	}
	created, err := s.financial.CreateQuest(ctx, questRequest)
	if err != nil {
		return Purchase{}, fmt.Errorf("create marketplace escrow quest: %w", err)
	}
	if created == nil || created.Quest == nil {
		return Purchase{}, errors.New("create marketplace escrow quest: empty financial response")
	}
	purchaseID, err := secureID("purchase")
	if err != nil {
		return Purchase{}, err
	}
	eventID, err := secureID("event")
	if err != nil {
		return Purchase{}, err
	}
	now := s.now().UTC().Truncate(time.Millisecond)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Purchase{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO bazaar_purchases
		(id, listing_id, revision, buyer_user_id, target_workspace_id, idempotency_key,
		 request_hash, quest_id, quest_status, quest_version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(buyer_user_id, idempotency_key) DO NOTHING`, purchaseID, listing.ID, listing.CurrentRevision,
		actor.ID, targetWorkspaceID, idempotencyKey, requestHash, created.Quest.ID,
		string(created.Quest.Status), created.Quest.Version, now.UnixMilli(), now.UnixMilli())
	if err != nil {
		return Purchase{}, err
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return Purchase{}, rollbackErr
		}
		existing, loadErr := s.purchaseByIdempotency(ctx, actor.ID, idempotencyKey)
		if loadErr != nil {
			return Purchase{}, loadErr
		}
		var storedHash string
		if loadErr = s.db.QueryRowContext(ctx, `SELECT request_hash FROM bazaar_purchases WHERE id = ?`, existing.ID).Scan(&storedHash); loadErr != nil {
			return Purchase{}, loadErr
		}
		if storedHash != requestHash {
			return Purchase{}, ErrIdempotencyConflict
		}
		return existing, nil
	}
	detail, _ := json.Marshal(map[string]string{"questId": created.Quest.ID})
	if err := appendEvent(ctx, tx, Event{ID: eventID, EntityType: "purchase", EntityID: purchaseID,
		EntityVersion: created.Quest.Version, Revision: listing.CurrentRevision, Type: "PURCHASE_QUEST_CREATED",
		ActorUserID: actor.ID, Detail: detail, CreatedAt: now}); err != nil {
		return Purchase{}, err
	}
	if err := tx.Commit(); err != nil {
		return Purchase{}, err
	}
	return s.getPurchase(ctx, actor.ID, purchaseID)
}

func (s *Service) RefreshPurchase(ctx context.Context, actor *models.User, purchaseID string) (Purchase, error) {
	if actor == nil || actor.ID <= 0 {
		return Purchase{}, ErrForbidden
	}
	purchase, err := s.getPurchase(ctx, actor.ID, purchaseID)
	if err != nil {
		return Purchase{}, err
	}
	quest, err := s.financial.GetQuest(ctx, purchase.QuestID)
	if err != nil {
		return Purchase{}, fmt.Errorf("read marketplace escrow quest: %w", err)
	}
	if quest == nil {
		return Purchase{}, errors.New("read marketplace escrow quest: empty financial response")
	}
	revision, err := s.getRevision(ctx, purchase.ListingID, purchase.Revision)
	if err != nil {
		return Purchase{}, err
	}
	if quest.FlowID != financial.FlowMarketplaceEscrow || quest.WorkspaceID != int64(purchase.TargetWorkspaceID) ||
		quest.Intent.AmountMinor != revision.PriceMinor || quest.Intent.Currency != revision.Currency {
		return Purchase{}, ErrInvalid
	}
	now := s.now().UTC().Truncate(time.Millisecond)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Purchase{}, err
	}
	defer tx.Rollback()
	entitlementID := purchase.EntitlementID
	if quest.Status == financial.QuestSettled {
		if quest.ReconciliationReference == "" {
			return Purchase{}, ErrSettlementPending
		}
		if entitlementID == "" {
			entitlementID, err = secureID("entitlement")
			if err != nil {
				return Purchase{}, err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO bazaar_entitlements
				(id, purchase_id, listing_id, revision, content_hash, buyer_user_id, target_workspace_id, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, entitlementID, purchase.ID, purchase.ListingID,
				purchase.Revision, revision.ContentHash, purchase.BuyerUserID, purchase.TargetWorkspaceID, now.UnixMilli()); err != nil {
				return Purchase{}, err
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE bazaar_purchases SET quest_status = ?, quest_version = ?,
		reconciliation_reference = ?, entitlement_id = ?, updated_at = ? WHERE id = ?`,
		string(quest.Status), quest.Version, quest.ReconciliationReference, entitlementID, now.UnixMilli(), purchase.ID); err != nil {
		return Purchase{}, err
	}
	eventID, err := secureID("event")
	if err != nil {
		return Purchase{}, err
	}
	detail, _ := json.Marshal(map[string]string{"questStatus": string(quest.Status), "reconciliationReference": quest.ReconciliationReference})
	if err := appendEvent(ctx, tx, Event{ID: eventID, EntityType: "purchase", EntityID: purchase.ID,
		EntityVersion: quest.Version, Revision: purchase.Revision, Type: "PURCHASE_RECONCILED",
		ActorUserID: actor.ID, Detail: detail, CreatedAt: now}); err != nil {
		return Purchase{}, err
	}
	if err := tx.Commit(); err != nil {
		return Purchase{}, err
	}
	refreshed, err := s.getPurchase(ctx, actor.ID, purchase.ID)
	if err != nil {
		return Purchase{}, err
	}
	if quest.Status != financial.QuestSettled {
		return refreshed, ErrSettlementPending
	}
	return refreshed, nil
}

func (s *Service) Install(ctx context.Context, actor *models.User, entitlementID string, expectedVersion int64) (Installation, error) {
	if actor == nil || actor.ID <= 0 {
		return Installation{}, ErrForbidden
	}
	var entitlement Entitlement
	var entitlementCreatedAt int64
	err := s.db.QueryRowContext(ctx, `SELECT id, purchase_id, listing_id, revision, content_hash,
		buyer_user_id, target_workspace_id, created_at FROM bazaar_entitlements WHERE id = ? AND buyer_user_id = ?`,
		entitlementID, actor.ID).Scan(&entitlement.ID, &entitlement.PurchaseID, &entitlement.ListingID,
		&entitlement.Revision, &entitlement.ContentHash, &entitlement.BuyerUserID, &entitlement.TargetWorkspaceID, &entitlementCreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Installation{}, ErrNotEntitled
	}
	if err != nil {
		return Installation{}, err
	}
	if _, err := s.authorize(actor, entitlement.TargetWorkspaceID, models.WorkspaceCapabilityBuild); err != nil {
		return Installation{}, err
	}
	if revision, err := s.getRevision(ctx, entitlement.ListingID, entitlement.Revision); err != nil || revision.ContentHash != entitlement.ContentHash {
		return Installation{}, ErrNotEntitled
	}
	now := s.now().UTC().Truncate(time.Millisecond)
	var current Installation
	var currentCreatedAt, currentUpdatedAt int64
	currentErr := s.db.QueryRowContext(ctx, `SELECT id, entitlement_id, listing_id, revision, content_hash,
		target_workspace_id, version, installed_by, created_at, updated_at FROM bazaar_installations
		WHERE target_workspace_id = ? AND listing_id = ?`, entitlement.TargetWorkspaceID, entitlement.ListingID).
		Scan(&current.ID, &current.EntitlementID, &current.ListingID, &current.Revision, &current.ContentHash,
			&current.TargetWorkspaceID, &current.Version, &current.InstalledBy, &currentCreatedAt, &currentUpdatedAt)
	if currentErr == nil && current.Version != expectedVersion {
		return Installation{}, ErrVersionConflict
	}
	if errors.Is(currentErr, sql.ErrNoRows) && expectedVersion != 0 {
		return Installation{}, ErrVersionConflict
	}
	if currentErr != nil && !errors.Is(currentErr, sql.ErrNoRows) {
		return Installation{}, currentErr
	}
	installationID := current.ID
	newVersion := int64(1)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Installation{}, err
	}
	defer tx.Rollback()
	if currentErr == nil {
		newVersion = current.Version + 1
		result, updateErr := tx.ExecContext(ctx, `UPDATE bazaar_installations SET entitlement_id = ?, revision = ?,
			content_hash = ?, version = ?, installed_by = ?, updated_at = ? WHERE id = ? AND version = ?`,
			entitlement.ID, entitlement.Revision, entitlement.ContentHash, newVersion, actor.ID,
			now.UnixMilli(), current.ID, expectedVersion)
		err = updateErr
		if err == nil {
			if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
				return Installation{}, ErrVersionConflict
			}
		}
	} else {
		installationID, err = secureID("install")
		if err != nil {
			return Installation{}, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO bazaar_installations
			(id, entitlement_id, listing_id, revision, content_hash, target_workspace_id, version,
			 installed_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?, ?)`,
			installationID, entitlement.ID, entitlement.ListingID, entitlement.Revision,
			entitlement.ContentHash, entitlement.TargetWorkspaceID, actor.ID, now.UnixMilli(), now.UnixMilli())
	}
	if err != nil {
		return Installation{}, err
	}
	eventID, err := secureID("event")
	if err != nil {
		return Installation{}, err
	}
	if err := appendEvent(ctx, tx, Event{ID: eventID, EntityType: "installation", EntityID: installationID,
		EntityVersion: newVersion, Revision: entitlement.Revision, Type: "REVISION_INSTALLED",
		ActorUserID: actor.ID, CreatedAt: now}); err != nil {
		return Installation{}, err
	}
	if err := tx.Commit(); err != nil {
		return Installation{}, err
	}
	return s.getInstallation(ctx, installationID)
}

func (s *Service) validateRevision(ctx context.Context, revision ListingRevision) error {
	if !validText(revision.Title, 120) || !validText(revision.Summary, 1000) ||
		!validIdentifier(revision.ArtifactID) || len(revision.ContentHash) != 64 ||
		!validIdentifier(revision.TemplateID) || !validVersion(revision.TemplateVersion) ||
		revision.PriceMinor <= 0 || !validCurrency(revision.Currency) || !validText(revision.License, 80) ||
		revision.SSOSeats < 0 || !validText(revision.RelayTier, 80) || revision.RelayBandwidthBytes < 0 ||
		revision.AZOASandboxCapacity < 0 || !validText(revision.DataCustodyStatement, 1000) || len(revision.Limitations) == 0 {
		return ErrInvalid
	}
	if _, err := originOf(revision.PublicTestDriveURL); err != nil || !revision.Compliance.Madhhab.Valid() {
		return ErrInvalid
	}
	if len(revision.Primitives) == 0 || len(revision.Primitives) > 50 || len(revision.Limitations) > 20 {
		return ErrInvalid
	}
	for _, limitation := range revision.Limitations {
		if !validText(limitation, 500) {
			return ErrInvalid
		}
	}
	built, err := s.artifacts.Open(ctx, revision.ContentHash)
	if err != nil || built.ArtifactID != revision.ArtifactID || built.ContentHash != revision.ContentHash ||
		built.Manifest.WorkspaceID != revision.CreatorWorkspaceID || built.Manifest.Template.ID != revision.TemplateID ||
		built.Manifest.Template.Version != revision.TemplateVersion {
		return fmt.Errorf("%w: immutable artifact binding failed", ErrInvalid)
	}
	manifestPrimitives := make([]PrimitiveVersion, len(built.Manifest.Modules))
	for i, module := range built.Manifest.Modules {
		manifestPrimitives[i] = PrimitiveVersion{ID: module.ID, Version: module.Version}
	}
	if !samePrimitives(revision.Primitives, manifestPrimitives) {
		return fmt.Errorf("%w: primitive versions do not match artifact", ErrInvalid)
	}
	return validateRevenueSplit(revision)
}

func (s *Service) validateCheckoutDisclosure(revision ListingRevision) error {
	if revision.StaticHosting.MemoryLimitMB <= 0 || revision.StaticHosting.StorageMB <= 0 ||
		!validText(revision.StaticHosting.CPULimit, 80) || !validText(revision.StaticHosting.UptimeGuarantee, 80) ||
		!revision.StaticHosting.PreviewExpiresAt.After(s.now().UTC()) {
		return ErrInvalid
	}
	spec := &ethics.DeploymentSpec{AppName: revision.Title, WorkspaceID: revision.CreatorWorkspaceID,
		CPULimit: revision.StaticHosting.CPULimit, MemoryLimitMB: revision.StaticHosting.MemoryLimitMB,
		StorageMB: revision.StaticHosting.StorageMB, UptimeGuarantee: revision.StaticHosting.UptimeGuarantee,
		PreviewURL: revision.PublicTestDriveURL, PreviewExpiry: revision.StaticHosting.PreviewExpiresAt,
		IsStagingVetted: true}
	if revision.Currency == "USD" {
		spec.MonthlyFeeUSD = float64(revision.PriceMinor) / 100
	}
	if err := s.gharar.ValidateForCheckout(spec); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	return nil
}

func validateRevenueSplit(revision ListingRevision) error {
	split := revision.RevenueSplit
	if split.CreatorBPS <= 0 || split.PlatformBPS <= 0 || split.CommunityBPS <= 0 ||
		split.CreatorBPS+split.PlatformBPS+split.CommunityBPS != 10000 {
		return fmt.Errorf("%w: revenue split must total 10000 bps", ErrInvalid)
	}
	request := financial.CreateQuestRequest{WorkspaceID: int64(revision.CreatorWorkspaceID),
		IdempotencyKey: "bazaar:split:validation", FlowID: financial.FlowRevenueSplit,
		Currency: revision.Currency, AmountMinor: revision.PriceMinor, ActorID: creatorParty(revision.CreatorUserID),
		Parties: []string{creatorParty(revision.CreatorUserID), "platform:taawun", communityParty(revision.CreatorWorkspaceID)},
		Allocations: []financial.Allocation{{PartyID: creatorParty(revision.CreatorUserID), BasisPoints: split.CreatorBPS},
			{PartyID: "platform:taawun", BasisPoints: split.PlatformBPS},
			{PartyID: communityParty(revision.CreatorWorkspaceID), BasisPoints: split.CommunityBPS}},
		Memo: "Bazaar revenue split contract validation"}
	if err := financial.ValidateCreateQuestRequest(request); err != nil {
		return fmt.Errorf("%w: revenue split is not a vetted financial flow: %v", ErrInvalid, err)
	}
	return nil
}

func revisionFromInput(listingID string, revisionNumber int64, actor *models.User, workspace *models.Workspace, input DraftInput, now time.Time) ListingRevision {
	primitives := append([]PrimitiveVersion(nil), input.Primitives...)
	limitations := append([]string(nil), input.Limitations...)
	return ListingRevision{ListingID: listingID, Revision: revisionNumber, CreatorUserID: actor.ID,
		CreatorName: actor.Username, CreatorWorkspaceID: workspace.ID, CreatorWorkspaceName: workspace.Name,
		Title: strings.TrimSpace(input.Title), Summary: strings.TrimSpace(input.Summary), ArtifactID: input.ArtifactID,
		ContentHash: input.ContentHash, PublicTestDriveURL: input.PublicTestDriveURL, TemplateID: input.TemplateID,
		TemplateVersion: input.TemplateVersion, Primitives: primitives, PriceMinor: input.PriceMinor,
		Currency: strings.ToUpper(input.Currency), License: input.License, StaticHosting: input.StaticHosting,
		SSOSeats: input.SSOSeats, RelayTier: input.RelayTier, RelayBandwidthBytes: input.RelayBandwidthBytes,
		AZOASandboxCapacity: input.AZOASandboxCapacity, DataCustodyStatement: input.DataCustodyStatement,
		Limitations: limitations, Compliance: ComplianceDisclosure{Madhhab: input.Madhhab, Status: "pending"},
		RevenueSplit: input.RevenueSplit, CreatedAt: now}
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

func (s *Service) authorizeCreator(actor *models.User, listing Listing) (*models.Workspace, error) {
	if actor == nil || (!s.platform(actor) && actor.ID != listing.Revision.CreatorUserID) {
		return nil, ErrForbidden
	}
	return s.authorize(actor, listing.Revision.CreatorWorkspaceID, models.WorkspaceCapabilityPublish)
}

func (s *Service) authorizeTransition(actor *models.User, listing Listing, to State) error {
	platformOnly := to == StateComplianceReview || to == StateShuraReview || to == StateApproved ||
		to == StateRejected || to == StateSuspended || listing.State == StateSuspended
	if platformOnly {
		if !s.platform(actor) {
			return ErrForbidden
		}
		return nil
	}
	_, err := s.authorizeCreator(actor, listing)
	return err
}

func (s *Service) platform(actor *models.User) bool {
	return actor != nil && actor.ID > 0 && actor.Role == models.RoleAdmin
}

func allowedTransition(from, to State) bool {
	switch from {
	case StateDraft:
		return to == StateSubmitted
	case StateSubmitted:
		return to == StateComplianceReview || to == StateRejected
	case StateComplianceReview:
		return to == StateShuraReview || to == StateRejected
	case StateShuraReview:
		return to == StateApproved || to == StateRejected
	case StateApproved:
		return to == StatePublished || to == StateRetired
	case StatePublished:
		return to == StateSuspended || to == StateRetired
	case StateSuspended:
		return to == StatePublished || to == StateRetired
	case StateRejected:
		return to == StateDraft
	default:
		return false
	}
}

func requiresReason(from, to State) bool {
	return to == StateRejected || to == StateSuspended || to == StateRetired || from == StateRejected
}

func (s *Service) getRevision(ctx context.Context, listingID string, revision int64) (ListingRevision, error) {
	var snapshot []byte
	if err := s.db.QueryRowContext(ctx, `SELECT snapshot_json FROM bazaar_listing_revisions WHERE listing_id = ? AND revision = ?`, listingID, revision).Scan(&snapshot); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ListingRevision{}, ErrNotFound
		}
		return ListingRevision{}, err
	}
	var result ListingRevision
	if err := json.Unmarshal(snapshot, &result); err != nil {
		return ListingRevision{}, err
	}
	return result, nil
}

func (s *Service) purchaseByIdempotency(ctx context.Context, buyerID int, key string) (Purchase, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM bazaar_purchases WHERE buyer_user_id = ? AND idempotency_key = ?`, buyerID, key).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return Purchase{}, ErrNotFound
	}
	if err != nil {
		return Purchase{}, err
	}
	return s.getPurchase(ctx, buyerID, id)
}

func (s *Service) getPurchase(ctx context.Context, buyerID int, purchaseID string) (Purchase, error) {
	var purchase Purchase
	var status string
	var createdAt, updatedAt int64
	err := s.db.QueryRowContext(ctx, `SELECT id, listing_id, revision, buyer_user_id,
		target_workspace_id, idempotency_key, quest_id, quest_status, quest_version,
		reconciliation_reference, entitlement_id, created_at, updated_at
		FROM bazaar_purchases WHERE id = ? AND buyer_user_id = ?`, purchaseID, buyerID).
		Scan(&purchase.ID, &purchase.ListingID, &purchase.Revision, &purchase.BuyerUserID,
			&purchase.TargetWorkspaceID, &purchase.IdempotencyKey, &purchase.QuestID, &status,
			&purchase.QuestVersion, &purchase.ReconciliationReference, &purchase.EntitlementID, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Purchase{}, ErrNotFound
	}
	if err != nil {
		return Purchase{}, err
	}
	purchase.QuestStatus = financial.QuestStatus(status)
	purchase.CreatedAt, purchase.UpdatedAt = time.UnixMilli(createdAt).UTC(), time.UnixMilli(updatedAt).UTC()
	return purchase, nil
}

func (s *Service) getInstallation(ctx context.Context, id string) (Installation, error) {
	var result Installation
	var createdAt, updatedAt int64
	err := s.db.QueryRowContext(ctx, `SELECT id, entitlement_id, listing_id, revision, content_hash,
		target_workspace_id, version, installed_by, created_at, updated_at FROM bazaar_installations WHERE id = ?`, id).
		Scan(&result.ID, &result.EntitlementID, &result.ListingID, &result.Revision, &result.ContentHash,
			&result.TargetWorkspaceID, &result.Version, &result.InstalledBy, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Installation{}, ErrNotFound
	}
	if err != nil {
		return Installation{}, err
	}
	result.CreatedAt, result.UpdatedAt = time.UnixMilli(createdAt).UTC(), time.UnixMilli(updatedAt).UTC()
	return result, nil
}

func samePrimitives(left, right []PrimitiveVersion) bool {
	if len(left) != len(right) {
		return false
	}
	leftCopy, rightCopy := append([]PrimitiveVersion(nil), left...), append([]PrimitiveVersion(nil), right...)
	sort.Slice(leftCopy, func(i, j int) bool { return leftCopy[i].ID < leftCopy[j].ID })
	sort.Slice(rightCopy, func(i, j int) bool { return rightCopy[i].ID < rightCopy[j].ID })
	for i := range leftCopy {
		if leftCopy[i] != rightCopy[i] || i > 0 && leftCopy[i].ID == leftCopy[i-1].ID {
			return false
		}
	}
	return true
}

func cloneCompliance(value ComplianceDisclosure) ComplianceDisclosure {
	value.Citations = append([]string(nil), value.Citations...)
	return value
}

func originOf(value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return "", ErrInvalid
	}
	return "https://" + strings.ToLower(parsed.Host), nil
}

func purchaseRequestHash(listing Listing, buyerID, workspaceID int, key string) string {
	encoded, _ := json.Marshal([]any{listing.ID, listing.CurrentRevision, listing.Revision.ContentHash, buyerID, workspaceID, key})
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func secureID(prefix string) (string, error) {
	value := make([]byte, 18)
	if _, err := io.ReadFull(rand.Reader, value); err != nil {
		return "", err
	}
	return prefix + ":" + base64.RawURLEncoding.EncodeToString(value), nil
}

func validText(value string, maximum int) bool {
	return strings.TrimSpace(value) == value && value != "" && len(value) <= maximum && !strings.ContainsAny(value, "\x00\r")
}

func validIdentifier(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || strings.ContainsRune("._:-", character) {
			continue
		}
		return false
	}
	return true
}

func validVersion(value string) bool { return validIdentifier(value) }

func validCurrency(value string) bool {
	return len(value) == 3 && value[0] >= 'A' && value[0] <= 'Z' && value[1] >= 'A' && value[1] <= 'Z' && value[2] >= 'A' && value[2] <= 'Z'
}

func actorParty(userID int) string          { return fmt.Sprintf("buyer:%d", userID) }
func creatorParty(userID int) string        { return fmt.Sprintf("creator:%d", userID) }
func communityParty(workspaceID int) string { return fmt.Sprintf("community:%d", workspaceID) }
