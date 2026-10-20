package repositories

import (
	"strings"
	"testing"

	"anahita-libs/constants"
	"anahita-libs/videos"

	sq "github.com/Masterminds/squirrel"
)

// Posts are every kind of post; media are photos and videos alone (the
// user, 2026-10-20).
func TestPostsAreEveryPostAndMediaArePicturesAndVideos(t *testing.T) {
	repo := &SearchRepo{}
	base := sq.Select("node.id").From("nodes AS node")

	sql, args, err := repo.getScopeQuery(base, constants.SearchScopePosts).ToSql()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "LIKE") || len(args) != 1 || args[0] != constants.NodeMediumPrefixEntityType+"%" {
		t.Errorf("posts: %s %v", sql, args)
	}

	sql, args, err = repo.getScopeQuery(base, constants.SearchScopeMedium).ToSql()
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 2 || args[0] != constants.NodeMediumPhotoEntityType || args[1] != videos.NodeType {
		t.Errorf("media: %s %v", sql, args)
	}
}
