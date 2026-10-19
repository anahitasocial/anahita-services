package permissions

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/policy"
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
			"in a group, as one of its followers",
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
		t.Error("a follower addressed a post to a group's administrators alone")
	}

	// No relation known is no relation.
	ctx.OwnerViewerRel = nil
	if IsMediumAudienceAllowed(ctx, constants.AnahitaAccessAdmins) {
		t.Error("admins was allowed with no relation to the group")
	}
}

// What a profile's response carries for the composer: the same rule, asked
// before the post exists.
func TestPostAudiencesAreMediumAudiencesForANewPost(t *testing.T) {
	const person, group = constants.NodeActorPersonEntityType, "node.actor.group-service.group.v1"

	node := func(id int64, nodeType string) *entities.AnahitaNode {
		n := entities.NewAnahitaNode()
		n.ID, n.Type = id, nodeType
		return n
	}
	member := audienceViewer(audGrace, constants.PersonTypeRegistered)

	for _, tc := range []struct {
		name string
		got  []string
		want string
	}{
		{"your own profile", PostAudiences(member, node(audGrace, person), nil),
			"[public registered followers leaders mutuals myself]"},
		{"somebody else's", PostAudiences(member, node(audAda, person), nil),
			"[public registered followers leaders mutuals]"},
		{"a group you are in", PostAudiences(member, node(audGroup, group), &entities.ActorRelation{}),
			"[public registered followers]"},
		{"a group you administer", PostAudiences(member, node(audGroup, group), &entities.ActorRelation{HasAdmin: true}),
			"[public registered followers admins]"},
		{"not signed in", PostAudiences(entities.NewViewer(), node(audAda, person), nil), "[]"},
	} {
		if got := fmt.Sprint(tc.got); got != tc.want {
			t.Errorf("%s: %s, want %s", tc.name, got, tc.want)
		}
	}
}

// An installation that allows nothing new to be public takes "public" out
// everywhere, for a new post and for one being changed.
func TestNoPublicAudienceWhereTheInstallationAllowsNone(t *testing.T) {
	restore := policy.SetForTest("registered")
	defer restore()

	const person, group = constants.NodeActorPersonEntityType, "node.actor.group-service.group.v1"
	member := audienceViewer(audAda, constants.PersonTypeRegistered)

	own := audienceCtx(person, audAda, audAda, member, false)
	if got := fmt.Sprint(MediumAudiences(own)); got != "[registered followers leaders mutuals myself]" {
		t.Errorf("on your own profile: %s", got)
	}
	if IsMediumAudienceAllowed(own, constants.AnahitaAccessPublic) {
		t.Error("a public post was allowed")
	}

	inGroup := audienceCtx(group, audGroup, audAda, member, true)
	if got := fmt.Sprint(MediumAudiences(inGroup)); got != "[registered followers admins]" {
		t.Errorf("in a group: %s", got)
	}

	// Not even a site administrator: the setting is the installation's.
	admin := audienceCtx(group, audGroup, audGrace, audienceViewer(99, constants.PersonTypeSuperAdmin), false)
	if IsMediumAudienceAllowed(admin, constants.AnahitaAccessPublic) {
		t.Error("a super administrator made a public post")
	}
}

// Changing a post's audience: the same rule as creating it, except that what
// it already has may stay.
func TestAPostMayKeepTheAudienceItHas(t *testing.T) {
	restore := policy.SetForTest("registered")
	defer restore()

	member := audienceViewer(audAda, constants.PersonTypeRegistered)
	ctx := audienceCtx(constants.NodeActorPersonEntityType, audAda, audAda, member, false)

	for _, tc := range []struct {
		current, requested string
		want               bool
	}{
		{"public", "public", true},     // it was public before the rule
		{"public", "followers", true},  // narrowed
		{"followers", "public", false}, // may not become public
		{"followers", "admins", false}, // never allowed on a person's profile
		{"admins", "admins", true},     // an old value nobody could choose now, kept
		{"myself", "registered", true},
	} {
		if got := CanChangeMediumAudience(ctx, tc.current, tc.requested); got != tc.want {
			t.Errorf("from %q to %q: %v, want %v", tc.current, tc.requested, got, tc.want)
		}
	}
}

// The rule is only worth having if every handler that sets an audience asks
// it. These read the handlers, the way the schema tests read the schema: a
// handler that creates a post, or changes who sees one, without the check
// would accept any of the seven levels anywhere, and nothing else would say
// so.
func TestEveryHandlerThatSetsAnAudienceChecksIt(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")

	read := func(path string) string {
		t.Helper()
		source, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		return string(source)
	}

	// Creating a post.
	for _, path := range []string{
		"text-service/cmd/api/handlers/note.go",
		"text-service/cmd/api/handlers/article.go",
		"text-service/cmd/api/handlers/topic.go",
		"photo-service/cmd/api/handlers/photo.go",
	} {
		source := read(path)
		add := strings.Index(source, ".CanAdd(mediumAccessCtx)")
		check := strings.Index(source, "permissions.IsMediumAudienceAllowed(mediumAccessCtx, req.Access)")
		// Saved with its locations, where the composer sends them (roadmap
		// row 82, part 3b), or without.
		save := strings.Index(source, "MediaGRPC.Add(")
		if withLocations := strings.Index(source, "MediaGRPC.AddWithLocations("); save == -1 || withLocations != -1 && withLocations < save {
			save = withLocations
		}

		if add == -1 || save == -1 {
			t.Fatalf("%s: could not find where a post is created; has it been rewritten?", path)
		}
		if check == -1 || check < add || check > save {
			t.Errorf("%s creates a post without checking its audience between "+
				"the permission check and the save", path)
		}
	}

	// Changing who sees one.
	for _, path := range []string{
		"text-service/cmd/api/handlers/medium_access.go",
		"photo-service/cmd/api/handlers/photo_access.go",
	} {
		source := read(path)
		check := strings.Index(source, "permissions.CanChangeMediumAudience(mediumAccessCtx, medium.Access, req.Access)")
		save := strings.Index(source, "MediaGRPC.Edit(")

		if save == -1 {
			t.Fatalf("%s: could not find where the change is saved", path)
		}
		if check == -1 || check > save {
			t.Errorf("%s changes a post's audience without checking it first", path)
		}
	}

	// Changing who sees a profile or a group.
	for path, current := range map[string]string{
		"actor-service/cmd/api/handlers/person_access.go": "person.Access",
		"actor-service/cmd/api/handlers/group_access.go":  "actor.Access",
	} {
		source := read(path)
		check := strings.Index(source, "policy.MayChangeAccess("+current+", req.Access)")
		set := strings.Index(source, current+" = an_handlers.Determine")

		if set == -1 {
			t.Fatalf("%s: could not find where the access is set", path)
		}
		if check == -1 || check > set {
			t.Errorf("%s sets a profile's access without asking the installation's "+
				"policy first", path)
		}
	}

	// A new account's profile. Four places make one, and each must take its
	// access from the policy, not from a constant.
	for _, path := range []string{
		"auth-service/cmd/api/handlers/signup.go",
		"auth-service/cmd/api/handlers/signup_requests.go",
		"auth-service/cmd/api/handlers/invite_accept.go",
		"auth-service/data/services/super_admin_seed.go",
	} {
		source := read(path)
		if !strings.Contains(source, "actor.Access = policy.MinContentAccess()") {
			t.Errorf("%s creates an account whose profile does not take its "+
				"access from the installation's policy", path)
		}
		if strings.Contains(source, "actor.Access = constants.AnahitaAccessPublic") ||
			strings.Contains(source, "actor.Access = libs_constants.AnahitaAccessPublic") {
			t.Errorf("%s creates a public profile whatever the installation allows", path)
		}
	}
}
