package remote

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"testing"

	"activitypub/keys"
)

var pair, _ = keys.Generate()

// A server that answers what it is given, at the addresses it is given.
type fakeFetcher struct {
	objects map[string]map[string]any
	fingers map[string]string
	// Where an address is finally answered from, when not itself.
	movedTo map[string]string
}

func (f *fakeFetcher) Object(_ context.Context, address string, into any) (string, error) {
	object, ok := f.objects[address]
	if !ok {
		return "", errors.New("nothing there")
	}
	raw, _ := json.Marshal(object)
	from := address
	if moved := f.movedTo[address]; moved != "" {
		from = moved
	}
	return from, json.Unmarshal(raw, into)
}

func (f *fakeFetcher) JSON(_ context.Context, address string, into any) (string, error) {
	asked, _ := url.Parse(address)
	for resource, self := range f.fingers {
		if asked != nil && asked.Query().Get("resource") == resource {
			raw, _ := json.Marshal(map[string]any{
				"subject": resource,
				"links": []map[string]string{
					{"rel": "http://webfinger.net/rel/profile-page", "type": "text/html", "href": "https://mastodon.example/@alice"},
					{"rel": "self", "type": "application/activity+json", "href": self},
				},
			})
			return address, json.Unmarshal(raw, into)
		}
	}
	return "", errors.New("nobody")
}

const aliceID = "https://mastodon.example/users/alice"

func alice() map[string]any {
	return map[string]any{
		"id":                aliceID,
		"type":              "Person",
		"preferredUsername": "Alice",
		"name":              "Alice",
		"summary":           `<p>Grows <a href="https://mastodon.example/tags/beans" class="mention hashtag" rel="tag">#<span>beans</span></a><script>alert(1)</script></p>`,
		"url":               "https://mastodon.example/@alice",
		"inbox":             aliceID + "/inbox",
		"endpoints":         map[string]any{"sharedInbox": "https://mastodon.example/inbox"},
		"publicKey": map[string]any{
			"id": aliceID + "#main-key", "owner": aliceID, "publicKeyPem": pair.PublicPEM,
		},
		"icon":                      map[string]any{"type": "Image", "url": "https://files.mastodon.example/a.png"},
		"manuallyApprovesFollowers": true,
	}
}

func ours(address string) bool {
	return strings.HasPrefix(address, "https://api.example.org/") || strings.HasPrefix(address, "https://example.org/")
}

func TestSomebodyElsewhereIsReadAsTheyDescribeThemselves(t *testing.T) {
	f := &fakeFetcher{objects: map[string]map[string]any{aliceID: alice()}}

	actor, err := Fetch(context.Background(), f, aliceID, ours)
	if err != nil {
		t.Fatal(err)
	}
	if actor.ID != aliceID || actor.Domain != "mastodon.example" || actor.Handle != "alice@mastodon.example" ||
		actor.Username != "Alice" || actor.Name != "Alice" || actor.Page != "https://mastodon.example/@alice" ||
		actor.Inbox != aliceID+"/inbox" || actor.SharedInbox != "https://mastodon.example/inbox" ||
		actor.KeyID != aliceID+"#main-key" || actor.Icon != "https://files.mastodon.example/a.png" || !actor.Locked {
		t.Errorf("read as %+v", actor)
	}
	if actor.Bio != "Grows [#beans](https://mastodon.example/tags/beans)" {
		t.Errorf("bio %q", actor.Bio)
	}

	// What servers send in more than one shape.
	shapes := alice()
	shapes["type"] = []string{"Service"}
	shapes["url"] = []any{map[string]any{"type": "Link", "href": "https://mastodon.example/@alice"}}
	shapes["icon"] = []any{map[string]any{"url": map[string]any{"href": "https://files.mastodon.example/b.png"}}}
	shapes["name"] = ""
	f.objects[aliceID] = shapes
	actor, err = Fetch(context.Background(), f, aliceID, ours)
	if err != nil || actor.Page != "https://mastodon.example/@alice" || actor.Icon != "https://files.mastodon.example/b.png" || actor.Name != "Alice" {
		t.Errorf("other shapes read as %+v (%v)", actor, err)
	}
}

// A server is believed about itself and nobody else.
func TestWhatAServerMayNotSay(t *testing.T) {
	cases := map[string]struct {
		change func(a map[string]any)
		want   error
	}{
		"that it is somebody on another server": {func(a map[string]any) { a["id"] = "https://other.example/users/alice" }, ErrNotTheirs},
		"that it is somebody here":              {func(a map[string]any) { a["id"] = "https://api.example.org/ap/users/1" }, ErrNotTheirs},
		"a key under another's name": {func(a map[string]any) {
			a["publicKey"].(map[string]any)["owner"] = "https://mastodon.example/users/bob"
		}, ErrNotTheirs},
		"a key on another server":    {func(a map[string]any) { a["publicKey"].(map[string]any)["id"] = "https://other.example/key" }, ErrNotTheirs},
		"a key that is no key":       {func(a map[string]any) { a["publicKey"].(map[string]any)["publicKeyPem"] = "nonsense" }, ErrNotAPerson},
		"an inbox in the clear":      {func(a map[string]any) { a["inbox"] = "http://mastodon.example/inbox" }, ErrNotAPerson},
		"an inbox here":              {func(a map[string]any) { a["inbox"] = "https://api.example.org/inbox" }, ErrNotAPerson},
		"no name to go by":           {func(a map[string]any) { a["preferredUsername"] = "" }, ErrNotAPerson},
		"a name with a handle in it": {func(a map[string]any) { a["preferredUsername"] = "ana@example.org" }, ErrNotAPerson},
		"a group":                    {func(a map[string]any) { a["type"] = "Group" }, ErrNotAPerson},
		"a post":                     {func(a map[string]any) { a["type"] = "Note" }, ErrNotAPerson},
	}

	for name, c := range cases {
		described := alice()
		c.change(described)
		f := &fakeFetcher{objects: map[string]map[string]any{aliceID: described}}
		if _, err := Fetch(context.Background(), f, aliceID, ours); !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", name, err, c.want)
		}
	}

	// An address that is answered from another server altogether.
	f := &fakeFetcher{objects: map[string]map[string]any{aliceID: alice()}, movedTo: map[string]string{aliceID: "https://evil.example/users/alice"}}
	if _, err := Fetch(context.Background(), f, aliceID, ours); !errors.Is(err, ErrNotTheirs) {
		t.Errorf("answered from elsewhere: %v", err)
	}

	// One of ours is never asked about.
	if _, err := Fetch(context.Background(), f, "https://api.example.org/ap/users/1", ours); !errors.Is(err, ErrNotTheirs) {
		t.Errorf("one of ours: %v", err)
	}
}

func TestAHandleIsTheirsWhenItLeadsToThem(t *testing.T) {
	// Somebody whose handle is on one host and whose address is on another,
	// as people here are.
	const id = "https://api.social.example/ap/users/77"
	described := alice()
	described["id"] = id
	described["inbox"] = id + "/inbox"
	described["publicKey"] = map[string]any{"id": id + "#main-key", "owner": id, "publicKeyPem": pair.PublicPEM}

	f := &fakeFetcher{
		objects: map[string]map[string]any{id: described},
		fingers: map[string]string{"acct:alice@social.example": id},
	}

	actor, err := Lookup(context.Background(), f, "alice", "social.example", ours)
	if err != nil {
		t.Fatal(err)
	}
	if actor.Handle != "alice@social.example" || actor.Domain != "api.social.example" {
		t.Errorf("called %s on %s", actor.Handle, actor.Domain)
	}

	// A handle that leads to somebody who goes by another name is not
	// theirs: they are called by where they are.
	f.fingers = map[string]string{"acct:somebody@social.example": id}
	actor, err = Lookup(context.Background(), f, "somebody", "social.example", ours)
	if err != nil || actor.Handle != "alice@api.social.example" {
		t.Errorf("called %v (%v)", actor, err)
	}

	if _, err := Lookup(context.Background(), f, "nobody", "social.example", ours); err == nil {
		t.Error("nobody was found")
	}
}
