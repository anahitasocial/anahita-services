package repositories

import (
	"testing"

	"anahita-libs/constants"
	"anahita-libs/videos"

	"github.com/DATA-DOG/go-sqlmock"
)

// A thread is its writer's chain: from the note, the earliest of their
// replies to it, then the earliest to that one. A later reply of theirs to
// the same note is not in it, as the thread's page does not draw it so
// (anahita-web row 126).
func TestAThreadIsTheWritersEarliestChain(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Note 1: replies 10 (to 1), 11 (to 1, later), 12 (to 10), 13 (to 12).
	// Note 2: one reply, 20. Note 3: none.
	mock.ExpectQuery(`WITH RECURSIVE chain`).
		WithArgs(int64(1), int64(2), int64(3),
			constants.NodeMediumNoteEntityType, constants.NodeMediumPhotoEntityType, videos.NodeType, videos.AudioNodeType,
			constants.NodeMediumNoteEntityType, constants.NodeMediumPhotoEntityType, videos.NodeType, videos.AudioNodeType,
			ThreadDepthMax).
		WillReturnRows(sqlmock.NewRows([]string{"root_id", "id", "parent_id"}).
			AddRow(1, 10, 1).
			AddRow(1, 11, 1).
			AddRow(1, 12, 10).
			AddRow(1, 13, 12).
			AddRow(2, 20, 2))

	repo := &nodesRepo{db: db, dbTimeout: constants.DBTimeout}
	threads, err := repo.Threads([]int64{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}

	if got := threads[1]; got.Count != 4 || got.NextID != 10 {
		t.Errorf("note 1: %+v, want 4 notes and 10 second", got)
	}
	if got := threads[2]; got.Count != 2 || got.NextID != 20 {
		t.Errorf("note 2: %+v, want 2 notes and 20 second", got)
	}
	if _, ok := threads[3]; ok {
		t.Error("note 3 has no thread")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

// Pictures, GIFs, videos and sounds are in a thread too (roadmap row 143):
// a chain is of every kind of post, and begins at any.
func TestEveryKindOfPostIsInAThread(t *testing.T) {
	for _, kind := range []string{constants.NodeMediumNoteEntityType, constants.NodeMediumPhotoEntityType, videos.NodeType, videos.AudioNodeType} {
		if !IsThreadPostType(kind) {
			t.Errorf("%s is not in a thread", kind)
		}
	}
	if IsThreadPostType("node.medium.text-service.article.v1") {
		t.Error("an article is in a thread")
	}
}
