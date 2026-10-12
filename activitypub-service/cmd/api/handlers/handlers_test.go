package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"activitypub/ap"
	"activitypub/data/repositories"
	"activitypub/keys"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/policy"
	"anahita-libs/site"

	"github.com/labstack/echo/v4"
)

const (
	anaID   = 7
	anaUUID = "0962789f-aeca-4850-ac9b-41129e3b6f46"
)

type fakeNodes struct {
	services.GraphNodeGRPCInterface
	people []*entities.AnahitaNode
	hidden map[int64]bool
}

func (f *fakeNodes) find(match func(*entities.AnahitaNode) bool) (*entities.AnahitaNode, error) {
	for _, person := range f.people {
		if match(person) {
			copied := *person
			return &copied, nil
		}
	}
	return nil, nil
}

func (f *fakeNodes) Read(id int64) (*entities.AnahitaNode, error) {
	return f.find(func(n *entities.AnahitaNode) bool { return n.ID == id })
}

func (f *fakeNodes) ReadByUUID(uuid string) (*entities.AnahitaNode, error) {
	return f.find(func(n *entities.AnahitaNode) bool { return n.UUID == uuid })
}

func (f *fakeNodes) ReadByAlias(req *services.GraphNodeReadByAlias) (*entities.AnahitaNode, error) {
	return f.find(func(n *entities.AnahitaNode) bool {
		return strings.EqualFold(n.Alias, req.Alias) && n.Type == req.NodeType
	})
}

func (f *fakeNodes) FilterVisible(ids []int64, viewer *entities.Viewer) ([]int64, error) {
	var seen []int64
	for _, id := range ids {
		if !f.hidden[id] {
			seen = append(seen, id)
		}
	}
	return seen, nil
}

type fakeAccounts struct{ enabled map[int64]bool }

func (f *fakeAccounts) Read(id int64) (*repositories.Account, error) {
	return &repositories.Account{PersonID: id, Enabled: f.enabled[id]}, nil
}

func (f *fakeAccounts) Set(id int64, enabled bool) error {
	f.enabled[id] = enabled
	return nil
}

type fakeKeys struct {
	mu   sync.Mutex
	kept map[int64]*repositories.Key
	adds int
}

func (f *fakeKeys) Active(id int64) (*repositories.Key, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.kept[id], nil
}

func (f *fakeKeys) Add(key *repositories.Key) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.kept[key.NodeID] != nil {
		return repositories.ErrKeyExists
	}
	f.kept[key.NodeID] = key
	f.adds++
	return nil
}

// Kept in a way that shows whether it was encrypted at all.
type fakeCrypt struct{}

func (fakeCrypt) Encrypt(text string) (string, error) { return "sealed:" + text, nil }
func (fakeCrypt) Decrypt(text string) (string, error) {
	return strings.TrimPrefix(text, "sealed:"), nil
}

type world struct {
	h        *Handlers
	nodes    *fakeNodes
	accounts *fakeAccounts
	keys     *fakeKeys
	ana      *entities.AnahitaNode
}

func newWorld() *world {
	ana := entities.NewAnahitaNode()
	ana.ID = anaID
	ana.UUID = anaUUID
	ana.Type = constants.NodeActorPersonEntityType
	ana.Alias = "ana"
	ana.Name = "Ana"
	ana.Body = "Grows #beans. <script>alert(1)</script>"
	ana.Access = "public"
	ana.Enabled = true
	ana.AvatarFilename = "abc.png"
	ana.FollowerCount = 12
	ana.LeaderCount = 3
	ana.CreatedAt = time.Date(2020, 5, 1, 0, 0, 0, 0, time.UTC)

	w := &world{
		nodes:    &fakeNodes{people: []*entities.AnahitaNode{ana}, hidden: map[int64]bool{}},
		accounts: &fakeAccounts{enabled: map[int64]bool{anaID: true}},
		keys:     &fakeKeys{kept: map[int64]*repositories.Key{}},
		ana:      ana,
	}
	w.h = NewHandlers(&Env{
		Site:     site.Addresses{Domain: "example.org", WebURL: "https://example.org", APIURL: "https://api.example.org"},
		Nodes:    w.nodes,
		Accounts: w.accounts,
		Keys:     w.keys,
		Crypt:    fakeCrypt{},
		// Nothing crosses between servers in these: empty, and there so that
		// turning the fediverse off has somewhere to look.
		Follows:     &fakeFollows{kept: map[string]*repositories.Follow{}},
		Objects:     &fakeObjects{kept: map[int64]*repositories.Object{}},
		FollowEdges: &fakeEdges{},
		Queue:       &fakeQueue{},
	})
	return w
}

func (w *world) get(handler echo.HandlerFunc, target string, params ...string) *httptest.ResponseRecorder {
	return w.call(handler, http.MethodGet, target, "", 0, params...)
}

// For the tests' own shorthand.
type (
	entitiesNode = entities.AnahitaNode
	echoContext  = echo.Context
)

// callAs calls a handler as somebody of a given standing: "administrator".
func (w *world) callAs(personType string, handler echo.HandlerFunc, method, body string, params ...string) *httptest.ResponseRecorder {
	e := echo.New()
	req := httptest.NewRequest(method, "/", strings.NewReader(body))
	if body != "" {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	for i := 0; i+1 < len(params); i += 2 {
		c.SetParamNames(params[i])
		c.SetParamValues(params[i+1])
	}
	v := entities.NewViewer()
	v.ID = 2
	v.PersonType = personType
	c.Set(constants.ViewerContextKey, v)
	_ = handler(c)
	return rec
}

func (w *world) call(handler echo.HandlerFunc, method, target, body string, viewer int64, params ...string) *httptest.ResponseRecorder {
	e := echo.New()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	for i := 0; i+1 < len(params); i += 2 {
		c.SetParamNames(params[i])
		c.SetParamValues(params[i+1])
	}
	if viewer > 0 {
		v := entities.NewViewer()
		v.ID = viewer
		c.Set(constants.ViewerContextKey, v)
	}
	_ = handler(c)
	return rec
}

// Everything that makes somebody not be there, to another server. Each must
// answer exactly as a name nobody has.
func notThere() map[string]func(w *world) {
	return map[string]func(w *world){
		"never chose to be":       func(w *world) { w.accounts.enabled[anaID] = false },
		"a profile for members":   func(w *world) { w.ana.Access = "registered" },
		"a profile for followers": func(w *world) { w.ana.Access = "followers" },
		"disabled":                func(w *world) { w.ana.Enabled = false },
		"on the way to deleted":   func(w *world) { now := time.Now(); w.ana.DeletedAt = &now },
		"hidden by the graph":     func(w *world) { w.nodes.hidden[anaID] = true },
		"a group":                 func(w *world) { w.ana.Type = "node.actor.group-service.group.v1" },
		"somebody from elsewhere": func(w *world) { w.ana.URI = "https://mastodon.example/users/ana" },
		"a site with no address": func(w *world) {
			w.h.Site = site.Addresses{Domain: "localhost", WebURL: "http://localhost:3000", APIURL: "http://localhost"}
		},
	}
}

func TestWebFingerFindsWhoChoseToBeFound(t *testing.T) {
	w := newWorld()

	for _, resource := range []string{
		"acct:ana@example.org",
		"acct:Ana@EXAMPLE.org",
		// Asked of the API's host, as a server does that met her by her address.
		"acct:ana@api.example.org",
		"https://api.example.org/ap/users/" + anaUUID,
		"https://example.org/people/ana",
	} {
		rec := w.get(w.h.WebFinger, "/.well-known/webfinger?resource="+resource)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d", resource, rec.Code)
		}
		var jrd ap.JRD
		if err := json.Unmarshal(rec.Body.Bytes(), &jrd); err != nil {
			t.Fatal(err)
		}
		if jrd.Subject != "acct:ana@example.org" || len(jrd.Links) == 0 ||
			jrd.Links[0].Rel != "self" || jrd.Links[0].Type != "application/activity+json" ||
			jrd.Links[0].Href != "https://api.example.org/ap/users/"+anaUUID {
			t.Errorf("%s: answered %+v", resource, jrd)
		}
		if !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/jrd+json") || rec.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Errorf("%s: headers %v", resource, rec.Header())
		}
	}

	// Nobody, and somebody on another site, and nothing that is a handle.
	for _, resource := range []string{
		"acct:nobody@example.org",
		"acct:ana@elsewhere.example",
		"acct:ana",
		"acct:a/../ana@example.org",
		"https://evil.example/ap/users/" + anaUUID,
		"https://api.example.org/ap/users/" + anaUUID + "/inbox",
		"mailto:ana@example.org",
	} {
		if rec := w.get(w.h.WebFinger, "/.well-known/webfinger?resource="+resource); rec.Code != http.StatusNotFound || rec.Body.Len() != 0 {
			t.Errorf("%s: %d %q", resource, rec.Code, rec.Body.String())
		}
	}

	if rec := w.get(w.h.WebFinger, "/.well-known/webfinger"); rec.Code != http.StatusBadRequest {
		t.Errorf("asked for nobody: %d", rec.Code)
	}
}

func TestWebFingerFindsTheInstallationItself(t *testing.T) {
	w := newWorld()
	for _, resource := range []string{"acct:example.org@example.org", "acct:example.org@api.example.org", "https://api.example.org/ap/actor"} {
		rec := w.get(w.h.WebFinger, "/.well-known/webfinger?resource="+resource)
		var jrd ap.JRD
		_ = json.Unmarshal(rec.Body.Bytes(), &jrd)
		if rec.Code != http.StatusOK || jrd.Subject != "acct:example.org@example.org" || len(jrd.Links) != 1 || jrd.Links[0].Href != "https://api.example.org/ap/actor" {
			t.Errorf("%s: %d %+v", resource, rec.Code, jrd)
		}
	}

	w.h.Site = site.Addresses{Domain: "localhost", WebURL: "http://localhost:3000", APIURL: "http://localhost"}
	if rec := w.get(w.h.WebFinger, "/.well-known/webfinger?resource=acct:localhost@localhost"); rec.Code != http.StatusNotFound {
		t.Errorf("a site with no address: %d", rec.Code)
	}
}

func TestSomebodyWhoIsNotOnTheFediverseIsNotThere(t *testing.T) {
	for name, change := range notThere() {
		w := newWorld()
		change(w)
		// The fake hands out a copy; put the change where it is read.
		w.nodes.people[0] = w.ana

		asks := map[string]*httptest.ResponseRecorder{
			"webfinger": w.get(w.h.WebFinger, "/.well-known/webfinger?resource=acct:ana@example.org"),
			"actor":     w.get(w.h.Actor, "/ap/users/"+anaUUID, "uuid", anaUUID),
			"followers": w.get(w.h.Followers, "/", "uuid", anaUUID),
			"following": w.get(w.h.Following, "/", "uuid", anaUUID),
		}
		for what, rec := range asks {
			if rec.Code != http.StatusNotFound || rec.Body.Len() != 0 {
				t.Errorf("%s, %s: %d %q", name, what, rec.Code, rec.Body.String())
			}
		}
		if w.keys.adds != 0 {
			t.Errorf("%s: a key was made for somebody who is not there", name)
		}
	}
}

func TestASiteThatIsNotOpenHasNobodyOnTheFediverse(t *testing.T) {
	for _, access := range []string{policy.ReadPreview, policy.ReadRegistered} {
		restore := policy.SetReadAccessForTest(access)
		w := newWorld()
		if rec := w.get(w.h.Actor, "/", "uuid", anaUUID); rec.Code != http.StatusNotFound {
			t.Errorf("%s: actor %d", access, rec.Code)
		}
		rec := w.call(w.h.ReadSettings, http.MethodGet, "/", "", anaID)
		var settings Settings
		_ = json.Unmarshal(rec.Body.Bytes(), &settings)
		if settings.Available || settings.Enabled || settings.Reason != reasonSitePrivate {
			t.Errorf("%s: settings %+v", access, settings)
		}
		restore()
	}
}

func TestAPersonIsDescribedAsTheirPublicProfileIs(t *testing.T) {
	w := newWorld()

	rec := w.get(w.h.Actor, "/ap/users/"+anaUUID, "uuid", anaUUID)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != ap.ContentType {
		t.Fatalf("%d %s", rec.Code, rec.Header().Get("Content-Type"))
	}

	var actor ap.Actor
	if err := json.Unmarshal(rec.Body.Bytes(), &actor); err != nil {
		t.Fatal(err)
	}

	base := "https://api.example.org/ap/users/" + anaUUID
	if actor.ID != base || actor.Type != "Person" || actor.PreferredUsername != "ana" || actor.Name != "Ana" ||
		actor.Inbox != base+"/inbox" || actor.Outbox != base+"/outbox" ||
		actor.Followers != base+"/followers" || actor.Following != base+"/following" ||
		actor.URL != "https://example.org/people/ana" ||
		actor.Endpoints.SharedInbox != "https://api.example.org/inbox" ||
		actor.Published != "2020-05-01T00:00:00Z" {
		t.Errorf("described as %+v", actor)
	}
	if actor.Icon == nil || actor.Icon.URL != "https://api.example.org/avatars/"+anaUUID+"/large" || actor.Icon.MediaType != "image/png" || actor.Image != nil {
		t.Errorf("pictures %+v %+v", actor.Icon, actor.Image)
	}

	// What they say of themselves is sent as HTML that is safe to show.
	if !strings.Contains(actor.Summary, `href="https://example.org/hashtags/beans"`) || strings.Contains(actor.Summary, "<script") {
		t.Errorf("summary %q", actor.Summary)
	}

	// The key they are said to have is one that can be read, and it is
	// theirs.
	if actor.PublicKey.ID != base+"#main-key" || actor.PublicKey.Owner != base {
		t.Errorf("key %+v", actor.PublicKey)
	}
	if _, err := keys.ParsePublic(actor.PublicKey.PublicKeyPem); err != nil {
		t.Errorf("the key told cannot be read: %v", err)
	}

	// The private half was kept, encrypted, and matches the public one.
	kept := w.keys.kept[anaID]
	if kept == nil || !strings.HasPrefix(kept.PrivateEncrypted, "sealed:") || strings.Contains(kept.PublicKey, "PRIVATE") {
		t.Fatalf("kept %+v", kept)
	}
	private, err := keys.ParsePrivate(strings.TrimPrefix(kept.PrivateEncrypted, "sealed:"))
	if err != nil {
		t.Fatal(err)
	}
	public, _ := keys.ParsePublic(actor.PublicKey.PublicKeyPem)
	if private.PublicKey.N.Cmp(public.N) != 0 {
		t.Error("the two halves are of different keys")
	}

	// Asked again, and by many at once, it is the same key.
	var wait sync.WaitGroup
	for i := 0; i < 8; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			again := w.get(w.h.Actor, "/", "uuid", anaUUID)
			var a ap.Actor
			_ = json.Unmarshal(again.Body.Bytes(), &a)
			if a.PublicKey.PublicKeyPem != actor.PublicKey.PublicKeyPem {
				t.Error("somebody was told a different key")
			}
		}()
	}
	wait.Wait()
	if w.keys.adds != 1 {
		t.Errorf("%d keys were kept", w.keys.adds)
	}

	// How many follow, and not who.
	followers := w.get(w.h.Followers, "/", "uuid", anaUUID)
	var list map[string]any
	_ = json.Unmarshal(followers.Body.Bytes(), &list)
	if list["totalItems"] != float64(12) || list["orderedItems"] != nil || list["first"] != nil {
		t.Errorf("followers %v", list)
	}
}

func TestOnlyThePersonChoosesAndOnlyWhenTheyCould(t *testing.T) {
	w := newWorld()
	w.accounts.enabled[anaID] = false

	read := func() Settings {
		var settings Settings
		_ = json.Unmarshal(w.call(w.h.ReadSettings, http.MethodGet, "/", "", anaID).Body.Bytes(), &settings)
		return settings
	}

	if got := read(); !got.Available || got.Enabled || got.Handle != "ana@example.org" {
		t.Fatalf("to begin with %+v", got)
	}

	// On: a key is made first, and then they are there.
	if rec := w.call(w.h.EditSettings, http.MethodPut, "/", `{"enabled":true}`, anaID); rec.Code != http.StatusOK {
		t.Fatalf("turning it on: %d %s", rec.Code, rec.Body.String())
	}
	if !read().Enabled || w.keys.kept[anaID] == nil {
		t.Error("turned on, they are not on")
	}
	if rec := w.get(w.h.Actor, "/", "uuid", anaUUID); rec.Code != http.StatusOK {
		t.Errorf("turned on, they are not found: %d", rec.Code)
	}

	// Off: gone at once.
	if rec := w.call(w.h.EditSettings, http.MethodPut, "/", `{"enabled":false}`, anaID); rec.Code != http.StatusOK {
		t.Fatalf("turning it off: %d", rec.Code)
	}
	if rec := w.get(w.h.Actor, "/", "uuid", anaUUID); rec.Code != http.StatusNotFound {
		t.Errorf("turned off, they are still found: %d", rec.Code)
	}

	// Somebody whose profile is not open to everybody cannot turn it on,
	// and is told why; they can always turn it off.
	w.nodes.people[0].Access = "followers"
	rec := w.call(w.h.EditSettings, http.MethodPut, "/", `{"enabled":true}`, anaID)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), reasonProfileNotPublic) || w.accounts.enabled[anaID] {
		t.Errorf("a profile for followers: %d %s", rec.Code, rec.Body.String())
	}
	if rec := w.call(w.h.EditSettings, http.MethodPut, "/", `{"enabled":false}`, anaID); rec.Code != http.StatusOK {
		t.Errorf("turning it off with a profile for followers: %d", rec.Code)
	}

	// Nothing said is not a choice.
	if rec := w.call(w.h.EditSettings, http.MethodPut, "/", `{}`, anaID); rec.Code != http.StatusBadRequest {
		t.Errorf("nothing said: %d", rec.Code)
	}
}

// Whether somebody is on the fediverse is theirs to know, and an
// administrator's, who can change their profile in ways that end it.
func TestWhoMayAskWhetherSomebodyIsOnTheFediverse(t *testing.T) {
	w := newWorld()

	e := func(viewer int64, personType, query string) *httptest.ResponseRecorder {
		ec := echo.New()
		rec := httptest.NewRecorder()
		c := ec.NewContext(httptest.NewRequest(http.MethodGet, "/activitypub/settings"+query, nil), rec)
		v := entities.NewViewer()
		v.ID = viewer
		v.PersonType = personType
		c.Set(constants.ViewerContextKey, v)
		_ = w.h.ReadSettings(c)
		return rec
	}

	if rec := e(anaID, "registered", "?person_id=7"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"enabled":true`) {
		t.Errorf("herself: %d %s", rec.Code, rec.Body.String())
	}
	if rec := e(99, "administrator", "?person_id=7"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"enabled":true`) {
		t.Errorf("an administrator: %d %s", rec.Code, rec.Body.String())
	}
	if rec := e(99, "registered", "?person_id=7"); rec.Code != http.StatusForbidden {
		t.Errorf("somebody else: %d", rec.Code)
	}
	if rec := e(99, "administrator", "?person_id=nobody"); rec.Code != http.StatusBadRequest {
		t.Errorf("no number: %d", rec.Code)
	}
	if rec := e(99, "administrator", "?person_id=4040"); rec.Code != http.StatusNotFound {
		t.Errorf("nobody: %d", rec.Code)
	}
}

func TestHostMetaSaysWhereWebFingerIs(t *testing.T) {
	w := newWorld()
	rec := w.get(w.h.HostMeta, "/.well-known/host-meta")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `template="https://api.example.org/.well-known/webfinger?resource={uri}"`) {
		t.Errorf("%d %s", rec.Code, rec.Body.String())
	}
}
