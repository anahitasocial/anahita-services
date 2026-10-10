package repositories

import (
	"testing"

	"anahita-libs/constants"
	"anahita-libs/proto/graphpb"
)

// A profile's followers, found by name, against a real database.
//
//	ACCESS_TEST_DSN="root@tcp(127.0.0.1:13309)/fresh?parseTime=true" \
//	  go test ./data/repositories/ -run TestFollowersByName -v
//
// The search was accepted by the API and dropped before it reached the
// query, so every search returned every follower.
func TestFollowersByName(t *testing.T) {
	db := accessTestDB(t)
	seedRepliesFixture(t, db)
	repo := NewGraphRepo(db)

	// The group's followers in the fixture are Alice and Carol.
	followers := func(search string) (map[int64]bool, uint64) {
		t.Helper()

		req := &graphpb.BrowseConnectionNodesRequest{
			NodeId:    replyGroup,
			EdgeType:  constants.EdgeFollowEntityType,
			Direction: constants.GraphQueryDirectionInbound,
			Limit:     50,
			Search:    search,
		}

		nodes, err := repo.BrowseConnectionNodes(req, replyViewer(replyAlice))
		got := nodeIDs(t, nodes, err)

		total, err := repo.CountConnectionNodes(req, replyViewer(replyAlice))
		if err != nil {
			t.Fatalf("count %q: %v", search, err)
		}

		return got, total
	}

	cases := []struct {
		name   string
		search string
		want   []int64
	}{
		{"nothing typed", "", []int64{replyAlice, replyCarol}},
		{"part of a name", "aro", []int64{replyCarol}},
		{"an alias, whatever the case", "ALICE", []int64{replyAlice}},
		{"somebody who does not follow it", "bob", nil},
		// What a pattern would do with these is match everybody.
		{"a wildcard is not one", "%", []int64{replyAlice, replyCarol}},
		{"nor is an underscore", "_____", []int64{replyAlice, replyCarol}},
	}

	for _, c := range cases {
		got, total := followers(c.search)
		if !sameSet(got, c.want...) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
		// The total is of the same list, or paging runs past its end.
		if int(total) != len(c.want) {
			t.Errorf("%s: counted %d, want %d", c.name, total, len(c.want))
		}
	}
}

// A profile's followers in the order they followed, newest first.
func TestFollowersNewestFirst(t *testing.T) {
	db := accessTestDB(t)
	seedRepliesFixture(t, db)
	repo := NewGraphRepo(db)

	// Alice followed the group first, Carol after her. Alice joined the
	// site after Carol, which is what the list used to go by.
	for _, stmt := range []string{
		`UPDATE edges SET created_at = NOW() - INTERVAL 2 DAY WHERE type = 'edge.actor.follow.v1' AND node_a_id = 1 AND node_b_id = 3`,
		`UPDATE edges SET created_at = NOW() - INTERVAL 1 DAY WHERE type = 'edge.actor.follow.v1' AND node_a_id = 4 AND node_b_id = 3`,
		`UPDATE nodes SET created_at = NOW() - INTERVAL 9 YEAR WHERE id = 4`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	order := func(ordering string) []int64 {
		t.Helper()

		nodes, err := repo.BrowseConnectionNodes(&graphpb.BrowseConnectionNodesRequest{
			NodeId:    replyGroup,
			EdgeType:  constants.EdgeFollowEntityType,
			Direction: constants.GraphQueryDirectionInbound,
			Limit:     50,
			Ordering:  ordering,
		}, replyViewer(replyAlice))
		if err != nil {
			t.Fatal(err)
		}
		ids := []int64{}
		for _, node := range nodes {
			ids = append(ids, node.ID)
		}
		return ids
	}

	if got := order(constants.OrderingConnected); len(got) != 2 || got[0] != replyCarol || got[1] != replyAlice {
		t.Errorf("by when they followed: %v, want Carol then Alice", got)
	}
	// The old order, by when each joined, puts them the other way round.
	if got := order(constants.OrderingRecent); len(got) != 2 || got[0] != replyAlice {
		t.Errorf("by when they joined: %v, want Alice first", got)
	}
}
