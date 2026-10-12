package linkpreviews

import (
	"strings"
	"testing"
	"time"
)

type memory struct {
	kept map[string]string
	ttl  map[string]time.Duration
}

func (m *memory) Get(key string) (string, bool, error) {
	value, ok := m.kept[key]
	return value, ok, nil
}

func (m *memory) Set(key, value string, ttl time.Duration) error {
	m.kept[key] = value
	m.ttl[key] = ttl
	return nil
}

func TestNormaliseTakesOnlyWebAddresses(t *testing.T) {
	good := map[string]string{
		" https://Example.org/a?b=1#top ": "https://example.org/a?b=1",
		"http://example.org":              "http://example.org",
	}
	for given, want := range good {
		if got, ok := Normalise(given); !ok || got != want {
			t.Errorf("%q: got %q %v, want %q", given, got, ok, want)
		}
	}

	for _, bad := range []string{
		"", "example.org", "ftp://example.org/x", "javascript:alert(1)", "file:///etc/passwd",
		"https://", "https://user:pass@example.org/", "http://" + strings.Repeat("a", MaxURLLength),
	} {
		if got, ok := Normalise(bad); ok {
			t.Errorf("%q was taken, as %q", bad, got)
		}
	}
}

func TestFirstURLFindsTheLinkAndNotWhatFollowsIt(t *testing.T) {
	cases := map[string]string{
		"Read this: https://example.org/post. It is good.":   "https://example.org/post",
		"(see https://example.org/a) and https://other.org/": "https://example.org/a",
		"two\nlines http://example.org/x?y=1, then more":     "http://example.org/x?y=1",
		"no link here":               "",
		"mailto:someone@example.org": "",
	}
	for text, want := range cases {
		if got := FirstURL(text); got != want {
			t.Errorf("%q: got %q, want %q", text, got, want)
		}
	}
}

func TestStoreKeepsAGoodPreviewLongerThanAFailure(t *testing.T) {
	cache := &memory{kept: map[string]string{}, ttl: map[string]time.Duration{}}
	store := NewStore(cache)

	good := &Preview{URL: "https://example.org/a", Title: "A", Status: StatusOK}
	bad := &Preview{URL: "https://example.org/b", Status: StatusFailed}
	if err := store.Put(good); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(bad); err != nil {
		t.Fatal(err)
	}

	if got, ok := store.Get(good.URL); !ok || got.Title != "A" || !got.Usable() {
		t.Errorf("read back %+v %v", got, ok)
	}
	if got, ok := store.Get(bad.URL); !ok || got.Usable() {
		t.Errorf("a failure read back as %+v %v", got, ok)
	}
	if _, ok := store.Get("https://example.org/never"); ok {
		t.Errorf("a preview nobody made was found")
	}
	if cache.ttl[Key(good.URL)] != FreshFor || cache.ttl[Key(bad.URL)] != FailedFor {
		t.Errorf("kept for %v and %v", cache.ttl[Key(good.URL)], cache.ttl[Key(bad.URL)])
	}

	// With nowhere to keep them, nothing is kept and nothing breaks.
	var none *Store
	if _, ok := none.Get(good.URL); ok || none.Put(good) != nil {
		t.Errorf("a store with no cache answered")
	}
}

func TestASnapshotIsKeptInANotesMetaBesideWhatIsThere(t *testing.T) {
	preview := &Preview{
		URL:         "https://example.org/a",
		Title:       "  A title\nover two lines  ",
		Description: strings.Repeat("word ", 200),
		ImageKey:    "abc.jpg",
		Status:      StatusOK,
	}

	snapshot := NewSnapshot(preview)
	if snapshot.Title != "A title over two lines" || len([]rune(snapshot.Description)) != MaxText || !strings.HasSuffix(snapshot.Description, "…") {
		t.Fatalf("snapshot %+v", snapshot)
	}
	if NewSnapshot(&Preview{URL: "https://example.org", Status: StatusFailed}) != nil ||
		NewSnapshot(&Preview{URL: "https://example.org", Status: StatusOK}) != nil ||
		NewSnapshot(nil) != nil {
		t.Errorf("a snapshot was taken of nothing")
	}

	meta := IntoMeta([]byte(`{"other":"kept"}`), snapshot)
	if !strings.Contains(string(meta), `"other":"kept"`) {
		t.Errorf("what was in the meta was lost: %s", meta)
	}
	if back := FromMeta(meta); back == nil || back.URL != preview.URL || back.ImageKey != "abc.jpg" {
		t.Errorf("read back %+v", back)
	}

	// Taken out again, and the rest left.
	cleared := IntoMeta(meta, nil)
	if FromMeta(cleared) != nil || !strings.Contains(string(cleared), `"other":"kept"`) {
		t.Errorf("after taking it out: %s", cleared)
	}

	// Meta with none, with nothing, and that is not meta.
	for _, raw := range []string{``, `{}`, `not json`, `{"link_preview":{"url":""}}`, `{"link_preview":"x"}`} {
		if got := FromMeta([]byte(raw)); got != nil {
			t.Errorf("%q read as %+v", raw, got)
		}
	}
	if got := FromMeta(IntoMeta([]byte(`not json`), snapshot)); got == nil {
		t.Errorf("a snapshot could not be put into meta that was unreadable")
	}
}
