package handlers

import (
	"fmt"
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

// THE RULE THIS GUARDS: a photo post written here goes out as a note with
// its pictures, each at an address of this installation's that gives the
// picture only while the post is one that goes out.
func TestAPhotoPostGoesOutWithItsPictures(t *testing.T) {
	picturesWait = 0

	w := notesWorld()
	files := &fakeFiles{stored: map[string]int{}}
	listed := &fakePhotoFiles{kept: map[int64][]*entities.PhotoFile{}}
	w.h.Files, w.h.PhotoFiles = files, listed
	w.note().Type = constants.NodeMediumPhotoEntityType
	w.note().Service = "photo-service"
	w.note().Body = "The beans #garden"

	photoMade := made(noteID)
	photoMade.ObjectType = constants.NodeMediumPhotoEntityType

	// Heard of before its pictures are listed: not sent bare.
	_ = w.h.HandleEvent(photoMade)
	if len(w.queue.sent) != 0 {
		t.Fatalf("a photo post went out without its pictures: %+v", w.queue.sent)
	}

	listed.kept[noteID] = []*entities.PhotoFile{
		{Filename: "one.png", Mimetype: "image/png", Width: 800, Height: 600, AltText: "A row of beans"},
		{Filename: "two.jpg", Mimetype: "image/jpeg", Width: 640, Height: 480},
	}
	_ = w.h.HandleEvent(photoMade)
	if len(w.queue.sent) != 2 || w.queue.sent[0].payload["type"] != "Create" {
		t.Fatalf("queued %+v", w.queue.sent)
	}
	object, _ := w.queue.sent[0].payload["object"].(map[string]any)
	if object["type"] != "Note" || object["url"] != "https://example.org/photos/500" {
		t.Errorf("sent as %v at %v", object["type"], object["url"])
	}
	attached, _ := object["attachment"].([]any)
	if len(attached) != 2 {
		t.Fatalf("attachments %v", object["attachment"])
	}
	first, _ := attached[0].(map[string]any)
	if first["url"] != noteURL+"/pictures/0" || first["mediaType"] != "image/png" || first["name"] != "A row of beans" || first["width"] != float64(800) {
		t.Errorf("the first picture: %v", first)
	}

	// The picture itself, for a server that asks: sent on to storage.
	if rec := w.get(w.h.Picture, "/", "uuid", noteUUID, "position", "1"); rec.Code != 302 {
		t.Errorf("a picture of a post that went out: %d", rec.Code)
	}
	// Not one there is not, and not once the post is closed.
	if rec := w.get(w.h.Picture, "/", "uuid", noteUUID, "position", "2"); rec.Code != 404 {
		t.Errorf("a picture there is not: %d", rec.Code)
	}
	w.note().Access = "followers"
	if rec := w.get(w.h.Picture, "/", "uuid", noteUUID, "position", "0"); rec.Code != 404 {
		t.Errorf("a picture of a post that was closed: %d", rec.Code)
	}
}

// More pictures than Mastodon shows: every one is sent, and the words end
// with a link to the post here, where they all are (roadmap row 100).
func TestAPhotoPostWithMorePicturesThanMastodonShows(t *testing.T) {
	picturesWait = 0

	w := notesWorld()
	listed := &fakePhotoFiles{kept: map[int64][]*entities.PhotoFile{}}
	w.h.Files, w.h.PhotoFiles = &fakeFiles{stored: map[string]int{}}, listed
	w.note().Type = constants.NodeMediumPhotoEntityType
	w.note().Service = "photo-service"
	w.note().Body = "The garden"
	for i := 0; i < 6; i++ {
		listed.kept[noteID] = append(listed.kept[noteID], &entities.PhotoFile{Filename: fmt.Sprintf("%d.png", i), Mimetype: "image/png"})
	}

	photoMade := made(noteID)
	photoMade.ObjectType = constants.NodeMediumPhotoEntityType
	_ = w.h.HandleEvent(photoMade)
	if len(w.queue.sent) == 0 {
		t.Fatal("nothing sent")
	}
	object, _ := w.queue.sent[0].payload["object"].(map[string]any)
	if attached, _ := object["attachment"].([]any); len(attached) != 6 {
		t.Errorf("%d attachments", len(attached))
	}
	content, _ := object["content"].(string)
	if want := `<p><a href="https://example.org/photos/500">example.org/photos/500</a></p>`; !strings.HasSuffix(content, want) {
		t.Errorf("content %q", content)
	}
	for language, words := range object["contentMap"].(map[string]any) {
		if !strings.HasSuffix(words.(string), "example.org/photos/500</a></p>") {
			t.Errorf("contentMap[%s] %q", language, words)
		}
	}
}
