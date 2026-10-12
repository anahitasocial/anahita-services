package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"activitypub/data/repositories"
	"activitypub/fetch"
	"activitypub/keys"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

var remoteKey, _ = keys.Generate()

const aliceID = "https://mastodon.example/users/alice"

// Another server: answers WebFinger for alice, and describes her.
type fakeFetcher struct {
	asked []string
	gone  bool
	kind  string
	name  string
	// Other things at her server's addresses: the list of what she posted.
	pages map[string]string
}

func (f *fakeFetcher) nameOrAlice() string {
	if f.name != "" {
		return f.name
	}
	return "Alice"
}

func (f *fakeFetcher) JSON(_ context.Context, address string, into any) (string, error) {
	f.asked = append(f.asked, address)
	asked, _ := url.Parse(address)
	if asked.Host != "mastodon.example" || asked.Query().Get("resource") != "acct:alice@mastodon.example" {
		return "", fetch.ErrGone
	}
	return address, json.Unmarshal([]byte(`{"subject":"acct:alice@mastodon.example","links":[{"rel":"self","type":"application/activity+json","href":"`+aliceID+`"}]}`), into)
}

func (f *fakeFetcher) Object(_ context.Context, address string, into any) (string, error) {
	f.asked = append(f.asked, address)
	if page, ok := f.pages[address]; ok {
		return address, json.Unmarshal([]byte(page), into)
	}
	// A key named as its owner's address with a fragment is its owner's.
	address, _, _ = strings.Cut(address, "#")
	if f.gone || address != aliceID {
		return "", fetch.ErrGone
	}
	kind := f.kind
	if kind == "" {
		kind = "Person"
	}
	raw, _ := json.Marshal(map[string]any{
		"id": aliceID, "type": kind, "preferredUsername": "alice", "name": f.nameOrAlice(),
		"summary": "<p>Grows <strong>beans</strong></p><script>alert(1)</script>",
		"url":     "https://mastodon.example/@alice",
		"outbox":  aliceID + "/outbox",
		"inbox":   aliceID + "/inbox", "endpoints": map[string]any{"sharedInbox": "https://mastodon.example/inbox"},
		"publicKey":                 map[string]any{"id": aliceID + "#main-key", "owner": aliceID, "publicKeyPem": remoteKey.PublicPEM},
		"icon":                      map[string]any{"url": "https://mastodon.example/a.png"},
		"manuallyApprovesFollowers": true,
	})
	return address, json.Unmarshal(raw, into)
}

type fakeActors struct {
	kept map[int64]*repositories.RemoteActor
}

func (f *fakeActors) Save(actor *repositories.RemoteActor) error {
	f.kept[actor.NodeID] = actor
	return nil
}
func (f *fakeActors) ReadByKeyID(keyID string) (*repositories.RemoteActor, error) {
	for _, actor := range f.kept {
		if actor.KeyID == keyID {
			return actor, nil
		}
	}
	return nil, nil
}
func (f *fakeActors) Delete(id int64) error {
	delete(f.kept, id)
	return nil
}
func (f *fakeActors) ReadByNode(id int64) (*repositories.RemoteActor, error) { return f.kept[id], nil }
func (f *fakeActors) ReadByURI(uri string) (*repositories.RemoteActor, error) {
	for _, actor := range f.kept {
		if actor.URI == uri {
			return actor, nil
		}
	}
	return nil, nil
}

// UpsertRemote as the graph does it, as far as these tests go: one node for
// each address, and no two with one handle.
func (f *fakeNodes) UpsertRemote(node *entities.AnahitaNode) (*entities.AnahitaNode, bool, error) {
	for _, kept := range f.people {
		if kept.URI == node.URI {
			kept.Name, kept.Alias, kept.Body = node.Name, node.Alias, node.Body
			if node.AvatarFilename != "" {
				kept.AvatarFilename = node.AvatarFilename
			}
			copied := *kept
			return &copied, false, nil
		}
		if node.Alias != "" && kept.Alias == node.Alias && kept.URI != "" {
			return nil, false, errors.New("rpc error: " + node.Alias + " is somebody already known")
		}
	}
	saved := *node
	saved.ID = int64(1000 + len(f.people))
	f.people = append(f.people, &saved)
	copied := saved
	return &copied, true, nil
}

func lookupWorld() (*world, *fakeFetcher, *fakeActors) {
	w := newWorld()
	fetcher := &fakeFetcher{}
	actors := &fakeActors{kept: map[int64]*repositories.RemoteActor{}}
	w.h.Fetcher = fetcher
	w.h.Actors = actors
	return w, fetcher, actors
}

func TestSomebodyOnAnotherServerIsFoundAndKept(t *testing.T) {
	for _, query := range []string{"@alice@mastodon.example", "alice@mastodon.example", " @Alice@Mastodon.Example ", aliceID} {
		w, _, actors := lookupWorld()

		rec := w.call(w.h.Lookup, http.MethodGet, "/activitypub/lookup?q="+url.QueryEscape(query), "", anaID)
		if rec.Code != http.StatusOK {
			t.Fatalf("%q: %d %s", query, rec.Code, rec.Body.String())
		}

		var found RemotePerson
		_ = json.Unmarshal(rec.Body.Bytes(), &found)
		if found.ID == 0 || found.Type != constants.NodeActorActivityPubPersonEntityType || found.Name != "Alice" ||
			found.Alias != "alice@mastodon.example" || found.URI != aliceID || found.OriginDomain != "mastodon.example" ||
			found.URL != "https://mastodon.example/@alice" || !found.Locked {
			t.Errorf("%q: found %+v", query, found)
		}
		// What she says of herself is kept as Markdown, with nothing that runs.
		if found.Body != "Grows **beans**" {
			t.Errorf("%q: body %q", query, found.Body)
		}

		kept := actors.kept[found.ID]
		if kept == nil || kept.Inbox != aliceID+"/inbox" || kept.SharedInbox != "https://mastodon.example/inbox" ||
			kept.KeyID != aliceID+"#main-key" || kept.PublicKey != remoteKey.PublicPEM || kept.IconURL != "https://mastodon.example/a.png" {
			t.Errorf("%q: kept %+v", query, kept)
		}
	}

	// Looked up twice, she is one node.
	w, _, actors := lookupWorld()
	for i := 0; i < 2; i++ {
		w.call(w.h.Lookup, http.MethodGet, "/activitypub/lookup?q=@alice@mastodon.example", "", anaID)
	}
	if len(actors.kept) != 1 {
		t.Errorf("%d kept", len(actors.kept))
	}
}

func TestWhoIsNotFound(t *testing.T) {
	cases := map[string]struct {
		query  string
		change func(f *fakeFetcher)
		status int
		code   string
	}{
		"nothing asked":           {"", nil, http.StatusBadRequest, errLookupQuery},
		"a name with no server":   {"@alice", nil, http.StatusBadRequest, errLookupQuery},
		"two names":               {"@alice@mastodon.example @bob@mastodon.example", nil, http.StatusBadRequest, errLookupQuery},
		"words":                   {"alice from mastodon", nil, http.StatusBadRequest, errLookupQuery},
		"somebody here":           {"@ana@example.org", nil, http.StatusNotFound, errLookupOurs},
		"an address of ours":      {"https://api.example.org/ap/users/" + anaUUID, nil, http.StatusNotFound, errLookupOurs},
		"nobody of that name":     {"@nobody@mastodon.example", nil, http.StatusNotFound, errLookupNobody},
		"a server with no such":   {"@alice@elsewhere.example", nil, http.StatusNotFound, errLookupNobody},
		"gone since":              {"@alice@mastodon.example", func(f *fakeFetcher) { f.gone = true }, http.StatusNotFound, errLookupNobody},
		"a group":                 {"@alice@mastodon.example", func(f *fakeFetcher) { f.kind = "Group" }, http.StatusNotFound, errLookupKind},
		"an address in the clear": {"http://mastodon.example/users/alice", nil, http.StatusBadRequest, errLookupQuery},
	}

	for name, c := range cases {
		w, fetcher, actors := lookupWorld()
		if c.change != nil {
			c.change(fetcher)
		}
		rec := w.call(w.h.Lookup, http.MethodGet, "/activitypub/lookup?q="+url.QueryEscape(c.query), "", anaID)
		if rec.Code != c.status || !strings.Contains(rec.Body.String(), c.code) {
			t.Errorf("%s: %d %s, want %d %s", name, rec.Code, rec.Body.String(), c.status, c.code)
		}
		if len(actors.kept) != 0 {
			t.Errorf("%s: somebody was kept", name)
		}
	}

	// One of ours is never asked of anybody.
	w, fetcher, _ := lookupWorld()
	w.call(w.h.Lookup, http.MethodGet, "/activitypub/lookup?q=@ana@example.org", "", anaID)
	if len(fetcher.asked) != 0 {
		t.Errorf("asked %v about somebody here", fetcher.asked)
	}

	// A site that does not federate looks nobody up.
	w, fetcher, _ = lookupWorld()
	w.h.Site.Domain, w.h.Site.WebURL, w.h.Site.APIURL = "localhost", "http://localhost:3000", "http://localhost"
	rec := w.call(w.h.Lookup, http.MethodGet, "/activitypub/lookup?q=@alice@mastodon.example", "", anaID)
	if rec.Code != http.StatusConflict || len(fetcher.asked) != 0 {
		t.Errorf("a site with no address: %d, asked %v", rec.Code, fetcher.asked)
	}
}

// Somebody else kept here goes by the handle: the newcomer is called by the
// server they are on.
func TestAHandleSomebodyElseHas(t *testing.T) {
	w, _, _ := lookupWorld()
	w.nodes.people = append(w.nodes.people, &entities.AnahitaNode{
		ID: 900, URI: "https://old.example/users/alice", Alias: "alice@mastodon.example",
		Type: constants.NodeActorActivityPubPersonEntityType,
	})

	rec := w.call(w.h.Lookup, http.MethodGet, "/activitypub/lookup?q="+url.QueryEscape(aliceID), "", anaID)
	// Fetched by address, her handle is made of the server she is on, which
	// is the one taken; there is no other to fall back on.
	if rec.Code != http.StatusBadGateway {
		t.Errorf("%d %s", rec.Code, rec.Body.String())
	}
}

func TestTheInstallationDescribesItself(t *testing.T) {
	w := newWorld()
	rec := w.get(w.h.InstanceActor, "/ap/actor")
	if rec.Code != http.StatusOK {
		t.Fatalf("%d", rec.Code)
	}
	var actor map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &actor)
	key, _ := actor["publicKey"].(map[string]any)
	if actor["id"] != "https://api.example.org/ap/actor" || actor["type"] != "Application" ||
		key["id"] != "https://api.example.org/ap/actor#main-key" || key["owner"] != actor["id"] {
		t.Errorf("described as %v", actor)
	}

	// The key it is said to have is the one it signs with.
	keyID, private, err := w.h.SignAsInstance()
	if err != nil || keyID != "https://api.example.org/ap/actor#main-key" {
		t.Fatalf("%q %v", keyID, err)
	}
	public, _ := keys.ParsePublic(key["publicKeyPem"].(string))
	if public == nil || private.PublicKey.N.Cmp(public.N) != 0 {
		t.Error("it signs with a key other than the one it tells")
	}

	// A site that does not federate has no such actor.
	w.h.Site.Domain, w.h.Site.WebURL, w.h.Site.APIURL = "localhost", "http://localhost:3000", "http://localhost"
	if rec := w.get(w.h.InstanceActor, "/ap/actor"); rec.Code != http.StatusNotFound {
		t.Errorf("with no address: %d", rec.Code)
	}
}
