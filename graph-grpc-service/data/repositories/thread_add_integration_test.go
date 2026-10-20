package repositories

import (
	"fmt"
	"testing"

	"anahita-libs/data/entities"
)

// A thread written in one go (roadmap row 101), against a real database, on
// the fixture the thread tests use (ACCESS_TEST_DSN; see
// access_integration_test.go):
//
//	go test ./data/repositories/ -run TestAThread -v
//
// TWO RULES THESE GUARD. Each part answers the one before, under one root.
// And a thread is all or nothing: a part that cannot be made leaves none.

func threadPart(body string) *entities.AnahitaNode {
	return &entities.AnahitaNode{
		Type: noteType, Service: "text-service", Body: body,
		OwnerID: replyBob, OwnerType: "node.actor.person-service.person.v1", CreatedBy: replyBob,
		Access: "public", Enabled: true,
	}
}

func TestAThreadIsMadeInOrderUnderOneRoot(t *testing.T) {
	db := accessTestDB(t)
	seedRepliesFixture(t, db)
	repo := NewNodesRepo(db)

	parent, err := repo.Read(rootPublic)
	if err != nil {
		t.Fatal(err)
	}
	parts := []*entities.AnahitaNode{threadPart("Thread test one"), threadPart("Thread test two"), threadPart("Thread test three")}
	made, err := repo.AddThread(parent, parts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, part := range made {
			db.Exec(`DELETE FROM nodes WHERE id = ?`, part.ID)
		}
	})

	before := int64(rootPublic)
	for i, part := range made {
		if part.ParentID != before || part.RootID != rootPublic || part.Access != "" || part.Body != fmt.Sprintf("Thread test %s", []string{"one", "two", "three"}[i]) {
			t.Errorf("part %d: parent %d, root %d, access %q, %q", i+1, part.ParentID, part.RootID, part.Access, part.Body)
		}
		before = part.ID
	}

	// A post at the top: the first part is the root of the rest.
	top, err := repo.AddThread(nil, []*entities.AnahitaNode{threadPart("Thread test top"), threadPart("Thread test under")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, part := range top {
			db.Exec(`DELETE FROM nodes WHERE id = ?`, part.ID)
		}
	})
	if top[0].RootID != 0 || top[0].Access != "public" || top[1].ParentID != top[0].ID || top[1].RootID != top[0].ID {
		t.Errorf("top %+v / under parent %d root %d", top[0].RootID, top[1].ParentID, top[1].RootID)
	}
}

func TestAThreadThatCannotBeMadeWholeIsNotMadeAtAll(t *testing.T) {
	db := accessTestDB(t)
	seedRepliesFixture(t, db)
	repo := NewNodesRepo(db)

	broken := threadPart("Thread test broken")
	broken.Service = ""
	if _, err := repo.AddThread(nil, []*entities.AnahitaNode{threadPart("Thread test first"), threadPart("Thread test second"), broken}); err == nil {
		t.Fatal("a thread with a part that cannot be made was made")
	}
	var left int
	if err := db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE body LIKE 'Thread test %'`).Scan(&left); err != nil || left != 0 {
		t.Errorf("%d parts left behind (err %v)", left, err)
	}
}
