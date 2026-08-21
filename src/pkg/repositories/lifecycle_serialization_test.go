package repositories

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"taawun/pkg/database"
	"taawun/pkg/models"
)

func TestLifecycleSerializationCreationWinsAcrossHandles(t *testing.T) {
	first, second, sqlDB := newLifecycleRepositoryHandles(t)
	user := seedLifecycleUser(t, first, "create-wins")
	before := readLifecycleUserSnapshot(t, sqlDB, user.ID)
	reached, release := blockLifecycleCreate(t, first)
	workspace := &models.Workspace{Name: "Creation wins", OwnerID: user.ID, Status: models.WorkspaceStatusActive}
	createDone := make(chan error, 1)
	go func() {
		createDone <- NewWorkspaceRepository(first).CreateWithOwnerMembership(context.Background(), workspace, user.SessionVersion)
	}()
	waitLifecycleBarrier(t, reached)

	deleteDone := make(chan error, 1)
	deleteAttempted := make(chan struct{})
	go func() {
		close(deleteAttempted)
		deleteDone <- NewUserRepository(second).DeactivateAndAnonymize(user.ID, "deleted-create-wins", "deleted-create-wins@deleted.invalid", "tombstone", time.Now())
	}()
	<-deleteAttempted
	assertLifecycleOperationBlocked(t, deleteDone)
	close(release)
	if err := <-createDone; err != nil {
		t.Fatalf("create error = %v", err)
	}
	if err := <-deleteDone; !errors.Is(err, ErrOwnedWorkspacesRemaining) {
		t.Fatalf("delete error = %v, want ownership conflict", err)
	} else {
		assertNoRawSQLiteLock(t, err)
	}
	assertLifecycleUserUnchanged(t, sqlDB, user.ID, before)
	assertLifecycleState(t, sqlDB, user.ID, models.StatusActive, 1, 1, 1)
}

func TestLifecycleSerializationDeletionWinsAcrossHandles(t *testing.T) {
	first, second, sqlDB := newLifecycleRepositoryHandles(t)
	user := seedLifecycleUser(t, first, "delete-wins")
	reached, release := blockLifecycleUserUpdate(t, first)
	deleteDone := make(chan error, 1)
	go func() {
		deleteDone <- NewUserRepository(first).DeactivateAndAnonymize(user.ID, "deleted-delete-wins", "deleted-delete-wins@deleted.invalid", "tombstone", time.Now())
	}()
	waitLifecycleBarrier(t, reached)

	workspace := &models.Workspace{Name: "Stale create", OwnerID: user.ID, Status: models.WorkspaceStatusActive}
	createDone := make(chan error, 1)
	createAttempted := make(chan struct{})
	go func() {
		close(createAttempted)
		createDone <- NewWorkspaceRepository(second).CreateWithOwnerMembership(context.Background(), workspace, user.SessionVersion)
	}()
	<-createAttempted
	assertLifecycleOperationBlocked(t, createDone)
	close(release)
	if err := <-deleteDone; err != nil {
		t.Fatalf("delete error = %v", err)
	}
	if err := <-createDone; !errors.Is(err, ErrWorkspaceOwnerInactive) {
		t.Fatalf("stale create error = %v, want inactive owner", err)
	} else {
		assertNoRawSQLiteLock(t, err)
	}
	assertLifecycleState(t, sqlDB, user.ID, models.StatusDeleted, 2, 0, 0)
	if err := NewUserRepository(second).DeactivateAndAnonymize(user.ID, "ignored", "ignored@deleted.invalid", "ignored", time.Now()); err != nil {
		t.Fatalf("idempotent delete error = %v", err)
	}
	assertLifecycleState(t, sqlDB, user.ID, models.StatusDeleted, 2, 0, 0)
}

func TestLifecycleSerializationManyCreatesAgainstDelete(t *testing.T) {
	first, second, sqlDB := newLifecycleRepositoryHandles(t)
	user := seedLifecycleUser(t, first, "many-race")
	const creates = 8
	start := make(chan struct{})
	results := make(chan error, creates+1)
	for index := 0; index < creates; index++ {
		db := first
		if index%2 != 0 {
			db = second
		}
		go func(index int, handle *gorm.DB) {
			<-start
			workspace := &models.Workspace{Name: fmt.Sprintf("Concurrent %d", index), OwnerID: user.ID, Status: models.WorkspaceStatusActive}
			results <- NewWorkspaceRepository(handle).CreateWithOwnerMembership(context.Background(), workspace, user.SessionVersion)
		}(index, db)
	}
	go func() {
		<-start
		results <- NewUserRepository(second).DeactivateAndAnonymize(user.ID, "deleted-many", "deleted-many@deleted.invalid", "tombstone", time.Now())
	}()
	close(start)
	for index := 0; index < creates+1; index++ {
		err := <-results
		if err != nil && !errors.Is(err, ErrWorkspaceOwnerInactive) && !errors.Is(err, ErrOwnedWorkspacesRemaining) {
			assertNoRawSQLiteLock(t, err)
			t.Fatalf("unexpected concurrent lifecycle error = %v", err)
		}
		assertNoRawSQLiteLock(t, err)
	}
	var status string
	var workspaceCount int
	if err := sqlDB.QueryRow(`SELECT status FROM users WHERE id = ?`, user.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM workspaces WHERE owner_id = ?`, user.ID).Scan(&workspaceCount); err != nil {
		t.Fatal(err)
	}
	if status == models.StatusDeleted && workspaceCount != 0 {
		t.Fatalf("deleted user owns %d workspaces", workspaceCount)
	}
	if status == models.StatusActive && workspaceCount == 0 {
		t.Fatal("active race winner has no workspace and deletion did not commit")
	}
	var membershipCount int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM workspace_users WHERE user_id = ?`, user.ID).Scan(&membershipCount); err != nil || membershipCount != workspaceCount {
		t.Fatalf("workspace/member counts = %d/%d, err = %v", workspaceCount, membershipCount, err)
	}
}

func TestLifecycleTransactionsRollbackAndReleaseForFailureAndCancellation(t *testing.T) {
	t.Run("failed owner membership insert", func(t *testing.T) {
		db, _, sqlDB := newLifecycleRepositoryHandles(t)
		user := seedLifecycleUser(t, db, "create-rollback")
		before := readLifecycleUserSnapshot(t, sqlDB, user.ID)
		if _, err := sqlDB.Exec(`CREATE TRIGGER fail_owner_membership BEFORE INSERT ON workspace_users BEGIN SELECT RAISE(ABORT, 'forced owner membership failure'); END`); err != nil {
			t.Fatal(err)
		}
		workspace := &models.Workspace{Name: "Rollback create", OwnerID: user.ID, Status: models.WorkspaceStatusActive}
		if err := NewWorkspaceRepository(db).CreateWithOwnerMembership(context.Background(), workspace, user.SessionVersion); err == nil {
			t.Fatal("create unexpectedly succeeded")
		}
		assertLifecycleUserUnchanged(t, sqlDB, user.ID, before)
		assertLifecycleState(t, sqlDB, user.ID, models.StatusActive, 1, 0, 0)
		if _, err := sqlDB.Exec(`DROP TRIGGER fail_owner_membership`); err != nil {
			t.Fatal(err)
		}
		workspace.ID = 0
		if err := NewWorkspaceRepository(db).CreateWithOwnerMembership(context.Background(), workspace, user.SessionVersion); err != nil {
			t.Fatalf("retry create error = %v", err)
		}
		assertLifecycleState(t, sqlDB, user.ID, models.StatusActive, 1, 1, 1)
	})

	t.Run("commit failure has exact readback and retry", func(t *testing.T) {
		db, second, sqlDB := newLifecycleRepositoryHandles(t)
		user := seedLifecycleUser(t, db, "commit-rollback")
		before := readLifecycleUserSnapshot(t, sqlDB, user.ID)
		for _, statement := range []string{
			`CREATE TABLE lifecycle_commit_parents (id INTEGER PRIMARY KEY)`,
			`CREATE TABLE lifecycle_commit_probe (id INTEGER PRIMARY KEY, parent_id INTEGER NOT NULL, FOREIGN KEY(parent_id) REFERENCES lifecycle_commit_parents(id) DEFERRABLE INITIALLY DEFERRED)`,
			`CREATE TRIGGER fail_workspace_commit AFTER INSERT ON workspace_users BEGIN INSERT INTO lifecycle_commit_probe (id, parent_id) VALUES (1, 999); END`,
		} {
			if _, err := sqlDB.Exec(statement); err != nil {
				t.Fatal(err)
			}
		}
		mutationSQLCompleted := make(chan struct{})
		var mutationOnce sync.Once
		if err := db.Callback().Create().After("gorm:create").Register("test:observe_deferred_commit_failure", func(tx *gorm.DB) {
			if lifecycleStatementTable(tx) == "workspace_users" && tx.Error == nil {
				mutationOnce.Do(func() { close(mutationSQLCompleted) })
			}
		}); err != nil {
			t.Fatal(err)
		}

		workspace := &models.Workspace{Name: "Commit rollback", OwnerID: user.ID, Status: models.WorkspaceStatusActive}
		createErr := NewWorkspaceRepository(db).CreateWithOwnerMembership(context.Background(), workspace, user.SessionVersion)
		select {
		case <-mutationSQLCompleted:
		default:
			t.Fatal("mutation SQL did not complete before deferred constraint failure")
		}
		if createErr == nil {
			t.Fatal("create unexpectedly survived deferred commit failure")
		}
		secondSQL, err := database.SQLDB(second)
		if err != nil {
			t.Fatal(err)
		}
		assertLifecycleUserUnchanged(t, secondSQL, user.ID, before)
		assertLifecycleState(t, secondSQL, user.ID, models.StatusActive, 1, 0, 0)
		var probeCount int
		if err := secondSQL.QueryRow(`SELECT COUNT(*) FROM lifecycle_commit_probe`).Scan(&probeCount); err != nil || probeCount != 0 {
			t.Fatalf("commit probe rows = %d, err = %v", probeCount, err)
		}
		if _, err := secondSQL.Exec(`DROP TRIGGER fail_workspace_commit`); err != nil {
			t.Fatalf("commit failure did not release lifecycle transaction: %v", err)
		}
		workspace.ID = 0
		if err := NewWorkspaceRepository(second).CreateWithOwnerMembership(context.Background(), workspace, user.SessionVersion); err != nil {
			t.Fatalf("post-commit-failure create retry error = %v", err)
		}
		assertLifecycleState(t, secondSQL, user.ID, models.StatusActive, 1, 1, 1)
	})

	t.Run("failed tombstone rotation", func(t *testing.T) {
		db, _, sqlDB := newLifecycleRepositoryHandles(t)
		user := seedLifecycleUser(t, db, "delete-rollback")
		seedLifecycleDependents(t, db, sqlDB, user)
		before := readLifecycleUserSnapshot(t, sqlDB, user.ID)
		if _, err := sqlDB.Exec(`CREATE TRIGGER fail_tombstone BEFORE DELETE ON workspace_users WHEN OLD.user_id = ` + strconv.Itoa(user.ID) + ` BEGIN SELECT RAISE(ABORT, 'forced tombstone failure'); END`); err != nil {
			t.Fatal(err)
		}
		repo := NewUserRepository(db)
		if err := repo.DeactivateAndAnonymize(user.ID, "deleted-rollback", "deleted-rollback@deleted.invalid", "tombstone", time.Now()); err == nil {
			t.Fatal("delete unexpectedly succeeded")
		}
		assertLifecycleUserUnchanged(t, sqlDB, user.ID, before)
		assertLifecycleDependents(t, sqlDB, user.ID, false)
		if _, err := sqlDB.Exec(`DROP TRIGGER fail_tombstone`); err != nil {
			t.Fatal(err)
		}
		if err := repo.DeactivateAndAnonymize(user.ID, "deleted-rollback", "deleted-rollback@deleted.invalid", "tombstone", time.Now()); err != nil {
			t.Fatalf("retry delete error = %v", err)
		}
		assertLifecycleDependents(t, sqlDB, user.ID, true)
	})

	t.Run("failed late OAuth revocation", func(t *testing.T) {
		db, _, sqlDB := newLifecycleRepositoryHandles(t)
		user := seedLifecycleUser(t, db, "oauth-rollback")
		seedLifecycleDependents(t, db, sqlDB, user)
		before := readLifecycleUserSnapshot(t, sqlDB, user.ID)
		if _, err := sqlDB.Exec(`CREATE TRIGGER fail_oauth_revocation BEFORE UPDATE ON oauth_refresh_tokens BEGIN SELECT RAISE(ABORT, 'forced OAuth revocation failure'); END`); err != nil {
			t.Fatal(err)
		}
		repo := NewUserRepository(db)
		if err := repo.DeactivateAndAnonymize(user.ID, "deleted-oauth", "deleted-oauth@deleted.invalid", "tombstone", time.Now()); err == nil {
			t.Fatal("delete unexpectedly succeeded")
		}
		assertLifecycleUserUnchanged(t, sqlDB, user.ID, before)
		assertLifecycleDependents(t, sqlDB, user.ID, false)
		if _, err := sqlDB.Exec(`DROP TRIGGER fail_oauth_revocation`); err != nil {
			t.Fatal(err)
		}
		if err := repo.DeactivateAndAnonymize(user.ID, "deleted-oauth", "deleted-oauth@deleted.invalid", "tombstone", time.Now()); err != nil {
			t.Fatalf("retry delete error = %v", err)
		}
		assertLifecycleDependents(t, sqlDB, user.ID, true)
	})

	t.Run("context deadline rolls back create", func(t *testing.T) {
		db, _, sqlDB := newLifecycleRepositoryHandles(t)
		user := seedLifecycleUser(t, db, "cancel-create")
		before := readLifecycleUserSnapshot(t, sqlDB, user.ID)
		reached, _ := cancelLifecycleCreateAtBarrier(t, db)
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
		defer cancel()
		workspace := &models.Workspace{Name: "Cancelled", OwnerID: user.ID, Status: models.WorkspaceStatusActive}
		done := make(chan error, 1)
		go func() {
			done <- NewWorkspaceRepository(db).CreateWithOwnerMembership(ctx, workspace, user.SessionVersion)
		}()
		waitLifecycleBarrier(t, reached)
		if err := <-done; !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("cancelled create error = %v", err)
		}
		assertLifecycleUserUnchanged(t, sqlDB, user.ID, before)
		assertLifecycleState(t, sqlDB, user.ID, models.StatusActive, 1, 0, 0)
		workspace.ID = 0
		if err := NewWorkspaceRepository(db).CreateWithOwnerMembership(context.Background(), workspace, user.SessionVersion); err != nil {
			t.Fatalf("post-cancel create retry error = %v", err)
		}
	})

	t.Run("context cancellation rolls back delete", func(t *testing.T) {
		db, _, sqlDB := newLifecycleRepositoryHandles(t)
		user := seedLifecycleUser(t, db, "cancel-delete")
		before := readLifecycleUserSnapshot(t, sqlDB, user.ID)
		reached := cancelLifecycleUpdateAtBarrier(t, db)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			done <- NewUserRepository(db).DeactivateAndAnonymizeContext(ctx, user.ID, "deleted-cancel", "deleted-cancel@deleted.invalid", "tombstone", time.Now())
		}()
		waitLifecycleBarrier(t, reached)
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled delete error = %v", err)
		}
		assertLifecycleUserUnchanged(t, sqlDB, user.ID, before)
		assertLifecycleState(t, sqlDB, user.ID, models.StatusActive, 1, 0, 0)
		if err := NewUserRepository(db).DeactivateAndAnonymize(user.ID, "deleted-cancel", "deleted-cancel@deleted.invalid", "tombstone", time.Now()); err != nil {
			t.Fatalf("post-cancel delete retry error = %v", err)
		}
	})
}

func TestLifecycleLockTimeoutHasCertainReadbackAndRetry(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		first, second, sqlDB := newLifecycleRepositoryHandles(t)
		user := seedLifecycleUser(t, first, "locked-create")
		before := readLifecycleUserSnapshot(t, sqlDB, user.ID)
		holder, err := sqlDB.BeginTx(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
		workspace := &models.Workspace{Name: "Locked create", OwnerID: user.ID, Status: models.WorkspaceStatusActive}
		err = NewWorkspaceRepository(second).CreateWithOwnerMembership(ctx, workspace, user.SessionVersion)
		cancel()
		if err == nil || (!errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, ErrLifecycleUnavailable)) {
			t.Fatalf("locked create error = %v", err)
		}
		assertNoRawSQLiteLock(t, err)
		assertLifecycleUserUnchanged(t, sqlDB, user.ID, before)
		assertLifecycleState(t, sqlDB, user.ID, models.StatusActive, 1, 0, 0)
		if rollbackErr := holder.Rollback(); rollbackErr != nil {
			t.Fatal(rollbackErr)
		}
		workspace.ID = 0
		if err := NewWorkspaceRepository(second).CreateWithOwnerMembership(context.Background(), workspace, user.SessionVersion); err != nil {
			t.Fatalf("locked create retry error = %v", err)
		}
		assertLifecycleState(t, sqlDB, user.ID, models.StatusActive, 1, 1, 1)
	})

	t.Run("delete", func(t *testing.T) {
		first, second, sqlDB := newLifecycleRepositoryHandles(t)
		user := seedLifecycleUser(t, first, "locked-delete")
		before := readLifecycleUserSnapshot(t, sqlDB, user.ID)
		holder, err := sqlDB.BeginTx(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
		err = NewUserRepository(second).DeactivateAndAnonymizeContext(ctx, user.ID, "deleted-locked", "deleted-locked@deleted.invalid", "tombstone", time.Now())
		cancel()
		if err == nil || (!errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, ErrLifecycleUnavailable)) {
			t.Fatalf("locked delete error = %v", err)
		}
		assertNoRawSQLiteLock(t, err)
		assertLifecycleUserUnchanged(t, sqlDB, user.ID, before)
		assertLifecycleState(t, sqlDB, user.ID, models.StatusActive, 1, 0, 0)
		if rollbackErr := holder.Rollback(); rollbackErr != nil {
			t.Fatal(rollbackErr)
		}
		if err := NewUserRepository(second).DeactivateAndAnonymize(user.ID, "deleted-locked", "deleted-locked@deleted.invalid", "tombstone", time.Now()); err != nil {
			t.Fatalf("locked delete retry error = %v", err)
		}
		assertLifecycleState(t, sqlDB, user.ID, models.StatusDeleted, 2, 0, 0)
	})
}

func TestLifecycleConcurrentSameOperationSemantics(t *testing.T) {
	t.Run("stale session create is rejected", func(t *testing.T) {
		db, _, sqlDB := newLifecycleRepositoryHandles(t)
		user := seedLifecycleUser(t, db, "stale-session")
		before := readLifecycleUserSnapshot(t, sqlDB, user.ID)
		workspace := &models.Workspace{Name: "Stale session", OwnerID: user.ID, Status: models.WorkspaceStatusActive}
		if err := NewWorkspaceRepository(db).CreateWithOwnerMembership(context.Background(), workspace, user.SessionVersion+1); !errors.Is(err, ErrWorkspaceOwnerInactive) {
			t.Fatalf("stale-session create error = %v", err)
		}
		assertLifecycleUserUnchanged(t, sqlDB, user.ID, before)
		assertLifecycleState(t, sqlDB, user.ID, models.StatusActive, 1, 0, 0)
	})

	t.Run("delete delete is idempotent", func(t *testing.T) {
		first, second, sqlDB := newLifecycleRepositoryHandles(t)
		user := seedLifecycleUser(t, first, "double-delete")
		start := make(chan struct{})
		results := make(chan error, 2)
		for index, db := range []*gorm.DB{first, second} {
			go func(index int, handle *gorm.DB) {
				<-start
				results <- NewUserRepository(handle).DeactivateAndAnonymize(user.ID, fmt.Sprintf("deleted-%d", index), fmt.Sprintf("deleted-%d@deleted.invalid", index), "tombstone", time.Now())
			}(index, db)
		}
		close(start)
		for index := 0; index < 2; index++ {
			if err := <-results; err != nil {
				t.Fatalf("concurrent delete error = %v", err)
			}
		}
		assertLifecycleState(t, sqlDB, user.ID, models.StatusDeleted, 2, 0, 0)
	})

	t.Run("create create remains additive", func(t *testing.T) {
		first, second, sqlDB := newLifecycleRepositoryHandles(t)
		user := seedLifecycleUser(t, first, "double-create")
		start := make(chan struct{})
		results := make(chan error, 2)
		for index, db := range []*gorm.DB{first, second} {
			go func(index int, handle *gorm.DB) {
				<-start
				workspace := &models.Workspace{Name: fmt.Sprintf("Created %d", index), OwnerID: user.ID, Status: models.WorkspaceStatusActive}
				results <- NewWorkspaceRepository(handle).CreateWithOwnerMembership(context.Background(), workspace, user.SessionVersion)
			}(index, db)
		}
		close(start)
		for index := 0; index < 2; index++ {
			if err := <-results; err != nil {
				t.Fatalf("concurrent create error = %v", err)
			}
		}
		assertLifecycleState(t, sqlDB, user.ID, models.StatusActive, 1, 2, 2)
	})
}

func TestLifecycleSerializationAcrossProcess(t *testing.T) {
	db, _, sqlDB, path := newLifecycleRepositoryHandlesWithPath(t)
	user := seedLifecycleUser(t, db, "process-create-wins")
	before := readLifecycleUserSnapshot(t, sqlDB, user.ID)
	reached, release := blockLifecycleCreate(t, db)
	workspace := &models.Workspace{Name: "Process creation", OwnerID: user.ID, Status: models.WorkspaceStatusActive}
	createDone := make(chan error, 1)
	go func() {
		createDone <- NewWorkspaceRepository(db).CreateWithOwnerMembership(context.Background(), workspace, user.SessionVersion)
	}()
	waitLifecycleBarrier(t, reached)

	command := exec.Command(os.Args[0], "-test.run=^TestLifecycleSubprocess$")
	command.Env = append(os.Environ(),
		"TAWUN_LIFECYCLE_CHILD=delete",
		"TAWUN_LIFECYCLE_DB="+path,
		"TAWUN_LIFECYCLE_USER="+strconv.Itoa(user.ID),
	)
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(stdout)
	line, err := reader.ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "READY" {
		t.Fatalf("child readiness = %q, %v, stderr=%s", line, err, stderr.String())
	}
	processDone := make(chan error, 1)
	go func() { processDone <- command.Wait() }()
	assertLifecycleOperationBlocked(t, processDone)
	close(release)
	if err := <-createDone; err != nil {
		t.Fatal(err)
	}
	if err := <-processDone; err != nil {
		t.Fatalf("child lifecycle process failed: %v, stderr=%s", err, stderr.String())
	}
	assertLifecycleUserUnchanged(t, sqlDB, user.ID, before)
	assertLifecycleState(t, sqlDB, user.ID, models.StatusActive, 1, 1, 1)
}

func TestLifecycleSubprocess(t *testing.T) {
	if os.Getenv("TAWUN_LIFECYCLE_CHILD") == "" {
		t.Skip("subprocess helper")
	}
	path := os.Getenv("TAWUN_LIFECYCLE_DB")
	userID, err := strconv.Atoi(os.Getenv("TAWUN_LIFECYCLE_USER"))
	if err != nil {
		t.Fatal(err)
	}
	db, sqlDB := openLifecycleRepositoryDB(t, path)
	defer sqlDB.Close()
	fmt.Println("READY")
	err = NewUserRepository(db).DeactivateAndAnonymize(userID, "deleted-child", "deleted-child@deleted.invalid", "tombstone", time.Now())
	if !errors.Is(err, ErrOwnedWorkspacesRemaining) {
		t.Fatalf("child delete error = %v", err)
	}
}

func newLifecycleRepositoryHandles(t *testing.T) (*gorm.DB, *gorm.DB, *sql.DB) {
	first, second, sqlDB, _ := newLifecycleRepositoryHandlesWithPath(t)
	return first, second, sqlDB
}

func newLifecycleRepositoryHandlesWithPath(t *testing.T) (*gorm.DB, *gorm.DB, *sql.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "lifecycle.db")
	first, firstSQL := openLifecycleRepositoryDB(t, path)
	for _, statement := range lifecycleExtraSchemaStatements() {
		if _, err := firstSQL.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	second, secondSQL := openLifecycleRepositoryDB(t, path)
	t.Cleanup(func() {
		_ = secondSQL.Close()
		_ = firstSQL.Close()
	})
	return first, second, firstSQL, path
}

func openLifecycleRepositoryDB(t *testing.T, path string) (*gorm.DB, *sql.DB) {
	t.Helper()
	t.Setenv("APP_DB_PATH", path)
	t.Setenv("TAWUN_BOOTSTRAP_ADMIN_USERNAME", "")
	t.Setenv("TAWUN_BOOTSTRAP_ADMIN_EMAIL", "")
	t.Setenv("TAWUN_BOOTSTRAP_ADMIN_PASSWORD", "")
	db, err := database.InitDB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := database.SQLDB(db)
	if err != nil {
		t.Fatal(err)
	}
	return db, sqlDB
}

func lifecycleExtraSchemaStatements() []string {
	return []string{
		`CREATE TABLE oauth_sessions (session_hash TEXT PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id), revoked_at INTEGER)`,
		`CREATE TABLE oauth_consents (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id), revoked_at INTEGER)`,
		`CREATE TABLE oauth_authorization_codes (code_hash TEXT PRIMARY KEY, user_id INTEGER NOT NULL, consumed_at INTEGER)`,
		`CREATE TABLE oauth_access_tokens (token_hash TEXT PRIMARY KEY, user_id INTEGER NOT NULL, revoked_at INTEGER)`,
		`CREATE TABLE oauth_refresh_tokens (token_hash TEXT PRIMARY KEY, user_id INTEGER NOT NULL, revoked_at INTEGER)`,
		`CREATE TABLE lifecycle_retained_refs (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id), detail TEXT NOT NULL)`,
	}
}

type lifecycleUserSnapshot struct {
	Username       string
	Email          string
	Password       string
	Role           string
	Status         string
	SessionVersion int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func readLifecycleUserSnapshot(t *testing.T, db *sql.DB, userID int) lifecycleUserSnapshot {
	t.Helper()
	var snapshot lifecycleUserSnapshot
	if err := db.QueryRow(`SELECT username, email, password, role, status, session_version, created_at, updated_at FROM users WHERE id = ?`, userID).Scan(
		&snapshot.Username, &snapshot.Email, &snapshot.Password, &snapshot.Role, &snapshot.Status,
		&snapshot.SessionVersion, &snapshot.CreatedAt, &snapshot.UpdatedAt,
	); err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func assertLifecycleUserUnchanged(t *testing.T, db *sql.DB, userID int, before lifecycleUserSnapshot) {
	t.Helper()
	after := readLifecycleUserSnapshot(t, db, userID)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("user changed across rejected lifecycle operation: before=%+v after=%+v", before, after)
	}
}

func seedLifecycleUser(t *testing.T, db *gorm.DB, suffix string) *models.User {
	t.Helper()
	user := &models.User{Username: "user-" + suffix, Email: suffix + "@example.com", Password: "unchanged", Role: models.RoleUser, Status: models.StatusActive, SessionVersion: 1}
	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	return user
}

func seedLifecycleDependents(t *testing.T, db *gorm.DB, sqlDB *sql.DB, user *models.User) {
	t.Helper()
	owner := seedLifecycleUser(t, db, "dependency-owner")
	workspace := &models.Workspace{Name: "Dependency workspace", OwnerID: owner.ID, Status: models.WorkspaceStatusActive}
	if err := NewWorkspaceRepository(db).CreateWithOwnerMembership(context.Background(), workspace, owner.SessionVersion); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`INSERT INTO workspace_users (workspace_id, user_id, role) VALUES (?, ?, 'member')`,
		`INSERT INTO notifications (id, user_id, type, title, message, read) VALUES (1, ?, 'system', 'Private', 'Remove', 0)`,
		`INSERT INTO oauth_sessions (session_hash, user_id) VALUES ('session', ?)`,
		`INSERT INTO oauth_consents (id, user_id) VALUES (1, ?)`,
		`INSERT INTO oauth_authorization_codes (code_hash, user_id) VALUES ('code', ?)`,
		`INSERT INTO oauth_access_tokens (token_hash, user_id) VALUES ('access', ?)`,
		`INSERT INTO oauth_refresh_tokens (token_hash, user_id) VALUES ('refresh', ?)`,
		`INSERT INTO lifecycle_retained_refs (id, user_id, detail) VALUES (1, ?, 'retain')`,
	} {
		args := []any{user.ID}
		if strings.Contains(statement, "workspace_users") {
			args = []any{workspace.ID, user.ID}
		}
		if _, err := sqlDB.Exec(statement, args...); err != nil {
			t.Fatal(err)
		}
	}
}

func assertLifecycleDependents(t *testing.T, db *sql.DB, userID int, deleted bool) {
	t.Helper()
	wantStatus, wantMutableRows := models.StatusActive, 1
	wantVersion := int64(1)
	if deleted {
		wantStatus, wantVersion, wantMutableRows = models.StatusDeleted, 2, 0
	}
	assertLifecycleState(t, db, userID, wantStatus, wantVersion, 0, wantMutableRows)
	for _, table := range []string{"workspace_users", "notifications"} {
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE user_id = ?`, userID).Scan(&count); err != nil || count != wantMutableRows {
			t.Fatalf("%s count = %d, err = %v", table, count, err)
		}
	}
	for _, table := range []string{"oauth_sessions", "oauth_consents", "oauth_access_tokens", "oauth_refresh_tokens"} {
		var value any
		if err := db.QueryRow(`SELECT revoked_at FROM `+table+` WHERE user_id = ?`, userID).Scan(&value); err != nil {
			t.Fatal(err)
		}
		if deleted != (value != nil) {
			t.Fatalf("%s revoked_at = %v, deleted=%v", table, value, deleted)
		}
	}
	var consumed any
	if err := db.QueryRow(`SELECT consumed_at FROM oauth_authorization_codes WHERE user_id = ?`, userID).Scan(&consumed); err != nil {
		t.Fatal(err)
	}
	if deleted != (consumed != nil) {
		t.Fatalf("authorization code consumed_at = %v, deleted=%v", consumed, deleted)
	}
	var retained int
	if err := db.QueryRow(`SELECT COUNT(*) FROM lifecycle_retained_refs WHERE user_id = ?`, userID).Scan(&retained); err != nil || retained != 1 {
		t.Fatalf("retained refs = %d, err = %v", retained, err)
	}
}

func assertLifecycleState(t *testing.T, db *sql.DB, userID int, status string, version int64, workspaces, memberships int) {
	t.Helper()
	var actualStatus string
	var actualVersion int64
	if err := db.QueryRow(`SELECT status, session_version FROM users WHERE id = ?`, userID).Scan(&actualStatus, &actualVersion); err != nil {
		t.Fatal(err)
	}
	if actualStatus != status || actualVersion != version {
		t.Fatalf("user lifecycle = %s/%d, want %s/%d", actualStatus, actualVersion, status, version)
	}
	var workspaceCount, membershipCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM workspaces WHERE owner_id = ?`, userID).Scan(&workspaceCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM workspace_users WHERE user_id = ?`, userID).Scan(&membershipCount); err != nil {
		t.Fatal(err)
	}
	if workspaceCount != workspaces || membershipCount != memberships {
		t.Fatalf("workspace/member counts = %d/%d, want %d/%d", workspaceCount, membershipCount, workspaces, memberships)
	}
}

func blockLifecycleCreate(t *testing.T, db *gorm.DB) (<-chan struct{}, chan<- struct{}) {
	t.Helper()
	reached := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	if err := db.Callback().Create().Before("gorm:create").Register("test:block_lifecycle_create", func(tx *gorm.DB) {
		if lifecycleStatementTable(tx) == "workspaces" {
			once.Do(func() {
				close(reached)
				select {
				case <-release:
				case <-tx.Statement.Context.Done():
					tx.AddError(tx.Statement.Context.Err())
				}
			})
		}
	}); err != nil {
		t.Fatal(err)
	}
	return reached, release
}

func blockLifecycleUserUpdate(t *testing.T, db *gorm.DB) (<-chan struct{}, chan<- struct{}) {
	t.Helper()
	reached := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	if err := db.Callback().Update().Before("gorm:update").Register("test:block_lifecycle_update", func(tx *gorm.DB) {
		if lifecycleStatementTable(tx) == "users" {
			once.Do(func() {
				close(reached)
				select {
				case <-release:
				case <-tx.Statement.Context.Done():
					tx.AddError(tx.Statement.Context.Err())
				}
			})
		}
	}); err != nil {
		t.Fatal(err)
	}
	return reached, release
}

func cancelLifecycleCreateAtBarrier(t *testing.T, db *gorm.DB) (<-chan struct{}, chan<- struct{}) {
	return blockLifecycleCreate(t, db)
}

func cancelLifecycleUpdateAtBarrier(t *testing.T, db *gorm.DB) <-chan struct{} {
	reached, _ := blockLifecycleUserUpdate(t, db)
	return reached
}

func lifecycleStatementTable(tx *gorm.DB) string {
	if tx.Statement.Schema != nil {
		return tx.Statement.Schema.Table
	}
	return tx.Statement.Table
}

func waitLifecycleBarrier(t *testing.T, reached <-chan struct{}) {
	t.Helper()
	select {
	case <-reached:
	case <-time.After(3 * time.Second):
		t.Fatal("lifecycle transaction did not reach barrier")
	}
}

func assertLifecycleOperationBlocked(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		t.Fatalf("losing lifecycle operation completed before winner released BEGIN IMMEDIATE: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
}

func assertNoRawSQLiteLock(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "database is locked") || strings.Contains(message, "database is busy") || strings.Contains(message, "sqlite_busy") || strings.Contains(message, "sqlite_locked") {
		t.Fatalf("raw SQLite lock error leaked: %v", err)
	}
}
