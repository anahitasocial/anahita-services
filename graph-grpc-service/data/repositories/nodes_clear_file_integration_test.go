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

func filenamesOf(t *testing.T, repos *repositories.Repositories, id int64) (string, string) {
	t.Helper()

	node, err := repos.Nodes.Read(id)
	if err != nil {
		t.Fatalf("reading %d: %v", id, err)
	}
	return node.Filename, node.CoverFilename
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

	if avatar, cover := filenamesOf(t, repos, purgeAda); avatar != "" || cover != "cover.jpg" {
		t.Errorf("avatar %q, cover %q; want the avatar gone and the cover kept", avatar, cover)
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
	if _, cover := filenamesOf(t, repos, purgeAdaArticle); cover != "" {
		t.Errorf("the article still names %q", cover)
	}
}

// On a photo post the same column names the photo. Asking to remove an
// avatar must never remove that.
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
	if photo, _ := filenamesOf(t, repos, purgeAdaPost); photo != "photo.jpg" {
		t.Errorf("the photo post now names %q", photo)
	}
}

func TestClearFileRefusesAKindItDoesNotKnow(t *testing.T) {
	db := purgeTestDB(t)
	seedPurgeFixture(t, db)
	repos := repositories.NewRepositories(db)

	if _, err := repos.Nodes.ClearFile(purgeAda, "language", purgeAda); err == nil {
		t.Error("an unknown kind was accepted")
	}
	if avatar, _ := filenamesOf(t, repos, purgeAda); avatar != "avatar.jpg" {
		t.Errorf("the profile now names %q", avatar)
	}
}
