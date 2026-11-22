package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/federation"
	"anahita-libs/permissions"
	"anahita-libs/posts"
	"atproto/bsky"
	"atproto/data/repositories"
	"context"
	"errors"
	"strings"
	"time"
)

// webPathOf is where a post is read in the web app (anahita-libs/posts).
func webPathOf(note *entities.AnahitaNode) string {
	return posts.Page(note.Type, note.ID)
}

// cutTitle is what the card of a post cut for Bluesky is called: its title
// where it has one, or its opening words.
func cutTitle(note *entities.AnahitaNode) string {
	if title := strings.TrimSpace(note.Name); title != "" {
		return title
	}
	// In plain words: a card's title shows no Markdown, and a link in it
	// is its words or its short address (the user, 2026-11-16).
	words := strings.Fields(bsky.PlainText(note.Body))
	opening := strings.Join(words, " ")
	if letters := []rune(opening); len(letters) > 80 {
		opening = strings.TrimSpace(string(letters[:79])) + "…"
	}
	return opening
}

// quotedAddress is where a post that quotes leads to the post it quotes, and
// that post; or nothing: see federation.QuotedAddress.
func (h *Handlers) quotedAddress(note *entities.AnahitaNode) (string, *entities.AnahitaNode) {
	if note == nil || note.QuoteID <= 0 {
		return "", nil
	}
	quoted, err := h.Nodes.Read(note.QuoteID)
	if err != nil || quoted == nil {
		return "", nil
	}
	public := permissions.CanReadByID(h.Nodes, quoted.ID, entities.NewViewer())
	return federation.QuotedAddress(quoted, public, h.Site.Web), quoted
}

// quoteStaysOff says whether a post that quotes stays off Bluesky
// (roadmap row 162): it quotes a post from the fediverse, or one here that
// is not on Bluesky, or is not open to everybody. Consent to be quoted on
// one network is not consent to be carried to another. Decided by where
// the quoted post is, even once it is taken out of the quote: a quote of a
// post from the fediverse whose author refused it does not come to Bluesky
// as its words alone. A post whose quoted post is gone goes as its words.
func (h *Handlers) quoteStaysOff(note *entities.AnahitaNode) bool {
	if note == nil || note.QuoteID <= 0 {
		return false
	}
	quoted, err := h.Nodes.Read(note.QuoteID)
	if err != nil || quoted == nil || quoted.ID == 0 || quoted.DeletedAt != nil {
		return false
	}
	if quoted.IsRemote() {
		return !strings.HasPrefix(quoted.URI, "at://")
	}
	if !permissions.CanReadByID(h.Nodes, quoted.ID, entities.NewViewer()) {
		return true
	}
	return h.onBluesky(quoted) == ""
}

// quotedWait is how long a quoted post written from here is waited for on
// Bluesky before it is taken to be gone from there.
const quotedWait = 10 * time.Minute

// quotedRef is the post a post quotes as it is on Bluesky, or nil when it
// is not there: one from Bluesky, or one of this installation's written
// there. Only a quoted post that may be led to at all (`address`). A post
// written there and not yet readable is tried again shortly
// (errNotWrittenYet); one gone from Bluesky is led to by a line instead.
func (h *Handlers) quotedRef(ctx context.Context, quoted *entities.AnahitaNode, address string) (*bsky.Ref, error) {
	if quoted == nil || address == "" {
		return nil, nil
	}
	uri := ""
	if strings.HasPrefix(quoted.URI, "at://") {
		uri = quoted.URI
	} else if quoted.URI == "" && h.Sent != nil {
		sent, err := h.Sent.Read(repositories.SentPost, quoted.ID, 0)
		if err != nil {
			return nil, err
		}
		if sent != nil {
			uri = bsky.PostURI(sent.DID, sent.RKey)
		}
	}
	if uri == "" {
		return nil, nil
	}
	ref, _, err := h.refOf(ctx, uri)
	if errors.Is(err, bsky.ErrNobody) {
		return nil, nil
	}
	// Written from here and not readable there: about to be, when it was
	// just made, and tried again; gone, when it is older, and led to.
	if errors.Is(err, errNotWrittenYet) && time.Since(quoted.CreatedAt) > quotedWait {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return ref, nil
}
