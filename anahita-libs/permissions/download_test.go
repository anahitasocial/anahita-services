package permissions

import (
	"testing"
	"time"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/policy"
	"anahita-libs/videos"
)

func downloadable(kind string, allow bool) *entities.AnahitaNode {
	post := entities.NewAnahitaNode()
	post.ID = 12
	post.Type = kind
	post.CreatedBy = 5
	post.AllowDownload = allow
	return post
}

func signedIn(id int64) *entities.Viewer {
	viewer := entities.NewViewer()
	viewer.ID = id
	viewer.Authenticated = true
	return viewer
}

// Nobody is taken to have agreed: a post is downloaded only once whoever
// posted it has said so, and by them whatever they said.
func TestAPostIsDownloadedOnlyWhenItsAuthorSaidSo(t *testing.T) {
	photo, video := constants.NodeMediumPhotoEntityType, videos.NodeType

	for name, c := range map[string]struct {
		viewer *entities.Viewer
		post   *entities.AnahitaNode
		want   bool
	}{
		"allowed, to anybody signed in":  {signedIn(9), downloadable(photo, true), true},
		"allowed, to a visitor":          {entities.NewViewer(), downloadable(video, true), true},
		"not allowed, to somebody else":  {signedIn(9), downloadable(photo, false), false},
		"not allowed, to a visitor":      {entities.NewViewer(), downloadable(photo, false), false},
		"not allowed, to its own author": {signedIn(5), downloadable(video, false), true},
		"a note has nothing to download": {signedIn(5), downloadable("node.medium.text-service.note.v1", true), false},
		"nobody at all":                  {nil, downloadable(photo, false), false},
		"no post":                        {signedIn(5), nil, false},
	} {
		if got := CanDownload(c.viewer, c.post); got != c.want {
			t.Errorf("%s: got %v", name, got)
		}
	}

	removed := downloadable(photo, true)
	now := time.Now()
	removed.DeletedAt = &now
	if CanDownload(signedIn(5), removed) {
		t.Errorf("a removed post was downloadable")
	}
}

// Where visitors are shown only the start of things, they are sent small
// pictures and nothing to keep.
func TestAVisitorOnAPreviewSiteDownloadsNothing(t *testing.T) {
	defer policy.SetReadAccessForTest(policy.ReadPreview)()

	post := downloadable(constants.NodeMediumPhotoEntityType, true)
	if CanDownload(entities.NewViewer(), post) {
		t.Errorf("a visitor on a preview site could download")
	}
	if !CanDownload(signedIn(9), post) {
		t.Errorf("somebody signed in on a preview site could not")
	}
}
