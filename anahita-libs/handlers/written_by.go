package handlers

import (
	"log"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/permissions"
)

// Who wrote a channel's post (roadmap row 147): the channel is its owner and
// author, and the admin who wrote it is kept in its meta (WithWrittenBy).
// The channel's admins see "Written by" under it, and the site's
// administrators do; nobody else is told.

// WriterShown is the admin who wrote post as its channel, when the viewer
// may know: one of the channel's admins (rel, the channel's relationship to
// the viewer), or a site administrator. 0 otherwise.
func WriterShown(viewer *entities.Viewer, post *entities.AnahitaNode, rel *entities.ActorRelation) int64 {
	if viewer == nil || viewer.ID <= 0 || post == nil || post.OwnerType != constants.NodeActorChannelEntityType {
		return 0
	}
	if !viewer.IsAdmin() && (rel == nil || !rel.HasAdmin) {
		return 0
	}
	return WrittenBy(post.Meta)
}

// SetWrittenBy gives each post the viewer may know the writer of its
// WrittenBy, every writer read once. relOf is the post's owner's
// relationship to the viewer. A failure to read is logged, and nobody is
// named.
func SetWrittenBy(actors services.ActorsGRPCInterface, posts []*entities.AnahitaNode, viewer *entities.Viewer, relOf func(*entities.AnahitaNode) *entities.ActorRelation) {
	if actors == nil || viewer == nil || viewer.ID <= 0 {
		return
	}

	writers := map[*entities.AnahitaNode]int64{}
	ids := []int64{}
	seen := map[int64]bool{}
	for _, post := range posts {
		if post == nil {
			continue
		}
		id := WriterShown(viewer, post, relOf(post))
		if id <= 0 {
			continue
		}
		writers[post] = id
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return
	}

	req := services.NewActorGRPCRequest().BatchRead
	req.IDs = ids
	read, err := actors.BatchRead(req, viewer)
	if err != nil {
		log.Println("who wrote a channel's posts:", err)
		return
	}
	byID := make(map[int64]*entities.AnahitaNode, len(read))
	for _, person := range read {
		if person != nil {
			byID[person.ID] = person
		}
	}
	for post, id := range writers {
		if person, ok := byID[id]; ok {
			post.WrittenBy = person
		}
	}
}

// SameRel answers rel for every post: for a list whose posts are all on one
// profile, or a thread.
func SameRel(rel *entities.ActorRelation) func(*entities.AnahitaNode) *entities.ActorRelation {
	return func(*entities.AnahitaNode) *entities.ActorRelation { return rel }
}

// SetThreadWrittenBy is SetWrittenBy for replies in thread: a reply made as
// the channel is on the profile the thread is on, whose relationship to the
// viewer the thread has read already.
func SetThreadWrittenBy(actors services.ActorsGRPCInterface, thread *permissions.ReplyAccessContext, replies []*entities.AnahitaNode) {
	if thread == nil || thread.Root == nil {
		return
	}
	SetWrittenBy(actors, replies, thread.Viewer, func(reply *entities.AnahitaNode) *entities.ActorRelation {
		if reply.OwnerID != thread.Root.OwnerID {
			return nil
		}
		return thread.RootOwnerViewerRel
	})
}
