package handlers

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"activitypub/data/repositories"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// fakeQuotes keeps quotes between servers in memory.
type fakeQuotes struct {
	requests map[int64]*repositories.QuoteRequest
	stamps   []*repositories.QuoteStamp
}

func newFakeQuotes() *fakeQuotes {
	return &fakeQuotes{requests: map[int64]*repositories.QuoteRequest{}}
}

func (f *fakeQuotes) BeginRequest(request *repositories.QuoteRequest) (bool, error) {
	if _, ok := f.requests[request.NodeID]; ok {
		return false, nil
	}
	c := *request
	c.State = repositories.QuotePending
	f.requests[request.NodeID] = &c
	return true, nil
}
func (f *fakeQuotes) RequestOf(nodeID int64) (*repositories.QuoteRequest, error) {
	return f.requests[nodeID], nil
}
func (f *fakeQuotes) RequestByID(requestID string) (*repositories.QuoteRequest, error) {
	for _, request := range f.requests {
		if request.RequestID == requestID {
			return request, nil
		}
	}
	return nil, nil
}
func (f *fakeQuotes) Answer(nodeID int64, accepted bool, stampURI string) error {
	request := f.requests[nodeID]
	request.State, request.StampURI = repositories.QuoteRejected, ""
	if accepted {
		request.State, request.StampURI = repositories.QuoteAccepted, stampURI
	}
	return nil
}
func (f *fakeQuotes) AddStamp(stamp *repositories.QuoteStamp) (*repositories.QuoteStamp, error) {
	if have, _ := f.StampFor(stamp.QuotedNodeID, stamp.QuoteURI); have != nil {
		return have, nil
	}
	c := *stamp
	f.stamps = append(f.stamps, &c)
	return &c, nil
}
func (f *fakeQuotes) Stamp(uuid string) (*repositories.QuoteStamp, error) {
	for _, stamp := range f.stamps {
		if stamp.UUID == uuid {
			return stamp, nil
		}
	}
	return nil, nil
}
func (f *fakeQuotes) StampFor(quotedNodeID int64, quoteURI string) (*repositories.QuoteStamp, error) {
	for _, stamp := range f.stamps {
		if stamp.QuotedNodeID == quotedNodeID && stamp.QuoteURI == quoteURI {
			return stamp, nil
		}
	}
	return nil, nil
}

const (
	quoteID   = int64(503)
	quoteUUID = "8f1d2c3b-4a5e-4f60-9b7a-1c2d3e4f5a6b"
	quoteURL  = "https://api.example.org/ap/notes/" + quoteUUID
)

// anasQuote is a note of Ana's that quotes the post `quoted`.
func anasQuote(quoted int64) *entities.AnahitaNode {
	quote := entities.NewAnahitaNode()
	quote.ID, quote.UUID = quoteID, quoteUUID
	quote.Type = constants.NodeMediumNoteEntityType
	quote.Body = "Look at this."
	quote.Access = "public"
	quote.Enabled = true
	quote.OwnerID, quote.CreatedBy = anaID, anaID
	quote.QuoteID = quoted
	quote.CreatedAt = time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	quote.UpdatedAt = quote.CreatedAt
	return quote
}

func quotesWorld() (*inboxWorld, *fakeQuotes) {
	w := notesWorld()
	quotes := newFakeQuotes()
	w.h.Quotes = quotes
	return w, quotes
}

// THE RULE THIS GUARDS: a quote of a post here goes out as a quote, which
// Mastodon shows as one: the post named, the stamp that approves it served
// here, and the line that leads to it hidden from servers that show
// quotes. And every post says who may quote it.
func TestAQuoteOfAPostHereGoesAsAQuote(t *testing.T) {
	w, _ := quotesWorld()
	w.nodes.people = append(w.nodes.people, anasQuote(noteID))

	_ = w.h.HandleEvent(made(quoteID))
	if len(w.queue.sent) == 0 {
		t.Fatal("nothing was sent")
	}
	object, _ := w.queue.sent[0].payload["object"].(map[string]any)
	stamp := "https://api.example.org/ap/quotes/" + quoteUUID
	if object["quote"] != noteURL || object["_misskey_quote"] != noteURL || object["quoteAuthorization"] != stamp {
		t.Errorf("sent as quoting %v, approved by %v", object["quote"], object["quoteAuthorization"])
	}
	if content, _ := object["content"].(string); !strings.Contains(content, `<p class="quote-inline">RE: <a href="https://example.org/notes/500">`) {
		t.Errorf("content %q", content)
	}
	policy, _ := object["interactionPolicy"].(map[string]any)
	canQuote, _ := policy["canQuote"].(map[string]any)
	if approved, _ := canQuote["automaticApproval"].([]any); len(approved) != 1 || approved[0] != "https://www.w3.org/ns/activitystreams#Public" {
		t.Errorf("who may quote it: %v", policy)
	}

	// The stamp, as Mastodon reads it.
	rec := w.get(w.h.QuoteStamp, "/", "uuid", quoteUUID)
	body := rec.Body.String()
	for _, want := range []string{`"type":"QuoteAuthorization"`, `"attributedTo":"` + anaActor + `"`, `"interactingObject":"` + quoteURL + `"`, `"interactionTarget":"` + noteURL + `"`} {
		if rec.Code != http.StatusOK || !strings.Contains(body, want) {
			t.Errorf("the stamp (%d) lacks %s: %s", rec.Code, want, body)
		}
	}

	// Taken out of the quote, the stamp is gone.
	for _, node := range w.nodes.people {
		if node.ID == quoteID {
			node.QuoteDetached = true
		}
	}
	if rec := w.get(w.h.QuoteStamp, "/", "uuid", quoteUUID); rec.Code != http.StatusNotFound {
		t.Errorf("a stamp for a quote that was taken out: %d", rec.Code)
	}
}

// THE RULE THIS GUARDS: a quote here of a post on another server asks its
// author, once; their Accept sends the quote again with their stamp, their
// Reject sends it again without the quote. Only they are listened to.
func TestAQuoteOfAPostElsewhereAsksItsAuthor(t *testing.T) {
	w, quotes := quotesWorld()
	w.call(w.h.Lookup, http.MethodGet, "/activitypub/lookup?q=@alice@mastodon.example", "", anaID)
	var alice *repositories.RemoteActor
	for _, kept := range w.actors.kept {
		alice = kept
	}
	hers := entities.NewAnahitaNode()
	hers.ID, hers.URI, hers.Type = 9001, alicesNote, constants.NodeMediumNoteEntityType
	hers.OwnerID, hers.Access, hers.Enabled = alice.NodeID, "public", true
	hers.OriginDomain = "mastodon.example"
	w.nodes.people = append(w.nodes.people, hers, anasQuote(9001))

	_ = w.h.HandleEvent(made(quoteID))
	var created, asked map[string]any
	for _, each := range w.queue.sent {
		switch each.payload["type"] {
		case "Create":
			created = each.payload
		case "QuoteRequest":
			asked = each.payload
			if each.target != alice.Inbox {
				t.Errorf("asked at %s", each.target)
			}
		}
	}
	object, _ := created["object"].(map[string]any)
	if object["quote"] != alicesNote || object["quoteAuthorization"] != nil {
		t.Errorf("sent before her answer as %v, %v", object["quote"], object["quoteAuthorization"])
	}
	if asked == nil || asked["object"] != alicesNote || asked["actor"] != anaActor {
		t.Fatalf("asked %v", asked)
	}
	requestID := asked["id"].(string)

	// Somebody else answering is not listened to.
	w.queue.sent = nil
	w.deliver(`{"id":"https://other.example/a/1","type":"Accept","actor":"https://other.example/users/eve","object":"`+requestID+`","result":"https://other.example/stamps/1"}`,
		"https://other.example/users/eve#main-key", remoteKey.PrivatePEM, nil)
	if quotes.requests[quoteID].State != repositories.QuotePending {
		t.Errorf("an answer from somebody else was taken: %+v", quotes.requests[quoteID])
	}

	// She accepts: the quote goes again, with her stamp.
	stamp := "https://mastodon.example/users/alice/quote_authorizations/1"
	w.fromAlice(`{"id":"https://mastodon.example/a/2","type":"Accept","actor":"` + aliceID + `","object":{"id":"` + requestID + `","type":"QuoteRequest"},"result":"` + stamp + `"}`)
	if quotes.requests[quoteID].State != repositories.QuoteAccepted || quotes.requests[quoteID].StampURI != stamp {
		t.Fatalf("after her Accept: %+v", quotes.requests[quoteID])
	}
	updated := false
	for _, each := range w.queue.sent {
		if each.payload["type"] == "Update" {
			object, _ := each.payload["object"].(map[string]any)
			updated = object["quote"] == alicesNote && object["quoteAuthorization"] == stamp
		}
	}
	if !updated {
		t.Errorf("not sent again with her stamp: %+v", w.queue.sent)
	}

	// Had she said no, it goes without the quote, and leads to it.
	quotes.requests[quoteID].State = repositories.QuotePending
	w.queue.sent = nil
	w.fromAlice(`{"id":"https://mastodon.example/a/3","type":"Reject","actor":"` + aliceID + `","object":"` + requestID + `"}`)
	for _, each := range w.queue.sent {
		if each.payload["type"] == "Update" {
			object, _ := each.payload["object"].(map[string]any)
			content, _ := object["content"].(string)
			if object["quote"] != nil || !strings.Contains(content, "↪") {
				t.Errorf("after her Reject: quote %v, %q", object["quote"], content)
			}
		}
	}
}

// THE RULE THIS GUARDS: somebody elsewhere who asks to quote a post here
// is answered by its author's quote policy: an Accept with a stamp that is
// then served here, or a Reject.
func TestAQuoteRequestIsAnsweredByThePolicy(t *testing.T) {
	w, quotes := quotesWorld()
	request := func(id string) string {
		return `{"id":"` + id + `","type":"QuoteRequest","actor":"` + aliceID + `","object":"` + noteURL + `","instrument":{"id":"` + alicesNote + `","type":"Note","attributedTo":"` + aliceID + `"}}`
	}

	w.fromAlice(request("https://mastodon.example/users/alice/statuses/1/quote"))
	var answer map[string]any
	for _, each := range w.queue.sent {
		if each.payload["type"] == "Accept" || each.payload["type"] == "Reject" {
			answer = each.payload
		}
	}
	if answer == nil || answer["type"] != "Accept" || answer["actor"] != anaActor || len(quotes.stamps) != 1 {
		t.Fatalf("answered %v, %d stamps", answer, len(quotes.stamps))
	}
	stamp, _ := answer["result"].(string)
	uuid := strings.TrimPrefix(stamp, "https://api.example.org/ap/quotes/")
	rec := w.get(w.h.QuoteStamp, "/", "uuid", uuid)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"interactingObject":"`+alicesNote+`"`) ||
		!strings.Contains(rec.Body.String(), `"interactionTarget":"`+noteURL+`"`) {
		t.Errorf("her stamp (%d): %s", rec.Code, rec.Body.String())
	}

	// Ana lets nobody quote it: told no, and given no stamp.
	w.note().QuotePolicy = constants.QuotePolicyNobody
	w.queue.sent = nil
	w.fromAlice(strings.Replace(request("https://mastodon.example/users/alice/statuses/2/quote"), alicesNote, alicesNote+"0", 1))
	answer = nil
	for _, each := range w.queue.sent {
		if each.payload["type"] == "Accept" || each.payload["type"] == "Reject" {
			answer = each.payload
		}
	}
	if answer == nil || answer["type"] != "Reject" || answer["result"] != nil || len(quotes.stamps) != 1 {
		t.Errorf("for nobody: %v, %d stamps", answer, len(quotes.stamps))
	}
}

// THE RULE THIS GUARDS: a quote that comes in is kept as one only when its
// quoted post's author let it: for a post here, when this installation gave
// it a stamp. Otherwise it is kept as the words it is.
func TestAQuoteThatComesInIsKeptAsOneWhenApproved(t *testing.T) {
	w, quotes := quotesWorld()
	w.call(w.h.Lookup, http.MethodGet, "/activitypub/lookup?q=@alice@mastodon.example", "", anaID)
	var alice *repositories.RemoteActor
	for _, kept := range w.actors.kept {
		alice = kept
	}
	_ = w.follows.Save(&repositories.Follow{LocalID: anaID, RemoteID: alice.NodeID, Direction: repositories.FollowOut, ActivityID: "x", State: repositories.FollowAccepted})
	quoting := `,"quote":"` + noteURL + `"`
	line := `<p>Yes</p><p class=\"quote-inline\">RE: <a href=\"` + noteURL + `\">` + noteURL + `</a></p>`

	// No stamp: kept as words.
	w.fromAlice(received(alicesNote, line, quoting))
	kept, _ := w.nodes.find(func(n *entities.AnahitaNode) bool { return n.URI == alicesNote })
	if kept == nil || kept.QuoteID != 0 {
		t.Fatalf("a quote nobody let was kept as one: %+v", kept)
	}

	// Stamped here: kept as a quote of Ana's note, its line gone.
	quotes.stamps = append(quotes.stamps, &repositories.QuoteStamp{UUID: "s", QuotedNodeID: noteID, AuthorID: anaID, QuoteURI: alicesNote, QuoteAuthorURI: aliceID})
	w.fromAlice(strings.Replace(received(alicesNote, line, quoting), `"type":"Create"`, `"type":"Update"`, 1))
	kept, _ = w.nodes.find(func(n *entities.AnahitaNode) bool { return n.URI == alicesNote })
	if kept == nil || kept.QuoteID != noteID || strings.Contains(kept.Body, "RE:") {
		t.Errorf("an approved quote: %+v", kept)
	}
}
