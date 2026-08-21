package conductor

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sqlite3 "github.com/mattn/go-sqlite3"

	"taawun/pkg/artifacts"
	"taawun/pkg/domains"
	"taawun/pkg/models"
)

func (p *fakePublicationService) ActivePublication(_ context.Context, _ *models.User, workspaceID int, claimID string) (domains.Publication, error) {
	for _, publication := range p.history {
		if publication.WorkspaceID == workspaceID && publication.ClaimID == claimID && publication.Active {
			return publication, nil
		}
	}
	return domains.Publication{}, domains.ErrPublicationMissing
}

func TestPublicationTTLBoundsUseTheInjectedServerClock(t *testing.T) {
	fixed := time.Date(2026, 8, 21, 18, 0, 0, 321_000_000, time.UTC)
	for _, ttlHours := range []int{1, 24, 2160} {
		t.Run(fmt.Sprintf("ttl-%d", ttlHours), func(t *testing.T) {
			harness := newConductorHarness(t)
			service, err := NewServiceWithClock(harness.repository, harness.workspaces, ActorSubjectResolver{}, CuratedCompositionValidator{},
				harness.auditor, harness.builder, harness.origins, harness.publisher, func() time.Time { return fixed })
			if err != nil {
				t.Fatalf("NewServiceWithClock() error = %v", err)
			}
			request := validCompositionRequest()
			request.TTLHours = ttlHours
			result, err := service.Compose(context.Background(), harness.owner, request)
			if err != nil || result == nil || result.Track == nil || result.Track.BuildRequest == nil {
				t.Fatalf("Compose(ttl=%d) = %+v err=%v", ttlHours, result, err)
			}
			wantExpiry := fixed.Add(time.Duration(ttlHours) * time.Hour)
			if !result.Track.BuildRequest.ExpiresAt.Equal(wantExpiry) || !result.Track.Preview.AuthorizationExpiresAt.Equal(wantExpiry) {
				t.Fatalf("ttl=%d expiry build=%s preview=%s want=%s", ttlHours, result.Track.BuildRequest.ExpiresAt, result.Track.Preview.AuthorizationExpiresAt, wantExpiry)
			}
		})
	}

	harness := newConductorHarness(t)
	service, err := NewServiceWithClock(harness.repository, harness.workspaces, ActorSubjectResolver{}, CuratedCompositionValidator{},
		harness.auditor, harness.builder, harness.origins, harness.publisher, func() time.Time { return fixed })
	if err != nil {
		t.Fatal(err)
	}
	request := validCompositionRequest()
	request.TTLHours = 2161
	if result, err := service.Compose(context.Background(), harness.owner, request); result != nil || !errors.Is(err, ErrInvalidComposition) {
		t.Fatalf("Compose(ttl=2161) = %+v err=%v, want rejection", result, err)
	}
	var trackCount int
	if err := harness.repository.db.QueryRow(`SELECT COUNT(*) FROM conductor_tracks`).Scan(&trackCount); err != nil || trackCount != 0 || harness.builder.buildCalls != 0 {
		t.Fatalf("rejected ttl consumed durable state: tracks=%d builds=%d err=%v", trackCount, harness.builder.buildCalls, err)
	}
}

func TestPublicationRequestAndActivationUseExactVersionAndIdempotency(t *testing.T) {
	harness := newConductorHarness(t)
	request := validCompositionRequest()
	created, err := harness.service.Compose(context.Background(), harness.owner, request)
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}
	replayed, err := harness.service.Compose(context.Background(), harness.owner, request)
	if err != nil || replayed.Created || replayed.Track.ID != created.Track.ID || replayed.Track.Version != created.Track.Version {
		t.Fatalf("idempotent preview replay = %+v err=%v", replayed, err)
	}
	conflict := request
	conflict.AppName = "Conflicting application"
	if result, err := harness.service.Compose(context.Background(), harness.owner, conflict); result != nil || !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("conflicting preview idempotency = %+v err=%v", result, err)
	}

	requested, err := harness.service.RequestPublication(context.Background(), harness.owner, created.Track.ID, created.Track.Version, "claim-community")
	if err != nil || requested.Status != TrackPublicationRequested {
		t.Fatalf("RequestPublication() = %+v err=%v", requested, err)
	}
	repeated, err := harness.service.RequestPublication(context.Background(), harness.owner, requested.ID, requested.Version, "claim-community")
	if err != nil || repeated.Version != requested.Version || repeated.ClaimID != requested.ClaimID {
		t.Fatalf("idempotent RequestPublication() = %+v err=%v", repeated, err)
	}
	if stale, err := harness.service.ActivatePublication(context.Background(), harness.owner, requested.ID, created.Track.Version); stale != nil || !errors.Is(err, ErrTrackVersionConflict) {
		t.Fatalf("stale ActivatePublication() = %+v err=%v", stale, err)
	}
	published, err := harness.service.ActivatePublication(context.Background(), harness.owner, requested.ID, requested.Version)
	if err != nil || published.Status != TrackPublished || published.Publication == nil || published.PublicationID != published.Publication.ID {
		t.Fatalf("exact-version ActivatePublication() = %+v err=%v", published, err)
	}
	readback, err := harness.service.GetTrack(context.Background(), harness.owner, published.ID)
	if err != nil || readback.Version != published.Version || readback.PublicationID != published.Publication.ID || readback.Artifact.ContentHash != created.Track.Artifact.ContentHash {
		t.Fatalf("published readback = %+v err=%v", readback, err)
	}
}

type crashWindowPublicationService struct {
	mu           sync.Mutex
	now          time.Time
	active       *domains.Publication
	publishCalls int
	failOnce     bool
}

func (s *crashWindowPublicationService) Inspect(_ context.Context, _ *models.User, workspaceID int, claimID string) (domains.Claim, error) {
	expiresAt := s.now.Add(90 * 24 * time.Hour)
	return domains.Claim{ID: claimID, WorkspaceID: workspaceID, Status: domains.StatusVerified, VerificationExpiresAt: &expiresAt}, nil
}

func (s *crashWindowPublicationService) ActivePublication(_ context.Context, _ *models.User, _ int, _ string) (domains.Publication, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return domains.Publication{}, domains.ErrPublicationMissing
	}
	return *s.active, nil
}

func (s *crashWindowPublicationService) Publish(_ context.Context, _ *models.User, workspaceID int, claimID, contentHash string) (domains.Publication, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active != nil && s.active.WorkspaceID == workspaceID && s.active.ClaimID == claimID && s.active.ContentHash == contentHash && s.active.ArtifactID == "artifact_signed_1" {
		return *s.active, nil
	}
	s.publishCalls++
	publication := domains.Publication{
		ID: "publication-crash-window", WorkspaceID: workspaceID, ClaimID: claimID,
		Origin: "https://community.example", Host: "community.example", ContentHash: contentHash,
		ArtifactID: "artifact_signed_1", ActivatedBy: 7, ActivatedAt: s.now, Active: true,
	}
	s.active = &publication
	if s.failOnce {
		s.failOnce = false
		return domains.Publication{}, errors.New("connection lost after domain commit")
	}
	return publication, nil
}

func TestActivationRecoversTheExactDomainCommitAfterTrackPersistenceCrashWindow(t *testing.T) {
	fixed := time.Date(2026, 8, 21, 19, 0, 0, 0, time.UTC)
	harness := newConductorHarness(t)
	publisher := &crashWindowPublicationService{now: fixed, failOnce: true}
	service, err := NewServiceWithClock(harness.repository, harness.workspaces, ActorSubjectResolver{}, CuratedCompositionValidator{},
		harness.auditor, harness.builder, harness.origins, publisher, func() time.Time { return fixed })
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.Compose(context.Background(), harness.owner, validCompositionRequest())
	if err != nil {
		t.Fatal(err)
	}
	requested, err := service.RequestPublication(context.Background(), harness.owner, created.Track.ID, created.Track.Version, "claim-community")
	if err != nil {
		t.Fatal(err)
	}
	blocked, err := service.ActivatePublication(context.Background(), harness.owner, requested.ID, requested.Version)
	if blocked == nil || blocked.Status != TrackPublicationRequested || !errors.Is(err, ErrDependencyUnavailable) || publisher.active == nil || publisher.publishCalls != 1 {
		t.Fatalf("crash-window activation = %+v err=%v active=%+v calls=%d", blocked, err, publisher.active, publisher.publishCalls)
	}
	persisted, err := service.GetTrack(context.Background(), harness.owner, blocked.ID)
	if err != nil || persisted.Publication != nil || persisted.PublicationID != "" {
		t.Fatalf("track incorrectly persisted an unacknowledged publication: %+v err=%v", persisted, err)
	}
	recovered, err := service.ActivatePublication(context.Background(), harness.owner, blocked.ID, blocked.Version)
	if err != nil || recovered.Status != TrackPublished || recovered.PublicationID != publisher.active.ID || publisher.publishCalls != 1 {
		t.Fatalf("exact active recovery = %+v err=%v calls=%d", recovered, err, publisher.publishCalls)
	}
}

func TestConcurrentActivationHasOneVersionWinnerAndOneDomainMutation(t *testing.T) {
	fixed := time.Date(2026, 8, 21, 20, 0, 0, 0, time.UTC)
	harness := newConductorHarness(t)
	publisher := &crashWindowPublicationService{now: fixed}
	service, err := NewServiceWithClock(harness.repository, harness.workspaces, ActorSubjectResolver{}, CuratedCompositionValidator{},
		harness.auditor, harness.builder, harness.origins, publisher, func() time.Time { return fixed })
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.Compose(context.Background(), harness.owner, validCompositionRequest())
	if err != nil {
		t.Fatal(err)
	}
	requested, err := service.RequestPublication(context.Background(), harness.owner, created.Track.ID, created.Track.Version, "claim-community")
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		track *Track
		err   error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	for index := 0; index < 2; index++ {
		go func() {
			<-start
			track, err := service.ActivatePublication(context.Background(), harness.owner, requested.ID, requested.Version)
			results <- result{track: track, err: err}
		}()
	}
	close(start)
	first, second := <-results, <-results
	winners, conflicts := 0, 0
	for _, outcome := range []result{first, second} {
		switch {
		case outcome.err == nil && outcome.track != nil && outcome.track.Status == TrackPublished:
			winners++
		case errors.Is(outcome.err, ErrTrackVersionConflict):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent activation outcome track=%+v err=%v", outcome.track, outcome.err)
		}
	}
	if winners != 1 || conflicts != 1 || publisher.publishCalls != 1 {
		t.Fatalf("concurrent activation winners=%d conflicts=%d publishes=%d", winners, conflicts, publisher.publishCalls)
	}
}

func TestPublicationLinkMigrationIsAdditiveExactAndNonUnique(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+filepath.ToSlash(filepath.Join(t.TempDir(), "publication-link-migration.db"))+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	createLegacyPublicationLinkSchema(t, db)
	activatedAt := time.Date(2026, 8, 21, 21, 0, 0, 0, time.UTC)
	contentHash := "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	publication := domains.Publication{
		ID: "publication-shared", WorkspaceID: 42, ClaimID: "claim-community", Origin: "https://community.example", Host: "community.example",
		ContentHash: contentHash, ArtifactID: "artifact-signed-1", ActivatedBy: 7, ActivatedAt: activatedAt, Active: true,
	}
	if _, err := db.Exec(`INSERT INTO workspace_domain_publications
		(id, workspace_id, claim_id, origin, host, content_hash, artifact_id, source_publication_id, activated_by, activated_at, deactivated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, NULL)`, publication.ID, publication.WorkspaceID, publication.ClaimID,
		publication.Origin, publication.Host, publication.ContentHash, publication.ArtifactID, publication.ActivatedBy, publication.ActivatedAt.Unix()); err != nil {
		t.Fatal(err)
	}
	artifactJSON, _ := json.Marshal(ArtifactReference{
		ArtifactID: publication.ArtifactID, ContentHash: publication.ContentHash,
		Manifest: artifacts.Manifest{WorkspaceID: publication.WorkspaceID, ArtifactID: publication.ArtifactID, ContentHash: publication.ContentHash},
	})
	publicationJSON, _ := json.Marshal(publication)
	insertLegacyPublicationTrack(t, db, "track-valid-a", artifactJSON, publicationJSON)
	insertLegacyPublicationTrack(t, db, "track-valid-b", artifactJSON, publicationJSON)
	insertLegacyPublicationTrack(t, db, "track-malformed", artifactJSON, []byte(`{"id":`))
	mismatch := publication
	mismatch.Origin = "https://attacker.example"
	mismatchJSON, _ := json.Marshal(mismatch)
	insertLegacyPublicationTrack(t, db, "track-mismatch", artifactJSON, mismatchJSON)
	insertLegacyPublicationTrack(t, db, "track-no-link-heuristic", artifactJSON, nil)
	if _, err := db.Exec(`UPDATE workspace_domain_publications SET deactivated_at = ? WHERE id = ?`, activatedAt.Add(time.Hour).Unix(), publication.ID); err != nil {
		t.Fatalf("deactivate durable legacy publication: %v", err)
	}

	repository, err := NewRepository(db)
	if err != nil {
		t.Fatalf("NewRepository(legacy) error = %v", err)
	}
	for _, test := range []struct {
		trackID string
		want    string
	}{
		{trackID: "track-valid-a", want: publication.ID},
		{trackID: "track-valid-b", want: publication.ID},
		{trackID: "track-malformed"},
		{trackID: "track-mismatch"},
		{trackID: "track-no-link-heuristic"},
	} {
		var got string
		if err := db.QueryRow(`SELECT publication_id FROM conductor_tracks WHERE id = ?`, test.trackID).Scan(&got); err != nil || got != test.want {
			t.Fatalf("backfill %s = %q err=%v, want %q", test.trackID, got, err, test.want)
		}
	}
	if _, err := db.Exec(`INSERT INTO conductor_tracks
		(id, workspace_id, idempotency_key, request_hash, request_json, status, version, artifact_json, claim_id, publication_json, publication_id, created_by, created_at, updated_at)
		VALUES ('track-direct-malformed', 42, 'direct-malformed', 'hash', '{}', 'PUBLISHED', 2, ?, 'claim-community', '{', ?, 7, 1, 1)`, artifactJSON, publication.ID); err != nil {
		t.Fatalf("non-unique direct linkage insert: %v", err)
	}
	bindings, err := repository.ResolvePublicationTrackBindings(context.Background(), 42, "claim-community", []domains.Publication{publication})
	if err != nil {
		t.Fatalf("ResolvePublicationTrackBindings() error = %v", err)
	}
	if len(bindings[publication.ID]) != 0 {
		t.Fatalf("ambiguous duplicate binding exposed a cardinality oracle: %+v", bindings[publication.ID])
	}
	var unique int
	rows, err := db.Query(`PRAGMA index_list(conductor_tracks)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var sequence, partial int
		var name, origin string
		if err := rows.Scan(&sequence, &name, &unique, &origin, &partial); err != nil {
			t.Fatal(err)
		}
		if name == "idx_conductor_tracks_publication_id" {
			found = true
			if unique != 0 {
				t.Fatal("publication linkage index unexpectedly enforces uniqueness")
			}
		}
	}
	if !found {
		t.Fatal("publication linkage index was not created additively")
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if err := repository.migrate(); err != nil {
		t.Fatalf("idempotent publication linkage migration error = %v", err)
	}
}

func TestPublicationBindingResolverUsesOneSetQueryForMaximumPage(t *testing.T) {
	driverName := fmt.Sprintf("sqlite3_publication_binding_query_count_%d", publicationQueryDriverSequence.Add(1))
	var enabled atomic.Bool
	var queryCount atomic.Int64
	sql.Register(driverName, publicationQueryCountingDriver{enabled: &enabled, queryCount: &queryCount})
	db, err := sql.Open(driverName, "file:"+filepath.ToSlash(filepath.Join(t.TempDir(), "publication-binding-query-count.db")))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	repository, err := NewRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	publications := make([]domains.Publication, 0, domains.MaximumPublicationContextLimit)
	for index := 0; index < domains.MaximumPublicationContextLimit; index++ {
		publication := domains.Publication{
			ID: fmt.Sprintf("publication-query-%02d", index), WorkspaceID: 42, ClaimID: "claim-community",
			Origin: "https://community.example", Host: "community.example",
			ContentHash: fmt.Sprintf("%064x", index+1), ArtifactID: fmt.Sprintf("artifact-query-%02d", index),
			ActivatedBy: 7, ActivatedAt: time.Unix(int64(1_800_000_000+index), 0).UTC(), Active: true,
		}
		artifactJSON, _ := json.Marshal(ArtifactReference{
			ArtifactID: publication.ArtifactID, ContentHash: publication.ContentHash,
			Manifest: artifacts.Manifest{WorkspaceID: publication.WorkspaceID, ArtifactID: publication.ArtifactID, ContentHash: publication.ContentHash},
		})
		publicationJSON, _ := json.Marshal(publication)
		_, err := db.Exec(`INSERT INTO conductor_tracks
			(id, workspace_id, idempotency_key, request_hash, request_json, status, version, artifact_json,
			 claim_id, publication_json, publication_id, created_by, created_at, updated_at)
			VALUES (?, 42, ?, 'hash', '{}', 'PUBLISHED', 2, ?, 'claim-community', ?, ?, 7, 1, 1)`,
			fmt.Sprintf("track-query-%02d", index), fmt.Sprintf("query-%02d", index), artifactJSON, publicationJSON, publication.ID)
		if err != nil {
			t.Fatalf("insert query-count binding %d: %v", index, err)
		}
		publications = append(publications, publication)
	}
	enabled.Store(true)
	bindings, err := repository.ResolvePublicationTrackBindings(context.Background(), 42, "claim-community", publications)
	enabled.Store(false)
	if err != nil {
		t.Fatalf("ResolvePublicationTrackBindings(max page) error = %v", err)
	}
	if queryCount.Load() != 1 {
		t.Fatalf("maximum-page binding QueryContext calls = %d, want one set query", queryCount.Load())
	}
	if len(bindings) != domains.MaximumPublicationContextLimit {
		t.Fatalf("maximum-page bindings = %d, want %d", len(bindings), domains.MaximumPublicationContextLimit)
	}
	for _, publication := range publications {
		if len(bindings[publication.ID]) != 1 {
			t.Fatalf("binding %s candidates = %+v", publication.ID, bindings[publication.ID])
		}
	}
}

var publicationQueryDriverSequence atomic.Int64

type publicationQueryCountingDriver struct {
	enabled    *atomic.Bool
	queryCount *atomic.Int64
}

func (d publicationQueryCountingDriver) Open(name string) (driver.Conn, error) {
	connection, err := (&sqlite3.SQLiteDriver{}).Open(name)
	if err != nil {
		return nil, err
	}
	return &publicationQueryCountingConn{Conn: connection, enabled: d.enabled, queryCount: d.queryCount}, nil
}

type publicationQueryCountingConn struct {
	driver.Conn
	enabled    *atomic.Bool
	queryCount *atomic.Int64
}

func (c *publicationQueryCountingConn) QueryContext(ctx context.Context, query string, arguments []driver.NamedValue) (driver.Rows, error) {
	if c.enabled.Load() && strings.Contains(query, "WITH candidates AS") {
		c.queryCount.Add(1)
	}
	return c.Conn.(driver.QueryerContext).QueryContext(ctx, query, arguments)
}

func (c *publicationQueryCountingConn) ExecContext(ctx context.Context, query string, arguments []driver.NamedValue) (driver.Result, error) {
	return c.Conn.(driver.ExecerContext).ExecContext(ctx, query, arguments)
}

func (c *publicationQueryCountingConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	return c.Conn.(driver.ConnPrepareContext).PrepareContext(ctx, query)
}

func (c *publicationQueryCountingConn) BeginTx(ctx context.Context, options driver.TxOptions) (driver.Tx, error) {
	return c.Conn.(driver.ConnBeginTx).BeginTx(ctx, options)
}

func createLegacyPublicationLinkSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`
		CREATE TABLE workspace_domain_publications (
			id TEXT PRIMARY KEY, workspace_id INTEGER NOT NULL, claim_id TEXT NOT NULL, origin TEXT NOT NULL, host TEXT NOT NULL,
			content_hash TEXT NOT NULL, artifact_id TEXT NOT NULL, source_publication_id TEXT, activated_by INTEGER NOT NULL,
			activated_at INTEGER NOT NULL, deactivated_at INTEGER
		);
		CREATE TABLE conductor_tracks (
			id TEXT PRIMARY KEY, workspace_id INTEGER NOT NULL, idempotency_key TEXT NOT NULL, request_hash TEXT NOT NULL,
			request_json TEXT NOT NULL, build_request_json TEXT NOT NULL DEFAULT '', status TEXT NOT NULL, version INTEGER NOT NULL,
			compliance_json TEXT NOT NULL DEFAULT '', artifact_json TEXT NOT NULL DEFAULT '', preview_json TEXT NOT NULL DEFAULT '',
			claim_id TEXT NOT NULL DEFAULT '', publication_json TEXT NOT NULL DEFAULT '', failure_code TEXT NOT NULL DEFAULT '',
			created_by INTEGER NOT NULL, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL,
			UNIQUE (workspace_id, idempotency_key)
		);`)
	if err != nil {
		t.Fatalf("create legacy publication linkage schema: %v", err)
	}
}

func insertLegacyPublicationTrack(t *testing.T, db *sql.DB, trackID string, artifactJSON, publicationJSON []byte) {
	t.Helper()
	if publicationJSON == nil {
		publicationJSON = []byte{}
	}
	_, err := db.Exec(`INSERT INTO conductor_tracks
		(id, workspace_id, idempotency_key, request_hash, request_json, status, version, artifact_json, claim_id, publication_json, created_by, created_at, updated_at)
		VALUES (?, 42, ?, 'hash', '{}', 'PUBLISHED', 2, ?, 'claim-community', ?, 7, 1, 1)`, trackID, trackID, artifactJSON, publicationJSON)
	if err != nil {
		t.Fatalf("insert legacy track %s: %v", trackID, err)
	}
}
