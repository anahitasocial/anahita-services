package permissions

import (
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// Under a channel's own post, its administrators reply as the channel
// (docs/plans/done/channel.md); anywhere else, and anybody else, as themselves.
func TestAChannelsAdministratorRepliesAsIt(t *testing.T) {
	channel := ownerWith(groupOwnerID, "public")
	channel.Type = constants.NodeActorChannelEntityType
	root, _ := thread(channel, "public", true)
	admin := &entities.ActorRelation{HasAdmin: true}
	stranger := &entities.ActorRelation{}

	if as := RepliesAs(ctxFor(replyViewer(replyAuthorID, constants.PersonTypeRegistered), root, nil, admin)); as == nil || as.ID != groupOwnerID {
		t.Errorf("an administrator replies as %v, want the Channel", as)
	}
	for who, ctx := range map[string]*ReplyAccessContext{
		"somebody else":        ctxFor(replyViewer(bystanderID, constants.PersonTypeRegistered), root, nil, stranger),
		"a site administrator": ctxFor(replyViewer(bystanderID, constants.PersonTypeAdmin), root, nil, stranger),
	} {
		if RepliesAs(ctx) != nil {
			t.Errorf("%s replies as the Channel", who)
		}
	}

	group := ownerWith(groupOwnerID, "public")
	group.Type = constants.NodeActorGroupEntityType
	groupRoot, _ := thread(group, "public", true)
	if RepliesAs(ctxFor(replyViewer(replyAuthorID, constants.PersonTypeRegistered), groupRoot, nil, admin)) != nil {
		t.Error("a group's administrator replies as the group")
	}
}

func TestTheChannelsRepliesAreChangedByThoseWhoReplyAsIt(t *testing.T) {
	channel := ownerWith(groupOwnerID, "public")
	channel.Type = constants.NodeActorChannelEntityType
	root, reply := thread(channel, "public", true)
	reply.OwnerID, reply.CreatedBy = groupOwnerID, groupOwnerID

	if !CanEditReply(ctxFor(replyViewer(replyAuthorID, constants.PersonTypeRegistered), root, reply, &entities.ActorRelation{HasAdmin: true})) {
		t.Error("an administrator cannot change the Channel's reply")
	}
	if CanEditReply(ctxFor(replyViewer(bystanderID, constants.PersonTypeRegistered), root, reply, &entities.ActorRelation{})) {
		t.Error("somebody else can change the Channel's reply")
	}

	// Somebody's own reply under the channel's post is still theirs alone.
	_, theirs := thread(channel, "public", true)
	if CanEditReply(ctxFor(replyViewer(rootAuthorID, constants.PersonTypeRegistered), root, theirs, &entities.ActorRelation{HasAdmin: true})) {
		t.Error("an administrator can change somebody else's reply")
	}
}

// A channel's notes made of its feed follow its site; what its admins write
// as it, and the replies under either, are theirs to change.
func TestAChannelsOwnNoteFollowsItsSite(t *testing.T) {
	item := entities.NewAnahitaNode()
	item.OwnerType = constants.NodeActorChannelEntityType
	item.Meta = WithFromFeed([]byte(`{"written_by":7}`))
	if !IsChannelFeedItem(item) {
		t.Error("a note made of a channel's feed can be changed")
	}
	if string(item.Meta) != `{"from_feed":true,"written_by":7}` {
		t.Errorf("the mark is not kept beside what was there: %s", item.Meta)
	}

	written := entities.NewAnahitaNode()
	written.OwnerType = constants.NodeActorChannelEntityType
	written.Meta = []byte(`{"written_by":7}`)
	if IsChannelFeedItem(written) {
		t.Error("what a channel's admins write as it cannot be changed")
	}

	reply := entities.NewAnahitaNode()
	reply.OwnerType, reply.ParentID, reply.RootID = constants.NodeActorChannelEntityType, 10, 10
	if IsChannelFeedItem(reply) {
		t.Error("a channel's reply cannot be changed by whoever replies as it")
	}

	someone := entities.NewAnahitaNode()
	someone.OwnerType = constants.NodeActorPersonEntityType
	if IsChannelFeedItem(someone) {
		t.Error("somebody's own note follows a channel")
	}
}
