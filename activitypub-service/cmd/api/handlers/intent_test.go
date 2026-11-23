package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"activitypub/remote"
)

// Another server's WebFinger, publishing where its people act on things
// from elsewhere, as mastodon.social does; or nothing of the kind.
type intentServer struct {
	templates map[string]string
}

func (s *intentServer) JSON(_ context.Context, address string, into any) (string, error) {
	asked, _ := url.Parse(address)
	links := []map[string]string{{"rel": "self", "type": "application/activity+json", "href": aliceID}}
	for rel, template := range s.templates {
		links = append(links, map[string]string{"rel": rel, "template": template})
	}
	raw, _ := json.Marshal(map[string]any{"subject": asked.Query().Get("resource"), "links": links})
	return address, json.Unmarshal(raw, into)
}

func (s *intentServer) Object(_ context.Context, address string, _ any) (string, error) {
	return address, nil
}

var _ remote.Fetcher = (*intentServer)(nil)

// THE RULE THIS GUARDS (roadmap row 134): somebody from another server is
// sent to their own server's page for a profile or a post here, and only
// for one that is on the fediverse.
func TestSomebodyFromElsewhereIsSentToTheirOwnServer(t *testing.T) {
	w := newWorld()
	w.h.Fetcher = &intentServer{templates: map[string]string{
		"http://ostatus.org/schema/1.0/subscribe": "https://mastodon.example/authorize_interaction?uri={uri}",
		"https://w3id.org/fep/3b86/Object":        "https://mastodon.example/authorize_interaction?uri={object}",
	}}
	ana := "https://api.example.org/ap/users/" + anaUUID

	ask := func(query string) (int, Intent) {
		rec := w.get(w.h.Intent, "/activitypub/intent?"+query)
		var answer Intent
		_ = json.Unmarshal(rec.Body.Bytes(), &answer)
		return rec.Code, answer
	}

	code, answer := ask("handle=@alice@mastodon.example&node_id=" + itoa(anaID))
	if code != http.StatusOK || answer.URL != "https://mastodon.example/authorize_interaction?uri="+url.QueryEscape(ana) {
		t.Errorf("following Ana: %d %+v", code, answer)
	}

	// A server that publishes no such page: what to search for instead.
	w.h.Fetcher = &intentServer{}
	code, answer = ask("handle=alice@mastodon.example&node_id=" + itoa(anaID))
	if code != http.StatusOK || answer.URL != "" || answer.Error != errIntentNone || answer.Address != ana {
		t.Errorf("a server with no page: %d %+v", code, answer)
	}

	// Not a handle, or a handle here.
	for _, handle := range []string{"alice", "@alice@", "ana@example.org", "https://mastodon.example/@alice"} {
		if code, _ := ask("handle=" + url.QueryEscape(handle) + "&node_id=" + itoa(anaID)); code != http.StatusBadRequest {
			t.Errorf("%q: %d", handle, code)
		}
	}

	// Nothing on the fediverse to act on.
	for _, id := range []string{"0", "999999", "abc"} {
		if code, _ := ask("handle=@alice@mastodon.example&node_id=" + id); code != http.StatusNotFound {
			t.Errorf("node %s: %d", id, code)
		}
	}
}
