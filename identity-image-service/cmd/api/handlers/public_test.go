package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"anahita-libs/data/entities"

	"github.com/labstack/echo/v4"
)

const adaUUID = "0962789f-aeca-4850-ac9b-41129e3b6f46"

func askPublic(h *imageHandler, k string, uuid, size string) *httptest.ResponseRecorder {
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)
	c.SetParamNames("uuid", "size")
	c.SetParamValues(uuid, size)

	if k == "cover" {
		_ = h.PublicCover(c)
	} else {
		_ = h.PublicAvatar(c)
	}
	return rec
}

func publicWorld() *world {
	w := newWorld(personType)
	w.node.UUID = adaUUID
	w.node.AvatarFilename = "abc123.jpg"
	w.node.CoverFilename = "def456.jpg"
	return w
}

// THE RULE THIS GUARDS: the address that never changes sends anybody on to
// a public profile's picture, and says nothing at all about anything else.
func TestAPublicProfilesPictureHasAnAddressThatStays(t *testing.T) {
	w := publicWorld()
	h := handlerFor(w)

	rec := askPublic(h, "avatar", adaUUID, entities.AvatarSizeLarge)
	if rec.Code != http.StatusFound {
		t.Fatalf("avatar: %d", rec.Code)
	}
	to := rec.Header().Get("Location")
	if !strings.HasPrefix(to, "https://media.example/assets/v1/identity-image-service/avatars/") || !strings.HasSuffix(to, "/abc123_large.jpg") {
		t.Errorf("sent on to %q", to)
	}
	if got := rec.Header().Get("Cache-Control"); got != publicImageCache {
		t.Errorf("cache %q", got)
	}

	rec = askPublic(h, "cover", adaUUID, entities.CoverSizeLarge)
	if rec.Code != http.StatusFound || !strings.Contains(rec.Header().Get("Location"), "/covers/") {
		t.Errorf("cover: %d to %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestEverythingElseIsNotFoundAndSaysNoMore(t *testing.T) {
	cases := map[string]struct {
		change     func(w *world)
		uuid, size string
	}{
		"nobody's uuid":            {func(w *world) {}, "11111111-1111-4111-8111-111111111111", "large"},
		"no uuid at all":           {func(w *world) {}, "1", "large"},
		"a size there is not":      {func(w *world) {}, adaUUID, "enormous"},
		"a path for a size":        {func(w *world) {}, adaUUID, "../../secret"},
		"no avatar":                {func(w *world) { w.node.AvatarFilename = "" }, adaUUID, "large"},
		"a profile for members":    {func(w *world) { w.node.Access = "registered" }, adaUUID, "large"},
		"a profile for followers":  {func(w *world) { w.node.Access = "followers" }, adaUUID, "large"},
		"one the graph hides":      {func(w *world) { w.hidden = true }, adaUUID, "large"},
		"somebody on another site": {func(w *world) { w.node.URI = "https://mastodon.example/users/ada" }, adaUUID, "large"},
	}

	for name, c := range cases {
		w := publicWorld()
		c.change(w)
		rec := askPublic(handlerFor(w), "avatar", c.uuid, c.size)
		if rec.Code != http.StatusNotFound || rec.Header().Get("Location") != "" || rec.Body.Len() != 0 {
			t.Errorf("%s: %d, to %q, saying %q", name, rec.Code, rec.Header().Get("Location"), rec.Body.String())
		}
	}

	// A post has no avatar, whatever its uuid.
	w := newWorld(noteType)
	w.node.UUID = adaUUID
	w.node.AvatarFilename = "abc123.jpg"
	if rec := askPublic(handlerFor(w), "avatar", adaUUID, "large"); rec.Code != http.StatusNotFound {
		t.Errorf("a post's avatar: %d", rec.Code)
	}
}
