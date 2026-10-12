package shell

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/policy"
)

const index = `<!doctype html><html lang="en"><head><meta charset="utf-8"/>
<title>The Allotment</title><meta property="og:title" content="The Allotment"/>
<meta name="description" content="Growers &amp; friends"/><meta property="og:description" content="Growers &amp; friends"/>
<meta property="og:image" content="/statics/media/ogimage.jpg"/>
<script defer="defer" src="/static/js/main.abc.js"></script></head><body><div id="root"></div></body></html>`

type fakeReader struct {
	people map[string]*entities.AnahitaNode
	actors map[int64]*entities.AnahitaNode
	media  map[int64]*entities.AnahitaNode
	reads  int
}

func (f *fakeReader) Person(alias string) (*entities.AnahitaNode, error) {
	f.reads++
	return f.people[alias], nil
}

func (f *fakeReader) Actor(id int64, nodeType string) (*entities.AnahitaNode, error) {
	f.reads++
	if node := f.actors[id]; node != nil && node.Type == nodeType {
		return node, nil
	}
	return nil, nil
}

func (f *fakeReader) Medium(id int64) (*entities.AnahitaNode, error) {
	f.reads++
	if node := f.media[id]; node != nil {
		return node, nil
	}
	return nil, errors.New("not found")
}

type fakeSigner struct{}

func (fakeSigner) Sign(key string) (string, error) {
	return "https://storage.example/" + key + "?signed", nil
}

func person(id int64, access string) *entities.AnahitaNode {
	node := &entities.AnahitaNode{}
	node.ID = id
	node.Type = constants.NodeActorPersonEntityType
	node.Name = `Ana "the grower" <b>`
	node.Alias = "ana"
	node.Body = "I grow **beans** and [write about it](https://example.org)."
	node.Access = access
	node.Enabled = true
	node.Avatar = map[string]entities.AnahitaNodeImage{entities.AvatarSizeLarge: {Url: "avatars/ana_large.jpg"}}
	return node
}

func note(id int64, access string, owner *entities.AnahitaNode) *entities.AnahitaNode {
	node := &entities.AnahitaNode{}
	node.ID = id
	node.Type = "node.medium.text-service.note.v1"
	node.Body = "The heap is <script>alert(1)</script> warm today"
	node.Access = access
	node.Enabled = true
	node.Owner = owner
	node.Author = owner
	return node
}

func newServer(t *testing.T) (*Server, *fakeReader) {
	t.Helper()

	template, err := NewTemplate(index)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "static", "js"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"index.html":            index,
		"static/js/main.abc.js": "console.log(1)",
		"manifest.json":         "{}",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	secret := filepath.Join(filepath.Dir(dir), "secret.txt")
	_ = os.WriteFile(secret, []byte("not the app's"), 0o644)
	t.Cleanup(func() { _ = os.Remove(secret) })

	ana := person(1, constants.AnahitaAccessPublic)
	shy := person(2, "followers")
	shy.Alias = "shy"

	photo := note(13, constants.AnahitaAccessPublic, ana)
	photo.Type = constants.NodeMediumPhotoEntityType
	photo.Name = "Sunset"
	photo.Portrait = map[string]entities.AnahitaNodeImage{entities.PortraitSizeLarge: {Url: "photos/13_large.jpg"}}

	reply := note(15, constants.AnahitaAccessPublic, ana)
	reply.RootID = 11

	gone := note(16, constants.AnahitaAccessPublic, ana)
	now := time.Now()
	gone.DeletedAt = &now

	reader := &fakeReader{
		people: map[string]*entities.AnahitaNode{"ana": ana, "shy": shy},
		actors: map[int64]*entities.AnahitaNode{1: ana, 2: shy},
		media: map[int64]*entities.AnahitaNode{
			10: note(10, constants.AnahitaAccessPublic, ana),
			11: note(11, "followers", ana),
			12: note(12, constants.AnahitaAccessPublic, shy),
			13: photo,
			15: reply,
			16: gone,
		},
	}

	return &Server{
		Template:  template,
		Reader:    reader,
		Signer:    fakeSigner{},
		Dir:       dir,
		PublicURL: "https://www.example.org",
	}, reader
}

func get(s *Server, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestAPageSaysWhatItIsAbout(t *testing.T) {
	s, _ := newServer(t)

	page := get(s, "/people/ana").Body.String()
	for _, want := range []string{
		`<title>Ana &#34;the grower&#34; &lt;b&gt; - The Allotment</title>`,
		`<meta property="og:title" content="Ana &#34;the grower&#34; &lt;b&gt;"/>`,
		`<meta property="og:description" content="I grow beans and write about it."/>`,
		`<meta property="og:type" content="profile"/>`,
		`<meta property="og:url" content="https://www.example.org/people/ana"/>`,
		`<meta property="og:image" content="https://www.example.org/_shell/image/person-1"/>`,
		`<meta property="og:site_name" content="The Allotment"/>`,
		`<meta name="twitter:card" content="summary"/>`,
		`<link rel="canonical" href="https://www.example.org/people/ana"/>`,
		`<script defer="defer" src="/static/js/main.abc.js"></script>`,
		`<div id="root"></div>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("missing %s in\n%s", want, page)
		}
	}
	// The build's own tags are gone: each is said once.
	for _, once := range []string{"<title>", `property="og:title"`, `name="description"`, `property="og:image"`} {
		if strings.Count(page, once) != 1 {
			t.Errorf("%s is there %d times", once, strings.Count(page, once))
		}
	}

	// A person's other tabs are about the same person.
	if page := get(s, "/people/ana/followers").Body.String(); !strings.Contains(page, `content="profile"`) {
		t.Errorf("a tab of a profile is not described as the profile")
	}

	photo := get(s, "/photos/13-sunset").Body.String()
	for _, want := range []string{
		`<title>Sunset - The Allotment</title>`,
		`<meta property="og:type" content="article"/>`,
		`<meta property="og:image" content="https://www.example.org/_shell/image/post-13"/>`,
		`<meta name="twitter:card" content="summary_large_image"/>`,
	} {
		if !strings.Contains(photo, want) {
			t.Errorf("missing %s in\n%s", want, photo)
		}
	}

	// Somebody's words are written as text, never as markup.
	note := get(s, "/notes/10").Body.String()
	if strings.Contains(note, "<script>alert") || !strings.Contains(note, `content="The heap is alert(1) warm today"`) {
		t.Errorf("a note's text was written as it came:\n%s", note)
	}
}

func TestWhatIsNotPublicIsNotDescribed(t *testing.T) {
	s, _ := newServer(t)

	for name, target := range map[string]string{
		"a profile for followers":              "/people/shy",
		"a note for followers":                 "/notes/11",
		"a public note on a private profile":   "/notes/12",
		"a reply under a note for followers":   "/notes/15",
		"a deleted note":                       "/notes/16",
		"a note that is not there":             "/notes/99",
		"a photo read as a note":               "/notes/13",
		"somebody's settings":                  "/people/ana/settings",
		"somebody's notifications":             "/people/ana/notifications",
		"the page that adds a person":          "/people/add",
		"home":                                 "/",
		"a page of the app with nothing in it": "/search?term=beans",
	} {
		page := get(s, target).Body.String()
		for _, want := range []string{
			`<title>The Allotment</title>`,
			`<meta property="og:description" content="Growers &amp; friends"/>`,
			`<meta property="og:image" content="https://www.example.org/statics/media/ogimage.jpg"/>`,
			`<meta property="og:type" content="website"/>`,
		} {
			if !strings.Contains(page, want) {
				t.Errorf("%s: missing %s in\n%s", name, want, page)
			}
		}
		if strings.Contains(page, "Ana") || strings.Contains(page, "heap") || strings.Contains(page, "_shell/image") {
			t.Errorf("%s is described:\n%s", name, page)
		}
	}

	// Nor is its picture any easier to reach.
	for _, name := range []string{"person-2", "post-11", "post-12", "post-15", "post-16", "post-99", "post-10", "person-1x", "person-1/x", ""} {
		if rec := get(s, ImagePath+name); rec.Code != http.StatusNotFound {
			t.Errorf("the picture %q answered %d, to %q", name, rec.Code, rec.Header().Get("Location"))
		}
	}
	if rec := get(s, ImagePath+"person-1"); rec.Code != http.StatusFound || rec.Header().Get("Location") != "https://storage.example/avatars/ana_large.jpg?signed" {
		t.Errorf("a public profile's picture answered %d, to %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestASiteForMembersDescribesNothing(t *testing.T) {
	s, reader := newServer(t)
	defer policy.SetReadAccessForTest(policy.ReadRegistered)()

	page := get(s, "/people/ana").Body.String()
	if strings.Contains(page, "Ana") || !strings.Contains(page, `<title>The Allotment</title>`) {
		t.Errorf("a members' site described a profile:\n%s", page)
	}
	if rec := get(s, ImagePath+"person-1"); rec.Code != http.StatusNotFound {
		t.Errorf("a members' site sent a picture: %d", rec.Code)
	}
	if reader.reads != 0 {
		t.Errorf("%d reads were made", reader.reads)
	}
}

func TestAPageIsReadOnceAMinute(t *testing.T) {
	s, reader := newServer(t)
	now := time.Now()
	s.now = func() time.Time { return now }

	get(s, "/people/ana")
	get(s, "/people/ana")
	if reader.reads != 1 {
		t.Fatalf("read %d times for two askings", reader.reads)
	}

	// Made private since: said so within the minute.
	reader.people["ana"].Access = "followers"
	now = now.Add(keptFor + time.Second)
	if page := get(s, "/people/ana").Body.String(); strings.Contains(page, "Ana") {
		t.Errorf("still described after it was made private")
	}
}

func TestTheAppsFilesAreSentAsTheyAre(t *testing.T) {
	s, _ := newServer(t)

	script := get(s, "/static/js/main.abc.js")
	if script.Body.String() != "console.log(1)" || !strings.Contains(script.Header().Get("Cache-Control"), "immutable") {
		t.Errorf("a script answered %q, kept %q", script.Body.String(), script.Header().Get("Cache-Control"))
	}
	if manifest := get(s, "/manifest.json"); manifest.Body.String() != "{}" || strings.Contains(manifest.Header().Get("Cache-Control"), "immutable") {
		t.Errorf("the manifest answered %q, kept %q", manifest.Body.String(), manifest.Header().Get("Cache-Control"))
	}
	// A script that is not there is not the app.
	if missing := get(s, "/static/js/old.js"); missing.Code != http.StatusNotFound {
		t.Errorf("a missing script answered %d", missing.Code)
	}
	// index.html is never sent bare, and nothing outside the build is sent.
	for _, target := range []string{"/index.html", "/../secret.txt", "/static/../../secret.txt", "/%2e%2e/secret.txt"} {
		rec := get(s, target)
		if strings.Contains(rec.Body.String(), "not the app's") {
			t.Errorf("%s sent a file from outside the build", target)
		}
		if target == "/index.html" && !strings.Contains(rec.Body.String(), `property="og:url"`) {
			t.Errorf("index.html was sent without tags")
		}
	}

	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/people/ana", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("a POST answered %d", rec.Code)
	}
}

func TestTextIsMadePlainAndCut(t *testing.T) {
	if got := Plain("# Hello\n\n> **bold** and `code` and ![alt](x.png) [link](https://e.org) #beans", 200); got != "Hello bold and code and alt link #beans" {
		t.Errorf("got %q", got)
	}
	long := Plain(strings.Repeat("é", 300), 200)
	if len([]rune(long)) != 200 || !strings.HasSuffix(long, "…") {
		t.Errorf("cut to %d runes: %q", len([]rune(long)), long[len(long)-6:])
	}
	if _, err := NewTemplate("<html><body></body></html>"); err == nil {
		t.Errorf("a page with no head was taken")
	}
}

// The web app's host is where other servers come looking; what they look
// for is the API's to answer.
func TestWhatOtherServersLookForIsSentOnToTheAPI(t *testing.T) {
	s, _ := newServer(t)
	s.APIURL = "https://api.example.org"

	found := get(s, "/.well-known/webfinger?resource=acct:ana@example.org")
	if found.Code != http.StatusFound || found.Header().Get("Location") != "https://api.example.org/.well-known/webfinger?resource=acct:ana@example.org" {
		t.Errorf("webfinger: %d to %q", found.Code, found.Header().Get("Location"))
	}
	if found.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("a page on another site could not ask")
	}
	for _, path := range []string{"/.well-known/host-meta", "/.well-known/nodeinfo"} {
		if got := get(s, path); got.Code != http.StatusFound || got.Header().Get("Location") != "https://api.example.org"+path {
			t.Errorf("%s: %d to %q", path, got.Code, got.Header().Get("Location"))
		}
	}

	// Nothing else under that name is sent anywhere, and it is not a page.
	if other := get(s, "/.well-known/webfinger/../../people/ana"); other.Header().Get("Location") != "" {
		t.Errorf("sent on to %q", other.Header().Get("Location"))
	}

	// With no API named, there is nothing to send anybody on to.
	s.APIURL = ""
	if got := get(s, "/.well-known/webfinger?resource=acct:ana@example.org"); got.Code != http.StatusNotFound {
		t.Errorf("with no API: %d", got.Code)
	}
}
