package repositories

import (
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// This statement runs on every sign-in on the site, so what it touches matters
// more than its size suggests. Three things about its shape:
//
//   - it writes one column;
//   - it narrows on the id;
//   - it skips soft-deleted rows.
//
// Losing any of those is silent. A missing predicate stamps every actor in the
// table, and stamping a deleted one records a visit to an account that, at that
// moment, does not exist.

func TestTouchLastVisitWritesOnlyTheVisitColumn(t *testing.T) {
	db, mock, seen := newNodesTestDB(t)
	mock.ExpectExec("update").WillReturnResult(sqlmock.NewResult(0, 1))

	if _, err := newRepo(db).TouchLastVisit(42); err != nil {
		t.Fatalf("touch: %v", err)
	}

	if len(*seen) != 1 {
		t.Fatalf("expected one statement, got %d: %v", len(*seen), *seen)
	}
	statement := strings.ToLower((*seen)[0])

	setClause := statement
	if i := strings.Index(statement, "where"); i >= 0 {
		setClause = statement[:i]
	}

	// A sign-in is not an edit. Anything else in the SET clause would mean a
	// login could change a profile as a side effect.
	for _, forbidden := range []string{
		"enabled", "person_type", "alias", "name", "deleted_at", "archived_at", "modified_by",
	} {
		if strings.Contains(setClause, forbidden) {
			t.Errorf("a sign-in writes %q:\n%s", forbidden, setClause)
		}
	}

	if !strings.Contains(setClause, "last_visit_date") {
		t.Errorf("the visit column is not written:\n%s", setClause)
	}

	if !strings.Contains(statement, "where id = ?") {
		t.Errorf("no predicate on the id — this stamps every actor in the "+
			"table on every sign-in:\n%s", statement)
	}

	// A deleted account can still reach the sign-in form; that is how
	// restore-on-login works. Stamping it before the restore is decided
	// records a visit to an account that does not exist yet.
	if !strings.Contains(statement, "deleted_at is null") {
		t.Errorf("a soft-deleted actor would be stamped:\n%s", statement)
	}

	// NOW(), not a value bound from the caller. The column means "when the
	// database saw this"; a timestamp from a client is a claim from a machine
	// whose clock nobody controls.
	if !strings.Contains(statement, "now()") {
		t.Errorf("the timestamp does not come from the database:\n%s", statement)
	}
}

// Nothing stamped is a real answer — an actor that is gone, or was never
// there. The caller has to be able to tell that from success, even though it
// never blocks a login on it.
func TestTouchLastVisitReportsWhetherItStamped(t *testing.T) {
	t.Run("stamped", func(t *testing.T) {
		db, mock, _ := newNodesTestDB(t)
		mock.ExpectExec("update").WillReturnResult(sqlmock.NewResult(0, 1))

		changed, err := newRepo(db).TouchLastVisit(42)
		if err != nil || !changed {
			t.Fatalf("got (%v, %v), want (true, nil)", changed, err)
		}
	})

	t.Run("no such actor", func(t *testing.T) {
		db, mock, _ := newNodesTestDB(t)
		mock.ExpectExec("update").WillReturnResult(sqlmock.NewResult(0, 0))

		changed, err := newRepo(db).TouchLastVisit(42)
		if err != nil || changed {
			t.Fatalf("got (%v, %v), want (false, nil)", changed, err)
		}
	})

	// Rejected before it reaches the database. A zero id from an unpopulated
	// struct would otherwise be a statement looking for actor 0.
	t.Run("no id", func(t *testing.T) {
		db, _, seen := newNodesTestDB(t)

		changed, err := newRepo(db).TouchLastVisit(0)
		if err != nil || changed {
			t.Fatalf("got (%v, %v), want (false, nil)", changed, err)
		}
		if len(*seen) != 0 {
			t.Errorf("a zero id reached the database: %v", *seen)
		}
	})
}
