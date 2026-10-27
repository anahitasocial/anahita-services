package permissions

import (
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

const (
	groupOwnerID  = 1  // the profile the thread is on
	rootAuthorID  = 2  // wrote the post at the top
	replyAuthorID = 3  // wrote the reply
	bystanderID   = 50 // nobody in particular
)

// thread is a post by rootAuthorID on owner's profile, and a reply to it by
// replyAuthorID. The reply is stored as replies are: owned by its author,
// with no audience of its own.
func thread(owner *entities.AnahitaNode, rootAccess string, open bool) (*entities.AnahitaNode, *entities.AnahitaNode) {
	root := entities.NewAnahitaNode()
	root.ID = 10
	root.Type = "node.medium.text-service.note.v1"
	root.Service = "text-service"
	root.Access = rootAccess
	root.OwnerID = owner.ID
	root.Owner = owner
	root.CreatedBy = rootAuthorID
	root.CommentStatus = open

	reply := entities.NewAnahitaNode()
	reply.ID = 20
	reply.Type = root.Type
	reply.Service = root.Service
	reply.OwnerID = replyAuthorID
	reply.CreatedBy = replyAuthorID
	reply.ParentID = root.ID
	reply.RootID = root.ID

	return root, reply
}

func replyViewer(id int64, personType string, scopes ...string) *entities.Viewer {
	if len(scopes) == 0 {
		scopes = []string{constants.OAuthScopeMediaWrite, constants.OAuthScopeLikesWrite}
	}
	v := viewerWith(personType, scopes...)
	v.ID = id
	return v
}

func ctxFor(viewer *entities.Viewer, root, reply *entities.AnahitaNode, rel *entities.ActorRelation) *ReplyAccessContext {
	return &ReplyAccessContext{Viewer: viewer, Root: root, Reply: reply, RootOwnerViewerRel: rel}
}

func TestWhoCanReply(t *testing.T) {
	stranger := &entities.ActorRelation{}
	follower := &entities.ActorRelation{IsLeading: true}
	admin := &entities.ActorRelation{HasAdmin: true}

	publicProfile := ownerWith(groupOwnerID, "public")
	followersToReply := ownerWith(groupOwnerID, "public", perm("comment", "followers"))
	privateGroup := ownerWith(groupOwnerID, "followers")

	cases := []struct {
		name   string
		owner  *entities.AnahitaNode
		access string
		open   bool
		viewer *entities.Viewer
		rel    *entities.ActorRelation
		want   bool
	}{
		{"anybody signed in, on a public post", publicProfile, "public", true,
			replyViewer(bystanderID, constants.PersonTypeRegistered), stranger, true},
		{"only followers, where the profile says so", followersToReply, "public", true,
			replyViewer(bystanderID, constants.PersonTypeRegistered), stranger, false},
		{"a follower, where the profile says followers", followersToReply, "public", true,
			replyViewer(bystanderID, constants.PersonTypeRegistered), follower, true},
		{"nobody outside a private group", privateGroup, "public", true,
			replyViewer(bystanderID, constants.PersonTypeRegistered), stranger, false},
		{"a follower of a private group", privateGroup, "public", true,
			replyViewer(bystanderID, constants.PersonTypeRegistered), follower, true},
		{"nobody who cannot see the post", publicProfile, "myself", true,
			replyViewer(bystanderID, constants.PersonTypeRegistered), follower, false},

		// The switch for comments, closed.
		{"nobody else once replies are closed", publicProfile, "public", false,
			replyViewer(bystanderID, constants.PersonTypeRegistered), follower, false},
		{"the author of the post, even when closed", publicProfile, "public", false,
			replyViewer(rootAuthorID, constants.PersonTypeRegistered), stranger, true},
		{"the profile it is on, even when closed", publicProfile, "public", false,
			replyViewer(groupOwnerID, constants.PersonTypeRegistered), stranger, true},
		{"an administrator of the group, even when closed", publicProfile, "public", false,
			replyViewer(bystanderID, constants.PersonTypeRegistered), admin, true},

		// A reply is a post, so it needs the scope for writing posts.
		{"not a token that cannot write posts", publicProfile, "public", true,
			replyViewer(bystanderID, constants.PersonTypeRegistered, constants.OAuthScopeLikesWrite), stranger, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root, _ := thread(c.owner, c.access, c.open)
			if got := CanReply(ctxFor(c.viewer, root, nil, c.rel)); got != c.want {
				t.Errorf("CanReply = %v, want %v", got, c.want)
			}
		})
	}

	// Nobody signed out, and nothing without the root loaded.
	root, _ := thread(publicProfile, "public", true)
	guest := entities.NewViewer()
	if CanReply(ctxFor(guest, root, nil, stranger)) {
		t.Error("a visitor may reply")
	}
	root.Owner = entities.NewAnahitaNode()
	if CanReply(ctxFor(replyViewer(bystanderID, constants.PersonTypeRegistered), root, nil, stranger)) {
		t.Error("a reply was allowed with the profile it is on not loaded")
	}
}

// A post from another server is open to reply here, and says so: its switch
// for replies is never set, and is not read as closed.
func TestAPostFromElsewhereIsOpenToReply(t *testing.T) {
	root, _ := thread(ownerWith(groupOwnerID, "public"), "public", false)
	root.URI = "at://did:plc:example/app.bsky.feed.post/1"

	if got := RepliesLimitedTo(root); got != constants.ReplyAccessAnyone {
		t.Errorf("RepliesLimitedTo = %q, want anyone", got)
	}
	viewer := replyViewer(bystanderID, constants.PersonTypeRegistered)
	if !CanReply(ctxFor(viewer, root, nil, &entities.ActorRelation{})) {
		t.Error("CanReply = false, want true")
	}

	root.URI = ""
	if got := RepliesLimitedTo(root); got != constants.ReplyAccessNobody {
		t.Errorf("a post here with replies off: RepliesLimitedTo = %q, want nobody", got)
	}
}

// The case the rules exist for. The reply's author has a public profile and
// the thread is in a private group. Judged as a post the reply would be
// readable by anybody; judged by its root it is not.
func TestAReplyIsSeenByItsRootNotItsAuthor(t *testing.T) {
	privateGroup := ownerWith(groupOwnerID, "followers")
	root, reply := thread(privateGroup, "public", true)

	outsider := replyViewer(bystanderID, constants.PersonTypeRegistered)
	if CanSeeThread(ctxFor(outsider, root, reply, &entities.ActorRelation{})) {
		t.Error("somebody outside a private group can see a reply in it")
	}
	if !CanSeeThread(ctxFor(outsider, root, reply, &entities.ActorRelation{IsLeading: true})) {
		t.Error("a follower of the group cannot see a reply in it")
	}
}

func TestOnlyItsAuthorEditsAReplyAndOnlyWhileTheyCanSeeTheThread(t *testing.T) {
	publicProfile := ownerWith(groupOwnerID, "public")
	stranger := &entities.ActorRelation{}
	root, reply := thread(publicProfile, "public", true)

	if !CanEditReply(ctxFor(replyViewer(replyAuthorID, constants.PersonTypeRegistered), root, reply, stranger)) {
		t.Error("the author cannot edit their own reply")
	}

	// Nobody who looks after the thread may put words under another name.
	for who, viewer := range map[string]*entities.Viewer{
		"the author of the post": replyViewer(rootAuthorID, constants.PersonTypeRegistered),
		"the profile it is on":   replyViewer(groupOwnerID, constants.PersonTypeRegistered),
		"a site administrator":   replyViewer(bystanderID, constants.PersonTypeAdmin),
		"somebody else":          replyViewer(bystanderID, constants.PersonTypeRegistered),
	} {
		if CanEditReply(ctxFor(viewer, root, reply, &entities.ActorRelation{HasAdmin: true})) {
			t.Errorf("%s can edit somebody else's reply", who)
		}
	}

	// The post has since been made private: its author can no longer edit
	// what they said under it.
	root.Access = "myself"
	if CanEditReply(ctxFor(replyViewer(replyAuthorID, constants.PersonTypeRegistered), root, reply, stranger)) {
		t.Error("the author can edit a reply in a thread they can no longer see")
	}
}

func TestWhoRemovesAndWhoHidesAReply(t *testing.T) {
	// A post by one person in somebody else's group: the three people who
	// look after the thread are all different from each other and from the
	// reply's author.
	group := ownerWith(groupOwnerID, "public")
	root, reply := thread(group, "public", true)

	stranger := &entities.ActorRelation{}
	admin := &entities.ActorRelation{HasAdmin: true}

	cases := []struct {
		who        string
		viewer     *entities.Viewer
		rel        *entities.ActorRelation
		wantDelete bool
		wantHide   bool
	}{
		{"the author of the reply", replyViewer(replyAuthorID, constants.PersonTypeRegistered), stranger, true, false},
		// The addition: somebody who posts in a group is not one of its
		// administrators, and can still deal with what is said under their
		// own post.
		{"the author of the post", replyViewer(rootAuthorID, constants.PersonTypeRegistered), stranger, true, true},
		{"the profile the post is on", replyViewer(groupOwnerID, constants.PersonTypeRegistered), stranger, true, true},
		{"an administrator of the group", replyViewer(bystanderID, constants.PersonTypeRegistered), admin, true, true},
		{"a site administrator", replyViewer(bystanderID, constants.PersonTypeAdmin), stranger, true, true},
		{"somebody else", replyViewer(bystanderID, constants.PersonTypeRegistered), stranger, false, false},
		{"a follower of the group", replyViewer(bystanderID, constants.PersonTypeRegistered), &entities.ActorRelation{IsLeading: true}, false, false},
		{"a visitor", entities.NewViewer(), stranger, false, false},
	}

	for _, c := range cases {
		t.Run(c.who, func(t *testing.T) {
			ctx := ctxFor(c.viewer, root, reply, c.rel)
			if got := CanDeleteReply(ctx); got != c.wantDelete {
				t.Errorf("CanDeleteReply = %v, want %v", got, c.wantDelete)
			}
			if got := CanHideReply(ctx); got != c.wantHide {
				t.Errorf("CanHideReply = %v, want %v", got, c.wantHide)
			}
		})
	}

	// Taking your own words down works after you have lost sight of the
	// thread.
	root.Access = "myself"
	if !CanDeleteReply(ctxFor(replyViewer(replyAuthorID, constants.PersonTypeRegistered), root, reply, stranger)) {
		t.Error("the author cannot remove their reply from a thread they can no longer see")
	}
}

// A reply is liked under the rule for its root, on the root's profile.
func TestAReplyIsLikedUnderTheRuleForItsRoot(t *testing.T) {
	followersLike := ownerWith(groupOwnerID, "public", perm("like", "followers"))
	root, reply := thread(followersLike, "public", true)

	viewer := replyViewer(bystanderID, constants.PersonTypeRegistered)
	if CanLikeReply(ctxFor(viewer, root, reply, &entities.ActorRelation{})) {
		t.Error("a stranger can like a reply where only followers may like")
	}
	if !CanLikeReply(ctxFor(viewer, root, reply, &entities.ActorRelation{IsLeading: true})) {
		t.Error("a follower cannot like a reply where followers may like")
	}
}
