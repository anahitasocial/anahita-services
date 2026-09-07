package repositories

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"anahita-libs/constants"

	"github.com/DATA-DOG/go-sqlmock"
)

// The floor under profile deletion: never zero super administrators.
//
// The failure it prevents is the last one deleting themselves, the grace
// period running out, the sweeper purging, and the site having nobody who can
// administer it and no route to appoint one.

func newPeopleTestDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, *[]string) {
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

// Both exclusions matter, and each has its own way of going wrong.
//
// Counting a soft-deleted super administrator satisfies the floor with
// somebody who will be purged, so the site reaches zero one deletion later —
// exactly the outcome, arrived at more slowly. Counting a disabled one
// satisfies it with somebody who cannot sign in to administer anything.
func TestCountOtherActiveSuperAdminsExcludesTheInactive(t *testing.T) {
	db, mock, seen := newPeopleTestDB(t)
	repo := &Person{db: db, dbTimeout: 3 * time.Second}

	mock.ExpectQuery("select").WillReturnRows(
		sqlmock.NewRows([]string{"count"}).AddRow(2))

	got, err := repo.CountOtherActiveSuperAdmins(5)
	if err != nil {
		t.Fatalf("CountOtherActiveSuperAdmins: %v", err)
	}
	if got != 2 {
		t.Errorf("got %d, want 2", got)
	}

	query := strings.ToLower((*seen)[0])

	for _, want := range []struct{ clause, why string }{
		{"n.deleted_at is null",
			"a super administrator inside their grace period will be purged; " +
				"counting them reaches zero one deletion later"},
		{"n.enabled = 1",
			"a disabled super administrator cannot sign in to administer anything"},
		{"p.node_id <> ?",
			"the person being deleted must not count towards their own floor"},
		{"p.usertype = ?",
			"without this it counts every account on the site"},
	} {
		if !strings.Contains(query, want.clause) {
			t.Errorf("missing %q — %s:\n%s", want.clause, want.why, query)
		}
	}
}

func TestCountOtherActiveSuperAdminsPassesTheRightArguments(t *testing.T) {
	db, mock, _ := newPeopleTestDB(t)
	repo := &Person{db: db, dbTimeout: 3 * time.Second}

	mock.ExpectQuery("select").
		WithArgs(constants.UserTypeSuperAdmin, int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	if _, err := repo.CountOtherActiveSuperAdmins(5); err != nil {
		t.Fatalf("arguments did not match — the usertype or the excluded id "+
			"is wrong: %v", err)
	}
}

// Zero others is the refusal case and must be reported as zero, not as an
// error — the handler distinguishes them.
func TestCountOtherActiveSuperAdminsReturnsZeroNotError(t *testing.T) {
	db, mock, _ := newPeopleTestDB(t)
	repo := &Person{db: db, dbTimeout: 3 * time.Second}

	mock.ExpectQuery("select").WillReturnRows(
		sqlmock.NewRows([]string{"count"}).AddRow(0))

	got, err := repo.CountOtherActiveSuperAdmins(5)
	if err != nil {
		t.Fatalf("zero others was reported as an error: %v", err)
	}
	if got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}
