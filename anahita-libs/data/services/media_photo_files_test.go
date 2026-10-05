package services

import (
	"errors"
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

type fakePhotoFiles struct {
	asked [][]int64
	files []*entities.PhotoFile
	err   error
}

func (f *fakePhotoFiles) BrowsePhotoFiles(nodeIDs []int64) ([]*entities.PhotoFile, error) {
	f.asked = append(f.asked, nodeIDs)
	return f.files, f.err
}

func photoNode(id, owner int64) *entities.AnahitaNode {
	node := entities.NewAnahitaNode()
	node.ID = id
	node.Type = constants.NodeMediumPhotoEntityType
	node.Service = constants.PhotoServiceName
	node.OwnerID = owner
	return node
}

// One call for the whole page, asking only about the photos on it.
func TestAttachPhotoFilesAsksOnceAndOnlyAboutPhotos(t *testing.T) {
	note := entities.NewAnahitaNode()
	note.ID = 5
	note.Type = "node.medium.text-service.note.v1"

	first, second := photoNode(10, 1), photoNode(11, 8)

	media := &fakePhotoFiles{files: []*entities.PhotoFile{
		{NodeID: 10, Position: 0, Filename: "a.jpg", AltText: "a heron"},
		{NodeID: 10, Position: 1, Filename: "b.jpg"},
		{NodeID: 11, Position: 0, Filename: "c.png"},
	}}

	AttachPhotoFiles(media, []*entities.AnahitaNode{note, first, nil, second})

	if len(media.asked) != 1 || len(media.asked[0]) != 2 {
		t.Fatalf("asked %v; want one call about the two photos", media.asked)
	}
	if note.Files != nil {
		t.Error("a note was given images")
	}
	if len(first.Files) != 2 || len(second.Files) != 1 {
		t.Fatalf("images attached: %d and %d, want 2 and 1", len(first.Files), len(second.Files))
	}
	if first.Files[0].AltText != "a heron" {
		t.Errorf("the description was lost: %+v", first.Files[0])
	}

	// Stored under the profile the post is on.
	if got := first.Files[1].Portrait[entities.PortraitSizeMedium].Url; got != "assets/v2/photo-service/photo/1/b_medium.jpg" {
		t.Errorf("second image of the first post is at %q", got)
	}
	if got := second.Files[0].Portrait[entities.PortraitSizeMedium].Url; got != "assets/v2/photo-service/photo/8/c_medium.png" {
		t.Errorf("image of the post in the group is at %q", got)
	}
}

// The same post twice on one page, as itself and as what somebody reposted:
// both get the images, and neither's addresses are the other's.
func TestAttachPhotoFilesGivesEachCopyOfAPostItsOwnImages(t *testing.T) {
	one, again := photoNode(10, 1), photoNode(10, 1)
	media := &fakePhotoFiles{files: []*entities.PhotoFile{{NodeID: 10, Filename: "a.jpg"}}}

	AttachPhotoFiles(media, []*entities.AnahitaNode{one, again})

	if len(media.asked[0]) != 1 {
		t.Errorf("asked about %v; want the post once", media.asked[0])
	}
	if len(one.Files) != 1 || len(again.Files) != 1 || one.Files[0] == again.Files[0] {
		t.Fatal("the two copies do not each have their own image")
	}
}

// Nothing to ask about is no call at all, and a failure leaves the post with
// the one image its node names.
func TestAttachPhotoFilesWithoutPhotosOrWithAFailure(t *testing.T) {
	media := &fakePhotoFiles{}
	AttachPhotoFiles(media, []*entities.AnahitaNode{entities.NewAnahitaNode()})
	if len(media.asked) != 0 {
		t.Error("asked about a page with no photos on it")
	}

	failing := &fakePhotoFiles{err: errors.New("graph is away")}
	photo := photoNode(10, 1)
	AttachPhotoFiles(failing, []*entities.AnahitaNode{photo})
	if photo.Files != nil {
		t.Error("images attached despite the failure")
	}
}

type fakePresigner struct {
	FilesGRPCInterface
	asked []string
}

func (f *fakePresigner) BatchPresign(paths []string) ([]string, error) {
	f.asked = paths
	out := make([]string, len(paths))
	for i, path := range paths {
		out[i] = "https://media.example/" + path
	}
	return out, nil
}

func TestBatchPresignPhotoFilesSignsEverySizeOfEveryImageAtOnce(t *testing.T) {
	photo := photoNode(10, 1)
	photo.Files = []*entities.PhotoFile{{Filename: "a.jpg"}, {Filename: "b.jpg"}}
	photo.SetFilePortraits()

	sizes := len(photo.GetPortraitSizes())
	files := &fakePresigner{}

	BatchPresignPhotoFiles(files, []*entities.AnahitaNode{photo, nil})

	if len(files.asked) != 2*sizes {
		t.Fatalf("signed %d paths, want %d", len(files.asked), 2*sizes)
	}
	if got := photo.Files[1].Portrait[entities.PortraitSizeSmall].Url; got != "https://media.example/assets/v2/photo-service/photo/1/b_small.jpg" {
		t.Errorf("second image, small: %q", got)
	}
}
