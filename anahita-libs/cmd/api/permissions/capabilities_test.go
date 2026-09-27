package permissions

import (
	"reflect"
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// profileWith is a person profile with text-service composing notes and
// articles, and the given add_permissions.
func profileWith(access string, perms ...entities.AnahitaNodeAddPermission) *entities.AnahitaNode {
	n := entities.NewAnahitaNode()
	n.ID = ownerID
	n.Type = "node.actor.person-service.person.v1"
	n.Access = access
	_ = n.SetFeatures([]*entities.AnahitaNodeFeature{
		{
			Service:        "text-service",
			Enabled:        true,
			Composers:      []string{"note", "article"},
			AddPermissions: perms,
		},
		{
			Service:   "photo-service",
			Enabled:   false,
			Composers: []string{"photo"},
			AddPermissions: []entities.AnahitaNodeAddPermission{
				perm("photo", "followers"),
			},
		},
	})
	return n
}

func viewerAs(id int64, scopes ...string) *entities.Viewer {
	v := viewerWith(constants.PersonTypeRegistered, scopes...)
	v.ID = id
	return v
}

func TestComposers(t *testing.T) {
	owner := profileWith("public",
		perm("note", "followers"),
		perm("article", "admins"),
		perm("like", "followers"),
	)
	follower := &entities.ActorRelation{IsLeading: true}
	media := constants.OAuthScopeMediaWrite

	cases := []struct {
		name   string
		viewer *entities.Viewer
		rel    *entities.ActorRelation
		want   []string
	}{
		// The web app used to read any permission in the service — here the
		// like one — as the article's, and offer the article composer.
		{"a follower gets only what each composer allows", viewerAs(100, media), follower, []string{"note"}},
		{"a stranger gets nothing", viewerAs(100, media), &entities.ActorRelation{}, []string{}},
		// photo-service is switched off, so its composer is never offered.
		{"the owner gets every enabled composer", viewerAs(ownerID, media), nil, []string{"note", "article"}},
		{"an admin of the profile gets every one", viewerAs(100, media), &entities.ActorRelation{HasAdmin: true}, []string{"note", "article"}},
		{"no media:write scope, nothing", viewerAs(ownerID), nil, []string{}},
	}

	for _, c := range cases {
		if got := Composers(c.viewer, owner, c.rel); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}

	// Leaders and mutuals, which the web app did not know about at all.
	mutualsOnly := profileWith("public", perm("note", "mutuals"), perm("article", "leaders"))
	mutual := &entities.ActorRelation{IsLeading: true, IsFollowing: true, IsMutual: true}
	if got := Composers(viewerAs(100, media), mutualsOnly, mutual); !reflect.DeepEqual(got, []string{"note", "article"}) {
		t.Errorf("mutual: got %v", got)
	}
	if got := Composers(viewerAs(100, media), mutualsOnly, follower); !reflect.DeepEqual(got, []string{}) {
		t.Errorf("one-way follower on mutuals/leaders: got %v", got)
	}
}

func postOn(owner *entities.AnahitaNode, access string) *entities.AnahitaNode {
	post := entities.NewAnahitaNode()
	post.ID = 10
	post.Type = "node.medium.text-service.note.v1"
	post.Service = "text-service"
	post.Access = access
	post.OwnerID = owner.ID
	post.Owner = owner
	post.CreatedBy = authorID
	return post
}

func TestCanLike(t *testing.T) {
	owner := profileWith("public", perm("like", "followers"))
	likes := constants.OAuthScopeLikesWrite
	follower := &entities.ActorRelation{IsLeading: true}

	cases := []struct {
		name   string
		viewer *entities.Viewer
		post   *entities.AnahitaNode
		rel    *entities.ActorRelation
		want   bool
	}{
		{"a follower", viewerAs(100, likes), postOn(owner, "public"), follower, true},
		{"a stranger", viewerAs(100, likes), postOn(owner, "public"), &entities.ActorRelation{}, false},
		{"the owner", viewerAs(ownerID, likes), postOn(owner, "public"), nil, true},
		{"a follower who cannot read the post", viewerAs(100, likes), postOn(owner, "admins"), follower, false},
		{"no likes:write scope", viewerAs(100), postOn(owner, "public"), follower, false},
		{"a guest", entities.NewViewer(), postOn(owner, "public"), nil, false},
		{"an owner that was never loaded", viewerAs(100, likes), postOn(entities.NewAnahitaNode(), "public"), follower, false},
	}

	for _, c := range cases {
		if got := CanLike(c.viewer, c.post, c.rel); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

// CanComment is CanAdd asked of a comment that does not exist yet, so only
// its own additions need pinning here: the guest refusal, and that it passes
// the post through.
func TestCanComment(t *testing.T) {
	owner := profileWith("public", perm("comment", "followers"))
	comments := constants.OAuthScopeCommentsWrite

	if !CanComment(viewerAs(100, comments), postOn(owner, "public"), &entities.ActorRelation{IsLeading: true}) {
		t.Error("refused a follower on followers-only comments")
	}
	if CanComment(viewerAs(100, comments), postOn(owner, "public"), &entities.ActorRelation{}) {
		t.Error("allowed a stranger on followers-only comments")
	}
	// CanAdd lets guests past its scope check, relying on the route; the
	// answer shown to a guest has to be no.
	if CanComment(entities.NewViewer(), postOn(profileWith("public"), "public"), nil) {
		t.Error("allowed a guest")
	}
}

func groupWith(access string, followerAccess string) *entities.AnahitaNode {
	n := entities.NewAnahitaNode()
	n.ID = 50
	n.Type = "node.actor.group-service.group.v1"
	n.Access = access
	_ = n.SetFeatures([]*entities.AnahitaNodeFeature{{
		Service:        "socialgraph-service",
		Enabled:        true,
		AddPermissions: []entities.AnahitaNodeAddPermission{perm("follower", followerAccess)},
	}})
	return n
}

func TestCanAddOthersAsFollowers(t *testing.T) {
	follower := &entities.ActorRelation{IsLeading: true}
	admin := &entities.ActorRelation{HasAdmin: true}
	siteAdmin := viewerWith(constants.PersonTypeAdmin)

	cases := []struct {
		name   string
		viewer *entities.Viewer
		group  *entities.AnahitaNode
		rel    *entities.ActorRelation
		want   bool
	}{
		{"admins: a group admin", viewerAs(100), groupWith("public", "admins"), admin, true},
		{"admins: a follower", viewerAs(100), groupWith("public", "admins"), follower, false},
		{"followers: a follower", viewerAs(100), groupWith("public", "followers"), follower, true},
		{"followers: a stranger", viewerAs(100), groupWith("public", "followers"), &entities.ActorRelation{}, false},
		{"a site administrator", siteAdmin, groupWith("public", "admins"), nil, true},
		{"a guest", entities.NewViewer(), groupWith("public", "followers"), nil, false},
		{"never a person", viewerAs(100), profileWith("public"), admin, false},
	}

	for _, c := range cases {
		if got := CanAddOthersAsFollowers(c.viewer, c.group, c.rel); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
