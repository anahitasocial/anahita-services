package permissions

import (
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

func personWhoLets(t *testing.T, access string) *entities.AnahitaNode {
	t.Helper()
	person := entities.NewAnahitaNode()
	person.ID = 7
	person.Type = constants.NodeActorPersonEntityType
	if access != "" {
		err := person.SetFeatures([]*entities.AnahitaNodeFeature{{
			Service: constants.MentionPermissionService,
			Enabled: true,
			AddPermissions: []entities.AnahitaNodeAddPermission{
				{Entity: constants.MentionPermission, Access: access},
			},
		}})
		if err != nil {
			t.Fatal(err)
		}
	}
	return person
}

func TestWhoMayMentionSomebody(t *testing.T) {
	author := entities.NewViewer()
	author.ID = 9
	author.Authenticated = true
	stranger := &entities.ActorRelation{}
	followed := &entities.ActorRelation{IsFollowing: true}

	if !CanMention(personWhoLets(t, ""), author, stranger) {
		t.Error("unset: anybody signed in may")
	}
	if CanMention(personWhoLets(t, "admins"), author, stranger) {
		t.Error("nobody may")
	}
	if CanMention(personWhoLets(t, "leaders"), author, stranger) {
		t.Error("only people they follow may, and they do not follow the author")
	}
	if !CanMention(personWhoLets(t, "leaders"), author, followed) {
		t.Error("they follow the author")
	}
	if CanMention(personWhoLets(t, ""), author, &entities.ActorRelation{IsBlocking: true}) {
		t.Error("they block the author")
	}

	self := personWhoLets(t, "admins")
	self.ID = author.ID
	if !CanMention(self, author, stranger) {
		t.Error("anybody may name themselves")
	}
	group := entities.NewAnahitaNode()
	group.Type = constants.NodeActorGroupEntityType
	if !CanMention(group, author, stranger) {
		t.Error("a group may always be named")
	}
}
