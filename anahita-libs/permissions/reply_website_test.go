package permissions

import (
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// Under a website's own post, its administrators reply as the website
// (docs/plans/done/website.md); anywhere else, and anybody else, as themselves.
func TestAWebsitesAdministratorRepliesAsIt(t *testing.T) {
	website := ownerWith(groupOwnerID, "public")
	website.Type = constants.NodeActorWebsiteEntityType
	root, _ := thread(website, "public", true)
	admin := &entities.ActorRelation{HasAdmin: true}
	stranger := &entities.ActorRelation{}

	if as := RepliesAs(ctxFor(replyViewer(replyAuthorID, constants.PersonTypeRegistered), root, nil, admin)); as == nil || as.ID != groupOwnerID {
		t.Errorf("an administrator replies as %v, want the Website", as)
	}
	for who, ctx := range map[string]*ReplyAccessContext{
		"somebody else":        ctxFor(replyViewer(bystanderID, constants.PersonTypeRegistered), root, nil, stranger),
		"a site administrator": ctxFor(replyViewer(bystanderID, constants.PersonTypeAdmin), root, nil, stranger),
	} {
		if RepliesAs(ctx) != nil {
			t.Errorf("%s replies as the Website", who)
		}
	}

	group := ownerWith(groupOwnerID, "public")
	group.Type = constants.NodeActorGroupEntityType
	groupRoot, _ := thread(group, "public", true)
	if RepliesAs(ctxFor(replyViewer(replyAuthorID, constants.PersonTypeRegistered), groupRoot, nil, admin)) != nil {
		t.Error("a group's administrator replies as the group")
	}
}

func TestTheWebsitesRepliesAreChangedByThoseWhoReplyAsIt(t *testing.T) {
	website := ownerWith(groupOwnerID, "public")
	website.Type = constants.NodeActorWebsiteEntityType
	root, reply := thread(website, "public", true)
	reply.OwnerID, reply.CreatedBy = groupOwnerID, groupOwnerID

	if !CanEditReply(ctxFor(replyViewer(replyAuthorID, constants.PersonTypeRegistered), root, reply, &entities.ActorRelation{HasAdmin: true})) {
		t.Error("an administrator cannot change the Website's reply")
	}
	if CanEditReply(ctxFor(replyViewer(bystanderID, constants.PersonTypeRegistered), root, reply, &entities.ActorRelation{})) {
		t.Error("somebody else can change the Website's reply")
	}

	// Somebody's own reply under the website's post is still theirs alone.
	_, theirs := thread(website, "public", true)
	if CanEditReply(ctxFor(replyViewer(rootAuthorID, constants.PersonTypeRegistered), root, theirs, &entities.ActorRelation{HasAdmin: true})) {
		t.Error("an administrator can change somebody else's reply")
	}
}

// A website's own notes follow its site; the replies under them, its own
// included, are replies like any.
func TestAWebsitesOwnNoteFollowsItsSite(t *testing.T) {
	item := entities.NewAnahitaNode()
	item.OwnerType = constants.NodeActorWebsiteEntityType
	if !IsWebsiteItem(item) {
		t.Error("a website's own note can be changed")
	}

	reply := entities.NewAnahitaNode()
	reply.OwnerType, reply.ParentID, reply.RootID = constants.NodeActorWebsiteEntityType, 10, 10
	if IsWebsiteItem(reply) {
		t.Error("a website's reply cannot be changed by whoever replies as it")
	}

	someone := entities.NewAnahitaNode()
	someone.OwnerType = constants.NodeActorPersonEntityType
	if IsWebsiteItem(someone) {
		t.Error("somebody's own note follows a website")
	}
}
