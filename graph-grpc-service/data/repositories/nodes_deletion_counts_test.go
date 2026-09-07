package repositories

import (
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// The counts shown in the Danger zone before somebody deletes their profile.
// They are the gate that makes people stop, so a number that is wrong in the
// alarming direction is as bad as one that is wrong in the reassuring
// direction — the first cries wolf, the second fails to warn.

func TestDeletionCountsUsesScalarSubqueriesNotJoins(t *testing.T) {
	db, mock, seen := newNodesTestDB(t)
	repo := newRepo(db)

	mock.ExpectQuery("select").WillReturnRows(
		sqlmock.NewRows([]string{"posts", "comments", "followers", "following",
			"groups", "member_since"}).
			AddRow(4826, 3868, 219, 234, 23, time.Now()))

	counts, err := repo.ReadDeletionCounts(5)
	if err != nil {
		t.Fatalf("ReadDeletionCounts: %v", err)
	}

	if counts.Posts != 4826 || counts.Followers != 219 || counts.GroupsAdministered != 23 {
		t.Errorf("counts came back wrong: %+v", counts)
	}

	query := strings.ToLower((*seen)[0])

	// Joining these together would multiply rows: 219 followers against 23
	// groups is 5,037 of something that is not a count of either. Each total
	// is over a different table on a different key, so each is its own scalar
	// subquery.
	if strings.Contains(query, " join ") {
		t.Errorf("a join appears; counts over different keys fan out into "+
			"products rather than totals:\n%s", query)
	}

	// One statement, not six. Six round trips sit in front of a dialog
	// somebody is waiting on, and six snapshots can disagree.
	if len(*seen) != 1 {
		t.Errorf("issued %d statements, want 1", len(*seen))
	}
}

// A missing actor yields zeroes, not an error. Every term is a scalar
// subquery, so a bad id returns a row of NULLs rather than no row — and the
// dialog can render "0 posts" while it can do nothing with an error.
func TestDeletionCountsSurvivesAnUnknownActor(t *testing.T) {
	db, mock, _ := newNodesTestDB(t)
	repo := newRepo(db)

	mock.ExpectQuery("select").WillReturnRows(
		sqlmock.NewRows([]string{"posts", "comments", "followers", "following",
			"groups", "member_since"}).
			AddRow(0, 0, 0, 0, 0, nil))

	counts, err := repo.ReadDeletionCounts(999999)
	if err != nil {
		t.Fatalf("a NULL member_since failed the whole read: %v", err)
	}
	if !counts.MemberSince.IsZero() {
		t.Errorf("MemberSince = %v; an unknown join date must stay zero rather "+
			"than becoming the epoch", counts.MemberSince)
	}
}

// An invalid id must not reach the database at all.
func TestDeletionCountsRefusesANonPositiveID(t *testing.T) {
	db, _, seen := newNodesTestDB(t)
	repo := newRepo(db)

	for _, id := range []int64{0, -1} {
		counts, err := repo.ReadDeletionCounts(id)
		if err != nil {
			t.Fatalf("ReadDeletionCounts(%d): %v", id, err)
		}
		if counts.Posts != 0 {
			t.Errorf("id %d returned counts", id)
		}
	}

	if len(*seen) != 0 {
		t.Errorf("issued %d statements for invalid ids", len(*seen))
	}
}

// Following and followers are the same edge type read in opposite directions.
// Getting both from node_a_id, or both from node_b_id, gives two copies of one
// number — and it looks entirely plausible.
func TestDeletionCountsReadsFollowInBothDirections(t *testing.T) {
	db, mock, seen := newNodesTestDB(t)
	repo := newRepo(db)

	mock.ExpectQuery("select").WillReturnRows(
		sqlmock.NewRows([]string{"posts", "comments", "followers", "following",
			"groups", "member_since"}).AddRow(0, 0, 0, 0, 0, nil))

	_, _ = repo.ReadDeletionCounts(5)

	query := strings.ToLower((*seen)[0])
	if !strings.Contains(query, "e.node_b_id = ?") {
		t.Error("no node_b_id term — followers is read from the wrong side")
	}
	if !strings.Contains(query, "e.node_a_id = ?") {
		t.Error("no node_a_id term — following is read from the wrong side")
	}
}
