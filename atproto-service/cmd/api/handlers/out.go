package handlers

import (
	"anahita-libs/permissions"
	"context"
	"encoding/json"
	"fmt"
	_ "image/gif"
	_ "image/png"

	"atproto/bsky"
	"atproto/data/repositories"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/federation"
	"anahita-libs/videos"
)

// What somebody here does to something from Bluesky, done there too.
//
// Only for somebody who connected their Bluesky account (settings.go). A
// like or a repost of a post that came from Bluesky, and a reply under
// one, or under a reply that itself went there. And a note or a photo
// post of their own that is open to everybody, on their own profile,
// which is posted there as it is here, pictures and all: connecting an account is saying so, as being on the
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
	UploadBlob(ctx context.Context, pds, accessJWT string, content []byte, mimeType string) (*bsky.OutBlob, error)
	Follows(ctx context.Context, pds, accessJWT, did string) (bool, error)
	Notifications(ctx context.Context, pds, accessJWT string, limit int) ([]bsky.Notification, error)
	RefOf(ctx context.Context, uri string) (post, root *bsky.Ref, err error)
	RecordRefOf(ctx context.Context, pds, uri string) (post, root *bsky.Ref, err error)
	GetRecord(ctx context.Context, pds, did, collection, rkey string) (map[string]any, error)
}

// Enqueuer is the table of what waits to be written.
type Enqueuer interface {
	Enqueue(ctx context.Context, actorID int64, target string, payload []byte, dedupeKey string) error
}

// write is one thing waiting to be written.
type write struct {
	// "create" or "delete", or "gate" for a post's threadgate (hidden.go).
	Op         string `json:"op"`
	Collection string `json:"collection"`
	RKey       string `json:"rkey"`
	// For a like or a repost, or a reply: the post it is of, or under.
	Subject string `json:"subject,omitempty"`
	// For a reply: the node that holds its words.
	NodeID int64 `json:"node_id,omitempty"`
	// For a threadgate: the reply hidden or shown under the post.
	ReplyID int64 `json:"reply_id,omitempty"`
}

var collections = map[string]string{
	repositories.SentPost:   bsky.PostCollection,
	repositories.SentLike:   "app.bsky.feed.like",
	repositories.SentRepost: bsky.RepostCollection,
	repositories.SentFollow: bsky.FollowCollection,
	repositories.SentBlock:  bsky.BlockCollection,
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
	// Only for somebody whose profile anybody may see, as on the
	// fediverse: a profile kept to followers or to members writes nothing
	// to Bluesky, which is read by anybody.
	if !permissions.CanReadByID(h.Nodes, account.PersonID, entities.NewViewer()) {
		return nil
	}
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
	key := fmt.Sprintf("%s:%s:%s", w.Op, w.Collection, w.RKey)
	if w.ReplyID > 0 {
		key += fmt.Sprintf(":%d", w.ReplyID)
	}
	return h.Queue.Enqueue(ctx, personID, "bluesky", payload, key)
}

// A follow of somebody on Bluesky, by somebody here who connected their
// account, is their account following that one: there is no other kind of
// follow on Bluesky. Whoever is followed is told, and whoever follows is
// among their followers.
//
// An account that follows them already is left as it is: nothing is
// written, and nothing is remembered, so that unfollowing here later does
// not take away a follow that was made on Bluesky. Only what this
// installation wrote is taken back by it.

// localFollowed is somebody here having followed somebody on Bluesky.
func (h *Handlers) localFollowed(ctx context.Context, personID int64, kept *repositories.RemoteActor) error {
	account := h.connected(personID)
	if account == nil || kept == nil || account.DID == kept.DID {
		return nil
	}
	return h.begin(ctx, account, repositories.SentFollow, kept.NodeID, kept.DID, 0)
}

// localUnfollowed is somebody here having stopped following somebody on
// Bluesky.
func (h *Handlers) localUnfollowed(ctx context.Context, personID int64, kept *repositories.RemoteActor) error {
	if !h.canWrite() || kept == nil {
		return nil
	}
	return h.takeBack(ctx, repositories.SentFollow, kept.NodeID, personID)
}

// localBlocked is somebody here having blocked somebody on Bluesky: their
// connected account blocks them there too, so that what was stopped here
// is stopped there. Written whatever the person's profile is open to: a
// block keeps somebody away, and says nothing to anybody else.
func (h *Handlers) localBlocked(ctx context.Context, personID int64, kept *repositories.RemoteActor) error {
	account := h.connected(personID)
	if account == nil || kept == nil || account.DID == kept.DID {
		return nil
	}
	sent := &repositories.Sent{
		Kind: repositories.SentBlock, PersonID: account.PersonID, LocalID: kept.NodeID, DID: account.DID, RKey: bsky.NewTID(),
	}
	begun, err := h.Sent.Begin(sent)
	if err != nil || !begun {
		return err
	}
	return h.enqueue(ctx, account.PersonID, &write{
		Op: "create", Collection: bsky.BlockCollection, RKey: sent.RKey, Subject: kept.DID,
	})
}

// localUnblocked is somebody here having unblocked somebody on Bluesky:
// the block written from here is taken back there. One made in Bluesky's
// own app is not this installation's to take back.
func (h *Handlers) localUnblocked(ctx context.Context, personID int64, kept *repositories.RemoteActor) error {
	if !h.canWrite() || kept == nil {
		return nil
	}
	return h.takeBack(ctx, repositories.SentBlock, kept.NodeID, personID)
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
	isPhoto := note.Type == constants.NodeMediumPhotoEntityType
	// A video post is written once it has its video.
	isVideo := note.Type == videos.NodeType && videos.FromMeta(note.Meta) != nil
	// An article, as a post of its title and opening with a card that leads
	// to it. A topic stays here (the user, 2026-10-17).
	isArticle := note.Type == constants.NodeMediumArticleEntityType
	// An audio post, as its title and opening with a card that leads to it:
	// Bluesky has no sound (roadmap row 137).
	isArticle = isArticle || (note.Type == videos.AudioNodeType && videos.AudioFromMeta(note.Meta) != nil)
	if (note.Type != constants.NodeMediumNoteEntityType && !isPhoto && !isVideo && !isArticle) || note.IsRemote() || !note.Enabled || note.DeletedAt != nil || note.Hidden {
		return nil
	}
	// In its author's media library only: announced to nobody.
	if note.LibraryOnly {
		return nil
	}
	if note.CreatedBy <= 0 {
		return nil
	}
	// It came from their own account on Bluesky: it is there already.
	if markedOwn(note.Meta) {
		return nil
	}
	account := h.connected(note.CreatedBy)
	if account == nil {
		return nil
	}

	// A quote goes to Bluesky only when what it quotes is there, by its
	// author's choice (roadmap row 162): a quote of a post from the
	// fediverse, or of one here that is not on Bluesky, stays off it. One
	// that went before is taken back.
	if h.quoteStaysOff(note) {
		return h.localRemoved(ctx, note.ID)
	}

	// A note that answers nothing: theirs, on their own profile, open to
	// everybody.
	if note.ParentID <= 0 && note.RootID <= 0 {
		if note.OwnerID != note.CreatedBy || note.Access != constants.AnahitaAccessPublic {
			// Not one that goes out. If it went before, open to everybody
			// and not now, it is taken back, as the fediverse takes it back
			// (the user, 2026-10-16); one never sent has nothing to take.
			return h.localRemoved(ctx, note.ID)
		}
		return h.begin(ctx, account, repositories.SentPost, note.ID, "", note.ID)
	}

	parent, err := h.Nodes.Read(note.ParentID)
	if err != nil {
		return err
	}
	subject := h.onBluesky(parent)
	// What it answers is the part before it in a thread written in one go,
	// and not on its way to Bluesky yet: the parts of such a thread are
	// heard of in no set order. That goes first, if it goes at all, so the
	// thread goes out whole and in order (roadmap row 101). An older post
	// of the writer's that never went still does not.
	if subject == "" && federation.WrittenTogether(parent, note) {
		if err := h.localPosted(ctx, parent.ID); err != nil {
			return err
		}
		subject = h.onBluesky(parent)
	}
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
	// A post's threadgate, if it has one, goes with it. A reply has none,
	// and removing what is not there is nothing.
	if sent, err := h.Sent.Read(repositories.SentPost, nodeID, 0); err != nil {
		return err
	} else if sent != nil {
		if err := h.enqueue(ctx, sent.PersonID, &write{Op: "delete", Collection: bsky.ThreadgateCollection, RKey: sent.RKey}); err != nil {
			return err
		}
	}
	for _, kind := range []string{repositories.SentPost, repositories.SentRepost} {
		if err := h.takeBack(ctx, kind, nodeID, 0); err != nil {
			return err
		}
	}
	return nil
}
