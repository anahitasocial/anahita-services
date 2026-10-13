package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"

	"atproto/bsky"
	"atproto/data/repositories"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/federation"
	"anahita-libs/linkpreviews"
)

// What somebody here does to something from Bluesky, done there too.
//
// Only for somebody who connected their Bluesky account (settings.go). A
// like or a repost of a post that came from Bluesky, and a reply under
// one, or under a reply that itself went there. And a note of their own
// that is open to everybody, on their own profile, which is posted there
// as it is here: connecting an account is saying so, as being on the
// fediverse is. All of it is written as their account, to the server that
// holds it.
//
// What is to be written waits in a table and is tried until it is done,
// as what goes to the fediverse does. Each thing is given its name before
// it is written and remembered by it: so that writing it twice writes it
// once, and so that when Bluesky tells of it, as it tells of everything
// the account does, it is known for this installation's own and is not
// kept a second time as something from elsewhere.

// Writer is what writes to an account's server.
type Writer interface {
	PDSOf(ctx context.Context, directory, did string) (string, error)
	CreateSession(ctx context.Context, pds, identifier, password string) (*bsky.Session, error)
	RefreshSession(ctx context.Context, pds, refreshJWT string) (*bsky.Session, error)
	CreateRecord(ctx context.Context, pds, accessJWT, did, collection, rkey string, record any) error
	DeleteRecord(ctx context.Context, pds, accessJWT, did, collection, rkey string) error
	RefOf(ctx context.Context, uri string) (post, root *bsky.Ref, err error)
}

// Enqueuer is the table of what waits to be written.
type Enqueuer interface {
	Enqueue(ctx context.Context, actorID int64, target string, payload []byte, dedupeKey string) error
}

// write is one thing waiting to be written.
type write struct {
	// "create" or "delete".
	Op         string `json:"op"`
	Collection string `json:"collection"`
	RKey       string `json:"rkey"`
	// For a like or a repost, or a reply: the post it is of, or under.
	Subject string `json:"subject,omitempty"`
	// For a reply: the node that holds its words.
	NodeID int64 `json:"node_id,omitempty"`
}

var collections = map[string]string{
	repositories.SentPost:   bsky.PostCollection,
	repositories.SentLike:   "app.bsky.feed.like",
	repositories.SentRepost: bsky.RepostCollection,
}

// onBluesky is the address on Bluesky of something here: what it came
// with, when it came from there, or what it was written as, when it is a
// reply that went there. "" for anything else.
func (h *Handlers) onBluesky(node *entities.AnahitaNode) string {
	if node == nil || node.ID == 0 {
		return ""
	}
	if node.IsRemote() {
		if _, _, ok := bsky.ParsePostURI(node.URI); ok {
			return node.URI
		}
		return ""
	}
	sent, err := h.Sent.Read(repositories.SentPost, node.ID, 0)
	if err != nil || sent == nil {
		return ""
	}
	return bsky.PostURI(sent.DID, sent.RKey)
}

// connected is the account of somebody here that can be written as, or
// nil.
func (h *Handlers) connected(personID int64) *repositories.Account {
	if !h.canWrite() || personID <= 0 {
		return nil
	}
	account, err := h.Accounts.Read(personID)
	if err != nil || account == nil || account.BrokenAt != nil {
		return nil
	}
	return account
}

// begin remembers something about to be written and puts it in the queue.
func (h *Handlers) begin(ctx context.Context, account *repositories.Account, kind string, localID int64, subject string, nodeID int64) error {
	sent := &repositories.Sent{
		Kind: kind, PersonID: account.PersonID, LocalID: localID, DID: account.DID, RKey: bsky.NewTID(),
	}
	begun, err := h.Sent.Begin(sent)
	if err != nil || !begun {
		// Written already, or being written: another copy of this
		// service heard it first, or it is a reply that was changed.
		return err
	}
	return h.enqueue(ctx, account.PersonID, &write{
		Op: "create", Collection: collections[kind], RKey: sent.RKey, Subject: subject, NodeID: nodeID,
	})
}

// takeBack forgets something that was written and has it removed.
func (h *Handlers) takeBack(ctx context.Context, kind string, localID, personID int64) error {
	sent, err := h.Sent.Read(kind, localID, personID)
	if err != nil || sent == nil {
		return err
	}
	if err := h.Sent.Delete(sent.ID); err != nil {
		return err
	}
	return h.enqueue(ctx, sent.PersonID, &write{Op: "delete", Collection: collections[kind], RKey: sent.RKey})
}

func (h *Handlers) enqueue(ctx context.Context, personID int64, w *write) error {
	payload, err := json.Marshal(w)
	if err != nil {
		return err
	}
	return h.Queue.Enqueue(ctx, personID, "bluesky", payload, fmt.Sprintf("%s:%s:%s", w.Op, w.Collection, w.RKey))
}

// localLiked is somebody here having liked something.
func (h *Handlers) localLiked(ctx context.Context, personID, nodeID int64) error {
	account := h.connected(personID)
	if account == nil {
		return nil
	}
	liked, err := h.Nodes.Read(nodeID)
	if err != nil {
		return err
	}
	subject := h.onBluesky(liked)
	if subject == "" {
		return nil
	}
	return h.begin(ctx, account, repositories.SentLike, liked.ID, subject, 0)
}

// localUnliked is somebody here having taken a like back.
func (h *Handlers) localUnliked(ctx context.Context, personID, nodeID int64) error {
	if !h.canWrite() {
		return nil
	}
	return h.takeBack(ctx, repositories.SentLike, nodeID, personID)
}

// localReposted is somebody here having reposted something.
func (h *Handlers) localReposted(ctx context.Context, repostID int64) error {
	if !h.canWrite() {
		return nil
	}
	repost, err := h.Nodes.Read(repostID)
	if err != nil || repost == nil || repost.ID == 0 {
		return err
	}
	if repost.Type != constants.NodeBaseRepostType || !repost.Enabled || repost.IsRemote() {
		return nil
	}
	if repost.CreatedBy <= 0 || repost.OwnerID != repost.CreatedBy {
		return nil
	}
	account := h.connected(repost.CreatedBy)
	if account == nil {
		return nil
	}
	post, err := h.Nodes.Read(repost.ParentID)
	if err != nil {
		return err
	}
	subject := h.onBluesky(post)
	if subject == "" {
		return nil
	}
	return h.begin(ctx, account, repositories.SentRepost, repost.ID, subject, 0)
}

// localPosted is somebody here having written something. A reply under
// something that is on Bluesky goes there; so does a note of their own
// that everybody may read.
func (h *Handlers) localPosted(ctx context.Context, noteID int64) error {
	if !h.canWrite() {
		return nil
	}
	note, err := h.Nodes.Read(noteID)
	if err != nil || note == nil || note.ID == 0 {
		return err
	}
	if note.Type != constants.NodeMediumNoteEntityType || note.IsRemote() || !note.Enabled || note.DeletedAt != nil || note.Hidden {
		return nil
	}
	if note.CreatedBy <= 0 {
		return nil
	}
	account := h.connected(note.CreatedBy)
	if account == nil {
		return nil
	}

	// A note that answers nothing: theirs, on their own profile, open to
	// everybody.
	if note.ParentID <= 0 && note.RootID <= 0 {
		if note.OwnerID != note.CreatedBy || note.Access != constants.AnahitaAccessPublic {
			return nil
		}
		return h.begin(ctx, account, repositories.SentPost, note.ID, "", note.ID)
	}

	parent, err := h.Nodes.Read(note.ParentID)
	if err != nil {
		return err
	}
	subject := h.onBluesky(parent)
	if subject == "" {
		return nil
	}
	return h.begin(ctx, account, repositories.SentPost, note.ID, subject, note.ID)
}

// localRemoved is something here being gone: a reply or a repost that
// went to Bluesky is removed there. The node is gone by now; what it was
// is in what is remembered of what was written.
func (h *Handlers) localRemoved(ctx context.Context, nodeID int64) error {
	if !h.canWrite() {
		return nil
	}
	for _, kind := range []string{repositories.SentPost, repositories.SentRepost} {
		if err := h.takeBack(ctx, kind, nodeID, 0); err != nil {
			return err
		}
	}
	return nil
}

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

	// What is to be written, made now: a reply's words as they are now,
	// and what it answers as Bluesky names it now.
	var record any
	if w.Op == "create" {
		// Taken back while it waited: a like given and taken back before
		// it could be written is not written afterwards. What waits is
		// not always done in the order it was asked for, since what
		// fails is tried again later.
		if kind := kindOf(w.Collection); kind != "" {
			if still, err := h.Sent.IsOurs(account.DID, kind, w.RKey); err != nil || !still {
				return err
			}
		}
		if record, err = h.record(ctx, w); err != nil {
			log.Printf("writing to Bluesky for %d (%s): what to write: %v", account.PersonID, w.Collection, err)
			return err
		}
		if record == nil {
			return nil
		}
	}

	do := func(token string) error {
		if w.Op == "delete" {
			return h.Writer.DeleteRecord(ctx, account.PDS, token, account.DID, w.Collection, w.RKey)
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

// kindOf is what kind of thing a collection holds, as it is remembered.
func kindOf(collection string) string {
	for kind, each := range collections {
		if each == collection {
			return kind
		}
	}
	return ""
}

// record makes what is written for a like, a repost or a reply. Nil, and
// no error, when there is no longer anything to write.
func (h *Handlers) record(ctx context.Context, w *write) (any, error) {
	var (
		subject, root *bsky.Ref
		err           error
	)
	if w.Subject != "" {
		subject, root, err = h.Writer.RefOf(ctx, w.Subject)
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
	post := bsky.NewOutPost(note.Body, note.Language, note.CreatedAt)
	if post.Text == "" {
		return nil, nil
	}
	if subject != nil {
		post.Reply = &bsky.OutReply{Root: *root, Parent: *subject}
	}
	// The card of the link in it, as it was kept when the note was made.
	if card := linkpreviews.FromMeta(note.Meta); card != nil {
		post.WithCard(card.URL, card.Title, card.Description)
	}
	return post, nil
}

// The short tokens, one for each connected person, kept only in memory.
type tokens struct {
	mu   sync.Mutex
	held map[int64]string
}

func (h *Handlers) remember(personID int64, token string) {
	h.tokens.mu.Lock()
	defer h.tokens.mu.Unlock()
	if h.tokens.held == nil {
		h.tokens.held = map[int64]string{}
	}
	h.tokens.held[personID] = token
}

func (h *Handlers) forgetToken(personID int64) {
	h.tokens.mu.Lock()
	defer h.tokens.mu.Unlock()
	delete(h.tokens.held, personID)
}

// token is the short token to write as an account with: the one held, or
// a new one got with the long token when there is none or `fresh` is
// asked for. An account whose long token is refused is marked as needing
// to be connected again.
func (h *Handlers) token(ctx context.Context, account *repositories.Account, fresh bool) (string, error) {
	h.tokens.mu.Lock()
	held := h.tokens.held[account.PersonID]
	h.tokens.mu.Unlock()
	if held != "" && !fresh {
		return held, nil
	}

	// The long token as it is kept now: another copy of this service may
	// have spent the one this copy read.
	current, err := h.Accounts.Read(account.PersonID)
	if err != nil {
		return "", err
	}
	if current == nil {
		return "", federation.Permanent(errors.New("no Bluesky account is connected"))
	}
	refresh, err := h.Crypt.Decrypt(current.Refresh)
	if err != nil {
		return "", federation.Permanent(err)
	}

	session, err := h.Writer.RefreshSession(ctx, current.PDS, refresh)
	if errors.Is(err, bsky.ErrDenied) || errors.Is(err, bsky.ErrExpired) {
		log.Printf("the Bluesky account of %d has to be connected again: %v", account.PersonID, err)
		if err := h.Accounts.SetBroken(account.PersonID); err != nil {
			log.Println("marking it so:", err)
		}
		h.forgetToken(account.PersonID)
		return "", federation.Permanent(err)
	}
	if err != nil {
		return "", err
	}

	kept, err := h.Crypt.Encrypt(session.RefreshJWT)
	if err != nil {
		return "", err
	}
	if err := h.Accounts.SetRefresh(account.PersonID, kept); err != nil {
		return "", err
	}
	h.remember(account.PersonID, session.AccessJWT)
	return session.AccessJWT, nil
}
