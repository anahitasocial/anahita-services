package handlers

import (
	"fmt"
	"strings"
	"testing"

	"activitypub/data/repositories"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/event"
)

// fakeReposts keeps who reposted what, as the graph does, and makes a node
// of each as the graph does.
type fakeReposts struct {
	services.RepostGRPCInterface
	nodes *fakeNodes
	kept  map[string]int64
}

func (f *fakeReposts) key(author, parent int64) string { return fmt.Sprintf("%d>%d", author, parent) }

func (f *fakeReposts) Add(req *services.RepostAddRequest) (*entities.AnahitaNode, error) {
	repost := entities.NewAnahitaNode()
	repost.ID = int64(2000 + len(f.kept))
	repost.Type = constants.NodeBaseRepostType
	repost.OwnerID, repost.CreatedBy, repost.ParentID = req.OwnerID, req.OwnerID, req.ParentID
	repost.Enabled = true
	f.kept[f.key(req.OwnerID, req.ParentID)] = repost.ID
	f.nodes.people = append(f.nodes.people, repost)
	copied := *repost
	return &copied, nil
}
func (f *fakeReposts) Delete(req *services.RepostDeleteRequest) error {
	delete(f.kept, f.key(req.AuthorID, req.ParentID))
	return nil
}
func (f *fakeReposts) IsReposted(author, parent int64) (bool, error) {
	_, has := f.kept[f.key(author, parent)]
	return has, nil
}

func (w *inboxWorld) withReposts() *fakeReposts {
	reposts := &fakeReposts{nodes: w.nodes, kept: map[string]int64{}}
	w.h.Reposts = reposts
	return reposts
}

const (
	repostID   = int64(601)
	repostUUID = "6d6d6d6d-1111-4222-8333-555555555555"
)

// anasRepost is Ana having reposted something.
func anasRepost(of int64) *entities.AnahitaNode {
	repost := entities.NewAnahitaNode()
	repost.ID, repost.UUID = repostID, repostUUID
	repost.Type = constants.NodeBaseRepostType
	repost.Enabled = true
	repost.OwnerID, repost.CreatedBy, repost.ParentID = anaID, anaID, of
	return repost
}

func reposting(name string) *event.EventPayload {
	return &event.EventPayload{Name: "node.repost.repost.note." + name + ".v1", SubjectID: anaID, ObjectID: repostID, ObjectType: constants.NodeBaseRepostType}
}

// THE RULE THIS GUARDS: a repost goes out only of something other servers
// can see, once, and is taken back by undoing it.
func TestARepostGoesOutAndIsTakenBack(t *testing.T) {
	w, _ := settledWorld()
	w.follows.inboxes = []string{"https://other.example/inbox"}
	w.fromAlice(received(alicesNote, "<p>Beans?</p>", ""))
	hers := w.keptAt(alicesNote)
	if hers == nil {
		t.Fatal("her note was not kept")
	}
	w.queue.sent = nil

	// Ana reposts what Alice wrote. Heard twice: sent once to each server.
	w.nodes.people = append(w.nodes.people, anasRepost(hers.ID))
	_ = w.h.HandleEvent(reposting("added"))
	_ = w.h.HandleEvent(reposting("added"))
	if got := targets(w.queue.sent); got != "https://other.example/inbox https://mastodon.example/inbox" {
		t.Fatalf("sent to %q", got)
	}
	sent := w.queue.sent[0].payload
	name := "https://api.example.org/ap/activities/announce-" + repostUUID
	if sent["type"] != "Announce" || sent["actor"] != anaActor || sent["object"] != alicesNote || sent["id"] != name {
		t.Errorf("sent %v", sent)
	}
	if cc := fmt.Sprint(sent["cc"]); !strings.Contains(cc, anaActor+"/followers") || !strings.Contains(cc, aliceID) {
		t.Errorf("cc %v", cc)
	}
	if kept, _ := w.objects.Read(repostID); kept == nil || kept.Kind != repositories.KindRepost || kept.ReplyInbox != "https://mastodon.example/inbox" {
		t.Errorf("on record: %+v", kept)
	}

	// Taken back, with the repost itself already gone: undone, at both.
	w.queue.sent = nil
	w.nodes.people = w.nodes.people[:len(w.nodes.people)-1]
	_ = w.h.HandleEvent(reposting("deleted"))
	if got := targets(w.queue.sent); got != "https://other.example/inbox https://mastodon.example/inbox" {
		t.Fatalf("the undoing was sent to %q", got)
	}
	undone, _ := w.queue.sent[0].payload["object"].(map[string]any)
	if w.queue.sent[0].payload["type"] != "Undo" || undone["type"] != "Announce" || undone["id"] != name {
		t.Errorf("took back %v", w.queue.sent[0].payload)
	}
	if kept, _ := w.objects.Read(repostID); kept != nil {
		t.Error("still on record")
	}

	// Her own note that went out is reposted by its address here; one that
	// stayed here is not reposted to anybody.
	w.queue.sent = nil
	w.nodes.people = append(w.nodes.people, anasRepost(noteID))
	_ = w.h.HandleEvent(reposting("added"))
	if len(w.queue.sent) != 1 || w.queue.sent[0].payload["object"] != noteURL {
		t.Errorf("a repost of her own note: %+v", w.queue.sent)
	}

	stay := notesWorld()
	stay.nodes.people = append(stay.nodes.people, anasRepost(noteID))
	_ = stay.h.HandleEvent(reposting("added"))
	if len(stay.queue.sent) != 0 {
		t.Errorf("a repost of a note that stayed here was sent: %+v", stay.queue.sent)
	}
}

func boostOf(what string) string {
	return `{"id":"https://mastodon.example/users/alice/statuses/9/activity","type":"Announce","actor":"` + aliceID + `","object":"` + what + `","to":["https://www.w3.org/ns/activitystreams#Public"]}`
}

// THE RULE THIS GUARDS: a repost from elsewhere is kept of something here
// that went out, by anybody; and of something from elsewhere, only by
// somebody followed from here.
func TestARepostFromElsewhere(t *testing.T) {
	w := notesWorld()
	reposts := w.withReposts()
	_ = w.h.HandleEvent(made(noteID))
	var said []string
	w.h.Announce = func(payload *event.EventPayload, name string) error {
		said = append(said, name)
		return nil
	}

	// Alice, who is nobody here, boosts Ana's note. Twice over.
	w.fromAlice(boostOf(noteURL))
	w.fromAlice(boostOf(noteURL))
	alice := w.keptAt(aliceID)
	if alice == nil || len(reposts.kept) != 1 {
		t.Fatalf("kept %v", reposts.kept)
	}
	if has, _ := reposts.IsReposted(alice.ID, noteID); !has {
		t.Error("her repost is not hers")
	}
	if strings.Join(said, " ") != "node.repost.repost.note.added.v1" {
		t.Errorf("said on the bus: %v", said)
	}

	// Taken back.
	w.fromAlice(`{"id":"https://mastodon.example/undo/9","type":"Undo","actor":"` + aliceID + `","object":` + boostOf(noteURL) + `}`)
	if len(reposts.kept) != 0 {
		t.Errorf("after it was taken back: %v", reposts.kept)
	}

	// Ana has blocked her: nothing is kept.
	blocked := notesWorld()
	theirs := blocked.withReposts()
	_ = blocked.h.HandleEvent(made(noteID))
	blocked.relations.relation = &entities.ActorRelation{IsBlocking: true}
	blocked.fromAlice(boostOf(noteURL))
	if len(theirs.kept) != 0 {
		t.Errorf("a repost by somebody blocked was kept: %v", theirs.kept)
	}

	// Something from elsewhere that nobody here has seen, boosted by
	// somebody Ana follows: it is read from its own server and kept.
	const unseen = "https://mastodon.example/users/alice/statuses/77"
	page := `{"id":"` + unseen + `","type":"Note","attributedTo":"` + aliceID + `","content":"<p>From a while ago.</p>","to":["https://www.w3.org/ns/activitystreams#Public"]}`

	followed, _ := settledWorld()
	seen := followed.withReposts()
	followed.fetcher.pages = map[string]string{unseen: page}
	followed.fromAlice(boostOf(unseen))
	kept := followed.keptAt(unseen)
	if kept == nil || kept.Body != "From a while ago." || len(seen.kept) != 1 {
		t.Fatalf("kept %+v, reposts %v", kept, seen.kept)
	}

	// Boosted by somebody nobody here follows: not read, not kept.
	stranger := notesWorld()
	none := stranger.withReposts()
	stranger.fetcher.pages = map[string]string{unseen: page}
	stranger.fromAlice(boostOf(unseen))
	if stranger.keptAt(unseen) != nil || len(none.kept) != 0 {
		t.Error("something boosted by a stranger was kept")
	}
}
