package repositories

import (
	"fmt"
	"testing"
)

// A post's quotes against a real database, on the fixture the thread tests
// use, for its Cited by.
//
//	ACCESS_TEST_DSN="root@tcp(127.0.0.1:13309)/fresh?parseTime=true" \
//	  go test ./data/repositories/ -run TestQuotes -v
//
// THE RULE THESE GUARD. A post's Cited by lists the notes quoting it, and a
// viewer is shown, and counted, only those they may see: a note in a private
// group is not shown to somebody outside it, nor an "only me" note to anybody
// but its author. One its quoted post was taken out of is shown to nobody.

func quoteIDs(t *testing.T, repo QuotesRepoInterface, viewer int64, limit, offset uint64) []int64 {
	t.Helper()

	nodes, err := repo.Page(rootPhoto, replyViewer(viewer), limit, offset)
	if err != nil {
		t.Fatalf("page: %v", err)
	}
	ids := []int64{}
	for _, node := range nodes {
		ids = append(ids, node.ID)
	}
	return ids
}

func TestQuotesAreListedAndCountedOnlyAsTheyCanBeSeen(t *testing.T) {
	db := accessTestDB(t)
	seedRepliesFixture(t, db)
	repo := NewQuotesRepo(db)

	// Four of Alice's notes quote her photo: the public one, the one in the
	// private group, her "only me" note, and Bob's answer, which the photo
	// was taken out of.
	for _, stmt := range []string{
		fmt.Sprintf(`UPDATE nodes SET quote_id = %d WHERE id IN (%d, %d, %d, %d)`,
			rootPhoto, rootPublic, rootInGroup, rootOnlyMe, replyByBob),
		fmt.Sprintf(`UPDATE nodes SET quote_detached_at = NOW() WHERE id = %d`, replyByBob),
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}

	cases := []struct {
		name   string
		viewer int64
		want   []int64
	}{
		{"somebody signed out sees the public one", 0, []int64{rootPublic}},
		{"Bob, outside the group, sees the public one", replyBob, []int64{rootPublic}},
		{"Carol, who follows the group, sees that one too", replyCarol, []int64{rootPublic, rootInGroup}},
		{"Alice sees her own, the only-me one as well", replyAlice, []int64{rootPublic, rootInGroup, rootOnlyMe}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := quoteIDs(t, repo, c.viewer, 20, 0)
			if !sameIDSet(got, c.want) {
				t.Errorf("listed %v, want %v", got, c.want)
			}

			total, err := repo.Count(rootPhoto, replyViewer(c.viewer))
			if err != nil {
				t.Fatalf("count: %v", err)
			}
			if int(total) != len(c.want) {
				t.Errorf("counted %d, want %d", total, len(c.want))
			}
		})
	}

	t.Run("a page from an offset takes up after the one before", func(t *testing.T) {
		first := quoteIDs(t, repo, replyAlice, 2, 0)
		rest := quoteIDs(t, repo, replyAlice, 2, 2)
		if len(first) != 2 || len(rest) != 1 {
			t.Fatalf("pages of %v and %v, want two and one", first, rest)
		}
		if sameIDSet(append(first, rest...), []int64{rootPublic, rootInGroup, rootOnlyMe}) == false {
			t.Errorf("pages %v and %v do not make up the three", first, rest)
		}
	})
}

// sameIDSet is whether two lists hold the same ids, in any order.
func sameIDSet(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	count := map[int64]int{}
	for _, id := range a {
		count[id]++
	}
	for _, id := range b {
		count[id]--
		if count[id] < 0 {
			return false
		}
	}
	return true
}
