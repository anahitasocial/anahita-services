package handlers

import (
	"testing"

	"anahita-libs/cmd/api/permissions"
	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

func TestAReplyUnderAWebsitesPostIsTheWebsites(t *testing.T) {
	website := entities.NewAnahitaNode()
	website.ID, website.Type = 900, constants.NodeActorWebsiteEntityType
	root := entities.NewAnahitaNode()
	root.ID, root.OwnerID, root.Owner = 10, 900, website

	viewer := entities.NewViewer()
	viewer.ID, viewer.Authenticated = 7, true
	thread := &permissions.ReplyAccessContext{Viewer: viewer, Root: root, RootOwnerViewerRel: &entities.ActorRelation{HasAdmin: true}}

	reply := entities.NewAnahitaNode()
	reply.Meta = []byte(`{"link_preview":{"url":"https://example.com"}}`)
	OwnReply(thread, reply, viewer)
	if reply.OwnerID != 900 || reply.OwnerType != constants.NodeActorWebsiteEntityType || reply.CreatedBy != 900 || reply.ModifiedBy != 7 {
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
