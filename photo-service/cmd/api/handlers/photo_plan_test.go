package handlers

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"strings"
	"testing"
)

const (
	testViewer = int64(7)
	testOwner  = int64(7)
)

func upload(id int64, name string) *entities.PhotoUpload {
	return &entities.PhotoUpload{
		ID: id, OwnerID: testOwner, CreatedBy: testViewer,
		Filename: name, Mimetype: "image/jpeg", Width: 800, Height: 600,
	}
}

func existingFile(position int, name, alt string) *entities.PhotoFile {
	return &entities.PhotoFile{NodeID: 1, Position: position, Filename: name, AltText: alt}
}

func names(files []*entities.PhotoFile) string {
	var out []string
	for _, file := range files {
		out = append(out, file.Filename)
	}
	return strings.Join(out, ",")
}

func TestANewPostTakesItsUploadsInTheOrderGiven(t *testing.T) {
	plan, refused := planFiles(nil,
		[]*entities.PhotoUpload{upload(1, "a.jpg"), upload(2, "b.jpg"), upload(3, "c.jpg")},
		[]fileRef{{UploadID: 3, AltText: "third"}, {UploadID: 1}, {UploadID: 2}},
		testViewer, testOwner,
	)
	if refused != "" {
		t.Fatalf("refused: %s", refused)
	}
	if got := names(plan.Files); got != "c.jpg,a.jpg,b.jpg" {
		t.Errorf("order = %s", got)
	}
	if plan.Files[0].AltText != "third" || plan.Files[0].Width != 800 {
		t.Errorf("the first image lost its description or size: %+v", plan.Files[0])
	}
	for i, file := range plan.Files {
		if file.Position != i {
			t.Errorf("image %d has position %d", i, file.Position)
		}
	}
	if len(plan.UploadIDs) != 3 || len(plan.Removed) != 0 {
		t.Errorf("uploads used = %v, removed = %d", plan.UploadIDs, len(plan.Removed))
	}
}

func TestAPostNeedsAnImageAndHoldsNoMoreThanTheLimit(t *testing.T) {
	if _, refused := planFiles(nil, nil, nil, testViewer, testOwner); refused != errFilesRequired {
		t.Errorf("no images: %q", refused)
	}

	var uploads []*entities.PhotoUpload
	var wanted []fileRef
	for i := int64(1); i <= constants.PhotoMaxFiles+1; i++ {
		uploads = append(uploads, upload(i, strings.Repeat("x", int(i))+".jpg"))
		wanted = append(wanted, fileRef{UploadID: i})
	}

	if _, refused := planFiles(nil, uploads, wanted, testViewer, testOwner); refused != errTooManyFiles {
		t.Errorf("one over the limit: %q", refused)
	}
	if _, refused := planFiles(nil, uploads, wanted[:constants.PhotoMaxFiles], testViewer, testOwner); refused != "" {
		t.Errorf("exactly the limit was refused: %q", refused)
	}
}

func TestSomebodyElsesUploadIsRefused(t *testing.T) {
	theirs := upload(1, "a.jpg")
	theirs.CreatedBy = 99

	if _, refused := planFiles(nil, []*entities.PhotoUpload{theirs}, []fileRef{{UploadID: 1}}, testViewer, testOwner); refused != errUploadNotYours {
		t.Errorf("another person's upload: %q", refused)
	}
}

// An image is stored under the profile it was uploaded for. Used on another
// profile its addresses would point at nothing.
func TestAnUploadForAnotherProfileIsRefused(t *testing.T) {
	elsewhere := upload(1, "a.jpg")
	elsewhere.OwnerID = 50

	if _, refused := planFiles(nil, []*entities.PhotoUpload{elsewhere}, []fileRef{{UploadID: 1}}, testViewer, testOwner); refused != errUploadNotYours {
		t.Errorf("an upload for another profile: %q", refused)
	}
}

func TestAnUploadThatIsGoneIsNotFound(t *testing.T) {
	if _, refused := planFiles(nil, nil, []fileRef{{UploadID: 5}}, testViewer, testOwner); refused != errFileNotFound {
		t.Errorf("a missing upload: %q", refused)
	}
	if _, refused := planFiles(nil, nil, []fileRef{{}}, testViewer, testOwner); refused != errFileNotFound {
		t.Errorf("an entry naming nothing: %q", refused)
	}
}

func TestEditingReordersRemovesAddsAndDescribes(t *testing.T) {
	existing := []*entities.PhotoFile{
		existingFile(0, "a.jpg", "was a"),
		existingFile(1, "b.jpg", ""),
		existingFile(2, "c.jpg", ""),
	}

	plan, refused := planFiles(existing,
		[]*entities.PhotoUpload{upload(9, "new.jpg")},
		[]fileRef{{ID: "c.jpg", AltText: "now first"}, {UploadID: 9}, {ID: "a.jpg"}},
		testViewer, testOwner,
	)
	if refused != "" {
		t.Fatalf("refused: %s", refused)
	}
	if got := names(plan.Files); got != "c.jpg,new.jpg,a.jpg" {
		t.Errorf("order = %s", got)
	}
	if plan.Files[0].AltText != "now first" {
		t.Errorf("the description was not taken: %q", plan.Files[0].AltText)
	}
	// Leaving the description out clears it: the request is the whole of
	// what the post is to have.
	if plan.Files[2].AltText != "" {
		t.Errorf("a description left out was kept: %q", plan.Files[2].AltText)
	}
	if got := names(plan.Removed); got != "b.jpg" {
		t.Errorf("removed = %s", got)
	}
	if len(plan.UploadIDs) != 1 || plan.UploadIDs[0] != 9 {
		t.Errorf("uploads used = %v", plan.UploadIDs)
	}
	// The post's own list is not touched until the change is saved.
	if existing[0].AltText != "was a" || existing[2].Position != 2 {
		t.Error("planning changed the images the post has now")
	}
}

func TestAnImageOfAnotherPostIsNotFound(t *testing.T) {
	existing := []*entities.PhotoFile{existingFile(0, "a.jpg", "")}

	if _, refused := planFiles(existing, nil, []fileRef{{ID: "other.jpg"}}, testViewer, testOwner); refused != errFileNotFound {
		t.Errorf("an image the post does not have: %q", refused)
	}
}

func TestNoImageTwice(t *testing.T) {
	existing := []*entities.PhotoFile{existingFile(0, "a.jpg", "")}

	if _, refused := planFiles(existing, nil, []fileRef{{ID: "a.jpg"}, {ID: "a.jpg"}}, testViewer, testOwner); refused != errFileRepeated {
		t.Errorf("the same image twice: %q", refused)
	}
	if _, refused := planFiles(nil, []*entities.PhotoUpload{upload(1, "a.jpg")}, []fileRef{{UploadID: 1}, {UploadID: 1}}, testViewer, testOwner); refused != errFileRepeated {
		t.Errorf("the same upload twice: %q", refused)
	}
}

// Counted in characters, not bytes: a description in Persian or Japanese
// gets the same room as one in English.
func TestADescriptionIsLimitedInCharacters(t *testing.T) {
	uploads := []*entities.PhotoUpload{upload(1, "a.jpg")}

	atLimit := strings.Repeat("ب", constants.PhotoAltTextMaxLength)
	if _, refused := planFiles(nil, uploads, []fileRef{{UploadID: 1, AltText: atLimit}}, testViewer, testOwner); refused != "" {
		t.Errorf("a description at the limit was refused: %q", refused)
	}
	if _, refused := planFiles(nil, uploads, []fileRef{{UploadID: 1, AltText: atLimit + "ب"}}, testViewer, testOwner); refused != errAltTextTooLong {
		t.Errorf("a description over the limit: %q", refused)
	}
}
