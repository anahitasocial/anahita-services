package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"

	"activitypub/ap"
	"activitypub/data/repositories"
	"activitypub/fetch"
	"activitypub/remote"

	"anahita-libs/constants"
	"anahita-libs/data/entities"

	"github.com/labstack/echo/v4"
)

// Quotes between servers (roadmap row 156), as Mastodon makes them
// (FEP-044f). A quote here names the post it quotes. For a post here, the
// stamp that says its author let it is this installation's to give, and is
// made from the two posts each time it is asked for. For a post on another
// server, its author is asked with a QuoteRequest, and their stamp kept
// when they accept. A post elsewhere that quotes one here is answered by
// its author's quote policy, and given a stamp that is kept. A quote that
// comes in is kept as one when its stamp checks out, or it quotes its own
// author.

// quoteOf is how a post that quotes names what it quotes between servers:
// the quoted post's address there, the stamp that approves it, and the
// page a server that shows no quotes leads to. Nothing when the quoted post
// has no address between servers (one on Bluesky alone, one here that does
// not go out), or its author refused the quote: then the post goes with a
// line that leads to it (federation.QuotedAddress), if anything.
func (h *Handlers) quoteOf(note *entities.AnahitaNode) (quoted, stamp, page string) {
	if note == nil || note.QuoteID <= 0 || note.QuoteDetached {
		return "", "", ""
	}
	post, err := h.Nodes.Read(note.QuoteID)
	if err != nil || post == nil || post.ID == 0 || post.DeletedAt != nil {
		return "", "", ""
	}
	page = h.quotedAddress(note)
	if page == "" {
		return "", "", ""
	}

	// One here: on the fediverse, and so with an address there.
	if post.URI == "" {
		if author, _ := h.goesOut(post); author == nil {
			return "", "", ""
		}
		return h.addresses.Note(post.UUID), h.addresses.QuoteStamp(note.UUID), page
	}

	// One on another server of the fediverse: as its author answered.
	if !strings.HasPrefix(post.URI, "https://") || h.Quotes == nil {
		return "", "", ""
	}
	request, err := h.Quotes.RequestOf(note.ID)
	if err != nil {
		return "", "", ""
	}
	if request == nil || request.State == repositories.QuotePending {
		return post.URI, "", page
	}
	if request.State == repositories.QuoteRejected {
		return "", "", ""
	}
	return post.URI, request.StampURI, page
}

// withQuote gives a post going out what it says of quotes: the post it
// quotes, by its address between servers or by a line that leads to it;
// and who may quote it, by its quote policy or its author's.
func (h *Handlers) withQuote(sent *ap.Note, note, author *entities.AnahitaNode) {
	if quoted, stamp, page := h.quoteOf(note); quoted != "" {
		sent.QuotesPost(quoted, stamp, page)
	} else if address := h.quotedAddress(note); address != "" {
		sent.EndWithQuote(address)
	}
	policy := constants.EffectiveQuotePolicy(note.QuotePolicy, author.QuotePolicy)
	reply := note.ParentID > 0 || note.RootID > 0
	sent.WithQuotePolicy(policy, h.addresses.Followers(author.UUID), reply)
}

// askToQuote asks the author of a post on another server to let a note
// here quote it, once for each note: their answer comes to the inbox.
func (h *Handlers) askToQuote(ctx context.Context, note, author *entities.AnahitaNode, sent *ap.Note) error {
	if h.Quotes == nil || note.QuoteID <= 0 || note.QuoteDetached {
		return nil
	}
	quoted, err := h.Nodes.Read(note.QuoteID)
	if err != nil || quoted == nil || !strings.HasPrefix(quoted.URI, "https://") {
		return err
	}
	writer, err := h.Actors.ReadByNode(quoted.OwnerID)
	if err != nil || writer == nil || writer.Inbox == "" {
		return err
	}
	request := &repositories.QuoteRequest{
		NodeID:          note.ID,
		RequestID:       h.addresses.QuoteRequest(note.UUID),
		QuotedURI:       quoted.URI,
		QuotedAuthorURI: writer.URI,
	}
	begun, err := h.Quotes.BeginRequest(request)
	if err != nil || !begun {
		return err
	}
	asked := ap.NewQuoteRequest(request.RequestID, sent, quoted.URI)
	return h.send(ctx, author.ID, writer.Inbox, asked, "quote-request:"+note.UUID)
}

// isQuoteAnswer says whether an Accept or a Reject answers a QuoteRequest
// of ours.
func (h *Handlers) isQuoteAnswer(activity *ap.Activity) bool {
	if h.Quotes == nil {
		return false
	}
	if inner := activity.Inner(); inner != nil && inner.Type == "QuoteRequest" {
		return true
	}
	request, err := h.Quotes.RequestByID(activity.ObjectID())
	return err == nil && request != nil
}

// quoteAnswered keeps what the author of a quoted post answered, and sends
// the quote again with their stamp, or without the quote when they said
// no. Only the author of the quoted post is listened to.
func (h *Handlers) quoteAnswered(ctx context.Context, from *remote.Actor, activity *ap.Activity, accepted bool) error {
	request, err := h.Quotes.RequestByID(activity.ObjectID())
	if err != nil || request == nil {
		return err
	}
	if from.ID != request.QuotedAuthorURI {
		log.Printf("inbox: an answer to %s from %s, not from its author", request.RequestID, from.ID)
		return nil
	}
	stamp := ""
	if accepted {
		stamp = activity.ResultID()
		if !strings.HasPrefix(stamp, "https://") || !sameHost(stamp, from.ID) {
			log.Printf("inbox: an Accept of %s with no stamp of its own", request.RequestID)
			return nil
		}
	}
	if err := h.Quotes.Answer(request.NodeID, accepted, stamp); err != nil {
		return err
	}
	// Sent again, as changed now: with its stamp, or without its quote.
	return h.relocated(ctx, request.NodeID)
}

// quoteRequested answers somebody on another server who asks to quote a
// post here: by its author's quote policy, an Accept with a stamp that is
// kept, or a Reject. Somebody its author blocked, or on a server they
// blocked, is told no; so is a request for a reply, or for a post that
// does not go out.
func (h *Handlers) quoteRequested(ctx context.Context, from *remote.Actor, body []byte) error {
	request, err := ap.ReadQuoteRequest(body)
	if err != nil || request.Actor != from.ID || !sameHost(request.Instrument, from.ID) {
		return nil
	}
	uuid := h.addresses.NoteUUIDOf(request.Object)
	if uuid == "" {
		return nil
	}
	quoted, err := h.Nodes.ReadByUUID(uuid)
	if err != nil || quoted == nil || quoted.ID == 0 {
		return err
	}
	author, _ := h.goesOut(quoted)
	if author == nil {
		return nil
	}
	actor := h.addresses.Actor(author.UUID)

	quoter, err := h.known(ctx, from)
	if err != nil || quoter == nil {
		return err
	}
	accepted, err := h.mayQuote(author, quoted, quoter)
	if err != nil {
		return err
	}
	stamp := ""
	if accepted {
		given, err := h.Quotes.AddStamp(&repositories.QuoteStamp{
			UUID:           newUUID(),
			QuotedNodeID:   quoted.ID,
			AuthorID:       author.ID,
			QuoteURI:       request.Instrument,
			QuoteAuthorURI: from.ID,
		})
		if err != nil || given == nil {
			return err
		}
		stamp = h.addresses.QuoteStamp(given.UUID)
	}
	answer := ap.NewQuoteAnswer(h.addresses.Activity(newUUID()), actor, request, accepted, stamp)
	return h.send(ctx, author.ID, from.Inbox, answer, "quote-answer:"+request.ID)
}

// mayQuote says whether somebody on another server may quote a post here,
// by the post's quote policy or its author's: anybody, the author's
// followers, or nobody. A reply is not quoted here.
func (h *Handlers) mayQuote(author, quoted, quoter *entities.AnahitaNode) (bool, error) {
	if quoted.ParentID > 0 || quoted.RootID > 0 || quoted.Access != constants.AnahitaAccessPublic {
		return false, nil
	}
	relation, err := h.Relations.Read(author.ID, quoter.ID)
	if err != nil {
		return false, err
	}
	if relation != nil && (relation.IsBlocking || relation.IsServerBlocked) {
		return false, nil
	}
	switch constants.EffectiveQuotePolicy(quoted.QuotePolicy, author.QuotePolicy) {
	case constants.QuotePolicyAnyone:
		return true, nil
	case constants.QuotePolicyFollowers:
		// The relation is the author's to them: "leading" is their
		// following the author.
		return relation != nil && relation.IsLeading, nil
	}
	return false, nil
}

// QuoteStamp serves a stamp given here (FEP-044f): that the author of a
// post here let another post quote it. One given to a post elsewhere is
// kept; one for a post here quoting another here is made from the two
// posts, and is there only while the quote is.
func (h *Handlers) QuoteStamp(c echo.Context) error {
	uuid := c.Param("uuid")
	if !uuidShape.MatchString(uuid) || h.Quotes == nil {
		return c.NoContent(http.StatusNotFound)
	}
	if ok, _ := h.siteFederates(); !ok {
		return c.NoContent(http.StatusNotFound)
	}
	id := h.addresses.QuoteStamp(uuid)

	if given, err := h.Quotes.Stamp(uuid); err != nil {
		return c.NoContent(http.StatusServiceUnavailable)
	} else if given != nil {
		quoted, err := h.Nodes.Read(given.QuotedNodeID)
		if err != nil || quoted == nil || quoted.ID == 0 {
			return c.NoContent(http.StatusNotFound)
		}
		author, _ := h.goesOut(quoted)
		if author == nil || author.ID != given.AuthorID {
			return c.NoContent(http.StatusNotFound)
		}
		return activityJSON(c, ap.NewQuoteAuthorization(id, h.addresses.Actor(author.UUID),
			given.QuoteURI, h.addresses.Note(quoted.UUID)))
	}

	note, err := h.Nodes.ReadByUUID(uuid)
	if err != nil || note == nil || note.ID == 0 || note.QuoteID <= 0 || note.QuoteDetached {
		return c.NoContent(http.StatusNotFound)
	}
	if writer, _ := h.goesOut(note); writer == nil {
		return c.NoContent(http.StatusNotFound)
	}
	quoted, err := h.Nodes.Read(note.QuoteID)
	if err != nil || quoted == nil || quoted.ID == 0 || quoted.URI != "" {
		return c.NoContent(http.StatusNotFound)
	}
	author, _ := h.goesOut(quoted)
	if author == nil {
		return c.NoContent(http.StatusNotFound)
	}
	return activityJSON(c, ap.NewQuoteAuthorization(id, h.addresses.Actor(author.UUID),
		h.addresses.Note(note.UUID), h.addresses.Note(quoted.UUID)))
}

// quotedHere is the post a note from elsewhere quotes, as it is kept here,
// when the quote is one: it quotes its own author, or carries a stamp its
// quoted post's author gave, checked where it is kept. 0 for a quote that
// is not approved, which is kept as the words it is, with the line that
// leads to the quoted post.
func (h *Handlers) quotedHere(ctx context.Context, note *ap.ReceivedNote) int64 {
	address := note.Quoted()
	if address == "" {
		return 0
	}

	// A post here: approved when this installation gave it a stamp.
	if uuid := h.addresses.NoteUUIDOf(address); uuid != "" {
		quoted, err := h.Nodes.ReadByUUID(uuid)
		if err != nil || quoted == nil || quoted.ID == 0 || h.Quotes == nil {
			return 0
		}
		given, err := h.Quotes.StampFor(quoted.ID, note.ID)
		if err != nil || given == nil || given.QuoteAuthorURI != note.Author() {
			return 0
		}
		return quoted.ID
	}

	// A post elsewhere, kept here or read now.
	quoted, err := h.Nodes.ReadByURI(address)
	if err != nil {
		return 0
	}
	if quoted == nil || quoted.ID == 0 {
		// Read now, unless this note was itself read for a quote: a quote
		// of a quote of a quote is not followed down.
		if note.ForAQuote {
			return 0
		}
		if quoted, err = h.fetchNoteFor(ctx, address, true); err != nil || quoted == nil {
			return 0
		}
	}
	writer, err := h.Actors.ReadByNode(quoted.OwnerID)
	if err != nil || writer == nil {
		return 0
	}
	if writer.URI == note.Author() {
		return quoted.ID
	}
	if h.stampChecksOut(ctx, note.QuoteStamp(), note.ID, address, writer.URI) {
		return quoted.ID
	}
	return 0
}

// stampChecksOut reads a stamp from where it is kept, the server of the
// quoted post's author, and says whether it approves this quote.
func (h *Handlers) stampChecksOut(ctx context.Context, stamp, quote, quoted, author string) bool {
	if h.Fetcher == nil || !strings.HasPrefix(stamp, "https://") || !sameHost(stamp, author) {
		return false
	}
	var read ap.QuoteAuthorization
	if _, err := h.Fetcher.Object(ctx, stamp, &read); err != nil {
		if !errors.Is(err, context.Canceled) {
			log.Printf("a quote's stamp at %s could not be read: %v", stamp, err)
		}
		return false
	}
	return read.ID == stamp && read.ValidFor(quote, quoted, author)
}

// quoteLine is the line a server leads a reader to a quoted post with,
// which a quote that is kept as one has no need of.
var quoteLine = regexp.MustCompile(`(?is)<(p|span)[^>]*class="[^"]*\bquote-inline\b[^"]*"[^>]*>.*?</(p|span)>`)

// withoutQuoteLine takes the quoted post's line out of a quote's words.
func withoutQuoteLine(content string) string {
	return quoteLine.ReplaceAllString(content, "")
}

// keepsQuote reads the post a note from elsewhere quotes, for it to be kept
// as a quote, within the time a server is given to answer.
func (h *Handlers) keepsQuote(note *ap.ReceivedNote) int64 {
	ctx, cancel := context.WithTimeout(context.Background(), 2*fetch.Timeout)
	defer cancel()
	return h.quotedHere(ctx, note)
}
