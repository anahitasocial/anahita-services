package handlers

import (
	"context"
	"log"
	"strings"
	"time"

	"activitypub/ap"
	"activitypub/data/repositories"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/event"
	"anahita-libs/videos"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// What happens here that another server has to be told.
//
// Somebody here follows somebody elsewhere the way they follow anybody: the
// same button, the same request to the same service. Nothing about that was
// changed for federation. What this does is hear that it happened, on the
// bus the services already talk over, and tell the other server.
//
// The bus can lose a message, and tells every copy of this service the same
// thing. So what is done here is safe to do twice (the follow is begun
// once, in the table), and what is lost is a follow the other server was
// never told of; that is put right by unfollowing and following again, and
// by a sweep that is still to be written.

// EventTopics is what this service listens for.
var EventTopics = []string{
	"edge.follow.*.*.v1",
	"edge.block.*.*.v1",
	"edge.voteup.*.*.v1",
	"node.repost.*.*.*.v1",
	"node.medium.*.*.*.v1",
	"node.actor.*.*.*.v1",
}

// HandleEvent is told of something that happened here.
func (h *Handlers) HandleEvent(payload *event.EventPayload) error {
	if ok, _ := h.siteFederates(); !ok {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var err error
	switch {
	case strings.HasPrefix(payload.Name, "edge.follow.") && strings.HasSuffix(payload.Name, ".added.v1"):
		err = h.localFollowed(ctx, payload.ObjectID, payload.TargetID)
	case strings.HasPrefix(payload.Name, "edge.follow.") && strings.HasSuffix(payload.Name, ".deleted.v1"):
		err = h.localUnfollowed(ctx, payload.ObjectID, payload.TargetID)

	// Somebody here blocked somebody elsewhere, or unblocked them.
	case strings.HasPrefix(payload.Name, "edge.block.") && strings.HasSuffix(payload.Name, ".added.v1"):
		err = h.localBlocked(ctx, payload.ObjectID, payload.TargetID)
	case strings.HasPrefix(payload.Name, "edge.block.") && strings.HasSuffix(payload.Name, ".deleted.v1"):
		err = h.localUnblocked(ctx, payload.ObjectID, payload.TargetID)

	// Somebody liked something, or took a like back. The payload names
	// who (the subject) and what (the object).
	case strings.HasPrefix(payload.Name, "edge.voteup.") && strings.HasSuffix(payload.Name, ".added.v1"):
		err = h.localLiked(ctx, payload.SubjectID, payload.ObjectID)
	case strings.HasPrefix(payload.Name, "edge.voteup.") && strings.HasSuffix(payload.Name, ".deleted.v1"):
		err = h.localUnliked(ctx, payload.SubjectID, payload.ObjectID)

	// Something reposted, or a repost taken back. Its node is gone by the
	// time the second is heard: what it was is in the record of what went.
	case strings.HasPrefix(payload.Name, "node.repost.") && strings.HasSuffix(payload.Name, ".added.v1"):
		err = h.reposted(ctx, payload.ObjectID)
	case strings.HasPrefix(payload.Name, "node.repost.") && strings.HasSuffix(payload.Name, ".deleted.v1"):
		err = h.removed(ctx, payload.ObjectID)

	// A person's profile edited, or their account disabled, archived,
	// deleted, purged, or brought back. What decides is asked of the person
	// as they are now.
	case strings.HasPrefix(payload.Name, "node.actor.") && !strings.HasSuffix(payload.Name, ".added.v1"):
		err = h.profileChanged(ctx, payload.ObjectID)

	// A post made, changed or deleted. Notes, photo posts and video posts
	// go out; the rest of what decides is asked of the post itself.
	case strings.HasPrefix(payload.Name, "node.medium.") &&
		(payload.ObjectType == constants.NodeMediumNoteEntityType || payload.ObjectType == constants.NodeMediumPhotoEntityType ||
			payload.ObjectType == videos.NodeType):
		if strings.HasSuffix(payload.Name, ".deleted.v1") {
			err = h.removed(ctx, payload.ObjectID)
		} else {
			err = h.posted(ctx, payload.ObjectID)
		}
	}
	if err != nil {
		log.Printf("event %s (%d > %d): %v", payload.Name, payload.ObjectID, payload.TargetID, err)
	}
	return err
}

// across reads the two ends of a follow, and says whether it is one this
// service has to do with: from somebody here to somebody kept from another
// server that speaks ActivityPub.
func (h *Handlers) across(sourceID, targetID int64) (source, target *entities.AnahitaNode, kept *repositories.RemoteActor, err error) {
	target, err = h.Nodes.Read(targetID)
	if err != nil || target == nil || target.Type != constants.NodeActorActivityPubPersonEntityType {
		return nil, nil, nil, err
	}
	source, err = h.Nodes.Read(sourceID)
	if err != nil || source == nil || source.IsRemote() {
		return nil, nil, nil, err
	}
	kept, err = h.Actors.ReadByNode(target.ID)
	if err != nil || kept == nil {
		return nil, nil, nil, err
	}
	return source, target, kept, nil
}

// localFollowed is somebody here having followed somebody elsewhere: the
// other server is asked.
//
// The follow is there already, as it is for anybody followed here, and what
// the other server answers settles it: yes leaves it, no takes it away.
// Until then it is kept as pending, which the web app can say.
func (h *Handlers) localFollowed(ctx context.Context, sourceID, targetID int64) error {
	source, target, kept, err := h.across(sourceID, targetID)
	if err != nil || source == nil {
		return err
	}

	// From the lab, somebody on a real server is looked at and not
	// followed: the request could not be checked there and would never be
	// answered. The follow is taken away again, so that it does not sit
	// here saying something that is not so.
	if h.Lab && !strings.HasSuffix(hostOf(kept.URI), ".test") {
		log.Printf("%s follows %s from the lab: real servers are not written to, and the follow is removed", source.Alias, target.Alias)
		viewer := entities.NewViewer()
		viewer.ID = source.ID
		remove := services.NewSocialgraphFollowRequest().Delete
		remove.Source, remove.Target, remove.Viewer = source, target, viewer
		return h.FollowEdges.Delete(remove)
	}

	// Only somebody who is on the fediverse can follow across it: the other
	// server has to be able to find out who is asking. Anybody else's
	// follow would wait for ever, so it is taken away again.
	if h.federating(source) == nil {
		log.Printf("%s follows %s and is not on the fediverse: the follow is removed", source.Alias, target.Alias)
		viewer := entities.NewViewer()
		viewer.ID = source.ID
		remove := services.NewSocialgraphFollowRequest().Delete
		remove.Source, remove.Target, remove.Viewer = source, target, viewer
		return h.FollowEdges.Delete(remove)
	}

	follow := &repositories.Follow{
		LocalID: source.ID, RemoteID: target.ID, Direction: repositories.FollowOut,
		ActivityID: h.addresses.Activity(newUUID()), State: repositories.FollowPending,
	}
	begun, err := h.Follows.Begin(follow)
	if err != nil || !begun {
		// Another copy of this service heard it first, or it is a follow
		// the other server already knows of.
		return err
	}

	actor := h.addresses.Actor(source.UUID)
	if err := h.send(ctx, source.ID, kept.Inbox, ap.NewFollow(follow.ActivityID, actor, kept.URI), "follow:"+follow.ActivityID); err != nil {
		return err
	}

	// What they posted lately, so that their profile here is not empty
	// until they next post. Not waited for.
	h.later(func() { h.backfill(kept) })
	return nil
}

// localUnfollowed is somebody here having stopped following somebody
// elsewhere: the other server is told, by quoting the follow back.
func (h *Handlers) localUnfollowed(ctx context.Context, sourceID, targetID int64) error {
	source, target, kept, err := h.across(sourceID, targetID)
	if err != nil || source == nil {
		return err
	}

	follow, err := h.Follows.Read(source.ID, target.ID, repositories.FollowOut)
	if err != nil || follow == nil {
		// Nothing the other server was told of: it was refused there, or
		// never sent.
		return err
	}
	if err := h.Follows.Delete(source.ID, target.ID, repositories.FollowOut); err != nil {
		return err
	}

	actor := h.addresses.Actor(source.UUID)
	return h.send(ctx, source.ID, kept.Inbox,
		ap.NewUndoFollow(h.addresses.Activity(newUUID()), actor, kept.URI, follow.ActivityID), "undo:"+follow.ActivityID)
}

// blockID is the name of the Block somebody here sends somebody elsewhere:
// the same for the same two, so that the Undo of it quotes it without its
// being kept anywhere.
func (h *Handlers) blockID(sourceUUID string, targetID int64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("block\n%s\n%d", sourceUUID, targetID)))
	return h.addresses.Activity("block-" + hex.EncodeToString(sum[:16]))
}

// localBlocked is somebody here having blocked somebody elsewhere. Any
// follow between the two is forgotten, both ways, so that nothing more is
// delivered to the blocked person's server: the graph ends the follows
// themselves without saying so. And for somebody on the fediverse, the
// other server is sent a Block, which Mastodon and what speaks to it keep
// to: the blocked person stops seeing their posts and answering them.
func (h *Handlers) localBlocked(ctx context.Context, sourceID, targetID int64) error {
	source, target, kept, err := h.across(sourceID, targetID)
	if err != nil || source == nil {
		return err
	}
	for _, direction := range []string{repositories.FollowIn, repositories.FollowOut} {
		if err := h.Follows.Delete(source.ID, target.ID, direction); err != nil {
			return err
		}
	}
	if h.federating(source) == nil {
		return nil
	}
	id := h.blockID(source.UUID, target.ID)
	return h.send(ctx, source.ID, kept.Inbox, ap.NewBlock(id, h.addresses.Actor(source.UUID), kept.URI), "block:"+id)
}

// localUnblocked is somebody here having unblocked somebody elsewhere: the
// Block is taken back. Follows ended by it are not made again.
func (h *Handlers) localUnblocked(ctx context.Context, sourceID, targetID int64) error {
	source, target, kept, err := h.across(sourceID, targetID)
	if err != nil || source == nil || h.federating(source) == nil {
		return err
	}
	id := h.blockID(source.UUID, target.ID)
	actor := h.addresses.Actor(source.UUID)
	return h.send(ctx, source.ID, kept.Inbox,
		ap.NewUndoBlock(h.addresses.Activity(newUUID()), actor, kept.URI, id), "unblock:"+newUUID())
}
