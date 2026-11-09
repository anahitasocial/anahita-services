package sweeper

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"anahita-libs/data/services"
	"anahita-libs/videos"

	"github.com/DATA-DOG/go-sqlmock"
)

const (
	avatars = "assets/v1/identity-image-service/avatars/"
	covers  = "assets/v1/identity-image-service/covers/"
	photos  = "assets/v1/photo-service/photo/"
)

// Each folder's keys are read for the row that should name them; a key in
// no shape this knows is not understood, and so kept.
func TestStoredFileKeysAreRead(t *testing.T) {
	cases := []struct {
		folder, prefix, key string
		want                storedFile
		ok                  bool
	}{
		{folderAvatars, avatars, avatars + "person/5/313741d5_square.jpg",
			storedFile{nodeID: 5, filename: "313741d5.jpg"}, true},
		{folderAvatars, avatars, avatars + "group/9/ab_xxlarge.png",
			storedFile{nodeID: 9, filename: "ab.png"}, true},
		{folderCovers, covers, covers + "article/5/3dd5_large.jpg",
			storedFile{nodeID: 5, filename: "3dd5.jpg"}, true},
		{folderPhotos, photos, photos + "5/a3f390_medium.jpg",
			storedFile{filename: "a3f390.jpg"}, true},
		{folderVideoUploads, videos.UploadsPrefix, videos.UploadsPrefix + "5/0b9e-41/source",
			storedFile{uploadID: "0b9e-41"}, true},
		{folderVideos, videos.StoragePrefix, videos.StoragePrefix + "5/0b9e-41/720.mp4",
			storedFile{uploadID: "0b9e-41"}, true},

		// Not in a shape this knows: kept.
		{folderAvatars, avatars, avatars + "person/5/no-size.jpg", storedFile{}, false},
		{folderAvatars, avatars, avatars + "person/x/a_small.jpg", storedFile{}, false},
		{folderAvatars, avatars, avatars + "person/0/a_small.jpg", storedFile{}, false},
		{folderPhotos, photos, photos + "5/deeper/a_small.jpg", storedFile{}, false},
		{folderPhotos, photos, photos + "5/../a_small.jpg", storedFile{}, false},
		{folderVideoUploads, videos.UploadsPrefix, videos.UploadsPrefix + "5/u/part-1", storedFile{}, false},
		{folderPhotos, photos, "assets/v1/elsewhere/a_small.jpg", storedFile{}, false},
	}
	for _, each := range cases {
		got, ok := parseStoredFile(each.folder, each.prefix, each.key)
		if ok != each.ok {
			t.Errorf("%s: understood %v, want %v", each.key, ok, each.ok)
			continue
		}
		if !ok {
			continue
		}
		if got.nodeID != each.want.nodeID || got.filename != each.want.filename || got.uploadID != each.want.uploadID {
			t.Errorf("%s: read as %+v, want %+v", each.key, got, each.want)
		}
	}
}

// A picture is named under the node's own id or its owner's: an article's
// cover is kept under its author's (AnahitaNode.GetCoverURL).
func TestAFileIsNamedByItsRow(t *testing.T) {
	n := named{
		avatars:   map[string]bool{portraitPair(5, "new.jpg"): true},
		covers:    map[string]bool{portraitPair(5, "article.jpg"): true, portraitPair(77, "article.jpg"): true},
		photos:    map[string]bool{"posted.jpg": true},
		uploads:   map[string]bool{"in-progress": true},
		videoKeys: map[string]bool{videos.StoragePrefix + "5/done/720.mp4": true},
	}
	cases := []struct {
		file storedFile
		want bool
	}{
		{storedFile{folder: folderAvatars, nodeID: 5, filename: "new.jpg"}, true},
		// A replaced avatar's old sizes.
		{storedFile{folder: folderAvatars, nodeID: 5, filename: "old.jpg"}, false},
		// A purged profile's.
		{storedFile{folder: folderAvatars, nodeID: 6, filename: "new.jpg"}, false},
		{storedFile{folder: folderCovers, nodeID: 5, filename: "article.jpg"}, true},
		{storedFile{folder: folderCovers, nodeID: 77, filename: "article.jpg"}, true},
		{storedFile{folder: folderPhotos, filename: "posted.jpg"}, true},
		{storedFile{folder: folderPhotos, filename: "never-posted.jpg"}, false},
		{storedFile{folder: folderVideoUploads, uploadID: "in-progress"}, true},
		{storedFile{folder: folderVideoUploads, uploadID: "abandoned"}, false},
		{storedFile{folder: folderVideos, key: videos.StoragePrefix + "5/done/720.mp4", uploadID: "done"}, true},
		{storedFile{folder: folderVideos, key: videos.StoragePrefix + "5/in-progress/720.mp4", uploadID: "in-progress"}, true},
		{storedFile{folder: folderVideos, key: videos.StoragePrefix + "5/gone/720.mp4", uploadID: "gone"}, false},
	}
	for _, each := range cases {
		if got := n.isNamed(each.file); got != each.want {
			t.Errorf("%+v: named %v, want %v", each.file, got, each.want)
		}
	}
}

// fakeStore is the bucket as file-grpc-service lists it, and what was
// removed from it.
type fakeStore struct {
	files   map[string][]services.BrowsedFile
	removed []string
	failOn  string
}

func (f *fakeStore) Browse(prefix, after string, limit int) ([]services.BrowsedFile, bool, error) {
	var page []services.BrowsedFile
	for _, file := range f.files[prefix] {
		if file.Path > after {
			page = append(page, file)
		}
	}
	if len(page) > limit {
		return page[:limit], true, nil
	}
	return page, false, nil
}

func (f *fakeStore) Delete(req *services.FileDeleteRequest) error {
	if req.Path == f.failOn {
		return errors.New("the store did not answer")
	}
	f.removed = append(f.removed, req.Path)
	return nil
}

// expectRun is the queries a run asks, in order, with what the database
// answers: the lock, every video post's files, then the avatars page.
func expectRun(mock sqlmock.Sqlmock, portraits *sqlmock.Rows, portraitsErr error) {
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT GET_LOCK(?, ?)`)).
		WillReturnRows(sqlmock.NewRows([]string{"got"}).AddRow(1))
	mock.ExpectQuery(`FROM nodes\s+WHERE type IN \(\?, \?\)`).WithArgs(videos.NodeType, videos.AudioNodeType).
		WillReturnRows(sqlmock.NewRows([]string{"meta"}))
	query := mock.ExpectQuery(`FROM nodes\s+WHERE \(id IN`)
	if portraitsErr != nil {
		query.WillReturnError(portraitsErr)
	} else {
		query.WillReturnRows(portraits)
	}
	mock.ExpectExec(regexp.QuoteMeta(`SELECT RELEASE_LOCK(?)`)).WillReturnResult(sqlmock.NewResult(0, 0))
}

func avatarsStore(now time.Time) *fakeStore {
	old := now.Add(-48 * time.Hour)
	return &fakeStore{files: map[string][]services.BrowsedFile{
		avatars: {
			// The avatar person 5 has now: kept.
			{Path: avatars + "person/5/new_small.jpg", ModifiedAt: old},
			// The sizes of the one it replaced: gone.
			{Path: avatars + "person/5/old_small.jpg", ModifiedAt: old},
			// A purged profile's: gone.
			{Path: avatars + "person/6/gone_small.jpg", ModifiedAt: old},
			// Made an hour ago, named by nothing yet: kept.
			{Path: avatars + "person/7/fresh_small.jpg", ModifiedAt: now.Add(-time.Hour)},
			// In no shape this knows: kept.
			{Path: avatars + "README.txt", ModifiedAt: old},
		},
	}}
}

func TestTheFileSweepRemovesOnlyWhatNothingNames(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := avatarsStore(time.Now())
	expectRun(mock, sqlmock.NewRows([]string{"id", "owner_id", "avatar_filename", "cover_filename"}).
		AddRow(5, nil, "new.jpg", ""), nil)

	swept, err := RunFiles(db, store, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}

	want := []string{avatars + "person/5/old_small.jpg", avatars + "person/6/gone_small.jpg"}
	if strings.Join(store.removed, ",") != strings.Join(want, ",") {
		t.Errorf("removed %v, want %v", store.removed, want)
	}
	count := swept.Folders[folderAvatars]
	if count.Seen != 5 || count.Removed != 2 || count.Kept != 2 || count.Unsure != 1 {
		t.Errorf("avatars counted %+v", *count)
	}
}

// A dry run counts what would go, and removes nothing.
func TestADryRunRemovesNothing(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()

	store := avatarsStore(time.Now())
	expectRun(mock, sqlmock.NewRows([]string{"id", "owner_id", "avatar_filename", "cover_filename"}).
		AddRow(5, nil, "new.jpg", ""), nil)

	swept, err := RunFiles(db, store, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(store.removed) != 0 {
		t.Errorf("a dry run removed %v", store.removed)
	}
	if got := swept.Folders[folderAvatars].Removed; got != 2 {
		t.Errorf("would remove %d, want 2", got)
	}
	if !strings.Contains(swept.String(), "would remove 2") {
		t.Errorf("the log says %q", swept.String())
	}
}

// A query that fails keeps everything in its page.
func TestAFailedQueryKeepsItsPage(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()

	store := avatarsStore(time.Now())
	expectRun(mock, nil, errors.New("connection lost"))

	swept, err := RunFiles(db, store, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(store.removed) != 0 {
		t.Errorf("removed %v after a failed query", store.removed)
	}
	if got := swept.Folders[folderAvatars].Removed; got != 0 {
		t.Errorf("counted %d removed", got)
	}
}
