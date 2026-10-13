package handlers

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"activitypub/data/repositories"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

const (
	replyID   = int64(501)
	replyUUID = "5d5c1f0e-2a53-4f43-9b6c-0b1f6d3d9a11"
	replyURL  = "https://api.example.org/ap/notes/" + replyUUID
)

// anasReply is something Ana says under a post: hers alone, and saying
// nothing of its own about who may read it.
func anasReply(parent, root int64) *entities.AnahitaNode {
	reply := entities.NewAnahitaNode()
	reply.ID = replyID
	reply.UUID = replyUUID
	reply.Type = constants.NodeMediumNoteEntityType
	reply.Body = "And the peas are up."
	reply.Enabled = true
	reply.OwnerID = anaID
	reply.CreatedBy = anaID
	reply.ParentID = parent
	reply.RootID = root
	reply.CreatedAt = time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	reply.UpdatedAt = reply.CreatedAt
	return reply
}

func targets(sent []queued) string {
	var to []string
	for _, each := range sent {
		to = append(to, each.target)
	}
	return strings.Join(to, " ")
}

// THE RULE THIS GUARDS: a reply goes out only when what it answers is
// something other servers can see, and then it says what it answers.
func TestAThreadOfOnesOwnNotesGoesOutWhole(t *testing.T) {
	w := notesWorld()
	w.nodes.people = append(w.nodes.people, anasReply(noteID, noteID))

	// Her note never went out: neither does what she says under it.
	_ = w.h.HandleEvent(made(replyID))
	if len(w.queue.sent) != 0 {
		t.Fatalf("a reply under a note that stayed here was sent: %+v", w.queue.sent)
	}

	// The note goes out, and then the reply does, to the same servers.
	_ = w.h.HandleEvent(made(noteID))
	w.queue.sent = nil
	_ = w.h.HandleEvent(made(replyID))
	if len(w.queue.sent) != 2 || w.queue.sent[0].payload["type"] != "Create" {
		t.Fatalf("queued %+v", w.queue.sent)
	}
	object, _ := w.queue.sent[0].payload["object"].(map[string]any)
	if object["id"] != replyURL || object["inReplyTo"] != noteURL {
		t.Errorf("sent as %v, answering %v", object["id"], object["inReplyTo"])
	}
	// Nobody is named: she is answering herself.
	if tags, _ := json.Marshal(object["tag"]); string(tags) != "null" {
		t.Errorf("tags %s", tags)
	}
	if kept, _ := w.objects.Read(replyID); kept == nil || kept.ReplyInbox != "" {
		t.Errorf("on record: %+v", kept)
	}

	// A server that asks for it is given it, with what it answers.
	asked := w.get(w.h.Note, "/", "uuid", replyUUID)
	if !strings.Contains(asked.Body.String(), `"inReplyTo":"`+noteURL+`"`) {
		t.Errorf("served as %s", asked.Body.String())
	}

	// The thread closed to her followers: the reply is taken back.
	w.queue.sent = nil
	w.note().Access = "followers"
	_ = w.h.HandleEvent(changed(replyID))
	if len(w.queue.sent) != 2 || w.queue.sent[0].payload["type"] != "Delete" {
		t.Errorf("after the thread was closed: %+v", w.queue.sent)
	}
}

func TestAReplyToSomebodyElsewhereReachesThem(t *testing.T) {
	w, alice := settledWorld()
	// Ana's followers are on one server, and Alice is on another.
	w.follows.inboxes = []string{"https://other.example/inbox"}

	// Something Alice wrote, kept here.
	w.fromAlice(received(alicesNote, "<p>Beans?</p>", ""))
	hers, _ := w.nodes.find(func(n *entities.AnahitaNode) bool { return n.URI == alicesNote })
	if hers == nil {
		t.Fatal("her note was not kept")
	}
	w.queue.sent = nil

	w.nodes.people = append(w.nodes.people, anasReply(hers.ID, hers.ID))
	_ = w.h.HandleEvent(made(replyID))

	// To Ana's followers, and to Alice's server.
	if got := targets(w.queue.sent); got != "https://other.example/inbox https://mastodon.example/inbox" {
		t.Fatalf("sent to %q", got)
	}
	object, _ := w.queue.sent[1].payload["object"].(map[string]any)
	if object["inReplyTo"] != alicesNote {
		t.Errorf("answering %v", object["inReplyTo"])
	}
	// Alice is named beside the text, which is what has her server tell
	// her, and not in it.
	tags, _ := json.Marshal(object["tag"])
	if string(tags) != `[{"href":"`+aliceID+`","name":"@alice@mastodon.example","type":"Mention"}]` {
		t.Errorf("tags %s", tags)
	}
	if cc, _ := json.Marshal(object["cc"]); !strings.Contains(string(cc), aliceID) {
		t.Errorf("cc %s", cc)
	}
	if strings.Contains(object["content"].(string), "alice") {
		t.Errorf("her name was written into the reply: %v", object["content"])
	}
	if kept, _ := w.objects.Read(replyID); kept == nil || kept.ReplyInbox != "https://mastodon.example/inbox" {
		t.Errorf("on record: %+v", kept)
	}

	// Taken away, with the reply itself already gone: both are told.
	w.queue.sent = nil
	w.nodes.people = w.nodes.people[:len(w.nodes.people)-1]
	_ = w.h.HandleEvent(deleted(replyID))
	if got := targets(w.queue.sent); got != "https://other.example/inbox https://mastodon.example/inbox" || w.queue.sent[0].payload["type"] != "Delete" {
		t.Errorf("deletion sent to %q: %+v", got, w.queue.sent)
	}

	// A server that is refused is not answered.
	w.blocks.domains["mastodon.example"] = "spam"
	w.nodes.people = append(w.nodes.people, anasReply(hers.ID, hers.ID))
	w.queue.sent = nil
	_ = w.h.HandleEvent(made(replyID))
	if len(w.queue.sent) != 0 {
		t.Errorf("a reply to a refused server was sent: %+v", w.queue.sent)
	}
	_ = alice
	_ = repositories.FollowOut
}
