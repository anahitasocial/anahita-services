package handlers

import (
	"strings"
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// fakePhotoFiles keeps each post's list of pictures, as the graph does.
type fakePhotoFiles struct {
	kept map[int64][]*entities.PhotoFile
}

func (f *fakePhotoFiles) BrowsePhotoFiles(ids []int64) ([]*entities.PhotoFile, error) {
	var all []*entities.PhotoFile
	for _, id := range ids {
		all = append(all, f.kept[id]...)
	}
	return all, nil
}
func (f *fakePhotoFiles) SetPhotoFiles(id int64, files []*entities.PhotoFile, _ []int64) ([]*entities.PhotoFile, error) {
	f.kept[id] = files
	return files, nil
}

func withPictures(extra string) string {
	return `,"attachment":[` +
		`{"type":"Document","mediaType":"image/png","url":"https://files.mastodon.example/1.png","name":"A row of beans"},` +
		`{"type":"Document","mediaType":"video/mp4","url":"https://files.mastodon.example/2.mp4"},` +
		`{"type":"Document","mediaType":"image/jpeg","url":"http://files.mastodon.example/plain.jpg"},` +
		`{"type":"Image","mediaType":"image/png","url":"https://files.mastodon.example/3.png"}]` + extra
}

// THE RULE THIS GUARDS: a post that comes with pictures is a photo post
// here, with its pictures copied into this installation's own storage and
// never shown from where they came from.
func TestAPostWithPicturesIsAPhotoPost(t *testing.T) {
	w, _ := settledWorld()
	pictures := &fakePictures{content: aPicture(t)}
	files := &fakeFiles{stored: map[string]int{}}
	listed := &fakePhotoFiles{kept: map[int64][]*entities.PhotoFile{}}
	w.h.Pictures, w.h.Files, w.h.PhotoFiles = pictures, files, listed

	w.fromAlice(received(alicesNote, "<p>The beans</p>", withPictures("")))
	kept := w.keptAt(alicesNote)
	if kept == nil || kept.Type != constants.NodeMediumPhotoEntityType || kept.Body != "The beans" {
		t.Fatalf("kept as %+v", kept)
	}

	// The two that say they are pictures, at addresses on the web that are
	// asked for safely: not the video, and not the one over plain http.
	if got := strings.Join(pictures.asked, " "); got != "https://files.mastodon.example/1.png https://files.mastodon.example/3.png" {
		t.Errorf("asked for %q", got)
	}
	its := listed.kept[kept.ID]
	if len(its) != 2 || its[0].AltText != "A row of beans" || its[0].Filename == "" || its[0].Width == 0 {
		t.Fatalf("listed %+v", its)
	}
	if len(files.stored) == 0 {
		t.Fatal("nothing was stored")
	}
	for path := range files.stored {
		if strings.Contains(path, "mastodon.example") {
			t.Errorf("stored under %q", path)
		}
	}

	// Told of again, its pictures are not fetched again.
	asked := len(pictures.asked)
	w.fromAlice(strings.Replace(received(alicesNote, "<p>The beans, at last</p>", withPictures("")), `"type":"Create"`, `"type":"Update"`, 1))
	if again := w.keptAt(alicesNote); again == nil || again.Type != constants.NodeMediumPhotoEntityType || len(pictures.asked) != asked {
		t.Errorf("after a change: %+v, asked %d more", again, len(pictures.asked)-asked)
	}

	// Taken away there: gone here, and so is what was stored of it.
	stored := len(files.stored)
	w.fromAlice(`{"id":"https://mastodon.example/d/1","type":"Delete","actor":"` + aliceID + `","object":{"id":"` + alicesNote + `","type":"Tombstone"}}`)
	if w.keptAt(alicesNote) != nil || len(files.deleted) != stored || len(files.stored) != 0 {
		t.Errorf("after it was taken away: stored %d, deleted %d of %d", len(files.stored), len(files.deleted), stored)
	}
}

func TestPicturesThatAreNotKept(t *testing.T) {
	// Marked as not for everybody: its words are kept, and no picture.
	w, _ := settledWorld()
	pictures := &fakePictures{content: aPicture(t)}
	w.h.Pictures, w.h.Files, w.h.PhotoFiles = pictures, &fakeFiles{stored: map[string]int{}}, &fakePhotoFiles{kept: map[int64][]*entities.PhotoFile{}}

	w.fromAlice(received(alicesNote, "<p>Careful</p>", withPictures(`,"sensitive":true`)))
	if kept := w.keptAt(alicesNote); kept == nil || kept.Type != constants.NodeMediumNoteEntityType || len(pictures.asked) != 0 {
		t.Errorf("a sensitive post: %+v, asked %v", kept, pictures.asked)
	}

	// Pictures and no words: still a post.
	other := "https://mastodon.example/users/alice/statuses/6"
	w.fromAlice(received(other, "", withPictures("")))
	if kept := w.keptAt(other); kept == nil || kept.Type != constants.NodeMediumPhotoEntityType {
		t.Errorf("pictures alone: %+v", kept)
	}

	// Something that is not a picture, whatever it says it is.
	bad, _ := settledWorld()
	listed := &fakePhotoFiles{kept: map[int64][]*entities.PhotoFile{}}
	bad.h.Pictures, bad.h.Files, bad.h.PhotoFiles = &fakePictures{content: []byte("<script>alert(1)</script>")}, &fakeFiles{stored: map[string]int{}}, listed
	bad.fromAlice(received(alicesNote, "<p>The beans</p>", withPictures("")))
	if kept := bad.keptAt(alicesNote); kept == nil || len(listed.kept) != 0 {
		t.Errorf("what is not a picture: %+v, listed %v", kept, listed.kept)
	}
}
