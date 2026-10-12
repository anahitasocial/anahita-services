package repositories

import (
	"testing"

	"anahita-libs/constants"
	"anahita-libs/proto/graphpb"
)

// Shared feeds among search results, against a real database, on the
// fixture the custom feed tests use: Alice has a feed kept to herself and a
// public one ("Gardens"), and Carol one for her followers.
//
//	ACCESS_TEST_DSN="root@tcp(127.0.0.1:13309)/fresh?parseTime=true" \
//	  go test ./data/repositories/ -run TestSearchFindsSharedFeeds -v
func TestSearchFindsSharedFeeds(t *testing.T) {
	db := accessTestDB(t)
	seedCustomFeedFixture(t, db)
	// Bob follows Carol, so her feed for followers is his to find.
	if _, err := db.Exec(`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,created_at) VALUES
		('edge.actor.follow.v1',2,'node.actor.person-service.person.v1',4,'node.actor.person-service.person.v1',NOW())`); err != nil {
		t.Fatal(err)
	}
	repo := NewSearchRepo(db)

	found := func(scope, keywords string, viewer int64) map[int64]bool {
		t.Helper()

		req := &graphpb.SearchBrowseRequest{Scope: scope, Keywords: keywords, Limit: 50}
		nodes, err := repo.Browse(req, feedViewer(viewer))
		seen := nodeIDs(t, nodes, err)

		total, err := repo.Count(&graphpb.SearchCountRequest{Scope: scope, Keywords: keywords}, feedViewer(viewer))
		if err != nil {
			t.Fatalf("count: %v", err)
		}
		if int(total) != len(seen) {
			t.Errorf("%s %q for %d: counted %d, listed %d", scope, keywords, viewer, total, len(seen))
		}

		return seen
	}

	cases := []struct {
		what   string
		viewer int64
		want   []int64
	}{
		{"a visitor", 0, []int64{feedForAll}},
		{"Alice, whose private one is not found even by her", feedAlice, []int64{feedForAll}},
		{"Bob, who follows Carol", feedBob, []int64{feedForAll, feedOfGroup}},
		{"Dave, who follows nobody", feedDave, []int64{feedForAll}},
		{"Carol, who made one for her followers", feedCarol, []int64{feedForAll, feedOfGroup}},
	}
	for _, c := range cases {
		if got := found(constants.SearchScopeFeed, "", c.viewer); !sameSet(got, c.want...) {
			t.Errorf("feeds for %s: %v, want %v", c.what, sortedIDs(got), c.want)
		}
	}

	// By part of its name, and among everything else that matches.
	if got := found(constants.SearchScopeFeed, "gard", feedDave); !sameSet(got, feedForAll) {
		t.Errorf("a feed by its name: %v", sortedIDs(got))
	}
	all := found(constants.SearchScopeAll, "garden", feedAlice)
	if !all[feedForAll] || !all[feedBobPublic] || all[feedOnlyMe] {
		t.Errorf("everything matching garden, to Alice: %v", sortedIDs(all))
	}

	// The other tabs have none of them.
	for _, scope := range []string{constants.SearchScopeActor, constants.SearchScopeMedium} {
		for id := range found(scope, "", feedAlice) {
			if id == feedOnlyMe || id == feedForAll || id == feedOfGroup {
				t.Errorf("a feed among %s", scope)
			}
		}
	}
}
