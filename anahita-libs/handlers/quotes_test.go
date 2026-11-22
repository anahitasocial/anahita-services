package handlers

import (
	"errors"
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/responses"
)

// A reader that hands over any post it has, to anybody, as the real one
// does, and counts how often it is asked.
type fakeQuoteReader struct {
	posts map[int64]*entities.AnahitaNode
	reads []int64
}

func (f *fakeQuoteReader) Read(id int64, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	f.reads = append(f.reads, id)
	if post, ok := f.posts[id]; ok {
		return post, nil
	}
	return nil, errors.New("not found")
}

// The gate: which ids this viewer may see.
type fakeVisibility struct {
	visible map[int64]bool
	err     error
	asked   int
}

func (f *fakeVisibility) FilterVisible(ids []int64, viewer *entities.Viewer) ([]int64, error) {
	f.asked++
	if f.err != nil {
		return nil, f.err
	}
	out := []int64{}
	for _, id := range ids {
		if f.visible[id] {
			out = append(out, id)
		}
	}
	return out, nil
}

func quotingNote(id int64, quotes int64) *entities.AnahitaNode {
	note := entities.NewAnahitaNode()
	note.ID = id
	note.QuoteID = quotes
	return note
}

func quotedPost(id int64) *entities.AnahitaNode {
	post := entities.NewAnahitaNode()
	post.ID = id
	post.Type = constants.NodeMediumNoteEntityType
	post.Body = "quoted"
	return post
}

// What a note carries about the post it quotes is decided for each reader:
// the post when they may see it, and otherwise only that there was one.
//
// THE CASE THIS EXISTS FOR is post 11. The reader would hand it over, and
// the viewer may not see it: it was made private after it was quoted, or
// its author blocked them. It must not be read, and must not be sent.
func TestANoteShowsWhatItQuotesOnlyToThoseWhoMaySeeIt(t *testing.T) {
	reader := &fakeQuoteReader{posts: map[int64]*entities.AnahitaNode{
		10: quotedPost(10),
		11: quotedPost(11),
	}}
	gate := &fakeVisibility{visible: map[int64]bool{10: true, 12: true}}

	shown := quotingNote(1, 10)
	again := quotingNote(2, 10)
	notTheirs := quotingNote(3, 11)
	alsoNotTheirs := quotingNote(4, 11)
	detached := quotingNote(5, 11)
	detached.QuoteDetached = true
	// Visible as far as the gate knows, and gone when read.
	gone := quotingNote(6, 12)
	plain := quotingNote(7, 0)

	attached := SetQuotes(reader, gate, []*entities.AnahitaNode{shown, again, notTheirs, alsoNotTheirs, detached, gone, plain, nil}, entities.NewViewer())

	if len(attached) != 1 || attached[0].ID != 10 {
		t.Errorf("attached %d posts, want the one that can be seen", len(attached))
	}
	// The gate is asked once for the whole page.
	if gate.asked != 1 {
		t.Errorf("the gate was asked %d times, want once", gate.asked)
	}
	for _, id := range reader.reads {
		if id == 11 {
			t.Fatal("a post the viewer may not see was read")
		}
	}

	for _, c := range []struct {
		name  string
		node  *entities.AnahitaNode
		state string
		post  bool
	}{
		{"one the viewer may see", shown, "", true},
		{"the same post, quoted again", again, "", true},
		{"one the viewer may not see", notTheirs, "unavailable", false},
		{"the same again", alsoNotTheirs, "unavailable", false},
		{"one its author took out", detached, "detached", false},
		{"one that is gone", gone, "unavailable", false},
	} {
		embed := responses.NewQuoteEmbed(c.node)
		if embed == nil {
			t.Fatalf("%s: nothing sent about the quote", c.name)
		}
		if embed.State != c.state || (embed.Post != nil) != c.post {
			t.Errorf("%s: state %q, post sent %v; want %q, %v", c.name, embed.State, embed.Post != nil, c.state, c.post)
		}
		if c.node.Quote != nil && !c.post {
			t.Errorf("%s: the post was attached to the note", c.name)
		}
	}

	if responses.NewQuoteEmbed(plain) != nil {
		t.Error("a note that quotes nothing was sent a quote")
	}
}

// When the gate cannot be asked, nothing is shown and nothing is read.
func TestNothingIsQuotedWhenTheGateCannotBeAsked(t *testing.T) {
	reader := &fakeQuoteReader{posts: map[int64]*entities.AnahitaNode{10: quotedPost(10)}}
	gate := &fakeVisibility{err: errors.New("unreachable")}
	note := quotingNote(1, 10)

	if attached := SetQuotes(reader, gate, []*entities.AnahitaNode{note}, entities.NewViewer()); len(attached) != 0 {
		t.Errorf("attached %d posts", len(attached))
	}
	if len(reader.reads) != 0 || note.Quote != nil || note.QuoteState != "unavailable" {
		t.Errorf("reads %v, quote %v, state %q", reader.reads, note.Quote, note.QuoteState)
	}
}

// A note whose handler never looked the quoted post up sends nothing about
// it: not the post, and not a guess that it cannot be shown.
func TestAQuoteNobodyLookedUpIsNotSent(t *testing.T) {
	if embed := responses.NewQuoteEmbed(quotingNote(1, 10)); embed != nil {
		t.Errorf("sent %+v", embed)
	}
}

// fakeBatchQuoteReader reads several at once, as services.MediaGRPC does,
// and counts how it was asked.
type fakeBatchQuoteReader struct {
	fakeQuoteReader
	batches [][]int64
	reads   int
}

func (f *fakeBatchQuoteReader) Read(id int64, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	f.reads++
	return f.fakeQuoteReader.Read(id, viewer)
}

func (f *fakeBatchQuoteReader) BatchRead(ids []int64, _ *entities.Viewer) ([]*entities.AnahitaNode, error) {
	f.batches = append(f.batches, ids)
	found := []*entities.AnahitaNode{}
	for _, id := range ids {
		if post, ok := f.posts[id]; ok {
			found = append(found, post)
		}
	}
	return found, nil
}

// THE RULE THIS GUARDS (roadmap row 170): a feed's quoted posts are read in
// one call, not one call each, and only those the viewer may see are asked
// for; a reply, or something that is not a post, is not shown as one.
func TestAFeedsQuotedPostsAreReadInOneCall(t *testing.T) {
	reply := quotedPost(12)
	reply.RootID = 9
	hashtag := quotedPost(13)
	hashtag.Type = constants.NodeBaseHashtagEntityType
	reader := &fakeBatchQuoteReader{fakeQuoteReader: fakeQuoteReader{posts: map[int64]*entities.AnahitaNode{
		10: quotedPost(10), 11: quotedPost(11), 12: reply, 13: hashtag, 14: quotedPost(14),
	}}}
	gate := &fakeVisibility{visible: map[int64]bool{10: true, 12: true, 13: true, 14: true}}

	notes := []*entities.AnahitaNode{
		quotingNote(1, 10), quotingNote(2, 11), quotingNote(3, 12), quotingNote(4, 13), quotingNote(5, 14), quotingNote(6, 10),
	}
	attached := SetQuotes(reader, gate, notes, entities.NewViewer())

	if len(reader.batches) != 1 || reader.reads != 0 {
		t.Fatalf("%d batch reads and %d single reads, want one batch and none", len(reader.batches), reader.reads)
	}
	if asked := reader.batches[0]; len(asked) != 4 {
		t.Errorf("asked for %v, want only the four the viewer may see, each once", asked)
	}
	if len(attached) != 2 || notes[0].Quote == nil || notes[4].Quote == nil || notes[5].Quote == nil {
		t.Errorf("attached %d, want posts 10 and 14, on the three notes quoting them", len(attached))
	}
	for _, i := range []int{1, 2, 3} {
		if notes[i].Quote != nil || notes[i].QuoteState != constants.QuoteStateUnavailable {
			t.Errorf("note %d: shown %v, state %q; want nothing shown, unavailable", notes[i].ID, notes[i].Quote != nil, notes[i].QuoteState)
		}
	}
}
