package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"activitypub/data/repositories"

	"anahita-libs/event"
	"anahita-libs/federation"
)

// A person with a follower elsewhere, somebody they follow elsewhere, and
// a note that went out.
func settledWorld() (*inboxWorld, *repositories.RemoteActor) {
	w := notesWorld()
	w.call(w.h.Lookup, http.MethodGet, "/activitypub/lookup?q=@alice@mastodon.example", "", anaID)
	var alice *repositories.RemoteActor
	for _, kept := range w.actors.kept {
		alice = kept
	}
	_ = w.follows.Save(&repositories.Follow{LocalID: anaID, RemoteID: alice.NodeID, Direction: repositories.FollowOut, ActivityID: "out-1", State: repositories.FollowAccepted})
	_ = w.follows.Save(&repositories.Follow{LocalID: anaID, RemoteID: alice.NodeID, Direction: repositories.FollowIn, ActivityID: "in-1", State: repositories.FollowAccepted})
	_ = w.h.HandleEvent(made(noteID))
	w.queue.sent = nil
	w.edges.log = nil
	return w, alice
}

// THE RULE THIS GUARDS: somebody who leaves the fediverse is forgotten by
// every server that knew them, and nothing that crossed between servers is
// left standing here.
func TestTurningItOffTellsEveryServerToForget(t *testing.T) {
	w, alice := settledWorld()

	if rec := w.call(w.h.EditSettings, http.MethodPut, "/", `{"enabled":false}`, anaID); rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}

	if len(w.queue.sent) != 2 {
		t.Fatalf("queued %d, want one for each server that knew her", len(w.queue.sent))
	}
	for _, sent := range w.queue.sent {
		if sent.payload["type"] != "Delete" || sent.payload["actor"] != anaActor || sent.payload["object"] != anaActor || sent.actorID != anaID {
			t.Errorf("sent %+v", sent)
		}
	}

	if len(w.follows.kept) != 0 || len(w.objects.kept) != 0 || w.accounts.enabled[anaID] {
		t.Errorf("left: %d follows, %d notes recorded, enabled %v", len(w.follows.kept), len(w.objects.kept), w.accounts.enabled[anaID])
	}
	edges := strings.Join(w.edges.log, ", ")
	if !strings.Contains(edges, "delete follow ana>alice@mastodon.example") || !strings.Contains(edges, "delete follow alice@mastodon.example>ana") {
		t.Errorf("edges %v", w.edges.log)
	}

	// The follows' going is heard on the bus, and is not taken for her
	// unfollowing: no Undo follows the Delete.
	before := len(w.queue.sent)
	_ = w.h.HandleEvent(&event.EventPayload{Name: "edge.follow.socialgraph.deleted.v1", ObjectID: anaID, TargetID: alice.NodeID})
	if len(w.queue.sent) != before {
		t.Error("an Undo was sent after she left")
	}

	// And she is not found.
	if rec := w.get(w.h.Actor, "/", "uuid", anaUUID); rec.Code != http.StatusNotFound {
		t.Errorf("found after leaving: %d", rec.Code)
	}

	// Turning off what was never on tells nobody anything.
	w = notesWorld()
	w.accounts.enabled[anaID] = false
	w.call(w.h.EditSettings, http.MethodPut, "/", `{"enabled":false}`, anaID)
	if len(w.queue.sent) != 0 {
		t.Errorf("queued %d for somebody who was never on it", len(w.queue.sent))
	}
}

func TestAProfileChangeIsToldAndAClosedProfileLeaves(t *testing.T) {
	edited := &event.EventPayload{Name: "node.actor.person.person.edited.v1", ObjectID: anaID, TargetID: anaID}

	// Changed, and still open to everybody: her followers' servers are
	// sent her as she is now.
	w, _ := settledWorld()
	w.nodes.people[0].Name = "Ana B."
	w.nodes.people[0].UpdatedAt = time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	if err := w.h.HandleEvent(edited); err != nil {
		t.Fatal(err)
	}
	if len(w.queue.sent) != 2 || w.queue.sent[0].payload["type"] != "Update" {
		t.Fatalf("queued %+v", w.queue.sent)
	}
	described, _ := w.queue.sent[0].payload["object"].(map[string]any)
	key, _ := described["publicKey"].(map[string]any)
	if described["id"] != anaActor || described["name"] != "Ana B." || described["type"] != "Person" || key["publicKeyPem"] == nil {
		t.Errorf("described as %v", described)
	}
	// Heard again, nothing more is sent.
	_ = w.h.HandleEvent(edited)
	if len(w.queue.sent) != 2 {
		t.Errorf("after hearing it twice: %d", len(w.queue.sent))
	}

	// Made for followers only: she leaves.
	for name, change := range map[string]func(w *inboxWorld){
		"a profile closed to followers": func(w *inboxWorld) { w.nodes.people[0].Access = "followers" },
		"disabled":                      func(w *inboxWorld) { w.nodes.people[0].Enabled = false },
		"hidden by the graph":           func(w *inboxWorld) { w.nodes.hidden[anaID] = true },
	} {
		w, _ := settledWorld()
		change(w)
		_ = w.h.HandleEvent(&event.EventPayload{Name: "node.actor.person.person.disabled.v1", ObjectID: anaID})
		if len(w.queue.sent) != 2 || w.queue.sent[0].payload["type"] != "Delete" || w.queue.sent[0].payload["object"] != anaActor || w.accounts.enabled[anaID] || len(w.follows.kept) != 0 {
			t.Errorf("%s: queued %+v, enabled %v, follows %d", name, w.queue.sent, w.accounts.enabled[anaID], len(w.follows.kept))
		}
	}

	// Somebody who is not on the fediverse changes their profile: nobody
	// is told.
	w = notesWorld()
	w.accounts.enabled[anaID] = false
	_ = w.h.HandleEvent(edited)
	if len(w.queue.sent) != 0 {
		t.Errorf("queued %d", len(w.queue.sent))
	}
}

func TestAnAccountClosedElsewhereIsForgottenHere(t *testing.T) {
	gone := `{"id":"https://mastodon.example/d/9","type":"Delete","actor":"` + aliceID + `","object":"` + aliceID + `"}`

	w, alice := settledWorld()
	w.fromAlice(received(alicesNote, "<p>hi</p>", ""))
	if found, _ := w.nodes.find(func(n *entitiesNode) bool { return n.URI == alicesNote }); found == nil {
		t.Fatal("her note was not kept to begin with")
	}

	if rec := w.fromAlice(gone); rec.Code != http.StatusAccepted {
		t.Fatalf("%d", rec.Code)
	}
	if found, _ := w.nodes.find(func(n *entitiesNode) bool { return n.URI == aliceID || n.URI == alicesNote }); found != nil {
		t.Errorf("still kept: %+v", found)
	}
	if w.actors.kept[alice.NodeID] != nil || len(w.follows.kept) != 0 {
		t.Errorf("still known: actor %v, follows %d", w.actors.kept[alice.NodeID], len(w.follows.kept))
	}

	// Somebody nobody here ever heard of says they are gone: taken, and
	// their server is not asked anything.
	w = newInboxWorld()
	if rec := w.fromAlice(gone); rec.Code != http.StatusAccepted {
		t.Fatalf("a stranger: %d", rec.Code)
	}
	if len(w.fetcher.asked) != 0 {
		t.Errorf("asked %v about a stranger who left", w.fetcher.asked)
	}

	// Somebody cannot close somebody else's account.
	w, _ = settledWorld()
	w.fromAlice(`{"id":"x","type":"Delete","actor":"` + aliceID + `","object":"https://mastodon.example/users/bob"}`)
	if found, _ := w.nodes.find(func(n *entitiesNode) bool { return n.URI == aliceID }); found == nil {
		t.Error("she went when she said somebody else had")
	}
}

func TestAProfileChangedElsewhereIsReadAgain(t *testing.T) {
	w, alice := settledWorld()
	w.fetcher.name = "Alice Liddell"
	w.fetcher.asked = nil

	// What the message says of her is not taken at its word.
	w.fromAlice(`{"id":"https://mastodon.example/u/1","type":"Update","actor":"` + aliceID + `","object":{"id":"` + aliceID + `","type":"Person","name":"Somebody Else Entirely"}}`)

	node, _ := w.nodes.Read(alice.NodeID)
	if node == nil || node.Name != "Alice Liddell" {
		t.Errorf("after her change: %+v", node)
	}
	if len(w.fetcher.asked) == 0 {
		t.Error("she was not read again from her own server")
	}
}

func TestAServerThatIsRefused(t *testing.T) {
	w, _ := settledWorld()
	w.blocks.domains["mastodon.example"] = "spam"
	w.fetcher.asked = nil

	// Not listened to, and not asked for its key.
	if rec := w.fromAlice(follow()); rec.Code != http.StatusForbidden {
		t.Errorf("a message from it: %d", rec.Code)
	}
	if len(w.fetcher.asked) != 0 || len(w.edges.log) != 0 {
		t.Errorf("asked %v, edges %v", w.fetcher.asked, w.edges.log)
	}

	// Nobody on it is looked up, by handle or by address, nor on a name
	// under it.
	for _, query := range []string{"@alice@mastodon.example", aliceID, "@bob@social.mastodon.example"} {
		rec := w.call(w.h.Lookup, http.MethodGet, "/activitypub/lookup?q="+query, "", anaID)
		if !strings.Contains(rec.Body.String(), errLookupBlocked) {
			t.Errorf("%s: %d %s", query, rec.Code, rec.Body.String())
		}
	}
	if len(w.fetcher.asked) != 0 {
		t.Errorf("asked %v", w.fetcher.asked)
	}

	// Nothing is sent to it, whenever it was queued, and it is not tried
	// again.
	poster := &fakePoster{}
	w.h.Poster = poster
	err := w.h.Deliver(context.Background(), federation.Delivery{ActorID: anaID, Target: "https://mastodon.example/inbox", Payload: []byte(`{}`)})
	if err == nil || len(poster.posted) != 0 {
		t.Errorf("sent to it: %v %v", err, poster.posted)
	}

	// Another server is untouched.
	if err := w.h.Deliver(context.Background(), federation.Delivery{ActorID: anaID, Target: "https://other.example/inbox", Payload: []byte(`{}`)}); err != nil || len(poster.posted) != 1 {
		t.Errorf("another server: %v %v", err, poster.posted)
	}
}

func TestOnlyAnAdministratorKeepsTheList(t *testing.T) {
	w := newInboxWorld()

	// Ana is nobody in particular.
	for name, rec := range map[string]int{
		"read":   w.call(w.h.BrowseBlocks, http.MethodGet, "/", "", anaID).Code,
		"add":    w.call(w.h.AddBlock, http.MethodPost, "/", `{"domain":"spam.example"}`, anaID).Code,
		"delete": w.call(w.h.DeleteBlock, http.MethodDelete, "/", "", anaID, "domain", "spam.example").Code,
	} {
		if rec != http.StatusForbidden {
			t.Errorf("%s by somebody who is not an administrator: %d", name, rec)
		}
	}
	if len(w.blocks.domains) != 0 {
		t.Fatal("somebody who is not an administrator refused a server")
	}

	admin := func(handler func(c echoContext) error, method, body string, params ...string) (int, string) {
		rec := w.callAs("administrator", handler, method, body, params...)
		return rec.Code, rec.Body.String()
	}

	// However it is typed, it is kept as the server's name.
	for typed, kept := range map[string]string{
		"Spam.Example":                "spam.example",
		"https://bad.example/users/x": "bad.example",
		"@troll@Worse.Example":        "worse.example",
		" trailing.example. ":         "trailing.example",
	} {
		body, _ := json.Marshal(map[string]string{"domain": typed, "reason": "because"})
		if code, answer := admin(w.h.AddBlock, http.MethodPost, string(body)); code != http.StatusCreated || w.blocks.domains[kept] != "because" {
			t.Errorf("%q: %d %s, kept %v", typed, code, answer, w.blocks.domains)
		}
	}

	// What is no server's name, and this installation's own.
	for _, typed := range []string{"", "localhost", "not a domain", "example.org", "api.example.org"} {
		body, _ := json.Marshal(map[string]string{"domain": typed})
		if code, _ := admin(w.h.AddBlock, http.MethodPost, string(body)); code != http.StatusBadRequest {
			t.Errorf("%q: %d", typed, code)
		}
	}

	if code, answer := admin(w.h.BrowseBlocks, http.MethodGet, ""); code != http.StatusOK || !strings.Contains(answer, "spam.example") {
		t.Errorf("list: %d %s", code, answer)
	}
	if code, _ := admin(w.h.DeleteBlock, http.MethodDelete, "", "domain", "spam.example"); code != http.StatusNoContent || w.blocks.IsBlocked("spam.example") {
		t.Errorf("delete: %d", code)
	}
}
