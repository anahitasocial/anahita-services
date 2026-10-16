package handlers

import (
	"context"
	"errors"
	"slices"

	"atproto/bsky"
	"atproto/data/repositories"

	"anahita-libs/data/entities"
)

// A reply hidden here, hidden on Bluesky too.
//
// On Bluesky the author of a post keeps, beside it, a record listing the
// replies hidden under it, its threadgate, and every app reads it and
// tucks those replies away. When somebody here hides a reply that is on
// Bluesky, under a post of theirs that went there as their connected
// account, it is added to that list; shown again, it is taken out.
//
// Only that reply is changed in the list. The rest of the record is left
// as it is: replies hidden in Bluesky's own app, and who may reply, when
// that was set there.
//
// Whether the reply is hidden is read when the list is written, not when
// the hiding was heard, so hiding a reply and showing it again before it
// was written writes what is so at the end.

// hiddenChanged is the reply `replyID` having been hidden from its thread,
// or shown again, under the post `rootID`.
func (h *Handlers) hiddenChanged(ctx context.Context, rootID, replyID int64) error {
	if !h.canWrite() || h.Media == nil || rootID <= 0 || replyID <= 0 {
		return nil
	}
	// The post went to Bluesky from here, or came from there as somebody's
	// own: either way it is remembered by the name it has there.
	sent, err := h.Sent.Read(repositories.SentPost, rootID, 0)
	if err != nil || sent == nil {
		return err
	}
	account := h.connected(sent.PersonID)
	if account == nil || account.DID != sent.DID {
		return nil
	}
	return h.enqueue(ctx, account.PersonID, &write{
		Op: "gate", Collection: bsky.ThreadgateCollection, RKey: sent.RKey, NodeID: rootID, ReplyID: replyID,
	})
}

// writeGate puts one reply in a post's threadgate, or takes it out.
func (h *Handlers) writeGate(ctx context.Context, account *repositories.Account, token string, w *write) error {
	// The post is still there, and still this account's.
	sent, err := h.Sent.Read(repositories.SentPost, w.NodeID, account.PersonID)
	if err != nil || sent == nil || sent.RKey != w.RKey || sent.DID != account.DID {
		return err
	}

	// The reply as the thread's author sees it now.
	viewer := entities.NewViewer()
	viewer.ID = account.PersonID
	replies, err := h.Media.Thread(w.NodeID, viewer)
	if err != nil {
		return err
	}
	var reply *entities.AnahitaNode
	for _, each := range replies {
		if each.ID == w.ReplyID {
			reply = each
		}
	}
	// Gone from the thread, or not on Bluesky.
	uri := h.onBluesky(reply)
	if uri == "" {
		return nil
	}

	gate, err := h.Writer.GetRecord(ctx, account.PDS, account.DID, bsky.ThreadgateCollection, w.RKey)
	if errors.Is(err, bsky.ErrNobody) {
		gate, err = nil, nil
	}
	if err != nil {
		return err
	}

	var (
		hidden    []string
		wasHidden bool
	)
	if gate != nil {
		was, _ := gate["hiddenReplies"].([]any)
		for _, each := range was {
			each, ok := each.(string)
			switch {
			case !ok || slices.Contains(hidden, each):
			case each == uri:
				wasHidden = true
			default:
				hidden = append(hidden, each)
			}
		}
	}
	// Already as it should be: nothing is written.
	if wasHidden == reply.Hidden {
		return nil
	}
	if reply.Hidden {
		hidden = append(hidden, uri)
	}
	// No more than a threadgate holds: the oldest go first.
	if len(hidden) > bsky.ThreadgateHiddenMax {
		hidden = hidden[len(hidden)-bsky.ThreadgateHiddenMax:]
	}

	if gate == nil {
		// Who may reply is left out, which is anybody.
		gate = map[string]any{
			"$type":     bsky.ThreadgateCollection,
			"post":      bsky.PostURI(account.DID, w.RKey),
			"createdAt": bsky.Stamp(h.now()),
		}
	}
	if len(hidden) == 0 {
		delete(gate, "hiddenReplies")
	} else {
		gate["hiddenReplies"] = hidden
	}
	return h.Writer.CreateRecord(ctx, account.PDS, token, account.DID, bsky.ThreadgateCollection, w.RKey, gate)
}
