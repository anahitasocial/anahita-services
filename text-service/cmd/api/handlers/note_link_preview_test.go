package handlers

import (
	"testing"
	"time"

	"anahita-libs/linkpreviews"
)

type keptPreviews map[string]string

func (k keptPreviews) Get(key string) (string, bool, error) {
	value, ok := k[key]
	return value, ok, nil
}

func (k keptPreviews) Set(key, value string, ttl time.Duration) error {
	k[key] = value
	return nil
}

func TestTheCardANoteCarries(t *testing.T) {
	store := linkpreviews.NewStore(keptPreviews{})
	if err := store.Put(&linkpreviews.Preview{URL: "https://example.org/heap", Title: "Compost", Status: linkpreviews.StatusOK}); err != nil {
		t.Fatal(err)
	}
	h := &noteHandler{Env: &Env{Previews: store}}
	titleOf := func(meta []byte) string {
		if card := linkpreviews.FromMeta(meta); card != nil {
			return card.Title
		}
		return ""
	}

	// A new note: the card of a link that is in it and was read.
	with := h.withLinkPreview(nil, "https://Example.org/heap", "Read https://Example.org/heap today", false)
	if titleOf(with) != "Compost" {
		t.Fatalf("no card on a note with a link that was read: %s", with)
	}
	for name, got := range map[string][]byte{
		"no address sent":        h.withLinkPreview(nil, "", "Read https://example.org/heap", false),
		"a link not in it":       h.withLinkPreview(nil, "https://example.org/heap", "Nothing here", false),
		"a link never read":      h.withLinkPreview(nil, "https://example.org/other", "Read https://example.org/other", false),
		"what is not an address": h.withLinkPreview(nil, "javascript:alert(1)", "javascript:alert(1)", false),
	} {
		if titleOf(got) != "" || got != nil {
			t.Errorf("%s: a card, or meta, where there was none: %s", name, got)
		}
	}

	// An edited note keeps its card while the link is in it, whatever is
	// sent, and loses it with the link.
	if kept := h.withLinkPreview(with, "", "Do read https://Example.org/heap today", false); titleOf(kept) != "Compost" {
		t.Errorf("an edit took the card off: %s", kept)
	}
	if gone := h.withLinkPreview(with, "", "Never mind", false); titleOf(gone) != "" {
		t.Errorf("the card outlived its link: %s", gone)
	}
	// Taken off by its writer, the link still there.
	if gone := h.withLinkPreview(with, "https://Example.org/heap", "Read https://Example.org/heap", true); titleOf(gone) != "" {
		t.Errorf("a card its writer took off is still there: %s", gone)
	}
	// The same link sent again: the card it has, not the page as it is now.
	_ = store.Put(&linkpreviews.Preview{URL: "https://example.org/heap", Title: "Compost, revised", Status: linkpreviews.StatusOK})
	if same := h.withLinkPreview(with, "https://Example.org/heap", "Read https://Example.org/heap again", false); titleOf(same) != "Compost" {
		t.Errorf("an edit read the page again: %s", same)
	}
	if gone := h.withLinkPreview(with, "https://example.org/heap", "Never mind", false); titleOf(gone) != "" {
		t.Errorf("a card for a link that is not in the note: %s", gone)
	}
}
