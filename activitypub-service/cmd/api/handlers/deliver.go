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
	"sync"
	"time"

	"activitypub/fetch"

	"anahita-libs/federation"
)

// Enqueuer is what something to send is handed to. It is sent later, and
// again until it arrives or two days have passed (anahita-libs/federation).
type Enqueuer interface {
	EnqueueMany(ctx context.Context, actorID int64, payload []byte, to []federation.Destination) error
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
	return h.sendMany(ctx, actorID, []string{inbox}, activity, what)
}

// sendMany queues one thing for many inboxes, as send does for one: made
// once, and queued in one go, however many servers it goes to (roadmap row
// 170).
func (h *Handlers) sendMany(ctx context.Context, actorID int64, inboxes []string, activity any, what string) error {
	if len(inboxes) == 0 {
		return nil
	}
	payload, err := json.Marshal(activity)
	if err != nil {
		return err
	}
	to := make([]federation.Destination, 0, len(inboxes))
	seen := make(map[string]bool, len(inboxes))
	for _, inbox := range inboxes {
		if inbox == "" || seen[inbox] {
			continue
		}
		seen[inbox] = true
		sum := sha256.Sum256([]byte(what + "\n" + inbox))
		to = append(to, federation.Destination{Target: inbox, DedupeKey: hex.EncodeToString(sum[:])})
	}
	return h.Queue.EnqueueMany(ctx, actorID, payload, to)
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

	uuid, err := h.senderUUID(delivery.ActorID)
	if err != nil {
		return err
	}
	if uuid == "" {
		// Whoever it was sent for is gone. Nobody is left to sign it.
		return federation.Permanent(errors.New("nobody here to send it for"))
	}

	keyID := h.addresses.Key(uuid)
	err = h.Poster.Post(ctx, delivery.Target, delivery.Payload, func() (string, *rsaKey, error) {
		return h.signerFor(delivery.ActorID, keyID)
	})

	switch {
	case err == nil:
		return nil
	case errors.Is(err, fetch.ErrGone), errors.Is(err, fetch.ErrRefused), errors.Is(err, fetch.ErrNotAllowed):
		return federation.Permanent(err)
	}
	return err
}

// senders is who things were lately sent for, so that a post going to
// many servers reads its author once and not once for each server
// (roadmap row 170). Only their UUID, which does not change.
type senders struct {
	mu   sync.Mutex
	kept map[int64]keptSender
}

type keptSender struct {
	uuid string
	at   time.Time
}

// senderKept is how long a sender is remembered: a little longer than a
// post takes to go to every server, and short enough that somebody who
// has gone is soon found gone.
const senderKept = time.Minute

// senderUUID is the UUID of who a delivery is sent for, or "" when nobody
// here is left to send it for.
func (h *Handlers) senderUUID(id int64) (string, error) {
	h.senders.mu.Lock()
	kept, ok := h.senders.kept[id]
	h.senders.mu.Unlock()
	if ok && time.Since(kept.at) < senderKept {
		return kept.uuid, nil
	}

	sender, err := h.Nodes.Read(id)
	if err != nil {
		return "", err
	}
	if sender == nil || sender.ID == 0 || sender.UUID == "" || sender.IsRemote() {
		return "", nil
	}

	h.senders.mu.Lock()
	defer h.senders.mu.Unlock()
	if h.senders.kept == nil {
		h.senders.kept = map[int64]keptSender{}
	}
	// Only who sends now: the map does not grow with everybody who ever
	// sent anything.
	for other, was := range h.senders.kept {
		if time.Since(was.at) >= senderKept {
			delete(h.senders.kept, other)
		}
	}
	h.senders.kept[id] = keptSender{uuid: sender.UUID, at: time.Now()}
	return sender.UUID, nil
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
