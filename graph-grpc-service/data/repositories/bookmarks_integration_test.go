package repositories

import (
	"fmt"
	"testing"

	"anahita-libs/data/entities"
)

// Saved posts against a real database, on the fixture the thread tests use.
//
//	ACCESS_TEST_DSN="root@tcp(127.0.0.1:13309)/fresh?parseTime=true" \
//	  go test ./data/repositories/ -run TestSaved -v
//
// TWO RULES THESE GUARD. What somebody saved is theirs alone: nothing here
// answers about anybody else's. And saving a post does not keep it open: the
// list shows a post only while the person may still see it.

func savedIDs(t *testing.T, repo BookmarksRepoInterface, viewer int64) []int64 {
	t.Helper()

	nodes, err := repo.List(replyViewer(viewer), 50, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	ids := []int64{}
	for _, node := range nodes {
		ids = append(ids, node.ID)
	}

	total, err := repo.Count(replyViewer(viewer))
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if int(total) != len(ids) {
		t.Errorf("counted %d, listed %d", total, len(ids))
	}

	return ids
}

func TestSavedPostsAreTheirOwnersAndOnlyWhileTheyCanBeSeen(t *testing.T) {
	db := accessTestDB(t)
	seedRepliesFixture(t, db)
	repo := NewBookmarksRepo(db)

	post := func(id int64) *entities.AnahitaNode {
		node := &entities.AnahitaNode{}
		node.ID = id
		node.Type = noteType
		return node
	}

	// Bob saves a public post, a reply under it, and a post in a group he
	// is not in. The last is not his to see; the repository does not
	// refuse it (the service asks before saving), and the list leaves it
	// out.
	for i, id := range []int64{rootPublic, replyByBob, rootInGroup} {
		if err := repo.Set(replyBob, post(id), true); err != nil {
			t.Fatalf("save %d: %v", id, err)
		}
		// Each a moment after the last, so the order is the order saved in.
		if _, err := db.Exec(`UPDATE edges SET created_at = created_at + INTERVAL ? SECOND WHERE type = 'edge.base.bookmark.v1' AND node_b_id = ?`, i, id); err != nil {
			t.Fatal(err)
		}
	}

	// Saving twice is saving once.
	if err := repo.Set(replyBob, post(rootPublic), true); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE edges SET created_at = created_at + INTERVAL 10 SECOND WHERE type = 'edge.base.bookmark.v1' AND node_b_id = ?`, rootPublic); err != nil {
		t.Fatal(err)
	}
	var edges int
	if err := db.QueryRow(`SELECT COUNT(*) FROM edges WHERE type = 'edge.base.bookmark.v1' AND node_a_id = ? AND node_b_id = ?`, replyBob, rootPublic).Scan(&edges); err != nil || edges != 1 {
		t.Errorf("%d edges for one saved post (err %v)", edges, err)
	}

	// Most recently saved first; saving the public post again moved it to
	// the front. Not the post in the group.
	if got := fmt.Sprint(savedIDs(t, repo, replyBob)); got != fmt.Sprint([]int64{rootPublic, replyByBob}) {
		t.Errorf("Bob's saved posts: %s", got)
	}

	// Nobody else's list has them: not the post's author, not a visitor.
	for _, viewer := range []int64{replyAlice, replyCarol, 0} {
		if got := savedIDs(t, repo, viewer); len(got) != 0 {
			t.Errorf("viewer %d is shown %v", viewer, got)
		}
	}

	// Which of some posts he saved, for marking them where they are shown.
	saved, err := repo.IsSavedMany(replyBob, []int64{rootPublic, rootInGroup, rootOnlyMe, 9999})
	if err != nil {
		t.Fatal(err)
	}
	if !saved[rootPublic] || !saved[rootInGroup] || saved[rootOnlyMe] || saved[9999] {
		t.Errorf("marks: %v", saved)
	}
	if mine, _ := repo.IsSavedMany(replyAlice, []int64{rootPublic}); mine[rootPublic] {
		t.Error("Alice is told her post is saved, by Bob's saving it")
	}

	// The public post is made private. It leaves his list, and is still
	// his to take out; when it is public again it is back.
	if _, err := db.Exec(`UPDATE nodes SET access = 'myself' WHERE id = ?`, rootPublic); err != nil {
		t.Fatal(err)
	}
	if got := savedIDs(t, repo, replyBob); len(got) != 0 {
		t.Errorf("after the post was made private Bob is shown %v", got)
	}
	if _, err := db.Exec(`UPDATE nodes SET access = 'public' WHERE id = ?`, rootPublic); err != nil {
		t.Fatal(err)
	}
	if got := savedIDs(t, repo, replyBob); len(got) != 2 {
		t.Errorf("after it was made public again Bob is shown %v", got)
	}

	// Taken out, and taken out again.
	for range 2 {
		if err := repo.Set(replyBob, post(rootPublic), false); err != nil {
			t.Fatal(err)
		}
	}
	if got := fmt.Sprint(savedIDs(t, repo, replyBob)); got != fmt.Sprint([]int64{replyByBob}) {
		t.Errorf("after taking one out: %s", got)
	}
}
