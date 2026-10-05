package repositories_test

import (
	"testing"
	"time"

	"anahita-libs/data/entities"

	"graph/data/repositories"
	"graph/sweeper"
)

// The images of photo posts against real MySQL.
//
//	ACCESS_TEST_DSN="root@tcp(127.0.0.1:13309)/fresh?parseTime=true" \
//	  go test ./data/repositories/ -run TestPhotoFiles -v
//
// Never the development database: the fixture starts by emptying nodes.

func filenames(files []*entities.PhotoFile) []string {
	var out []string
	for _, file := range files {
		out = append(out, file.Filename)
	}
	return out
}

func sameNames(got []string, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestPhotoFilesAreSetFromUploadsAndTheFirstIsCopiedToThePost(t *testing.T) {
	db := purgeTestDB(t)
	seedPurgeFixture(t, db)
	repo := repositories.NewPhotoFilesRepo(db)

	var uploads []int64
	for _, name := range []string{"one.jpg", "two.png"} {
		mimetype := "image/jpeg"
		if name == "two.png" {
			mimetype = "image/png"
		}
		id, err := repo.AddUpload(&entities.PhotoUpload{
			OwnerID: purgeAda, CreatedBy: purgeAda,
			Filename: name, Mimetype: mimetype, Width: 1200, Height: 800,
		})
		if err != nil {
			t.Fatalf("upload %s: %v", name, err)
		}
		uploads = append(uploads, id)
	}

	read, err := repo.ReadUploads(uploads)
	if err != nil || len(read) != 2 || read[0].CreatedBy != purgeAda || read[0].Width != 1200 {
		t.Fatalf("reading the uploads back: %+v, %v", read, err)
	}

	// The second upload first: the order is the caller's, not the uploads'.
	err = repo.Set(purgeAdaPost, []*entities.PhotoFile{
		{Filename: "two.png", Mimetype: "image/png", Width: 1200, Height: 800, AltText: "a heron"},
		{Filename: "one.jpg", Mimetype: "image/jpeg", Width: 1200, Height: 800},
	}, uploads)
	if err != nil {
		t.Fatalf("set: %v", err)
	}

	files, err := repo.Browse([]int64{purgeAdaPost})
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	if !sameNames(filenames(files), "two.png", "one.jpg") {
		t.Errorf("images = %v; want two.png then one.jpg", filenames(files))
	}
	if files[0].AltText != "a heron" || files[0].Position != 0 || files[1].Position != 1 {
		t.Errorf("description or positions wrong: %+v %+v", files[0], files[1])
	}

	// The images the post had before are replaced, not added to.
	if n := count(t, db, `SELECT COUNT(*) FROM photo_files WHERE node_id = ?`, purgeAdaPost); n != 2 {
		t.Errorf("the post lists %d images, want 2", n)
	}

	// Used, so no longer uploads: the sweeper must never remove the stored
	// images of a post.
	if n := count(t, db, `SELECT COUNT(*) FROM photo_uploads`); n != 0 {
		t.Errorf("%d upload(s) left after being put in a post", n)
	}

	var filename, mimetype string
	if err := db.QueryRow(`SELECT filename, mimetype FROM nodes WHERE id = ?`, purgeAdaPost).
		Scan(&filename, &mimetype); err != nil {
		t.Fatalf("reading the post: %v", err)
	}
	if filename != "two.png" || mimetype != "image/png" {
		t.Errorf("the post names %q (%s); want the first image, two.png", filename, mimetype)
	}
}

// One image belongs to one post. A second post claiming it fails and changes
// nothing, which is what stops two requests using the same upload at once
// from both succeeding.
func TestPhotoFilesAnImageCannotBeInTwoPosts(t *testing.T) {
	db := purgeTestDB(t)
	seedPurgeFixture(t, db)
	repo := repositories.NewPhotoFilesRepo(db)

	// The fixture gives Ada's post photo.jpg. Her article is another node.
	err := repo.Set(purgeAdaArticle, []*entities.PhotoFile{
		{Filename: "fresh.jpg"},
		{Filename: "photo.jpg"},
	}, nil)
	if err == nil {
		t.Fatal("an image already in a post was put in another")
	}

	if n := count(t, db, `SELECT COUNT(*) FROM photo_files WHERE node_id = ?`, purgeAdaArticle); n != 0 {
		t.Errorf("the refused change left %d image(s) behind", n)
	}

	files, err := repo.Browse([]int64{purgeAdaPost})
	if err != nil || !sameNames(filenames(files), "photo.jpg", "second.jpg") {
		t.Errorf("the post that had the image lost it: %v, %v", filenames(files), err)
	}
}

func TestPhotoFilesExpiredUploadsAreOnlyTheOldOnes(t *testing.T) {
	db := purgeTestDB(t)
	seedPurgeFixture(t, db)
	repo := repositories.NewPhotoFilesRepo(db)

	if _, err := db.Exec(`
		INSERT INTO photo_uploads (id, owner_id, created_by, filename, created_at) VALUES
		 (1, 1, 1, 'old.jpg', UTC_TIMESTAMP() - INTERVAL 2 DAY),
		 (2, 1, 1, 'new.jpg', UTC_TIMESTAMP() - INTERVAL 1 HOUR)`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	expired, err := repo.ExpiredUploads(time.Now().Add(-24*time.Hour), 100)
	if err != nil {
		t.Fatalf("expired: %v", err)
	}
	if len(expired) != 1 || expired[0].Filename != "old.jpg" {
		t.Fatalf("expired = %+v; want only old.jpg", expired)
	}

	if err := repo.DeleteUploads([]int64{expired[0].ID}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM photo_uploads`); n != 1 {
		t.Errorf("%d upload(s) left, want the recent one", n)
	}
}

// An upload nobody used is removed with what was stored for it; one made a
// moment ago, which somebody may still be writing the post for, is not.
func TestPhotoFilesTheSweeperRemovesUnusedUploadsAndTheirImages(t *testing.T) {
	db := purgeTestDB(t)
	seedPurgeFixture(t, db)
	repos := repositories.NewRepositories(db)
	files := &fakeFiles{}

	if _, err := db.Exec(`
		INSERT INTO photo_uploads (id, owner_id, created_by, filename, created_at) VALUES
		 (1, 8, 1, 'old.jpg', UTC_TIMESTAMP() - INTERVAL 2 DAY),
		 (2, 8, 1, 'new.jpg', UTC_TIMESTAMP() - INTERVAL 1 HOUR)`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if removed := sweeper.Uploads(repos, files); removed != 1 {
		t.Errorf("removed %d upload(s), want 1", removed)
	}

	deleted := map[string]bool{}
	for _, path := range files.deleted {
		deleted[path] = true
	}
	// Under the profile it was uploaded for, the group, not the uploader.
	if !deleted["assets/v2/photo-service/photo/8/old_medium.jpg"] {
		t.Errorf("the unused upload's images were not deleted: %v", files.deleted)
	}
	for path := range deleted {
		if path == "assets/v2/photo-service/photo/8/new_medium.jpg" {
			t.Error("a recent upload's images were deleted")
		}
	}

	if got := ids(t, db, `SELECT id FROM photo_uploads`); !sameIDs(got, []int64{2}) {
		t.Errorf("uploads left = %v, want only the recent one", got)
	}

	// The images of posts are not uploads and are never touched.
	if n := count(t, db, `SELECT COUNT(*) FROM photo_files WHERE node_id = ?`, purgeAdaPost); n != 2 {
		t.Errorf("the sweeper left %d of a post's 2 images listed", n)
	}
}
