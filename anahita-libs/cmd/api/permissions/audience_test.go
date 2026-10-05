package permissions

import (
	"fmt"
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

const (
	audAda   = 1 // whose profile it is
	audGrace = 2 // somebody else
	audGroup = 8
)

func audienceCtx(ownerType string, ownerID, authorID int64, viewer *entities.Viewer, adminsOwner bool) *MediumAccessContext {
	owner := entities.NewAnahitaNode()
	owner.ID, owner.Type = ownerID, ownerType

	medium := entities.NewAnahitaNode()
	medium.Owner, medium.OwnerID, medium.CreatedBy = owner, ownerID, authorID

	return &MediumAccessContext{
		Viewer:         viewer,
		Medium:         medium,
		OwnerViewerRel: &entities.ActorRelation{HasAdmin: adminsOwner},
	}
}

func audienceViewer(id int64, personType string) *entities.Viewer {
	viewer := entities.NewViewer()
	viewer.ID, viewer.PersonType = id, personType
	return viewer
}

func TestMediumAudiencesDependOnWhereThePostIs(t *testing.T) {
	member := constants.PersonTypeRegistered
	const person, group = constants.NodeActorPersonEntityType, "node.actor.group-service.group.v1"

	for _, tc := range []struct {
		name string
		ctx  *MediumAccessContext
		want string
	}{
		{
			"on your own profile",
			audienceCtx(person, audAda, audAda, audienceViewer(audAda, member), false),
			"[public registered followers leaders mutuals myself]",
		},
		{
			// "Only me" here would be a message only the two of them can see.
			"on somebody else's profile",
			audienceCtx(person, audAda, audGrace, audienceViewer(audGrace, member), false),
			"[public registered followers leaders mutuals]",
		},
		{
			// Keyed on the author: an administrator changing Grace's post on
			// Ada's profile cannot make it a private one either.
			"a site administrator changing that post",
			audienceCtx(person, audAda, audGrace, audienceViewer(99, constants.PersonTypeAdmin), false),
			"[public registered followers leaders mutuals]",
		},
		{
			"in a group, as a member",
			audienceCtx(group, audGroup, audGrace, audienceViewer(audGrace, member), false),
			"[public registered followers]",
		},
		{
			"in a group, as one of its administrators",
			audienceCtx(group, audGroup, audAda, audienceViewer(audAda, member), true),
			"[public registered followers admins]",
		},
		{
			"in a group, as a site administrator",
			audienceCtx(group, audGroup, audGrace, audienceViewer(99, constants.PersonTypeAdmin), false),
			"[public registered followers admins]",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := fmt.Sprint(MediumAudiences(tc.ctx)); got != tc.want {
				t.Errorf("audiences = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestAnAudienceNotSaidIsAllowed(t *testing.T) {
	ctx := audienceCtx("node.actor.group-service.group.v1", audGroup, audGrace,
		audienceViewer(audGrace, constants.PersonTypeRegistered), false)

	if !IsMediumAudienceAllowed(ctx, "") {
		t.Error("an empty audience was refused; it means the default")
	}
	if IsMediumAudienceAllowed(ctx, constants.AnahitaAccessMutuals) {
		t.Error("mutuals was allowed in a group, which has none")
	}
	if IsMediumAudienceAllowed(ctx, constants.AnahitaAccessAdmins) {
		t.Error("a member addressed a post to a group's administrators alone")
	}

	// No relation known is no relation.
	ctx.OwnerViewerRel = nil
	if IsMediumAudienceAllowed(ctx, constants.AnahitaAccessAdmins) {
		t.Error("admins was allowed with no relation to the group")
	}
}
