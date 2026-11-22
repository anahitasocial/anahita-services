package handlers

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/federation"
	"anahita-libs/linkpreviews"
	"anahita-libs/permissions"
	"anahita-libs/videos"
	"atproto/bsky"
	"atproto/data/repositories"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

// Write does one thing from the queue. What it answers decides whether
// the queue tries again.
func (h *Handlers) Write(ctx context.Context, delivery federation.Delivery) error {
	w := &write{}
	if err := json.Unmarshal(delivery.Payload, w); err != nil {
		return federation.Permanent(err)
	}
	account, err := h.Accounts.Read(delivery.ActorID)
	if err != nil {
		return err
	}
	if account == nil || account.BrokenAt != nil {
		return federation.Permanent(errors.New("no Bluesky account is connected"))
	}

	// Taken back while it waited: a like given and taken back before it
	// could be written is not written afterwards. What waits is not
	// always done in the order it was asked for, since what fails is
	// tried again later.
	if w.Op == "create" {
		if kind := kindOf(w.Collection); kind != "" {
			if still, err := h.Sent.IsOurs(account.DID, kind, w.RKey); err != nil || !still {
				return err
			}
		}
	}

	do := func(token string) error {
		if w.Op == "delete" {
			return h.Writer.DeleteRecord(ctx, account.PDS, token, account.DID, w.Collection, w.RKey)
		}
		if w.Op == "gate" {
			return h.writeGate(ctx, account, token, w)
		}
		// A follow the account has already is not made a second time,
		// and is not this installation's to take back.
		if w.Collection == bsky.FollowCollection {
			already, err := h.Writer.Follows(ctx, account.PDS, token, w.Subject)
			if err != nil {
				return err
			}
			if already {
				return h.forgetSent(account, repositories.SentFollow, w.RKey)
			}
			return h.Writer.CreateRecord(ctx, account.PDS, token, account.DID, w.Collection, w.RKey,
				&bsky.OutFollow{Type: bsky.FollowCollection, Subject: w.Subject, CreatedAt: bsky.Stamp(h.now())})
		}
		if w.Collection == bsky.BlockCollection {
			return h.Writer.CreateRecord(ctx, account.PDS, token, account.DID, w.Collection, w.RKey,
				&bsky.OutFollow{Type: bsky.BlockCollection, Subject: w.Subject, CreatedAt: bsky.Stamp(h.now())})
		}
		// What is to be written, made now: a post's words as they are
		// now, what it answers as Bluesky names it now, and its pictures
		// given to the account's server first.
		record, err := h.record(ctx, w, func(content []byte, mimeType string) (*bsky.OutBlob, error) {
			return h.Writer.UploadBlob(ctx, account.PDS, token, content, mimeType)
		})
		if err != nil || record == nil {
			return err
		}
		return h.Writer.CreateRecord(ctx, account.PDS, token, account.DID, w.Collection, w.RKey, record)
	}

	token, err := h.token(ctx, account, false)
	if err != nil {
		return err
	}
	err = do(token)
	if errors.Is(err, bsky.ErrExpired) || errors.Is(err, bsky.ErrDenied) {
		// The short token ran out: another is got with the long one.
		if token, err = h.token(ctx, account, true); err != nil {
			return err
		}
		err = do(token)
	}
	if err != nil {
		log.Printf("writing to Bluesky for %d (%s %s): %v", account.PersonID, w.Op, w.Collection, err)
	}
	return err
}

// forgetSent forgets something that turned out not to need writing.
func (h *Handlers) forgetSent(account *repositories.Account, kind, rkey string) error {
	return h.Sent.DeleteByName(account.DID, kind, rkey)
}

// errNotWrittenYet is a reply to a post from here that is to be written to
// Bluesky and is not there yet: tried again shortly, never given up on,
// so that a thread goes out whole and in order (roadmap row 101).
var errNotWrittenYet = errors.New("what it answers is not written to Bluesky yet")

// refOf is what a post on Bluesky is named by, and the post that began its
// thread. One written from here is read from the server of the account
// that wrote it, which has it the moment it is written; anything else from
// the AppView.
func (h *Handlers) refOf(ctx context.Context, uri string) (*bsky.Ref, *bsky.Ref, error) {
	if did, rkey, ok := bsky.ParsePostURI(uri); ok {
		if sent, err := h.Sent.ReadByName(did, repositories.SentPost, rkey); err != nil {
			return nil, nil, err
		} else if sent != nil {
			account, err := h.Accounts.Read(sent.PersonID)
			if err != nil {
				return nil, nil, err
			}
			if account != nil && account.PDS != "" {
				post, root, err := h.Writer.RecordRefOf(ctx, account.PDS, uri)
				if !errors.Is(err, bsky.ErrNobody) {
					return post, root, err
				}
				// Not on its server: written there some other way, or not
				// yet. Nowhere is not yet.
				post, root, err = h.Writer.RefOf(ctx, uri)
				if errors.Is(err, bsky.ErrNobody) {
					return nil, nil, errNotWrittenYet
				}
				return post, root, err
			}
		}
	}
	return h.Writer.RefOf(ctx, uri)
}

// kindOf is what kind of thing a collection holds, as it is remembered.
func kindOf(collection string) string {
	for kind, each := range collections {
		if each == collection {
			return kind
		}
	}
	return ""
}

// writtenAt is the time a post says it was written, on Bluesky: when it
// was written here, which is kept to the second, and its id's last three
// digits as the milliseconds. The parts of a thread are written in the
// same second, and Bluesky lists one post of those at the same moment, the
// last: the first part was in no tab of its author's profile (the user,
// 2026-11-17). Their ids follow each other, so they stay in order but for
// a thread whose ids pass a thousand.
func writtenAt(note *entities.AnahitaNode) time.Time {
	at := note.CreatedAt
	if at.IsZero() || at.Nanosecond() != 0 || note.ID <= 0 {
		return at
	}
	return at.Add(time.Duration(note.ID%1000) * time.Millisecond)
}

// record makes what is written for a like, a repost or a reply. Nil, and
// no error, when there is no longer anything to write.
func (h *Handlers) record(ctx context.Context, w *write, upload uploader) (any, error) {
	var (
		subject, root *bsky.Ref
		err           error
	)
	if w.Subject != "" {
		subject, root, err = h.refOf(ctx, w.Subject)
		if errors.Is(err, errNotWrittenYet) {
			return nil, err
		}
		if errors.Is(err, bsky.ErrNobody) {
			// What it was of is gone from Bluesky.
			return nil, federation.Permanent(err)
		}
		if err != nil {
			return nil, err
		}
	} else if w.Collection != bsky.PostCollection {
		return nil, federation.Permanent(errors.New("a like or a repost of nothing"))
	}

	if w.Collection != bsky.PostCollection {
		return &bsky.OutSubject{Type: w.Collection, Subject: *subject, CreatedAt: bsky.Stamp(h.now())}, nil
	}

	note, err := h.Nodes.Read(w.NodeID)
	if err != nil {
		return nil, err
	}
	if note == nil || note.ID == 0 || note.DeletedAt != nil || !note.Enabled {
		// Deleted before it was written.
		return nil, nil
	}
	words := note.Body
	// A video post's title comes first, as its own line, and an article's:
	// a post on Bluesky has no title of its own.
	if videos.IsOurs(note.Type) || note.Type == constants.NodeMediumArticleEntityType {
		if title := strings.TrimSpace(note.Name); title != "" {
			words = strings.TrimSpace(title + "\n\n" + words)
		}
	}
	post := bsky.NewOutPost(words, note.Language, writtenAt(note))
	// Its content warning, as the self-label of its kind; Other, in its
	// author's words, has none on Bluesky (roadmap row 122).
	post.WithLabel(entities.WarningLabel(note.Warning))
	// More pictures than Bluesky takes.
	morePictures := false
	if subject != nil {
		post.Reply = &bsky.OutReply{Root: *root, Parent: *subject}
	}
	// Its pictures, when it is a photo post.
	if note.Type == constants.NodeMediumPhotoEntityType {
		more, err := h.withPictures(post, note, upload)
		if err != nil {
			return nil, err
		}
		morePictures = more
	}
	// Its video, when it is a video post.
	if note.Type == videos.NodeType {
		if err := h.withVideo(ctx, post, note, upload); err != nil {
			return nil, err
		}
	}
	// An article is read here, whatever its length: its card leads to it,
	// with its title, its opening and its cover.
	if note.Type == constants.NodeMediumArticleEntityType {
		post.WithCard(h.Site.Web(webPathOf(note)), cutTitle(note), articleOpening(note), h.coverPicture(note, upload))
	}
	// An audio post is listened to here: its card leads to it, with its
	// title, its opening and its cover.
	if note.Type == videos.AudioNodeType {
		post.WithCard(h.Site.Web(webPathOf(note)), cutTitle(note), articleOpening(note), h.audioCover(note, upload))
	}
	// The post it quotes, quoted there as a quote made there is (roadmap
	// row 156). One not on Bluesky is not carried there: the quote is not
	// written (roadmap row 162).
	address, quoted := h.quotedAddress(note)
	quotedRef, err := h.quotedRef(ctx, quoted, address)
	if err != nil {
		return nil, err
	}
	if note.QuoteID > 0 && !note.QuoteDetached && quoted != nil && quotedRef == nil {
		return nil, nil
	}
	if post.Text == "" && post.Embed == nil {
		return nil, nil
	}
	// Cut to what Bluesky takes: the card is of the post itself, here,
	// where the rest of it is (the user, 2026-10-17), when it carries
	// nothing else. A post has pictures or a card, not both. Not a
	// channel's note from its feed: the whole of it is at its link, whose
	// card it keeps, with its picture (the user, 2026-11-22).
	if post.Cut && post.Embed == nil && !permissions.FromFeed(note.Meta) {
		post.WithCard(h.Site.Web(fmt.Sprintf("/notes/%d", note.ID)), cutTitle(note), h.Site.Domain, nil)
	}
	// One with pictures or a video ends with a link to the rest instead,
	// shown as its address, which needs no words in any language; and so
	// does one with more pictures than Bluesky takes (roadmap row 100).
	if (post.Cut || morePictures) && post.Embed != nil && post.Embed.Type != "app.bsky.embed.external" {
		address := h.Site.Web(webPathOf(note))
		post.EndWithLink(strings.TrimPrefix(address, "https://"), address)
	}
	// The post it quotes, embedded as it is on Bluesky.
	if quotedRef != nil && !note.QuoteDetached {
		post.WithQuote(quotedRef)
	}
	// The card of the link in it, as it was kept when the note was made.
	if card := linkpreviews.FromMeta(note.Meta); card != nil && post.Embed == nil {
		post.WithCard(card.URL, card.Title, card.Description, h.cardPicture(card.ImageKey, upload))
	}
	return post, nil
}
