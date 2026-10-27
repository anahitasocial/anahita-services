package handlers

import (
	"context"
	"encoding/json"
	"log"

	"atproto/bsky"
	"atproto/data/repositories"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/federation"
)

// What somebody here posts on Bluesky itself, with the account they
// connected.
//
// The account is theirs, so what it posts is theirs: it is kept here as a
// post of their own, on their own profile, open to everybody as it is
// there. Not as something from another server: there is one of them, and
// one profile. It is then a post like any they wrote here, and goes
// wherever those go.
//
// It is remembered by the name it has on Bluesky, as what is written from
// here is: so that it is not sent back to where it came from, so that a
// like or an answer given to it here reaches it there, and so that it
// goes from here when it is deleted there.
//
// From the time the account was connected on: what they posted before is
// not gone back for. A reply is kept when what it answers is here, and is
// put under it: the rest of a thread they posted there, or their answer to
// a post from Bluesky that is kept here. One that answers anything else is
// left, as a reply from somebody followed is (posts.go): it would stand
// without what it answers.

// Where a post says, in what is kept beside its words, that it came from
// the person's own account on Bluesky.
const ownMetaKey = "bluesky"

type ownMark struct {
	URI string `json:"uri"`
}

// markedOwn says whether a post here came from its author's own Bluesky
// account.
func markedOwn(meta []byte) bool {
	if len(meta) == 0 {
		return false
	}
	fields := map[string]json.RawMessage{}
	if json.Unmarshal(meta, &fields) != nil {
		return false
	}
	_, marked := fields[ownMetaKey]
	return marked
}

func markOwn(meta []byte, uri string) []byte {
	fields := map[string]json.RawMessage{}
	if len(meta) > 0 {
		_ = json.Unmarshal(meta, &fields)
	}
	raw, err := json.Marshal(ownMark{URI: uri})
	if err != nil {
		return meta
	}
	fields[ownMetaKey] = raw
	out, err := json.Marshal(fields)
	if err != nil {
		return meta
	}
	return out
}

// ownPosted is somebody here having posted on Bluesky with their own
// account.
func (h *Handlers) ownPosted(account *repositories.Account, rkey string, post *bsky.Post) error {
	if h.Media == nil || !bsky.ValidRkey(rkey) || post == nil {
		return nil
	}
	// Written from here, or kept already.
	if known, err := h.Sent.ReadByName(account.DID, repositories.SentPost, rkey); err != nil || known != nil {
		return err
	}
	var parentID int64
	if post.IsReply() {
		answered, err := h.ownAnswered(account, post.Answers())
		if err != nil || answered == 0 {
			return err
		}
		parentID = answered
	}
	person, err := h.Nodes.Read(account.PersonID)
	if err != nil || person == nil || person.ID == 0 || person.IsRemote() || !person.Enabled || person.DeletedAt != nil {
		return err
	}
	// A website's posts come from its feed alone: what its account posts on
	// Bluesky itself is not made a post of it here (docs/websites.md).
	if person.Type == constants.NodeActorWebsiteEntityType {
		return nil
	}

	var pictures []federation.Picture
	if h.Keeper != nil && h.Keeper.CanKeepPictures() && !post.IsSensitive() {
		for _, each := range post.Pictures() {
			if len(pictures) == federation.PostPicturesMax() {
				break
			}
			pictures = append(pictures, federation.Picture{
				URL: bsky.PictureURL(h.CDN, account.DID, each.CID),
				Alt: each.Alt,
			})
		}
	}

	body := federation.PostToMarkdown(post.HTML())
	if quoted := post.Quoted(); quoted != "" {
		if body != "" {
			body += "\n\n"
		}
		body += "↪ [bsky.app](" + quoted + ")"
	}
	body = cutRunes(body, postMax)
	if body == "" && len(pictures) == 0 {
		return nil
	}

	viewer := entities.NewViewer()
	viewer.ID = person.ID

	node := entities.NewAnahitaNode()
	node.Type, node.Service = constants.NodeMediumNoteEntityType, "text-service"
	if len(pictures) > 0 {
		node.Type, node.Service = constants.NodeMediumPhotoEntityType, "photo-service"
	}
	node.Body = body
	node.Language = post.Language()
	node.OwnerID, node.OwnerType, node.Owner = person.ID, person.Type, person
	node.CreatedBy, node.ModifiedBy = person.ID, person.ID
	node.Enabled = true
	node.CommentStatus = true
	node.Access = constants.AnahitaAccessPublic
	// A reply sits in its thread, where the graph gives it its place.
	node.ParentID = parentID
	// Marked as having come from Bluesky before it is made: that it was
	// made is said at once, and whoever hears it must not send it back.
	node.Meta = markOwn(federation.CardMeta(h.Cards, body, post.Link()), bsky.PostURI(account.DID, rkey))

	made, err := h.Media.Add(node, viewer)
	if err != nil || made == nil || made.ID == 0 {
		return err
	}
	if _, err := h.Sent.Begin(&repositories.Sent{
		Kind: repositories.SentPost, PersonID: person.ID, LocalID: made.ID, DID: account.DID, RKey: rkey,
	}); err != nil {
		return err
	}

	if len(pictures) > 0 {
		h.later(func() { h.Keeper.CopyPostPictures(made, pictures) })
	}
	return nil
}

// ownAnswered is the post here that a reply of somebody's own account
// answers, or 0: one of theirs, written here or kept from Bluesky, both
// remembered by their name there; or a post from Bluesky kept here.
func (h *Handlers) ownAnswered(account *repositories.Account, uri string) (int64, error) {
	did, rkey, ok := bsky.ParsePostURI(uri)
	if !ok {
		return 0, nil
	}
	if did == account.DID {
		sent, err := h.Sent.ReadByName(did, repositories.SentPost, rkey)
		if err != nil || sent == nil {
			return 0, err
		}
		return sent.LocalID, nil
	}
	kept, err := h.Nodes.ReadByURI(uri)
	if err != nil || kept == nil || kept.ID == 0 {
		return 0, err
	}
	return kept.ID, nil
}

// ownRemoved is somebody here having deleted, on Bluesky, a post of their
// own account: one that was kept from there, or one that went there from
// here. It goes from here too.
func (h *Handlers) ownRemoved(account *repositories.Account, rkey string) error {
	if h.Media == nil || !bsky.ValidRkey(rkey) {
		return nil
	}
	sent, err := h.Sent.ReadByName(account.DID, repositories.SentPost, rkey)
	if err != nil || sent == nil {
		return err
	}
	// Forgotten first, so that its going from here is not taken for
	// something to remove on Bluesky, where it is gone already.
	if err := h.Sent.Delete(sent.ID); err != nil {
		return err
	}
	post, err := h.Nodes.Read(sent.LocalID)
	if err != nil || post == nil || post.ID == 0 || post.CreatedBy != account.PersonID {
		return err
	}
	viewer := entities.NewViewer()
	viewer.ID = account.PersonID
	return h.Media.Delete(post.ID, viewer)
}

// heardOwn deals with something heard of an account that somebody here
// connected. It says whether the account was one.
func (h *Handlers) heardOwn(_ context.Context, heard *bsky.Event) bool {
	if !h.canWrite() {
		return false
	}
	personID, err := h.Accounts.WhoConnected(heard.DID)
	if err != nil || personID == 0 {
		return false
	}
	if heard.Kind != "commit" || heard.Commit == nil || heard.Commit.Collection != bsky.PostCollection {
		return true
	}
	account, err := h.Accounts.Read(personID)
	if err != nil || account == nil {
		return true
	}

	if heard.Commit.Operation == "delete" {
		err = h.ownRemoved(account, heard.Commit.RKey)
	} else {
		post := &bsky.Post{}
		if json.Unmarshal(heard.Commit.Record, post) != nil {
			return true
		}
		err = h.ownPosted(account, heard.Commit.RKey, post)
	}
	if err != nil {
		log.Printf("heard a post of %s, connected by %d: %v", account.Handle, personID, err)
	}
	return true
}
