package repositories

import (
	"strings"
	"testing"

	"auth/constants"

	libs_requests "anahita-libs/requests"

	"github.com/DATA-DOG/go-sqlmock"
)

// The invitation list is the viewer's own, and these pin the predicate
// that makes it so.
//
// A tokens row stores the address an invite was sent to. Browsing them
// unscoped therefore hands the caller the email addresses of people
// somebody else invited — people with no account on this site, who
// never agreed to appear on anybody's screen. The endpoint did exactly
// that: it filtered on type and nothing else, for every administrator.
//
// Asserted against the generated SQL rather than through a seeded
// database, matching TestInvalidateOutstandingRetiresOnlyTheRightRows
// next door. The failure being guarded against is a missing WHERE
// clause, and a missing WHERE clause is visible in the statement —
// where a fixture-based test would only catch it if the fixture
// happened to contain two issuers.

func TestBrowseFromScopesToTheIssuer(t *testing.T) {
	db, mock, seen := newCapturingDB(t)
	repo := &tokens{db: db, sb: stmtBuilder}

	// Empty result: this test is about the statement, not the rows.
	mock.ExpectQuery("select").WillReturnRows(sqlmock.NewRows([]string{}))

	req := libs_requests.NewBrowse()
	if _, err := repo.BrowseFrom(constants.TokenTypeInvite, 42, req); err != nil {
		t.Fatalf("BrowseFrom: %v", err)
	}

	query := strings.ToLower(statementTouching(t, seen, "SELECT"))

	// Both predicates matter, and for different reasons. Without the
	// type it would list password resets and email-change links, which
	// carry addresses too. Without created_by it would list everybody
	// else's invitations — the bug this method exists to close.
	for _, predicate := range []string{"t.type = ?", "t.created_by = ?"} {
		if !strings.Contains(query, predicate) {
			t.Errorf("BrowseFrom is missing %q\n%s", predicate, query)
		}
	}
}

func TestCountFromScopesToTheIssuer(t *testing.T) {
	db, mock, seen := newCapturingDB(t)
	repo := &tokens{db: db, sb: stmtBuilder}

	mock.ExpectQuery("select").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	if _, err := repo.CountFrom(constants.TokenTypeInvite, 42); err != nil {
		t.Fatalf("CountFrom: %v", err)
	}

	query := strings.ToLower(statementTouching(t, seen, "SELECT"))

	// The count has to be scoped the same way as the list it paginates.
	// A total that answers the wider question would report rows the
	// caller is never shown, and page them into an empty screen.
	for _, predicate := range []string{"t.type = ?", "t.created_by = ?"} {
		if !strings.Contains(query, predicate) {
			t.Errorf("CountFrom is missing %q\n%s", predicate, query)
		}
	}
}
