package repositories

import (
	"database/sql"
	"errors"
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

// statementMatching returns the first captured statement containing want.
//
// Positional indexing into the capture list broke the moment a destructive
// transition grew a guard query in front of it — the assertions were about the
// UPDATE, not about whichever statement happened to run first.
func statementMatching(t *testing.T, seen *[]string, want string) string {
	t.Helper()
	for _, stmt := range *seen {
		if strings.Contains(strings.ToLower(stmt), want) {
			return strings.ToLower(stmt)
		}
	}
	t.Fatalf("no captured statement contains %q; saw:\n%s", want, strings.Join(*seen, "\n"))
	return ""
}

// expectSuperAdminGuard seeds the lookup guardLastSuperAdmin makes before every
// destructive lifecycle transition.
//
// The guard reads person_type first and only counts other administrators when
// the target is a super administrator, so an ordinary person costs exactly one
// query and never reaches the count.
func expectSuperAdminGuard(mock sqlmock.Sqlmock) {
	mock.ExpectQuery("select").WillReturnRows(
		sqlmock.NewRows([]string{"person_type"}).AddRow("registered"))
}

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

	expectSuperAdminGuard(mock)
	mock.ExpectExec("update").WillReturnResult(sqlmock.NewResult(0, 1))

	if _, err := repo.SoftDeleteActor(42); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	stmt := statementMatching(t, seen, "update")
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
			expectSuperAdminGuard(mock)
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
		if err := repo.PreparePurge(id); err != nil {
			t.Errorf("PreparePurge(%d): %v", id, err)
		}
		if err := repo.FinishPurge(id); err != nil {
			t.Errorf("FinishPurge(%d): %v", id, err)
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
//
// And it must not take what the actor merely created. A person is the
// created_by of the groups, hashtags and places they started, and those belong
// to everybody in them.
func TestPurgeListsContentButNotTheActorOrWhatItFounded(t *testing.T) {
	db, mock, seen := newNodesTestDB(t)
	repo := newRepo(db)

	expectSuperAdminGuard(mock)
	for range 8 {
		mock.ExpectExec("insert").WillReturnResult(sqlmock.NewResult(0, 1))
	}

	if err := repo.PreparePurge(42); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	// The first statement after the guard lists the content.
	listing := strings.ToLower((*seen)[1])

	if !strings.Contains(listing, "where owner_id = ?") {
		t.Errorf("content is not scoped by owner:\n%s", listing)
	}
	if !strings.Contains(listing, "where created_by = ?") {
		t.Errorf("what the actor wrote elsewhere is not listed, so it would "+
			"survive credited to a tombstone:\n%s", listing)
	}
	for _, kept := range []string{"node.actor.%", "node.tag.%"} {
		if !strings.Contains(listing, "type not like '"+kept+"'") {
			t.Errorf("authorship is not limited to content, so %s the actor "+
				"created would be deleted:\n%s", kept, listing)
		}
	}
	if !strings.Contains(listing, "recursive") {
		t.Errorf("descendants are not walked, so comments on a purged post would "+
			"be orphaned rather than removed:\n%s", listing)
	}
	if !strings.Contains(listing, "where id <> ?") {
		t.Errorf("the actor's own row is listed for deletion, which frees the "+
			"handle for somebody else to register:\n%s", listing)
	}
}

// Edges go before nodes, and both before the list of work. Every statement of
// a batch finds its rows through the list, so removing that first would leave
// the rest matching nothing.
func TestPurgeBatchDeletesInOrder(t *testing.T) {
	db, mock, seen := newNodesTestDB(t)
	repo := newRepo(db)

	mock.ExpectBegin()
	for range 8 {
		mock.ExpectExec("delete").WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectCommit()

	if err := repo.DeletePurgeBatch(42, &PurgeBatch{First: 100, Last: 200}); err != nil {
		t.Fatalf("delete batch: %v", err)
	}

	position := func(fragment string) int {
		for i, stmt := range *seen {
			if strings.Contains(strings.ToLower(stmt), fragment) {
				return i
			}
		}
		t.Fatalf("no statement contains %q", fragment)
		return -1
	}

	edges := position("from edges")
	photoFiles := position("from photo_files")
	nodes := position("from nodes")
	work := position("delete from purge_work")

	// The list of a photo post's images goes with the post, in the same
	// batch, so none is left naming a post that is gone.
	if !(photoFiles < work) {
		t.Errorf("photo files at %d, work at %d; want the images before the list", photoFiles, work)
	}

	if !(edges < nodes && nodes < work) {
		t.Errorf("order is edges %d, nodes %d, work %d; want edges, nodes, work",
			edges, nodes, work)
	}

	// Every statement is bounded by the batch.
	for _, stmt := range *seen {
		if !strings.Contains(strings.ToLower(stmt), "between ? and ?") {
			t.Errorf("a statement is not limited to the batch:\n%s", stmt)
		}
	}
}

// A failure part-way through must leave the batch whole and still listed.
func TestPurgeBatchRollsBackOnFailure(t *testing.T) {
	db, mock, _ := newNodesTestDB(t)
	repo := newRepo(db)

	mock.ExpectBegin()
	mock.ExpectExec("delete").WillReturnResult(sqlmock.NewResult(0, 5))
	mock.ExpectExec("delete").WillReturnError(errors.New("deadlock"))
	mock.ExpectRollback()

	if err := repo.DeletePurgeBatch(42, &PurgeBatch{First: 100, Last: 200}); err == nil {
		t.Fatal("a failed batch reported success")
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
	for _, column := range []string{"name", "body", "meta", "website_url", "geo_city", "filename", "avatar_filename"} {
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

// The floor under the whole installation: losing the last person who can
// administer it leaves nobody able to appoint another, and no route back.
//
// Enforced in the repository rather than in each caller because person-service
// had this written out four times — Delete, Archive, SetEnabled and Purge — and
// a check every caller has to remember is one a new caller will not.
func TestLifecycleRefusesTheLastSuperAdministrator(t *testing.T) {
	for _, tc := range []struct {
		name   string
		others int
		want   error
	}{
		{"the last one", 0, ErrLastSuperAdmin},
		{"one of several", 2, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, _ := newNodesTestDB(t)

			mock.ExpectQuery("select").WillReturnRows(
				sqlmock.NewRows([]string{"person_type"}).AddRow("super-administrator"))
			mock.ExpectQuery("select").WillReturnRows(
				sqlmock.NewRows([]string{"count"}).AddRow(tc.others))
			if tc.want == nil {
				mock.ExpectExec("update").WillReturnResult(sqlmock.NewResult(0, 1))
			}

			_, err := newRepo(db).SoftDeleteActor(42)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

// Enabling cannot reduce the number of people who can administer the site, and
// guarding it would leave the last super administrator unable to re-enable
// themselves — refusing the one transition that fixes the situation.
func TestEnablingIsNotGuarded(t *testing.T) {
	db, mock, _ := newNodesTestDB(t)
	mock.ExpectExec("update").WillReturnResult(sqlmock.NewResult(0, 1))

	if _, err := newRepo(db).SetEnabled(42, true); err != nil {
		t.Fatalf("enable: %v", err)
	}
}
