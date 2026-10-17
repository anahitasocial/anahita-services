package repositories_test

import (
	"testing"

	"graph/data/repositories"
)

// Taking an avatar or a cover off a node, against real MySQL.
//
//	ACCESS_TEST_DSN="root@tcp(127.0.0.1:13309)/fresh?parseTime=true" \
//	  go test ./data/repositories/ -run TestClearFile -v
//
// Never the development database: the fixture starts by emptying nodes.

// filesOf is what a node names: its avatar, its cover, and the file it is.
type filesOf struct {
	avatar, cover, own string
}

func namedBy(t *testing.T, repos *repositories.Repositories, id int64) filesOf {
	t.Helper()

	node, err := repos.Nodes.Read(id)
	if err != nil {
		t.Fatalf("reading %d: %v", id, err)
	}
	return filesOf{avatar: node.AvatarFilename, cover: node.CoverFilename, own: node.Filename}
}

func TestClearFileTakesAnAvatarOffAnActorAndLeavesItsCover(t *testing.T) {
	db := purgeTestDB(t)
	seedPurgeFixture(t, db)
	repos := repositories.NewRepositories(db)

	if _, err := db.Exec(`UPDATE nodes SET cover_filename = 'cover.jpg' WHERE id = ?`, purgeAda); err != nil {
		t.Fatal(err)
	}

	changed, err := repos.Nodes.ClearFile(purgeAda, repositories.NodeFileAvatar, purgeAda)
	if err != nil || !changed {
		t.Fatalf("clearing the avatar: changed=%v, %v", changed, err)
	}

	if named := namedBy(t, repos, purgeAda); named.avatar != "" || named.cover != "cover.jpg" {
		t.Errorf("avatar %q, cover %q; want the avatar gone and the cover kept", named.avatar, named.cover)
	}

	if n := count(t, db, `SELECT COUNT(*) FROM nodes WHERE id = ? AND modified_by = ?`, purgeAda, purgeAda); n != 1 {
		t.Error("the change was not recorded as made by anybody")
	}
}

func TestClearFileTakesACoverOffAPost(t *testing.T) {
	db := purgeTestDB(t)
	seedPurgeFixture(t, db)
	repos := repositories.NewRepositories(db)

	if _, err := db.Exec(`UPDATE nodes SET cover_filename = 'cover.jpg' WHERE id = ?`, purgeAdaArticle); err != nil {
		t.Fatal(err)
	}

	if changed, err := repos.Nodes.ClearFile(purgeAdaArticle, repositories.NodeFileCover, purgeAda); err != nil || !changed {
		t.Fatalf("clearing the cover: changed=%v, %v", changed, err)
	}
	if named := namedBy(t, repos, purgeAdaArticle); named.cover != "" {
		t.Errorf("the article still names %q", named.cover)
	}
}

// A photo post has no avatar, and asking to remove one must never take the
// photo it is.
func TestClearFileDoesNotTakeThePhotoOffAPhotoPost(t *testing.T) {
	db := purgeTestDB(t)
	seedPurgeFixture(t, db)
	repos := repositories.NewRepositories(db)

	changed, err := repos.Nodes.ClearFile(purgeAdaPost, repositories.NodeFileAvatar, purgeAda)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("an avatar was reported removed from a photo post")
	}
	if named := namedBy(t, repos, purgeAdaPost); named.own != "photo.jpg" {
		t.Errorf("the photo post now names %q", named.own)
	}
}

func TestClearFileRefusesAKindItDoesNotKnow(t *testing.T) {
	db := purgeTestDB(t)
	seedPurgeFixture(t, db)
	repos := repositories.NewRepositories(db)

	if _, err := repos.Nodes.ClearFile(purgeAda, "language", purgeAda); err == nil {
		t.Error("an unknown kind was accepted")
	}
	if named := namedBy(t, repos, purgeAda); named.avatar != "avatar.jpg" {
		t.Errorf("the profile now names %q", named.avatar)
	}
}

// An avatar is written to, and read from, the column that is its own, and an
// actor's `filename` is left empty: on every row that has one, `filename` is
// the file the node is.
func TestAnActorsAvatarIsKeptApartFromFilename(t *testing.T) {
	db := purgeTestDB(t)
	seedPurgeFixture(t, db)
	repos := repositories.NewRepositories(db)

	grace, err := repos.Nodes.Read(purgeGrace)
	if err != nil {
		t.Fatal(err)
	}
	grace.AvatarFilename = "grace.png"
	grace.ModifiedBy = purgeGrace
	if _, err := repos.Nodes.Edit(grace); err != nil {
		t.Fatalf("saving an avatar: %v", err)
	}

	if named := namedBy(t, repos, purgeGrace); named.avatar != "grace.png" || named.own != "" {
		t.Errorf("avatar %q, own file %q; want the avatar set and no file of her own", named.avatar, named.own)
	}

	var inColumn, inFilename string
	if err := db.QueryRow(`SELECT avatar_filename, filename FROM nodes WHERE id = ?`, purgeGrace).
		Scan(&inColumn, &inFilename); err != nil {
		t.Fatal(err)
	}
	if inColumn != "grace.png" || inFilename != "" {
		t.Errorf("stored as avatar_filename=%q, filename=%q", inColumn, inFilename)
	}

	// And it is what the owner of a post is read with, which is where most
	// avatars are shown.
	post, err := repos.Nodes.Read(purgeGracePost)
	if err != nil {
		t.Fatal(err)
	}
	if post.Owner == nil || post.Owner.AvatarFilename != "grace.png" {
		t.Errorf("the post's owner is read with avatar %+v", post.Owner)
	}
}
