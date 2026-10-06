package handlers

import (
	"errors"
	"testing"

	"anahita-libs/data/entities"
	"anahita-libs/responses"
)

// A reader that has one post the viewer may see, and counts how often it is
// asked.
type fakeQuoteReader struct {
	visible map[int64]*entities.AnahitaNode
	reads   int
}

func (f *fakeQuoteReader) Read(id int64, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	f.reads++
	if post, ok := f.visible[id]; ok {
		return post, nil
	}
	return nil, errors.New("not found")
}

func quotingNote(id int64, quotes int64) *entities.AnahitaNode {
	note := entities.NewAnahitaNode()
	note.ID = id
	note.QuoteID = quotes
	return note
}

// What a note carries about the post it quotes is decided for each reader:
// the post when they may see it, and otherwise only that there was one.
func TestANoteShowsWhatItQuotesOnlyToThoseWhoMaySeeIt(t *testing.T) {
	seen := entities.NewAnahitaNode()
	seen.ID = 10
	seen.Body = "the quoted post"

	reader := &fakeQuoteReader{visible: map[int64]*entities.AnahitaNode{10: seen}}

	shown := quotingNote(1, 10)
	again := quotingNote(2, 10)
	hiddenFromViewer := quotingNote(3, 11)
	alsoHidden := quotingNote(4, 11)
	detached := quotingNote(5, 10)
	detached.QuoteDetached = true
	plain := quotingNote(6, 0)

	attached := SetQuotes(reader, []*entities.AnahitaNode{shown, again, hiddenFromViewer, alsoHidden, detached, plain, nil}, entities.NewViewer())

	if len(attached) != 1 || attached[0] != seen {
		t.Errorf("attached %d posts, want the one that can be seen", len(attached))
	}
	// Each quoted post is asked about once, whether it is there or not.
	if reader.reads != 2 {
		t.Errorf("read %d times, want 2", reader.reads)
	}

	for _, c := range []struct {
		name  string
		node  *entities.AnahitaNode
		state string
		post  bool
	}{
		{"one the viewer may see", shown, "", true},
		{"the same post, quoted again", again, "", true},
		{"one the viewer may not see", hiddenFromViewer, "unavailable", false},
		{"the same again", alsoHidden, "unavailable", false},
		{"one its author took out", detached, "detached", false},
	} {
		embed := responses.NewQuoteEmbed(c.node)
		if embed == nil {
			t.Fatalf("%s: nothing sent about the quote", c.name)
		}
		if embed.State != c.state || (embed.Post != nil) != c.post {
			t.Errorf("%s: state %q, post sent %v; want %q, %v", c.name, embed.State, embed.Post != nil, c.state, c.post)
		}
	}

	if responses.NewQuoteEmbed(plain) != nil {
		t.Error("a note that quotes nothing was sent a quote")
	}
}

// A note whose handler never looked the quoted post up sends nothing about
// it: not the post, and not a guess that it cannot be shown.
func TestAQuoteNobodyLookedUpIsNotSent(t *testing.T) {
	if embed := responses.NewQuoteEmbed(quotingNote(1, 10)); embed != nil {
		t.Errorf("sent %+v", embed)
	}
}
