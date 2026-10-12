package purge

import (
	"slices"
	"strings"
	"testing"

	"anahita-libs/data/entities"
	"anahita-libs/videos"
)

// A video's files are named nowhere but in its post's meta. A purge that
// did not read them there would leave every video its owner posted in
// storage, after the account that was promised their removal is gone.
func TestAVideosFilesAreRemovedWithIt(t *testing.T) {
	folder := videos.Folder(9, "0b8f6a52-1f0e-4a7e-9a52-0a6f6f1f0e4a")
	meta, err := videos.IntoMeta(nil, &videos.Video{
		Poster: videos.PosterKey(folder),
		Renditions: []videos.Rendition{
			{Height: 720, Key: videos.RenditionKey(folder, 720)},
			{Height: 360, Key: videos.RenditionKey(folder, 360)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	node := entities.NewAnahitaNode()
	node.ID = 77
	node.Type = videos.NodeType
	node.Meta = meta

	paths := filePaths(node)
	for _, want := range []string{folder + "720.mp4", folder + "360.mp4", folder + "poster.jpg"} {
		if !slices.Contains(paths, want) {
			t.Errorf("%s is not removed; removed: %v", want, paths)
		}
	}

	// Meta that says the same on a post that is not a video names nothing.
	node.Type = "node.medium.text-service.note.v1"
	for _, path := range filePaths(node) {
		if strings.HasPrefix(path, videos.StoragePrefix) {
			t.Errorf("a note was taken to have a video's files: %s", path)
		}
	}
}
