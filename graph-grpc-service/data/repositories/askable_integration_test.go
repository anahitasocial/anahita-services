package repositories

import (
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"graph/requests"
)

// The lists of people and of groups, widened to the profiles a viewer may
// only ask to follow, against a real database.
//
//	ACCESS_TEST_DSN="root@tcp(127.0.0.1:13309)/fresh?parseTime=true" \
//	  go test ./data/repositories/ -run TestAskable -v
//
// WHAT THIS GUARDS. A profile nobody but its followers may see is listed,
// for somebody signed in, only when it has said it may be asked, and never
// to somebody it blocked or to a visitor. And being listed is not being
// seen: FilterVisible still says no, which is what cuts it down to a name.
func TestAskableProfilesAreListedAndStillNotSeen(t *testing.T) {
	db := accessTestDB(t)
	seedRepliesFixture(t, db)
	repo := NewNodesRepo(db)

	const personType = "node.actor.person-service.person.v1"
	const groupType = "node.actor.group-service.group.v1"

	// Alice goes private. She has blocked Dave in the fixture. The group
	// is followers-only already.
	if _, err := db.Exec(`UPDATE nodes SET access = 'followers' WHERE id = ?`, replyAlice); err != nil {
		t.Fatal(err)
	}

	list := func(nodeType string, viewer *entities.Viewer, askable bool) map[int64]bool {
		t.Helper()

		req := requests.NewNodeRequest().Browse
		req.Types = []string{nodeType}
		req.Viewer = viewer
		req.Enabled = true
		req.Limit = 50
		req.IncludeAskable = askable
		nodes, err := repo.Browse(req)
		got := nodeIDs(t, nodes, err)

		cReq := requests.NewNodeRequest().Count
		cReq.Types = []string{nodeType}
		cReq.Viewer = viewer
		cReq.Enabled = true
		cReq.IncludeAskable = askable
		total, err := repo.Count(cReq)
		if err != nil {
			t.Fatalf("count: %v", err)
		}
		if int(total) != len(got) {
			t.Errorf("counted %d, listed %d", total, len(got))
		}

		return got
	}

	bob := replyViewer(replyBob)
	const dave = 5

	// Nobody has said they may be asked: the wider list is the same list.
	if got := list(personType, bob, true); got[replyAlice] {
		t.Error("a private person who may not be asked is listed")
	}
	if got := list(groupType, bob, true); got[replyGroup] {
		t.Error("a private group that may not be asked is listed")
	}

	if _, err := db.Exec(`UPDATE nodes SET allow_follow_request = 1 WHERE id IN (?, ?)`, replyAlice, replyGroup); err != nil {
		t.Fatal(err)
	}

	if got := list(personType, bob, true); !got[replyAlice] {
		t.Error("a private person who may be asked is not listed")
	}
	if got := list(groupType, bob, true); !got[replyGroup] {
		t.Error("a private group that may be asked is not listed")
	}
	// Only where it was asked for.
	if got := list(personType, bob, false); got[replyAlice] {
		t.Error("the ordinary list shows a person the viewer may not see")
	}
	// Never to somebody she blocked, nor to a visitor.
	if got := list(personType, replyViewer(dave), true); got[replyAlice] {
		t.Error("listed to somebody she blocked")
	}
	if got := list(personType, entities.NewViewer(), true); got[replyAlice] {
		t.Error("listed to a visitor")
	}

	// Listed, and still not seen.
	visible, err := repo.FilterVisible([]int64{replyAlice, replyGroup, replyCarol}, bob)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[int64]bool{}
	for _, id := range visible {
		seen[id] = true
	}
	if seen[replyAlice] || seen[replyGroup] || !seen[replyCarol] {
		t.Errorf("visible to Bob: %v, want Carol alone", visible)
	}

	// Asking shows in the relation, and is not a follow.
	if _, err := db.Exec(`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,created_at) VALUES (?,?,?,?,?,NOW())`,
		constants.EdgeFollowRequestEntityType, replyBob, personType, replyAlice, personType); err != nil {
		t.Fatal(err)
	}
	rels, err := NewSocialgraphRelRepo(db).GetMany([]int64{replyAlice, replyCarol}, replyBob)
	if err != nil {
		t.Fatal(err)
	}
	if !rels[replyAlice].HasRequested || rels[replyAlice].IsLeading {
		t.Errorf("Bob and Alice: %+v, want asked and not following", rels[replyAlice])
	}
	if rels[replyCarol].HasRequested {
		t.Error("Bob has not asked Carol anything")
	}
}
