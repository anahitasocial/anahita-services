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
	made, err := repo.AddThread(parent, parts, nil)
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
	top, err := repo.AddThread(nil, []*entities.AnahitaNode{threadPart("Thread test top"), threadPart("Thread test under")}, nil)
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
	if _, err := repo.AddThread(nil, []*entities.AnahitaNode{threadPart("Thread test first"), threadPart("Thread test second"), broken}, nil); err == nil {
		t.Fatal("a thread with a part that cannot be made was made")
	}
	var left int
	if err := db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE body LIKE 'Thread test %'`).Scan(&left); err != nil || left != 0 {
		t.Errorf("%d parts left behind (err %v)", left, err)
	}
}

// A thread's part with pictures (roadmap row 143) has them in the same
// transaction: set, its first named on the post, its uploads no longer
// uploads. A picture that cannot be set leaves no part and every upload.
func TestAThreadsPicturesAreMadeWithIt(t *testing.T) {
	db := accessTestDB(t)
	seedRepliesFixture(t, db)
	repo := NewNodesRepo(db)

	upload := func(filename string) int64 {
		result, err := db.Exec(`INSERT INTO photo_uploads (owner_id, created_by, filename, mimetype, width, height, created_at) VALUES (?, ?, ?, 'image/jpeg', 800, 600, NOW())`, replyBob, replyBob, filename)
		if err != nil {
			t.Fatal(err)
		}
		id, _ := result.LastInsertId()
		t.Cleanup(func() { db.Exec(`DELETE FROM photo_uploads WHERE id = ?`, id) })
		return id
	}
	picture := func(filename string) *entities.PhotoFile {
		return &entities.PhotoFile{Filename: filename, Mimetype: "image/jpeg", Width: 800, Height: 600, AltText: "Ceres from the Rocinante"}
	}
	photoPart := func(body string) *entities.AnahitaNode {
		part := threadPart(body)
		part.Type = "node.medium.photo-service.photo.v1"
		part.Service = "photo-service"
		return part
	}

	first := upload("thread-test-a.jpg")
	made, err := repo.AddThread(nil, []*entities.AnahitaNode{threadPart("Thread test words"), photoPart("Thread test pictures")},
		map[int]PartFiles{1: {Files: []*entities.PhotoFile{picture("thread-test-a.jpg")}, UploadIDs: []int64{first}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, part := range made {
			db.Exec(`DELETE FROM photo_files WHERE node_id = ?`, part.ID)
			db.Exec(`DELETE FROM nodes WHERE id = ?`, part.ID)
		}
	})
	var filename, alt string
	if err := db.QueryRow(`SELECT n.filename, f.alt_text FROM nodes n JOIN photo_files f ON f.node_id = n.id WHERE n.id = ?`, made[1].ID).Scan(&filename, &alt); err != nil || filename != "thread-test-a.jpg" || alt != "Ceres from the Rocinante" {
		t.Errorf("the part's picture: %q %q (err %v)", filename, alt, err)
	}
	var uploads int
	db.QueryRow(`SELECT COUNT(*) FROM photo_uploads WHERE id = ?`, first).Scan(&uploads)
	if uploads != 0 {
		t.Error("the upload is still an upload")
	}

	// The same file again: it is a picture of a post already, so the
	// thread cannot be made, and nothing of it is.
	second := upload("thread-test-b.jpg")
	if _, err := repo.AddThread(nil, []*entities.AnahitaNode{photoPart("Thread test again"), photoPart("Thread test twice")},
		map[int]PartFiles{
			0: {Files: []*entities.PhotoFile{picture("thread-test-b.jpg")}, UploadIDs: []int64{second}},
			1: {Files: []*entities.PhotoFile{picture("thread-test-a.jpg")}},
		}); err == nil {
		t.Fatal("a thread whose picture could not be set was made")
	}
	var left int
	db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE body IN ('Thread test again', 'Thread test twice')`).Scan(&left)
	db.QueryRow(`SELECT COUNT(*) FROM photo_uploads WHERE id = ?`, second).Scan(&uploads)
	if left != 0 || uploads != 1 {
		t.Errorf("%d parts left behind, upload kept: %v", left, uploads == 1)
	}
}
