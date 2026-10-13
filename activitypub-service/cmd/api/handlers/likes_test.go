package handlers

import (
	"fmt"
	"strings"
	"testing"

	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/event"
)

// fakeLikes keeps who likes what, as the graph does.
type fakeLikes struct {
	services.LikesGRPCInterface
	kept map[string]bool
}

func (f *fakeLikes) key(source, target int64) string { return fmt.Sprintf("%d>%d", source, target) }

func (f *fakeLikes) Add(source, target *entities.AnahitaNode, _ *entities.Viewer) error {
	f.kept[f.key(source.ID, target.ID)] = true
	return nil
}
func (f *fakeLikes) Delete(source, target *entities.AnahitaNode) error {
	delete(f.kept, f.key(source.ID, target.ID))
	return nil
}
func (f *fakeLikes) IsLiked(source, target int64) (bool, error) {
	return f.kept[f.key(source, target)], nil
}

func (w *inboxWorld) withLikes() *fakeLikes {
	likes := &fakeLikes{kept: map[string]bool{}}
	w.h.Likes = likes
	return likes
}

func voted(name string, liker, node int64) *event.EventPayload {
	return &event.EventPayload{Name: "edge.voteup.text." + name + ".v1", SubjectID: liker, ObjectID: node}
}

// THE RULE THIS GUARDS: a like made here of something from elsewhere
// reaches whoever wrote it, when whoever likes is on the fediverse; and it
// can be taken back by the name it was given.
func TestALikeOfSomethingFromElsewhereReachesItsAuthor(t *testing.T) {
	w, _ := settledWorld()
	w.withLikes()
	w.fromAlice(received(alicesNote, "<p>Beans?</p>", ""))
	hers := w.keptAt(alicesNote)
	if hers == nil {
		t.Fatal("her note was not kept")
	}
	hers.UUID = "0a0a0a0a-1111-4222-8333-444444444444"
	for _, node := range w.nodes.people {
		if node.ID == hers.ID {
			node.UUID = hers.UUID
		}
	}
	w.queue.sent = nil

	// Heard twice, as every copy of the service hears it: sent once.
	for i := 0; i < 2; i++ {
		_ = w.h.HandleEvent(voted("added", anaID, hers.ID))
	}
	if len(w.queue.sent) != 1 || w.queue.sent[0].target != "https://mastodon.example/inbox" {
		t.Fatalf("queued %+v", w.queue.sent)
	}
	like := w.queue.sent[0].payload
	name := "https://api.example.org/ap/activities/like-" + anaUUID + "-" + hers.UUID
	if like["type"] != "Like" || like["actor"] != anaActor || like["object"] != alicesNote || like["id"] != name {
		t.Errorf("sent %v", like)
	}

	// Taken back, quoting it.
	w.queue.sent = nil
	_ = w.h.HandleEvent(voted("deleted", anaID, hers.ID))
	if len(w.queue.sent) != 1 || w.queue.sent[0].payload["type"] != "Undo" {
		t.Fatalf("queued %+v", w.queue.sent)
	}
	quoted, _ := w.queue.sent[0].payload["object"].(map[string]any)
	if quoted["type"] != "Like" || quoted["id"] != name || quoted["object"] != alicesNote {
		t.Errorf("took back %v", quoted)
	}

	// What stays here.
	for what, change := range map[string]func(){
		"a like by somebody not on the fediverse": func() { w.accounts.enabled[anaID] = false },
		"a like of something written here":        func() { hers = w.note() },
		"a like of something on a refused server": func() { w.blocks.domains["mastodon.example"] = "spam" },
	} {
		w.queue.sent = nil
		change()
		_ = w.h.HandleEvent(voted("added", anaID, hers.ID))
		if len(w.queue.sent) != 0 {
			t.Errorf("%s was sent: %+v", what, w.queue.sent)
		}
		w.accounts.enabled[anaID] = true
		delete(w.blocks.domains, "mastodon.example")
	}
}

func likeOf(what string) string {
	return `{"id":"https://mastodon.example/likes/1","type":"Like","actor":"` + aliceID + `","object":"` + what + `"}`
}

// THE RULE THIS GUARDS: a like from elsewhere counts only for something
// here that other servers were told of, once, and its author is told.
func TestALikeFromElsewhereIsKeptAndTold(t *testing.T) {
	w := notesWorld()
	likes := w.withLikes()
	_ = w.h.HandleEvent(made(noteID))
	var said []string
	w.h.Announce = func(payload *event.EventPayload, name string) error {
		if payload.OwnerID != anaID || payload.ObjectID != noteID || payload.SubjectID == anaID {
			t.Errorf("said of %+v", payload)
		}
		said = append(said, name)
		return nil
	}

	// Alice, who is nobody here, likes Ana's note. Twice over.
	w.fromAlice(likeOf(noteURL))
	w.fromAlice(likeOf(noteURL))
	alice := w.keptAt(aliceID)
	if alice == nil || !likes.kept[likes.key(alice.ID, noteID)] || len(likes.kept) != 1 {
		t.Fatalf("kept %v", likes.kept)
	}
	if strings.Join(said, " ") != "edge.voteup.text.added.v1" {
		t.Errorf("said on the bus: %v", said)
	}

	// Taken back.
	w.fromAlice(`{"id":"https://mastodon.example/undo/1","type":"Undo","actor":"` + aliceID + `","object":` + likeOf(noteURL) + `}`)
	if len(likes.kept) != 0 || len(said) != 2 || said[1] != "edge.voteup.text.deleted.v1" {
		t.Errorf("after it was taken back: %v, said %v", likes.kept, said)
	}

	// What is not kept.
	for what, change := range map[string]func(w *inboxWorld) string{
		"a like of something nobody was sent": func(w *inboxWorld) string { _, _ = w.objects.End(noteID); return noteURL },
		"a like of somebody, not of a note":   func(w *inboxWorld) string { return anaActor },
		"a like of something elsewhere":       func(w *inboxWorld) string { return "https://other.example/n/1" },
		"a like by somebody Ana has blocked": func(w *inboxWorld) string {
			w.relations.relation = &entities.ActorRelation{IsBlocking: true}
			return noteURL
		},
	} {
		w := notesWorld()
		likes := w.withLikes()
		_ = w.h.HandleEvent(made(noteID))
		w.fromAlice(likeOf(change(w)))
		if len(likes.kept) != 0 {
			t.Errorf("%s was kept: %v", what, likes.kept)
		}
	}
}
