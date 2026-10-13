package handlers

import (
	"context"
	"fmt"
	"log"
	"strings"

	"activitypub/ap"
	"activitypub/remote"

	"anahita-libs/data/entities"
	"anahita-libs/event"
)

// Likes across servers.
//
// A like here is an edge from whoever likes to what is liked, made through
// like-service and said on the bus. Between servers it is a `Like`, taken
// back with an `Undo` that quotes it.
//
// Going out: somebody on the fediverse likes a note that came from another
// server, and its author's server is told. A like by somebody who is not
// on the fediverse stays here, as their reply does.
//
// Coming in: a `Like` of a note here that went out is kept as a like by
// the person kept for whoever sent it, counted, and its author told, as
// for a like made here.

// likeID is what a like is called between servers: made from who liked and
// what, so that taking it back can name it again with nothing kept.
func (h *Handlers) likeID(liker, liked *entities.AnahitaNode) string {
	return h.addresses.Activity("like-" + liker.UUID + "-" + liked.UUID)
}

// likedElsewhere reads the two ends of a like made here, and says whether
// it is one other servers are told of: by somebody here who is on the
// fediverse, of a note kept from another server.
func (h *Handlers) likedElsewhere(likerID, nodeID int64) (liker, liked *entities.AnahitaNode, inbox string, err error) {
	liked, err = h.Nodes.Read(nodeID)
	if err != nil || liked == nil || liked.ID == 0 || !liked.IsRemote() || !isPost(liked) {
		return nil, nil, "", err
	}
	liker, err = h.Nodes.Read(likerID)
	if err != nil || liker == nil || liker.IsRemote() || h.federating(liker) == nil {
		return nil, nil, "", err
	}
	if h.Blocks != nil && h.Blocks.IsBlocked(hostOf(liked.URI)) {
		return nil, nil, "", nil
	}
	kept, err := h.Actors.ReadByNode(liked.OwnerID)
	if err != nil || kept == nil {
		return nil, nil, "", err
	}

	inbox = kept.SharedInbox
	if inbox == "" {
		inbox = kept.Inbox
	}
	return liker, liked, inbox, nil
}

// localLiked is somebody here having liked something: when it came from
// another server, that server is told.
func (h *Handlers) localLiked(ctx context.Context, likerID, nodeID int64) error {
	liker, liked, inbox, err := h.likedElsewhere(likerID, nodeID)
	if err != nil || liker == nil {
		return err
	}

	id := h.likeID(liker, liked)
	return h.send(ctx, liker.ID, inbox, ap.NewLike(id, h.addresses.Actor(liker.UUID), liked.URI), "like:"+id)
}

// localUnliked is somebody here having taken a like back.
func (h *Handlers) localUnliked(ctx context.Context, likerID, nodeID int64) error {
	liker, liked, inbox, err := h.likedElsewhere(likerID, nodeID)
	if err != nil || liker == nil {
		return err
	}

	id := h.likeID(liker, liked)
	actor := h.addresses.Actor(liker.UUID)
	// A new name each time: liking, unliking and liking again are three
	// things that happened.
	undo := h.addresses.Activity(newUUID())
	return h.send(ctx, liker.ID, inbox, ap.NewUndoLike(undo, actor, liked.URI, id), "unlike:"+undo)
}

// liked is somebody elsewhere having liked something here.
func (h *Handlers) liked(ctx context.Context, from *remote.Actor, activity *ap.Activity) error {
	return h.likeFromElsewhere(ctx, from, activity.ObjectID(), true)
}

// unliked is somebody elsewhere having taken a like back.
func (h *Handlers) unliked(ctx context.Context, from *remote.Actor, inner *ap.Activity) error {
	if inner.ActorID() != "" && inner.ActorID() != from.ID {
		return nil
	}
	return h.likeFromElsewhere(ctx, from, inner.ObjectID(), false)
}

func (h *Handlers) likeFromElsewhere(ctx context.Context, from *remote.Actor, what string, on bool) error {
	// Only something of this installation's that other servers were told
	// of. A like of anything else is not this installation's to keep.
	note := h.answeredHere(what)
	if note == nil || note.IsRemote() {
		return nil
	}
	author, err := h.Nodes.Read(note.CreatedBy)
	if err != nil || author == nil || author.ID == 0 {
		return err
	}

	// Taking a like back is for somebody already known: nobody is looked
	// up to be told they never liked anything.
	var liker *entities.AnahitaNode
	if on {
		if liker, err = h.known(ctx, from); err != nil || liker == nil {
			return err
		}
	} else {
		kept, err := h.Actors.ReadByURI(from.ID)
		if err != nil || kept == nil {
			return err
		}
		if liker, err = h.Nodes.Read(kept.NodeID); err != nil || liker == nil || liker.ID == 0 {
			return err
		}
	}

	// Somebody the author has blocked does not like what the author wrote.
	if on {
		relation, err := h.Relations.Read(author.ID, liker.ID)
		if err != nil {
			return err
		}
		if relation != nil && relation.IsBlocking {
			return nil
		}
	}

	has, err := h.Likes.IsLiked(liker.ID, note.ID)
	if err != nil || has == on {
		// As it already is: said twice, or taken back twice.
		return err
	}

	viewer := entities.NewViewer()
	viewer.ID = liker.ID
	if on {
		err = h.Likes.Add(liker, note, viewer)
	} else {
		err = h.Likes.Delete(liker, note)
	}
	if err != nil {
		return err
	}

	// Said on the bus as like-service says it, so that the author is told
	// as for a like made here. Nothing else that hears it acts on a like
	// by somebody from another server: this service's own listener leaves
	// it, since it is not a like made here.
	h.announceLike(liker, note, author, on)
	return nil
}

func (h *Handlers) announceLike(liker, note, author *entities.AnahitaNode, on bool) {
	if h.Announce == nil {
		return
	}

	format := event.EventFormatEdgeVoteupDeleted
	if on {
		format = event.EventFormatEdgeVoteupAdded
	}
	// Named for the kind of thing liked, as like-service names it: "text"
	// for a note.
	kind := strings.Split(note.Service, "-")[0]
	if kind == "" {
		kind = "text"
	}
	name := fmt.Sprintf(format, kind)
	payload := &event.EventPayload{
		Name:          name,
		SourceService: "like-service",
		OwnerID:       author.ID,
		SubjectID:     liker.ID,
		ObjectID:      note.ID,
		TargetID:      author.ID,
		ViewerID:      liker.ID,
	}
	h.later(func() {
		if err := h.Announce(payload, name); err != nil {
			log.Printf("saying that %s liked %d: %v", liker.Alias, note.ID, err)
		}
	})
}
