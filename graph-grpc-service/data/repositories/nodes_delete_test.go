package repositories

import (
	"database/sql"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// The destructive paths. These pin the PREDICATES, because a wrong one here
// does not fail — it deletes the wrong rows and reports success.
//
// sqlmock rather than a database: what matters is which columns each statement
// narrows on and which tables it reaches, and those are facts about the SQL.
//
// NOTE ON SCOPE: whether deleting a person should also remove the posts they
// made inside groups is an OPEN decision (see docs/plans — the Danger zone copy
// currently promises more than the purge delivers). These tests pin the
// behaviour that exists today: purge is scoped by owner_id, so content posted
// INTO a group stays. Changing that is a deliberate act that updates these
// tests, rather than something discovered later from a support ticket.

func newNodesTestDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, *[]string) {
	t.Helper()

	var seen []string
	db, mock, err := sqlmock.New(
		sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(
			func(expected, actual string) error {
				seen = append(seen, actual)
				return nil
			},
		)),
	)
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	return db, mock, &seen
}

func newRepo(db *sql.DB) *nodesRepo {
	return &nodesRepo{db: db, dbTimeout: 3 * time.Second}
}

// ---------------------------------------------------------------------
// Soft delete
// ---------------------------------------------------------------------

// The `deleted_at IS NULL` guard is what stops a repeated delete from pushing
// the purge date further out. Without it, somebody pressing delete twice — or
// a retried request — silently restarts the 30-day clock, and the sweeper
// never reaches the account.
func TestSoftDeleteWillNotExtendAnExistingGracePeriod(t *testing.T) {
	db, mock, seen := newNodesTestDB(t)
	repo := newRepo(db)

	mock.ExpectExec("update").WillReturnResult(sqlmock.NewResult(0, 1))

	if _, err := repo.SoftDeleteActor(42); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	stmt := strings.ToLower((*seen)[0])
	if !strings.Contains(stmt, "deleted_at is null") {
		t.Errorf("the guard is missing; a second delete would restart the grace "+
			"period and the sweeper would never reach the account:\n%s", stmt)
	}
	// It must also clear enabled, which is what every login path checks.
	if !strings.Contains(stmt, "enabled = 0") {
		t.Errorf("deleting did not disable the account, so it can still sign in:\n%s", stmt)
	}
	if !strings.Contains(stmt, "where id = ?") {
		t.Errorf("no id predicate — this would delete every actor:\n%s", stmt)
	}
}

// The boolean is how a caller tells "deleted" from "it was already on its way
// out", and the restore path uses the same signal to tell "restored" from
// "the sweeper got there first".
func TestSoftDeleteAndRestoreReportWhetherARowChanged(t *testing.T) {
	for _, tc := range []struct {
		name     string
		affected int64
		want     bool
	}{
		{"changed a row", 1, true},
		{"nothing to change", 0, false},
	} {
		t.Run("delete/"+tc.name, func(t *testing.T) {
			db, mock, _ := newNodesTestDB(t)
			mock.ExpectExec("update").WillReturnResult(sqlmock.NewResult(0, tc.affected))

			got, err := newRepo(db).SoftDeleteActor(42)
			if err != nil {
				t.Fatalf("soft delete: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})

		t.Run("restore/"+tc.name, func(t *testing.T) {
			db, mock, _ := newNodesTestDB(t)
			mock.ExpectExec("update").WillReturnResult(sqlmock.NewResult(0, tc.affected))

			got, err := newRepo(db).RestoreActor(42)
			if err != nil {
				t.Fatalf("restore: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// An id of zero or less would match nothing useful and, in a statement without
// a proper predicate, everything. Refuse before reaching the database.
func TestDestructiveMethodsRefuseANonPositiveID(t *testing.T) {
	for _, id := range []int64{0, -1} {
		db, _, seen := newNodesTestDB(t)
		repo := newRepo(db)

		if ok, _ := repo.SoftDeleteActor(id); ok {
			t.Errorf("SoftDeleteActor(%d) reported success", id)
		}
		if ok, _ := repo.PurgeActorContent(id); ok {
			t.Errorf("PurgeActorContent(%d) reported success", id)
		}
		if len(*seen) != 0 {
			t.Errorf("id %d still reached the database: %v", id, *seen)
		}
	}
}

// ---------------------------------------------------------------------
// Purge
// ---------------------------------------------------------------------

// The purge must not take the actor's own node with the content. That row is
// the tombstone: it reserves the handle so nobody else can register it, which
// is what stops a recycled handle inheriting the previous person's followers
// on every remote instance that cached them.
func TestPurgeDeletesContentButNotTheActor(t *testing.T) {
	db, mock, seen := newNodesTestDB(t)
	repo := newRepo(db)

	mock.ExpectBegin()
	mock.ExpectExec("delete").WillReturnResult(sqlmock.NewResult(0, 5)) // edges
	mock.ExpectExec("delete").WillReturnResult(sqlmock.NewResult(0, 3)) // content nodes
	mock.ExpectCommit()

	if _, err := repo.PurgeActorContent(42); err != nil {
		t.Fatalf("purge: %v", err)
	}

	joined := strings.ToLower(strings.Join(*seen, "\n---\n"))

	// Content is found by ownership and descent, never by "everything".
	if !strings.Contains(joined, "where owner_id = ?") {
		t.Errorf("content is not scoped by owner:\n%s", joined)
	}
	if !strings.Contains(joined, "recursive") {
		t.Errorf("descendants are not walked, so comments on a purged post would "+
			"be orphaned rather than removed:\n%s", joined)
	}

	// The actor's own row must not appear in the node deletion.
	nodeDelete := ""
	for _, q := range *seen {
		if strings.Contains(strings.ToLower(q), "delete n") {
			nodeDelete = strings.ToLower(q)
		}
	}
	if nodeDelete == "" {
		t.Fatal("no node deletion statement was issued")
	}

	// The EDGE delete deliberately adds the actor to its target set — its
	// follows, admin edges and votes go with it. The NODE delete must not:
	// that extra CTE is precisely the difference between purging somebody's
	// content and erasing their tombstone.
	if strings.Contains(nodeDelete, "targets") {
		t.Errorf("the node deletion includes the actor's own row, which frees "+
			"the handle for somebody else to register:\n%s", nodeDelete)
	}
	if !strings.Contains(nodeDelete, "join content") {
		t.Errorf("the node deletion is not scoped to the owned-content set:\n%s", nodeDelete)
	}
}

// Edges go before nodes. Once a node is gone there is nothing left to find its
// edges by, and this schema has no foreign keys to clean up after us.
func TestPurgeDeletesEdgesBeforeNodes(t *testing.T) {
	db, mock, seen := newNodesTestDB(t)
	repo := newRepo(db)

	mock.ExpectBegin()
	mock.ExpectExec("delete").WillReturnResult(sqlmock.NewResult(0, 5))
	mock.ExpectExec("delete").WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectCommit()

	if _, err := repo.PurgeActorContent(42); err != nil {
		t.Fatalf("purge: %v", err)
	}

	if len(*seen) < 2 {
		t.Fatalf("expected at least two statements, got %d", len(*seen))
	}
	if !strings.Contains(strings.ToLower((*seen)[0]), "from edges") {
		t.Errorf("edges were not deleted first; their nodes would be gone:\n%s", (*seen)[0])
	}
}

// The whole purge is one transaction: a half-purged account has content
// without edges, which no read path expects.
func TestPurgeRollsBackOnFailure(t *testing.T) {
	db, mock, _ := newNodesTestDB(t)
	repo := newRepo(db)

	mock.ExpectBegin()
	mock.ExpectExec("delete").WillReturnResult(sqlmock.NewResult(0, 5))
	mock.ExpectExec("delete").WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	if _, err := repo.PurgeActorContent(42); err == nil {
		t.Error("a failed purge reported success")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("the transaction was not rolled back: %v", err)
	}
}

// ---------------------------------------------------------------------
// Tombstone
// ---------------------------------------------------------------------

// The tombstone keeps id, alias, type and deleted_at, and scrubs everything
// else. Keeping the alias is the entire point — it reserves the handle.
func TestTombstoneScrubsProfileDataButKeepsTheHandle(t *testing.T) {
	db, mock, seen := newNodesTestDB(t)
	repo := newRepo(db)

	mock.ExpectExec("update").WillReturnResult(sqlmock.NewResult(0, 1))

	if _, err := repo.TombstoneActor(42); err != nil {
		t.Fatalf("tombstone: %v", err)
	}

	stmt := strings.ToLower((*seen)[0])
	setClause := stmt
	if i := strings.Index(stmt, "where"); i >= 0 {
		setClause = stmt[:i]
	}

	// Personal data that must not survive a purge.
	for _, column := range []string{"name", "body", "meta", "website_url", "geo_city", "filename"} {
		if !regexp.MustCompile(`\b` + column + `\s*=`).MatchString(setClause) {
			t.Errorf("%s is not scrubbed; it survives the purge on a tombstone row:\n%s",
				column, setClause)
		}
	}

	// And the handle must NOT be cleared.
	if regexp.MustCompile(`\balias\s*=`).MatchString(setClause) {
		t.Errorf("the alias is scrubbed, which frees the handle for somebody else "+
			"to register:\n%s", setClause)
	}
}
