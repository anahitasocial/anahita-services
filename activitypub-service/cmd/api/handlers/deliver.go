package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"activitypub/fetch"

	"anahita-libs/federation"
)

// Enqueuer is what something to send is handed to. It is sent later, and
// again until it arrives or two days have passed (anahita-libs/federation).
type Enqueuer interface {
	Enqueue(ctx context.Context, actorID int64, target string, payload []byte, dedupeKey string) error
}

// Poster is what writes to another server's inbox.
type Poster interface {
	Post(ctx context.Context, inbox string, body []byte, signer fetch.Signer) error
}

// send queues something for one inbox, in the name of somebody here.
//
// Queued once however often it is asked: the key is made of what is sent
// and where to, so a message that arrives twice is answered once.
func (h *Handlers) send(ctx context.Context, actorID int64, inbox string, activity any, what string) error {
	payload, err := json.Marshal(activity)
	if err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(what + "\n" + inbox))
	return h.Queue.Enqueue(ctx, actorID, inbox, payload, hex.EncodeToString(sum[:]))
}

// Deliver sends one thing from the queue: signed by whoever it is sent
// for, to the inbox it is for. What it answers decides whether the queue
// tries again.
func (h *Handlers) Deliver(ctx context.Context, delivery federation.Delivery) error {
	// Nothing is sent to a server that is refused, whenever it was queued.
	if h.Blocks != nil && h.Blocks.IsBlocked(hostOf(delivery.Target)) {
		return federation.Permanent(errors.New("that server is refused"))
	}

	// The lab reaches the internet, and can read from real servers, but it
	// calls itself by a name no real server can look up. What it sent one
	// could never be checked, and would be tried for two days. So from the
	// lab, only the lab's own servers are written to.
	if h.Lab && !strings.HasSuffix(hostOf(delivery.Target), ".test") {
		return federation.Permanent(errors.New("the lab sends nothing to a real server"))
	}

	sender, err := h.Nodes.Read(delivery.ActorID)
	if err != nil {
		return err
	}
	if sender == nil || sender.ID == 0 || sender.UUID == "" || sender.IsRemote() {
		// Whoever it was sent for is gone. Nobody is left to sign it.
		return federation.Permanent(errors.New("nobody here to send it for"))
	}

	keyID := h.addresses.Key(sender.UUID)
	err = h.Poster.Post(ctx, delivery.Target, delivery.Payload, func() (string, *rsaKey, error) {
		return h.signerFor(sender.ID, keyID)
	})

	switch {
	case err == nil:
		return nil
	case errors.Is(err, fetch.ErrGone), errors.Is(err, fetch.ErrRefused), errors.Is(err, fetch.ErrNotAllowed):
		return federation.Permanent(err)
	}
	return err
}

// newUUID is a name for something this installation did.
func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
