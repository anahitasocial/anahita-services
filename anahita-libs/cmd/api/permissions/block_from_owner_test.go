package permissions

import (
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

func TestWhoBlocksFromAGroup(t *testing.T) {
	group := ownerWith(ownerID, "public")
	group.Type = "node.actor.group-service.group.v1"
	person := ownerWith(ownerID, "public")
	person.Type = "node.actor.person-service.person.v1"

	// A signed-in person whose session may block.
	may := func(id int64, personType string) *entities.Viewer {
		return replyViewer(id, personType, constants.OAuthScopeSocialgraphWrite)
	}

	var (
		nobody = &entities.ActorRelation{}
		admin  = &entities.ActorRelation{HasAdmin: true}
	)

	cases := []struct {
		name   string
		viewer *entities.Viewer
		owner  *entities.AnahitaNode
		author int64
		rel    *entities.ActorRelation
		want   bool
	}{
		{"an administrator of the group", may(bystanderID, constants.PersonTypeRegistered), group, authorID, admin, true},
		{"somebody who does not run it", may(bystanderID, constants.PersonTypeRegistered), group, authorID, nobody, false},
		{"a site administrator", may(bystanderID, constants.PersonTypeAdmin), group, authorID, nobody, true},
		{"an administrator, of what they wrote themselves", may(authorID, constants.PersonTypeRegistered), group, authorID, admin, false},
		{"what the group itself posted", may(bystanderID, constants.PersonTypeRegistered), group, ownerID, admin, false},
		// A person blocks as themselves, not as their profile.
		{"on a person's profile", may(ownerID, constants.PersonTypeRegistered), person, authorID, admin, false},
		{"a token that may not change follows and blocks", replyViewer(bystanderID, constants.PersonTypeRegistered, constants.OAuthScopeLikesWrite), group, authorID, admin, false},
		{"a visitor", entities.NewViewer(), group, authorID, nil, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CanBlockFromOwner(c.viewer, c.owner, c.author, c.rel); got != c.want {
				t.Errorf("CanBlockFromOwner = %v, want %v", got, c.want)
			}
		})
	}
}
