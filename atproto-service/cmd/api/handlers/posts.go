package handlers

import (
	"log"

	"atproto/bsky"
	"atproto/data/repositories"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/event"
	"anahita-libs/federation"
)

// The posts of people on Bluesky.
//
// A post is kept only when somebody here follows whoever wrote it: this
// installation is not a copy of Bluesky, and what nobody here asked for is
// not stored. Everything on Bluesky is open to everybody, so there is no
// asking who a post is for.
//
// A reply is kept when what it answers is kept here, and is put under it:
// somebody followed saying a long thing in several posts, or answering
// somebody else who is followed. One that answers something not kept
// here is left: it would stand without what it answers.
//
// It is kept the way a post from the fediverse is. With pictures it is a
// photo post, its pictures copied into this installation's storage; a
// link in it is given a card read by this installation itself; its
// hashtags are this installation's own.

// The most a kept post may be. A Bluesky post is three hundred letters,
// and the links in it somewhat more.
const postMax = 20000

// keepPost keeps one post of somebody on Bluesky, if it is one that is
// kept. The same rule whether it was heard as it was written or read from
// the list of what they have posted.
func (h *Handlers) keepPost(author *repositories.RemoteActor, rkey string, post *bsky.Post) error {
	if author == nil || post == nil || !bsky.ValidRkey(rkey) {
		return nil
	}
	if h.refused(author.Handle) || h.refusedAccount(author.DID) {
		return nil
	}
	followed, err := h.Actors.IsFollowedFromHere(author.NodeID)
	if err != nil || !followed {
		return err
	}

	// What it answers, when it answers something: a post from Bluesky
	// that is kept here.
	var parentID int64
	if post.IsReply() {
		parent, err := h.Nodes.ReadByURI(post.Answers())
		if err != nil || parent == nil || parent.ID == 0 {
			return err
		}
		parentID = parent.ID
	}
	_, err = h.storePost(author, rkey, post, parentID)
	return err
}

// storePost keeps a post that has been found to be one that is kept: on
// its author's profile, and under what it answers when it answers
// something. It answers with the post as it is kept, or nil when there
// was nothing of it to keep.
func (h *Handlers) storePost(author *repositories.RemoteActor, rkey string, post *bsky.Post, parentID int64) (*entities.AnahitaNode, error) {
	uri := bsky.PostURI(author.DID, rkey)

	// What its author marked as not to be shown plainly is kept without
	// its pictures: there is nothing here yet that puts a picture behind
	// a warning.
	var pictures []federation.Picture
	if h.Keeper != nil && h.Keeper.CanKeepPictures() && !post.IsSensitive() {
		for _, each := range post.Pictures() {
			if len(pictures) == federation.PostPicturesMax() {
				break
			}
			pictures = append(pictures, federation.Picture{
				URL: bsky.PictureURL(h.CDN, author.DID, each.CID),
				Alt: each.Alt,
			})
		}
	}

	// With pictures it is a photo post. What it was first kept as it
	// stays, whatever came after.
	kind := constants.NodeMediumNoteEntityType
	if known, err := h.Nodes.ReadByURI(uri); err == nil && known != nil && known.ID != 0 {
		kind = known.Type
	} else if len(pictures) > 0 {
		kind = constants.NodeMediumPhotoEntityType
	}

	body := federation.PostToMarkdown(post.HTML())
	// A post it quotes is led to: the quoted post is not kept here.
	if quoted := post.Quoted(); quoted != "" {
		if body != "" {
			body += "\n\n"
		}
		body += "↪ [bsky.app](" + quoted + ")"
	}
	body = cutRunes(body, postMax)
	if body == "" && kind != constants.NodeMediumPhotoEntityType {
		// Nothing to show: no words, and no picture that is kept.
		return nil, nil
	}

	node := entities.NewAnahitaNode()
	node.Type = kind
	node.URI = uri
	node.OriginDomain = originDomain
	node.OwnerID = author.NodeID
	node.ParentID = parentID
	node.Body = body
	node.Language = post.Language()
	node.CreatedAt = post.Created(h.now())
	// The card of the link it carries, or of the first one in its words,
	// read by this installation itself.
	node.Meta = federation.CardMeta(h.Cards, body, post.Link())

	saved, created, err := h.Nodes.UpsertRemote(node)
	if err != nil {
		return nil, err
	}
	h.announce(event.RemoteNoteEvent(created), saved, author.NodeID)

	// Its pictures are fetched after it is kept, and not waited for.
	if saved != nil && saved.Type == constants.NodeMediumPhotoEntityType && len(pictures) > 0 {
		h.later(func() { h.Keeper.CopyPostPictures(saved, pictures) })
	}
	return saved, nil
}

// removePost is somebody on Bluesky having taken a post away.
func (h *Handlers) removePost(author *repositories.RemoteActor, rkey string) error {
	if author == nil || !bsky.ValidRkey(rkey) {
		return nil
	}
	uri := bsky.PostURI(author.DID, rkey)

	// What it was here, to say afterwards that it is gone, and what
	// pictures of it are in storage.
	was, _ := h.Nodes.ReadByURI(uri)
	var pictures []*entities.PhotoFile
	if was != nil && was.Type == constants.NodeMediumPhotoEntityType && h.Keeper != nil && h.Keeper.PhotoFiles != nil {
		pictures, _ = h.Keeper.PhotoFiles.BrowsePhotoFiles([]int64{was.ID})
	}

	// The graph takes it away only if it is theirs.
	deleted, err := h.Nodes.DeleteRemote(uri, author.NodeID)
	if err != nil || !deleted || was == nil || was.ID == 0 {
		return err
	}
	if len(pictures) > 0 {
		h.later(func() { h.Keeper.DeletePostPictures(was, pictures) })
	}
	h.announce(event.RemoteNoteDeleted, was, author.NodeID)
	return nil
}

// announce says on the bus that a post from Bluesky was kept, changed or
// taken away: so that it is given its hashtags, as a post written here
// is, and so that nobody is left with news of one that is gone.
//
// Not waited for, and not tried again: a post whose hashtags were missed
// is still kept and shown.
func (h *Handlers) announce(name string, post *entities.AnahitaNode, authorID int64) {
	if h.Announce == nil || post == nil || post.ID == 0 || authorID == 0 {
		return
	}

	payload := &event.EventPayload{
		Name:          name,
		SourceService: "atproto-service",
		OwnerID:       authorID,
		OwnerType:     constants.NodeActorATProtoPersonEntityType,
		SubjectID:     authorID,
		SubjectType:   constants.NodeActorATProtoPersonEntityType,
		ObjectID:      post.ID,
		ObjectType:    post.Type,
		// Whoever wrote it is who a hashtag it is the first to use is put
		// down to.
		ViewerID: authorID,
	}

	h.later(func() {
		if err := h.Announce(payload, name); err != nil {
			log.Printf("saying %s of %s: %v", name, post.URI, err)
		}
	})
}

// forget removes somebody on Bluesky and everything of theirs, because
// their account was closed there.
func (h *Handlers) forget(kept *repositories.RemoteActor) error {
	if node, err := h.Nodes.Read(kept.NodeID); err == nil && h.Keeper != nil {
		h.Keeper.DeleteActorPictures(node)
	}
	// Their photo posts, with their pictures in storage, which the graph's
	// removal of them would leave.
	h.removePhotoPosts(kept.NodeID)
	if _, err := h.Nodes.DeleteRemoteActor(bsky.ActorURI(kept.DID)); err != nil {
		return err
	}
	if err := h.Actors.Delete(kept.NodeID); err != nil {
		return err
	}
	// The other services are told, as of a purge: the reports about them
	// are closed, and what was chosen about them goes.
	h.announceGone(kept.NodeID)
	h.changed()
	return nil
}
