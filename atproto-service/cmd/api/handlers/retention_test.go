package handlers

import (
	"strings"
	"testing"

	"atproto/bsky"

	"anahita-libs/event"
)

// THE RULE THIS GUARDS: what a rule says is to go, goes as it would have
// had Bluesky said so: the post with what is stored of its pictures, said
// on the bus, and then whoever nothing is left of. Somebody followed since
// they were found is not among them.
func TestWhatIsDueToGoIsRemovedAsIfTakenAwayThere(t *testing.T) {
	w := newWorld(t)
	alice := w.alice(true)

	const pictures = `"embed":{"$type":"app.bsky.embed.images","images":[
		{"alt":"A row of beans","image":{"ref":{"$link":"` + aCID + `"},"mimeType":"image/jpeg"}}]}`
	if err := w.h.keepPost(alice, "3l3photo", aPost(t, `{"text":"The beans",`+pictures+`}`)); err != nil {
		t.Fatal(err)
	}
	kept := w.keptAt(bsky.PostURI(aliceDID, "3l3photo"))
	if kept == nil {
		t.Fatal("the post was not kept to begin with")
	}
	stored := len(w.files.stored)
	// Whoever followed her from here has stopped.
	w.actors.followed[alice.NodeID] = false

	// A post that is not there any more is passed over.
	posts, people := w.h.Tidy([]int64{kept.ID, 99999}, nil)
	if posts != 1 || people != 0 {
		t.Fatalf("removed %d posts and %d people", posts, people)
	}
	if w.keptAt(bsky.PostURI(aliceDID, "3l3photo")) != nil || len(w.files.stored) >= stored {
		t.Errorf("still kept, or its pictures are: %d files of %d", len(w.files.stored), stored)
	}
	if last := w.said[len(w.said)-1]; !strings.HasPrefix(last, event.RemoteNoteDeleted) {
		t.Errorf("said %q", last)
	}

	// Followed between being found and being removed: she stays.
	w.actors.followed[alice.NodeID] = true
	if _, people := w.h.Tidy(nil, []int64{alice.NodeID}); people != 0 {
		t.Fatal("somebody followed from here was removed")
	}
	w.actors.followed[alice.NodeID] = false
	if _, people := w.h.Tidy(nil, []int64{alice.NodeID, 99999}); people != 1 {
		t.Fatalf("removed %d people", people)
	}
	if again, _ := w.actors.ReadByDID(aliceDID); again != nil {
		t.Errorf("still known: %+v", again)
	}
}
