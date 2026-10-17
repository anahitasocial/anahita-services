package permissions

import (
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// Under a Pulse's own post, its administrators reply as the Pulse
// (docs/plans/pulse.md); anywhere else, and anybody else, as themselves.
func TestAPulsesAdministratorRepliesAsIt(t *testing.T) {
	pulse := ownerWith(groupOwnerID, "public")
	pulse.Type = constants.NodeActorPulseEntityType
	root, _ := thread(pulse, "public", true)
	admin := &entities.ActorRelation{HasAdmin: true}
	stranger := &entities.ActorRelation{}

	if as := RepliesAs(ctxFor(replyViewer(replyAuthorID, constants.PersonTypeRegistered), root, nil, admin)); as == nil || as.ID != groupOwnerID {
		t.Errorf("an administrator replies as %v, want the Pulse", as)
	}
	for who, ctx := range map[string]*ReplyAccessContext{
		"somebody else":        ctxFor(replyViewer(bystanderID, constants.PersonTypeRegistered), root, nil, stranger),
		"a site administrator": ctxFor(replyViewer(bystanderID, constants.PersonTypeAdmin), root, nil, stranger),
	} {
		if RepliesAs(ctx) != nil {
			t.Errorf("%s replies as the Pulse", who)
		}
	}

	group := ownerWith(groupOwnerID, "public")
	group.Type = constants.NodeActorGroupEntityType
	groupRoot, _ := thread(group, "public", true)
	if RepliesAs(ctxFor(replyViewer(replyAuthorID, constants.PersonTypeRegistered), groupRoot, nil, admin)) != nil {
		t.Error("a group's administrator replies as the group")
	}
}

func TestThePulsesRepliesAreChangedByThoseWhoReplyAsIt(t *testing.T) {
	pulse := ownerWith(groupOwnerID, "public")
	pulse.Type = constants.NodeActorPulseEntityType
	root, reply := thread(pulse, "public", true)
	reply.OwnerID, reply.CreatedBy = groupOwnerID, groupOwnerID

	if !CanEditReply(ctxFor(replyViewer(replyAuthorID, constants.PersonTypeRegistered), root, reply, &entities.ActorRelation{HasAdmin: true})) {
		t.Error("an administrator cannot change the Pulse's reply")
	}
	if CanEditReply(ctxFor(replyViewer(bystanderID, constants.PersonTypeRegistered), root, reply, &entities.ActorRelation{})) {
		t.Error("somebody else can change the Pulse's reply")
	}

	// Somebody's own reply under the Pulse's post is still theirs alone.
	_, theirs := thread(pulse, "public", true)
	if CanEditReply(ctxFor(replyViewer(rootAuthorID, constants.PersonTypeRegistered), root, theirs, &entities.ActorRelation{HasAdmin: true})) {
		t.Error("an administrator can change somebody else's reply")
	}
}
