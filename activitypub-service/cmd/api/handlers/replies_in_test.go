package handlers

import (
	"net/http"
	"strings"
	"testing"

	"anahita-libs/data/entities"
	"anahita-libs/event"
)

const alicesAnswer = "https://mastodon.example/users/alice/statuses/2"

func (w *inboxWorld) keptAt(uri string) *entities.AnahitaNode {
	found, _ := w.nodes.find(func(n *entities.AnahitaNode) bool { return n.URI == uri })
	return found
}

// answerTo is Alice answering something, as her server sends it.
func answerTo(what string) string {
	return received(alicesAnswer, "<p>Mine are up too.</p>", `,"inReplyTo":"`+what+`"`)
}

// THE RULE THIS GUARDS: somebody elsewhere answers a thread here only by
// the thread's own rule for who may reply, and only a post other servers
// were told of.
func TestAReplyFromElsewhereIsKeptByTheThreadsRule(t *testing.T) {
	// Alice is nobody here: not followed, not known.
	open := func() *inboxWorld {
		w := notesWorld()
		w.note().CommentStatus = true
		_ = w.h.HandleEvent(made(noteID))
		return w
	}

	// Ana's note went out, and anybody may reply to it.
	w := open()
	var said []string
	w.h.Announce = func(payload *event.EventPayload, name string) error {
		said = append(said, name)
		return nil
	}
	if rec := w.fromAlice(answerTo(noteURL)); rec.Code != http.StatusAccepted {
		t.Fatalf("%d", rec.Code)
	}
	kept := w.keptAt(alicesAnswer)
	if kept == nil {
		t.Fatal("her reply was not kept")
	}
	alice := w.keptAt(aliceID)
	if alice == nil || kept.OwnerID != alice.ID || kept.ParentID != noteID || kept.RootID != noteID || kept.Access != "" || kept.Body != "Mine are up too." {
		t.Errorf("kept as %+v", kept)
	}
	if strings.Join(said, " ") != "federation.medium.text-service.note.added.v1" {
		t.Errorf("said on the bus: %v", said)
	}

	// Taken away where it was written, it is taken away here.
	w.fromAlice(`{"id":"https://mastodon.example/d/2","type":"Delete","actor":"` + aliceID + `","object":{"id":"` + alicesAnswer + `","type":"Tombstone"}}`)
	if w.keptAt(alicesAnswer) != nil {
		t.Error("her reply was still kept")
	}
	// And said to be gone, so that whoever was told of it is not left with
	// news that leads nowhere.
	if len(said) != 2 || said[1] != "federation.medium.text-service.note.deleted.v1" {
		t.Errorf("said on the bus after it went: %v", said)
	}

	cases := map[string]func(w *inboxWorld){
		"the thread is closed to replies":         func(w *inboxWorld) { w.note().CommentStatus = false },
		"nobody may reply":                        func(w *inboxWorld) { w.note().ReplyAccess = "nobody" },
		"only followers may, and she is not one":  func(w *inboxWorld) { w.note().ReplyAccess = "followers" },
		"only those Ana follows, and she is not":  func(w *inboxWorld) { w.note().ReplyAccess = "following" },
		"Ana has blocked her":                     func(w *inboxWorld) { w.relations.relation = &entities.ActorRelation{IsBlocking: true} },
		"the note was closed to followers since":  func(w *inboxWorld) { w.note().Access = "followers" },
		"the note is no longer one that went out": func(w *inboxWorld) { _, _ = w.objects.End(noteID) },
		"her server is refused":                   func(w *inboxWorld) { w.blocks.domains["mastodon.example"] = "spam" },
	}
	for name, change := range cases {
		w := open()
		change(w)
		w.fromAlice(answerTo(noteURL))
		if w.keptAt(alicesAnswer) != nil {
			t.Errorf("%s: her reply was kept", name)
		}
	}

	// Who the author lets reply is measured by the follows between them.
	for name, rel := range map[string]*entities.ActorRelation{
		"followers": {IsLeading: true},
		"following": {IsFollowing: true},
	} {
		w := open()
		w.note().ReplyAccess = name
		w.relations.relation = rel
		w.fromAlice(answerTo(noteURL))
		if w.keptAt(alicesAnswer) == nil {
			t.Errorf("limited to %s, and she is one: her reply was not kept", name)
		}
	}

	// What cannot be answered from elsewhere.
	for name, what := range map[string]string{
		"something nobody here has":          "https://other.example/n/9",
		"an address here that is no note":    "https://api.example.org/ap/users/" + anaUUID,
		"a note here that nobody was sent":   "https://api.example.org/ap/notes/" + replyUUID,
		"an address here that is nothing":    "https://api.example.org/ap/notes/not-a-uuid",
		"the page of the note, not the note": "https://example.org/notes/500",
	} {
		w := open()
		w.nodes.people = append(w.nodes.people, anasReply(noteID, noteID))
		w.fromAlice(answerTo(what))
		if w.keptAt(alicesAnswer) != nil {
			t.Errorf("answering %s: kept", name)
		}
	}
}

// Mastodon begins a reply with the name of whoever is answered, as a link.
func TestSomebodyHereNamedInAPostFromElsewhere(t *testing.T) {
	w := notesWorld()
	for written, want := range map[string]string{
		"[@ana@example.org](https://example.org/people/ana) Mine too.":                                "@ana Mine too.",
		"[@ana](https://api.example.org/ap/users/" + anaUUID + ") Mine too.":                          "@ana Mine too.",
		"[@bob@other.example](https://other.example/@bob) and [@ana](https://example.org/people/ana)": "[@bob@other.example](https://other.example/@bob) and @ana",
		// Reads as one name and leads to another: left to be seen.
		"[@alice](https://example.org/people/ana) hello":   "[@alice](https://example.org/people/ana) hello",
		"[@ana](https://example.org/people/ana/settings)":  "[@ana](https://example.org/people/ana/settings)",
		"[a page](https://example.org/people/ana) nothing": "[a page](https://example.org/people/ana) nothing",
	} {
		if got := w.h.namedHere(written); got != want {
			t.Errorf("%q:\n got %q\nwant %q", written, got, want)
		}
	}
}

// Under a thread that began elsewhere, the rule every note from elsewhere
// is kept by: somebody here follows whoever wrote it.
func TestAReplyInAThreadFromElsewhere(t *testing.T) {
	w, _ := settledWorld()
	w.fromAlice(received(alicesNote, "<p>Beans?</p>", ""))
	hers := w.keptAt(alicesNote)
	if hers == nil {
		t.Fatal("her note was not kept")
	}

	// Alice answers herself, and Ana follows her.
	w.fromAlice(answerTo(alicesNote))
	kept := w.keptAt(alicesAnswer)
	if kept == nil || kept.ParentID != hers.ID || kept.RootID != hers.ID {
		t.Fatalf("her reply to herself: %+v", kept)
	}

	// Nobody here follows her: nothing more of hers is kept.
	w2, alice := settledWorld()
	w2.fromAlice(received(alicesNote, "<p>Beans?</p>", ""))
	_ = w2.follows.Delete(anaID, alice.NodeID, "out")
	w2.fromAlice(answerTo(alicesNote))
	if w2.keptAt(alicesAnswer) != nil {
		t.Error("a reply from somebody nobody follows was kept")
	}
}
