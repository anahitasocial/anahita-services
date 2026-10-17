package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"activitypub/data/repositories"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/event"
	"anahita-libs/federation"
)

const (
	noteID   = 500
	noteUUID = "7429ca90-0252-4e9b-8494-749ddec89512"
	noteURL  = "https://api.example.org/ap/notes/" + noteUUID
)

func anasNote() *entities.AnahitaNode {
	note := entities.NewAnahitaNode()
	note.ID = noteID
	note.UUID = noteUUID
	note.Type = constants.NodeMediumNoteEntityType
	note.Body = "First **beans** of the year, thanks @alice@mastodon.example #garden <script>alert(1)</script>"
	note.Access = "public"
	note.Enabled = true
	note.OwnerID = anaID
	note.CreatedBy = anaID
	note.Language = "en"
	note.CreatedAt = time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	note.UpdatedAt = note.CreatedAt
	return note
}

func notesWorld() *inboxWorld {
	w := newInboxWorld()
	w.nodes.people = append(w.nodes.people, anasNote())
	w.follows.inboxes = []string{"https://mastodon.example/inbox", "https://other.example/inbox"}
	return w
}

func made(id int64) *event.EventPayload {
	return &event.EventPayload{Name: "node.medium.text.note.added.v1", ObjectID: id, ObjectType: constants.NodeMediumNoteEntityType}
}
func changed(id int64) *event.EventPayload {
	return &event.EventPayload{Name: "node.medium.text.note.edited.v1", ObjectID: id, ObjectType: constants.NodeMediumNoteEntityType}
}
func deleted(id int64) *event.EventPayload {
	return &event.EventPayload{Name: "node.medium.text.note.deleted.v1", ObjectID: id, ObjectType: constants.NodeMediumNoteEntityType}
}

func (w *inboxWorld) note() *entities.AnahitaNode {
	for _, node := range w.nodes.people {
		if node.ID == noteID {
			return node
		}
	}
	return nil
}

// THE RULE THIS GUARDS: a public note by somebody on the fediverse goes to
// every server that has a follower of theirs, once, as HTML that is safe to
// show; and what is not all of that goes nowhere.
func TestAPublicNoteGoesToFollowersOnce(t *testing.T) {
	w := notesWorld()

	// Heard twice, as every copy of the service hears it.
	for i := 0; i < 2; i++ {
		if err := w.h.HandleEvent(made(noteID)); err != nil {
			t.Fatal(err)
		}
	}
	if len(w.queue.sent) != 2 {
		t.Fatalf("queued %d, want one for each server", len(w.queue.sent))
	}

	first := w.queue.sent[0]
	object, _ := first.payload["object"].(map[string]any)
	if first.actorID != anaID || first.target != "https://mastodon.example/inbox" || first.payload["type"] != "Create" ||
		first.payload["id"] != noteURL+"/activity" || first.payload["actor"] != anaActor {
		t.Errorf("sent %+v", first)
	}
	if object["id"] != noteURL || object["type"] != "Note" || object["attributedTo"] != anaActor ||
		object["url"] != "https://example.org/notes/500" || object["published"] != "2026-10-01T09:00:00Z" || object["updated"] != nil {
		t.Errorf("the note sent: %v", object)
	}
	if to, _ := object["to"].([]any); len(to) != 1 || to[0] != "https://www.w3.org/ns/activitystreams#Public" {
		t.Errorf("to %v", object["to"])
	}
	if cc, _ := object["cc"].([]any); len(cc) != 1 || cc[0] != anaActor+"/followers" {
		t.Errorf("cc %v", object["cc"])
	}

	content, _ := object["content"].(string)
	if !strings.Contains(content, "<strong>beans</strong>") || !strings.Contains(content, `href="https://example.org/tags/garden"`) || strings.Contains(content, "<script") {
		t.Errorf("content %q", content)
	}
	if byLanguage, _ := object["contentMap"].(map[string]any); byLanguage["en"] != content {
		t.Errorf("contentMap %v", object["contentMap"])
	}
	// The hashtag is listed beside the text. Alice is not known here, so
	// her name is left as written and she is not listed.
	tags, _ := json.Marshal(object["tag"])
	if string(tags) != `[{"href":"https://example.org/tags/garden","name":"#garden","type":"Hashtag"}]` {
		t.Errorf("tags %s", tags)
	}

	// A server that asks for the note is given the same one.
	rec := w.get(w.h.Note, "/ap/notes/"+noteUUID, "uuid", noteUUID)
	var served map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &served)
	if rec.Code != http.StatusOK || served["id"] != noteURL || served["content"] != content || served["@context"] == nil {
		t.Errorf("served %d %v", rec.Code, served)
	}

	// And the outbox says there is one.
	out := w.get(w.h.Outbox, "/", "uuid", anaUUID)
	if !strings.Contains(out.Body.String(), `"totalItems":1`) {
		t.Errorf("outbox %s", out.Body.String())
	}
}

// An article goes out as an Article: its title, its first paragraph of
// words as its summary, all of it as its content, the page it is read at,
// and its cover. A topic does not go out (the user, 2026-10-17).
func TestAnArticleGoesOutAsAnArticle(t *testing.T) {
	w := notesWorld()
	article := w.note()
	article.Type = constants.NodeMediumArticleEntityType
	article.Name = "Beans in October"
	article.Body = "## Sowing\n\n![Rows of beans](https://example.org/photos/7)\n\nThe **beans** went in late this year, after the rain & the wind.\n\nMore below."
	article.CoverFilename = "cover.jpg"

	event := made(noteID)
	event.ObjectType = article.Type
	if err := w.h.HandleEvent(event); err != nil {
		t.Fatal(err)
	}
	if len(w.queue.sent) == 0 {
		t.Fatal("nothing queued")
	}
	object, _ := w.queue.sent[0].payload["object"].(map[string]any)
	if object["type"] != "Article" || object["name"] != "Beans in October" || object["url"] != "https://example.org/articles/500" ||
		object["attributedTo"] != anaActor {
		t.Errorf("the article sent: %v", object)
	}
	if object["summary"] != "The beans went in late this year, after the rain &amp; the wind." {
		t.Errorf("summary %q", object["summary"])
	}
	if content, _ := object["content"].(string); !strings.Contains(content, "<strong>beans</strong>") || !strings.Contains(content, "More below.") {
		t.Errorf("content %q", content)
	}
	if image, _ := object["image"].(map[string]any); image["url"] != "https://api.example.org/covers/"+noteUUID+"/large" || image["mediaType"] != "image/jpeg" {
		t.Errorf("image %v", object["image"])
	}

	// A server that asks for it is given the same.
	rec := w.get(w.h.Note, "/ap/notes/"+noteUUID, "uuid", noteUUID)
	var served map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &served)
	if rec.Code != http.StatusOK || served["type"] != "Article" || served["name"] != "Beans in October" {
		t.Errorf("served %d %v", rec.Code, served)
	}
}

// A long first paragraph is cut at a word, and an article with no
// paragraph of words has no summary.
func TestAnArticlesSummary(t *testing.T) {
	long := entities.NewAnahitaNode()
	long.Body = strings.Repeat("seed ", 100)
	if got := articleSummary(long, federation.Links{}); len([]rune(got)) > summaryMax+1 || !strings.HasSuffix(got, "seed…") {
		t.Errorf("long: %q", got)
	}
	none := entities.NewAnahitaNode()
	none.Body = "# Title\n\n![a picture](https://example.org/p.jpg)\n\nhttps://example.org/elsewhere"
	if got := articleSummary(none, federation.Links{}); got != "" {
		t.Errorf("none: %q", got)
	}
}

func TestWhatDoesNotGoOut(t *testing.T) {
	cases := map[string]func(w *inboxWorld){
		"for followers":            func(w *inboxWorld) { w.note().Access = "followers" },
		"for members":              func(w *inboxWorld) { w.note().Access = "registered" },
		"for the author alone":     func(w *inboxWorld) { w.note().Access = "myself" },
		"a reply to nothing known": func(w *inboxWorld) { w.note().ParentID, w.note().RootID = 77, 77 },
		"in a group":               func(w *inboxWorld) { w.note().OwnerID = 900 },
		"on somebody else's page":  func(w *inboxWorld) { w.note().CreatedBy = 901 },
		"a topic":                  func(w *inboxWorld) { w.note().Type = "node.medium.text-service.topic.v1" },
		"an article in a group":    func(w *inboxWorld) { w.note().Type, w.note().OwnerID = constants.NodeMediumArticleEntityType, 900 },
		"an article for followers": func(w *inboxWorld) {
			w.note().Type, w.note().Access = constants.NodeMediumArticleEntityType, "followers"
		},
		"disabled":                 func(w *inboxWorld) { w.note().Enabled = false },
		"hidden by the graph":      func(w *inboxWorld) { w.nodes.hidden[noteID] = true },
		"from elsewhere":           func(w *inboxWorld) { w.note().URI = "https://mastodon.example/n/1" },
		"its author not federated": func(w *inboxWorld) { w.accounts.enabled[anaID] = false },
		"its author not public":    func(w *inboxWorld) { w.nodes.people[0].Access = "followers" },
	}

	for name, change := range cases {
		w := notesWorld()
		change(w)

		event := made(noteID)
		event.ObjectType = w.note().Type
		_ = w.h.HandleEvent(event)
		if len(w.queue.sent) != 0 || len(w.objects.kept) != 0 {
			t.Errorf("%s: queued %d, recorded %d", name, len(w.queue.sent), len(w.objects.kept))
		}
		if rec := w.get(w.h.Note, "/", "uuid", noteUUID); rec.Code != http.StatusNotFound {
			t.Errorf("%s: served %d", name, rec.Code)
		}
	}
}

func TestAChangeAndADeletionFollowTheNote(t *testing.T) {
	w := notesWorld()
	_ = w.h.HandleEvent(made(noteID))
	w.queue.sent = nil

	// Changed: told, with when.
	w.note().Body = "First beans, at last."
	w.note().UpdatedAt = w.note().CreatedAt.Add(time.Hour)
	_ = w.h.HandleEvent(changed(noteID))
	if len(w.queue.sent) != 2 || w.queue.sent[0].payload["type"] != "Update" {
		t.Fatalf("after a change: %+v", w.queue.sent)
	}
	object, _ := w.queue.sent[0].payload["object"].(map[string]any)
	if object["updated"] != "2026-10-01T10:00:00Z" || !strings.Contains(object["content"].(string), "at last") {
		t.Errorf("the change sent: %v", object)
	}

	// Made for followers only after it went out: taken back.
	w.queue.sent = nil
	w.note().Access = "followers"
	_ = w.h.HandleEvent(changed(noteID))
	if len(w.queue.sent) != 2 || w.queue.sent[0].payload["type"] != "Delete" {
		t.Fatalf("after it was closed: %+v", w.queue.sent)
	}
	gone, _ := w.queue.sent[0].payload["object"].(map[string]any)
	if gone["id"] != noteURL || gone["type"] != "Tombstone" || len(w.objects.kept) != 0 {
		t.Errorf("taken back as %v, recorded %d", gone, len(w.objects.kept))
	}

	// Deleted after that: nothing more to say.
	w.queue.sent = nil
	_ = w.h.HandleEvent(deleted(noteID))
	if len(w.queue.sent) != 0 {
		t.Errorf("deleted twice: %+v", w.queue.sent)
	}

	// Deleted outright: its node is gone when it is heard of, and the
	// servers are told all the same.
	w = notesWorld()
	_ = w.h.HandleEvent(made(noteID))
	w.queue.sent = nil
	w.nodes.people = w.nodes.people[:1]
	_ = w.h.HandleEvent(deleted(noteID))
	if len(w.queue.sent) != 2 || w.queue.sent[1].payload["type"] != "Delete" || w.queue.sent[1].target != "https://other.example/inbox" {
		t.Errorf("after a delete: %+v", w.queue.sent)
	}

	// One that never went out is deleted at nobody.
	w = notesWorld()
	_ = w.h.HandleEvent(deleted(noteID))
	if len(w.queue.sent) != 0 {
		t.Errorf("a note nobody was told of: %+v", w.queue.sent)
	}
}

func received(id, content, extra string) string {
	return `{"id":"` + id + `/activity","type":"Create","actor":"` + aliceID + `","object":{"id":"` + id + `","type":"Note","attributedTo":"` + aliceID + `","content":"` + content + `","contentMap":{"fr":"` + content + `"},"published":"2026-09-01T12:00:00Z","to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["` + aliceID + `/followers"]` + extra + `}}`
}

const alicesNote = "https://mastodon.example/users/alice/statuses/1"

// THE RULE THIS GUARDS: a post from elsewhere is kept only when somebody
// here follows its author, and only as its author's own public post.
func TestAPostFromSomebodyFollowedIsKept(t *testing.T) {
	w := newInboxWorld()
	w.call(w.h.Lookup, http.MethodGet, "/activitypub/lookup?q=@alice@mastodon.example", "", anaID)
	var alice *repositories.RemoteActor
	for _, kept := range w.actors.kept {
		alice = kept
	}
	body := received(alicesNote, `<p>Bonjour <strong>tout le monde</strong></p><script>alert(1)</script>`, "")

	// Nobody here follows her: it is taken and not kept.
	if rec := w.fromAlice(body); rec.Code != http.StatusAccepted {
		t.Fatalf("%d", rec.Code)
	}
	// What is said on the bus, for the hashtag service.
	var said []string
	w.h.Announce = func(payload *event.EventPayload, name string) error {
		if payload.ObjectType != constants.NodeMediumNoteEntityType || payload.ObjectID == 0 || payload.ViewerID != alice.NodeID {
			t.Errorf("said of %+v", payload)
		}
		said = append(said, name)
		return nil
	}
	if found, _ := w.nodes.find(func(n *entities.AnahitaNode) bool { return n.URI == alicesNote }); found != nil {
		t.Fatal("a post from somebody nobody follows was kept")
	}

	// Ana follows her.
	_ = w.follows.Save(&repositories.Follow{LocalID: anaID, RemoteID: alice.NodeID, Direction: repositories.FollowOut, ActivityID: "x", State: repositories.FollowAccepted})
	w.fromAlice(body)
	kept, _ := w.nodes.find(func(n *entities.AnahitaNode) bool { return n.URI == alicesNote })
	if kept == nil {
		t.Fatal("her post was not kept")
	}
	if kept.Type != constants.NodeMediumNoteEntityType || kept.OwnerID != alice.NodeID || kept.OriginDomain != "mastodon.example" ||
		kept.Body != "Bonjour **tout le monde**" || kept.Language != "fr" || !kept.CreatedAt.Equal(time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("kept as %+v", kept)
	}

	// Changed there, it is changed here, and is still one post.
	w.fromAlice(strings.Replace(received(alicesNote, `<p>Bonsoir</p>`, ""), `"type":"Create"`, `"type":"Update"`, 1))
	kept, _ = w.nodes.find(func(n *entities.AnahitaNode) bool { return n.URI == alicesNote })
	if kept == nil || kept.Body != "Bonsoir" {
		t.Errorf("after her change: %+v", kept)
	}

	// Kept, then changed: said each time, by a name that nothing takes for
	// a post written here. Not said of the one that was not kept.
	if got := strings.Join(said, " "); got != "federation.medium.text-service.note.added.v1 federation.medium.text-service.note.edited.v1" {
		t.Errorf("said on the bus: %q", got)
	}

	// Taken away there, it is taken away here.
	w.fromAlice(`{"id":"https://mastodon.example/d/1","type":"Delete","actor":"` + aliceID + `","object":{"id":"` + alicesNote + `","type":"Tombstone"}}`)
	if kept, _ = w.nodes.find(func(n *entities.AnahitaNode) bool { return n.URI == alicesNote }); kept != nil {
		t.Error("her post was still kept")
	}
}

func TestWhatIsNotKept(t *testing.T) {
	cases := map[string]string{
		"for her followers only":      strings.Replace(received(alicesNote, "<p>psst</p>", ""), `"to":["https://www.w3.org/ns/activitystreams#Public"]`, `"to":["`+aliceID+`/followers"]`, 1),
		"a reply":                     received(alicesNote, "<p>re</p>", `,"inReplyTo":"https://other.example/n/9"`),
		"somebody else's post":        strings.Replace(received(alicesNote, "<p>hi</p>", ""), `"attributedTo":"`+aliceID+`"`, `"attributedTo":"https://mastodon.example/users/bob"`, 1),
		"at another server's address": received("https://other.example/n/1", "<p>hi</p>", ""),
		"at one of our addresses":     received("https://api.example.org/ap/notes/"+noteUUID, "<p>hi</p>", ""),
		"a picture alone":             received(alicesNote, "", ""),
		"only an address":             `{"id":"x","type":"Create","actor":"` + aliceID + `","object":"` + alicesNote + `"}`,
		"something that is no note":   strings.Replace(received(alicesNote, "<p>hi</p>", ""), `"type":"Note"`, `"type":"Question"`, 1),
	}

	for name, body := range cases {
		w := newInboxWorld()
		w.call(w.h.Lookup, http.MethodGet, "/activitypub/lookup?q=@alice@mastodon.example", "", anaID)
		for _, kept := range w.actors.kept {
			_ = w.follows.Save(&repositories.Follow{LocalID: anaID, RemoteID: kept.NodeID, Direction: repositories.FollowOut, ActivityID: "x", State: repositories.FollowAccepted})
		}
		before := len(w.nodes.people)

		if rec := w.fromAlice(body); rec.Code != http.StatusAccepted {
			t.Errorf("%s: %d", name, rec.Code)
		}
		if len(w.nodes.people) != before {
			t.Errorf("%s: something was kept", name)
		}
	}

	// She cannot take away what is not hers: something made here, or
	// somebody else's.
	w := notesWorld()
	w.call(w.h.Lookup, http.MethodGet, "/activitypub/lookup?q=@alice@mastodon.example", "", anaID)
	before := len(w.nodes.people)
	w.fromAlice(`{"id":"https://mastodon.example/d/2","type":"Delete","actor":"` + aliceID + `","object":"` + noteURL + `"}`)
	w.fromAlice(`{"id":"https://mastodon.example/d/3","type":"Delete","actor":"` + aliceID + `","object":"https://mastodon.example/users/bob/statuses/1"}`)
	if len(w.nodes.people) != before {
		t.Error("something that was not hers was taken away")
	}
}
