package repositories

import (
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// featured_at is read the way deleted_at is: selected bare, so NULL stays
// "not featured", and scanned into a pointer. A COALESCE to any date would
// make every actor featured.
func TestBaseSelectReadsFeaturedAtBare(t *testing.T) {
	sql, _, err := GetBaseSelectQuery().From("nodes AS node").ToSql()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if !strings.Contains(sql, "node.featured_at") {
		t.Errorf("featured_at is not selected, so no read carries it:\n%s", sql)
	}
	if strings.Contains(sql, "COALESCE(node.featured_at") {
		t.Errorf("featured_at is COALESCEd; NULL is how \"not featured\" is said:\n%s", sql)
	}
}

// Featuring twice must not move the date. It is also the order of the list,
// so re-featuring would be a way to bump somebody to the top, and the first
// date is the only record of when it happened.
//
// And the lifecycle guard sits in the statement itself, so an actor disabled,
// archived or deleted between a check and the write cannot reach the list.
func TestFeaturingIsGuardedInTheStatement(t *testing.T) {
	db, mock, seen := newNodesTestDB(t)
	mock.ExpectExec("update").WillReturnResult(sqlmock.NewResult(0, 1))

	changed, err := newRepo(db).SetFeatured(42, true)
	if err != nil {
		t.Fatalf("feature: %v", err)
	}
	if !changed {
		t.Error("a row was stamped and SetFeatured reported no change")
	}

	statement := strings.ToLower((*seen)[0])
	for _, guard := range []string{
		"where id = ?",
		"featured_at is null",
		"enabled = 1",
		"archived_at is null",
		"deleted_at is null",
	} {
		if !strings.Contains(statement, guard) {
			t.Errorf("featuring is missing %q:\n%s", guard, statement)
		}
	}
}

// Nothing stamped has two meanings, and the caller answers them differently:
// already featured is 204, not featurable is 409.
func TestFeaturingExplainsANoOp(t *testing.T) {
	for _, tc := range []struct {
		name    string
		refused int
		want    error
	}{
		{"already featured", 0, nil},
		{"disabled, archived or deleted", 1, ErrNotFeaturable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, _ := newNodesTestDB(t)
			mock.ExpectExec("update").WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectQuery("select").WillReturnRows(
				sqlmock.NewRows([]string{"count"}).AddRow(tc.refused))

			changed, err := newRepo(db).SetFeatured(42, true)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
			if changed {
				t.Error("nothing was stamped and SetFeatured reported a change")
			}
		})
	}
}

// Taking an actor off the list is always allowed — a disabled account is
// exactly the one somebody wants off it — so unfeaturing carries no lifecycle
// guard, and needs nothing but the id and a date to clear.
func TestUnfeaturingIsNotGuarded(t *testing.T) {
	db, mock, seen := newNodesTestDB(t)
	mock.ExpectExec("update").WillReturnResult(sqlmock.NewResult(0, 1))

	if _, err := newRepo(db).SetFeatured(42, false); err != nil {
		t.Fatalf("unfeature: %v", err)
	}

	statement := strings.ToLower((*seen)[0])
	if !strings.Contains(statement, "set featured_at = null") {
		t.Errorf("unfeaturing does not clear the column:\n%s", statement)
	}
	for _, guard := range []string{"enabled", "archived_at", "deleted_at"} {
		if strings.Contains(statement, guard) {
			t.Errorf("unfeaturing is guarded on %q, so a disabled actor "+
				"could not be taken off the list:\n%s", guard, statement)
		}
	}
	if len(*seen) != 1 {
		t.Errorf("unfeaturing ran %d statements, want 1", len(*seen))
	}
}
