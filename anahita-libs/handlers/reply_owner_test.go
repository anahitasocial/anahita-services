package handlers

import (
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/permissions"
)

func TestAReplyUnderAChannelsPostIsTheChannels(t *testing.T) {
	channel := entities.NewAnahitaNode()
	channel.ID, channel.Type = 900, constants.NodeActorChannelEntityType
	root := entities.NewAnahitaNode()
	root.ID, root.OwnerID, root.Owner = 10, 900, channel

	viewer := entities.NewViewer()
	viewer.ID, viewer.Authenticated = 7, true
	thread := &permissions.ReplyAccessContext{Viewer: viewer, Root: root, RootOwnerViewerRel: &entities.ActorRelation{HasAdmin: true}}

	reply := entities.NewAnahitaNode()
	reply.Meta = []byte(`{"link_preview":{"url":"https://example.com"}}`)
	OwnReply(thread, reply, viewer)
	if reply.OwnerID != 900 || reply.OwnerType != constants.NodeActorChannelEntityType || reply.CreatedBy != 900 || reply.ModifiedBy != 7 {
		t.Errorf("the reply: owner %d (%s), author %d, changed by %d", reply.OwnerID, reply.OwnerType, reply.CreatedBy, reply.ModifiedBy)
	}
	if WrittenBy(reply.Meta) != 7 || len(reply.Meta) < 40 {
		t.Errorf("who wrote it is not kept beside what was there: %s", reply.Meta)
	}

	thread.RootOwnerViewerRel = &entities.ActorRelation{}
	theirs := entities.NewAnahitaNode()
	OwnReply(thread, theirs, viewer)
	if theirs.OwnerID != 7 || theirs.OwnerType != constants.NodeActorPersonEntityType || theirs.CreatedBy != 7 || WrittenBy(theirs.Meta) != 0 {
		t.Errorf("somebody else's reply: %+v", theirs)
	}
}

func TestAPostOnAChannelIsTheChannels(t *testing.T) {
	post := entities.NewAnahitaNode()
	post.OwnerID, post.OwnerType, post.CreatedBy = 900, constants.NodeActorChannelEntityType, 7
	post.Meta = []byte(`{"link_preview":{"url":"https://example.com"}}`)
	OwnPost(post, 7)
	if post.CreatedBy != 900 || post.ModifiedBy != 7 || WrittenBy(post.Meta) != 7 || len(post.Meta) < 40 {
		t.Errorf("the post: author %d, changed by %d, meta %s", post.CreatedBy, post.ModifiedBy, post.Meta)
	}

	mine := entities.NewAnahitaNode()
	mine.OwnerID, mine.OwnerType, mine.CreatedBy = 7, constants.NodeActorPersonEntityType, 7
	OwnPost(mine, 7)
	if mine.CreatedBy != 7 || WrittenBy(mine.Meta) != 0 {
		t.Errorf("a person's own post was changed: author %d, meta %s", mine.CreatedBy, mine.Meta)
	}

	reply := entities.NewAnahitaNode()
	reply.OwnerID, reply.OwnerType, reply.CreatedBy, reply.ParentID = 900, constants.NodeActorChannelEntityType, 900, 10
	OwnPost(reply, 7)
	if WrittenBy(reply.Meta) != 0 {
		t.Errorf("a reply is OwnReply's, not OwnPost's: %s", reply.Meta)
	}
}

func TestWhoWroteAChannelsPostIsShownToItsAdmins(t *testing.T) {
	post := entities.NewAnahitaNode()
	post.OwnerID, post.OwnerType = 900, constants.NodeActorChannelEntityType
	post.Meta = WithWrittenBy(nil, 7)

	admin := entities.NewViewer()
	admin.ID, admin.Authenticated = 7, true
	follower := entities.NewViewer()
	follower.ID, follower.Authenticated = 8, true

	if got := WriterShown(admin, post, &entities.ActorRelation{HasAdmin: true}); got != 7 {
		t.Errorf("the channel's admin is not shown who wrote it: %d", got)
	}
	if got := WriterShown(follower, post, &entities.ActorRelation{IsLeading: true}); got != 0 {
		t.Errorf("a follower is shown who wrote it: %d", got)
	}
	if got := WriterShown(entities.NewViewer(), post, nil); got != 0 {
		t.Errorf("somebody signed out is shown who wrote it: %d", got)
	}
	mine := entities.NewAnahitaNode()
	mine.OwnerID, mine.OwnerType = 7, constants.NodeActorPersonEntityType
	mine.Meta = WithWrittenBy(nil, 7)
	if got := WriterShown(admin, mine, &entities.ActorRelation{HasAdmin: true}); got != 0 {
		t.Errorf("a post not a channel's names a writer: %d", got)
	}
}
