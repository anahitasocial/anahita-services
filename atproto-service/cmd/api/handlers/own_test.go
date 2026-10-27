package handlers

import (
	"testing"

	"anahita-libs/data/entities"
)

// A thread somebody posts on Bluesky with their own account arrives here as
// a post and its replies, each under the one it answers; a reply to
// something not here is left.
func TestAThreadPostedOnBlueskyArrivesWithItsReplies(t *testing.T) {
	w, _ := writingWorld(t)
	w.connect()
	w.h.Media = &fakeMedia{nodes: w.nodes}
	account := w.accounts.kept[anaID]

	posted := func(rkey, raw string) *entities.AnahitaNode {
		t.Helper()
		before := len(w.nodes.kept)
		if err := w.h.ownPosted(account, rkey, aPost(t, raw)); err != nil {
			t.Fatal(err)
		}
		if len(w.nodes.kept) == before {
			return nil
		}
		return w.nodes.kept[len(w.nodes.kept)-1]
	}
	reply := func(text, parent string) string {
		uri := "at://" + account.DID + "/app.bsky.feed.post/"
		return `{"text":"` + text + `","reply":{"root":{"uri":"` + uri + `3l3one","cid":"c"},"parent":{"uri":"` + uri + parent + `","cid":"c"}}}`
	}

	first := posted("3l3one", `{"text":"1/3 A long thought"}`)
	if first == nil || first.ParentID != 0 {
		t.Fatalf("the thread's first post: %+v", first)
	}
	second := posted("3l3two", reply("2/3 and more", "3l3one"))
	if second == nil || second.ParentID != first.ID {
		t.Fatalf("the second, under the first: %+v", second)
	}
	third := posted("3l3thr", reply("3/3 the end", "3l3two"))
	if third == nil || third.ParentID != second.ID {
		t.Fatalf("the third, under the second: %+v", third)
	}

	// Again, as it would be if heard twice: nothing more.
	if again := posted("3l3thr", reply("3/3 the end", "3l3two")); again != nil {
		t.Errorf("a reply kept twice")
	}

	// An answer to a post of somebody on Bluesky that nobody here keeps.
	elsewhere := `{"text":"Agreed","reply":{"root":{"uri":"at://did:plc:stranger/app.bsky.feed.post/3l3abc","cid":"c"},"parent":{"uri":"at://did:plc:stranger/app.bsky.feed.post/3l3abc","cid":"c"}}}`
	if kept := posted("3l3ans", elsewhere); kept != nil {
		t.Errorf("a reply to something not here was kept: %+v", kept)
	}
}
