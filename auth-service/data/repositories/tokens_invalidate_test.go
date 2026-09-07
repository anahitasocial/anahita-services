package repositories

import (
	"regexp"
	"strings"
	"testing"

	"auth/constants"

	"github.com/DATA-DOG/go-sqlmock"
)

// Asking for a second reset email used to leave the first link working. Both
// arrive in the same inbox, but a link that has been forwarded, or that
// somebody else reached first, keeps working for its full window after the
// owner has asked for a fresh one.

func TestInvalidateOutstandingRetiresOnlyTheRightRows(t *testing.T) {
	db, mock, seen := newCapturingDB(t)
	repo := &tokens{db: db, sb: stmtBuilder}

	mock.ExpectExec("update").WillReturnResult(sqlmock.NewResult(0, 2))

	retired, err := repo.InvalidateOutstanding(
		constants.TokenTypePasswordReset, "ada@example.com")
	if err != nil {
		t.Fatalf("InvalidateOutstanding: %v", err)
	}
	if retired != 2 {
		t.Errorf("retired = %d, want 2", retired)
	}

	query := strings.ToLower(statementTouching(t, seen, "UPDATE"))

	// All three predicates matter. Without the type it would also retire
	// invitations and email-change links; without is_used it would rewrite
	// rows that are already settled; without the address it would retire
	// every pending token on the site.
	for _, predicate := range []string{"type = ?", "recipient_email = ?", "is_used = 0"} {
		if !strings.Contains(query, predicate) {
			t.Errorf("missing %q — this UPDATE is wider than it should be:\n%s",
				predicate, query)
		}
	}

	if !regexp.MustCompile(`set\s+is_used\s*=\s*1`).MatchString(query) {
		t.Errorf("does not mark the rows used:\n%s", query)
	}
}

// One statement, not a read-then-write: two forgot-password requests arriving
// together would each read the other's token as pending and leave both live.
func TestInvalidateOutstandingIsASingleStatement(t *testing.T) {
	db, mock, seen := newCapturingDB(t)
	repo := &tokens{db: db, sb: stmtBuilder}

	mock.ExpectExec("update").WillReturnResult(sqlmock.NewResult(0, 1))

	if _, err := repo.InvalidateOutstanding(
		constants.TokenTypePasswordReset, "ada@example.com"); err != nil {
		t.Fatalf("InvalidateOutstanding: %v", err)
	}

	if len(*seen) != 1 {
		t.Errorf("issued %d statements; a select before the update is a race",
			len(*seen))
	}
}
