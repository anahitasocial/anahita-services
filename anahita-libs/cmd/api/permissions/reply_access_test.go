package permissions

import (
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// A public post on its author's own public profile, open to replies, with
// who may reply set by its author.
func limitedThread(replyAccess string, body string) *entities.AnahitaNode {
	root, _ := thread(ownerWith(rootAuthorID, "public"), "public", true)
	root.ReplyAccess = replyAccess
	root.Body = body

	return root
}

func namedViewer(id int64, username string) *entities.Viewer {
	v := replyViewer(id, constants.PersonTypeRegistered)
	v.Username = username
	return v
}

func TestTheAuthorChoosesWhoReplies(t *testing.T) {
	var (
		stranger     = &entities.ActorRelation{}
		followsThem  = &entities.ActorRelation{IsLeading: true}   // the viewer follows the author
		theyFollow   = &entities.ActorRelation{IsFollowing: true} // the author follows the viewer
		mentioning   = "Thanks @Sam and @alex-b for this"
		notMentioned = "Nobody is named here"
	)

	cases := []struct {
		name        string
		replyAccess string
		body        string
		username    string
		rel         *entities.ActorRelation
		want        bool
	}{
		{"nothing chosen, a stranger", "", notMentioned, "sam", stranger, true},
		{"anyone, a stranger", "anyone", notMentioned, "sam", stranger, true},
		{"nobody, a follower", "nobody", mentioning, "sam", followsThem, false},

		{"followers, a follower", "followers", notMentioned, "sam", followsThem, true},
		{"followers, somebody the author follows", "followers", notMentioned, "sam", theyFollow, false},
		{"followers, a stranger", "followers", notMentioned, "sam", stranger, false},

		{"following, somebody the author follows", "following", notMentioned, "sam", theyFollow, true},
		{"following, a follower", "following", notMentioned, "sam", followsThem, false},

		// Names are matched whatever their case, and whole.
		{"mentioned, named", "mentioned", mentioning, "sam", stranger, true},
		{"mentioned, named with a hyphen", "mentioned", mentioning, "Alex-B", stranger, true},
		{"mentioned, part of a name", "mentioned", mentioning, "alex", stranger, false},
		{"mentioned, not named", "mentioned", notMentioned, "sam", stranger, false},
		{"mentioned, a follower not named", "mentioned", notMentioned, "sam", followsThem, false},

		// Any one of the groups is enough.
		{"followers or mentioned, named only", "followers,mentioned", mentioning, "sam", stranger, true},
		{"followers or mentioned, follower only", "followers,mentioned", notMentioned, "sam", followsThem, true},
		{"followers or mentioned, neither", "followers,mentioned", notMentioned, "sam", stranger, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := limitedThread(c.replyAccess, c.body)
			ctx := ctxFor(namedViewer(bystanderID, c.username), root, nil, c.rel)
			ctx.RootAuthorViewerRel = c.rel

			if got := CanReply(ctx); got != c.want {
				t.Errorf("CanReply = %v, want %v", got, c.want)
			}
		})
	}
}

// Whoever looks after the thread replies whatever was chosen: the author
// under their own post, and a site administrator.
func TestThoseWhoLookAfterAThreadReplyWhateverWasChosen(t *testing.T) {
	for _, replyAccess := range []string{"nobody", "followers", "mentioned"} {
		root := limitedThread(replyAccess, "")

		author := ctxFor(namedViewer(rootAuthorID, "author"), root, nil, &entities.ActorRelation{})
		if !CanReply(author) {
			t.Errorf("%s: the author could not reply under their own post", replyAccess)
		}

		admin := replyViewer(bystanderID, constants.PersonTypeAdmin)
		if !CanReply(ctxFor(admin, root, nil, &entities.ActorRelation{})) {
			t.Errorf("%s: a site administrator could not reply", replyAccess)
		}
	}
}

// The choice narrows who may reply and never widens it. In a group that
// lets only its followers comment, being named in a post does not let
// somebody outside the group reply.
func TestBeingChosenDoesNotGetPastTheProfilesOwnRule(t *testing.T) {
	group := ownerWith(groupOwnerID, "public", perm("comment", "followers"))

	root, _ := thread(group, "public", true)
	root.ReplyAccess = "mentioned"
	root.Body = "What do you think @sam"

	outsider := ctxFor(namedViewer(bystanderID, "sam"), root, nil, &entities.ActorRelation{})
	outsider.RootAuthorViewerRel = &entities.ActorRelation{}
	if CanReply(outsider) {
		t.Error("somebody outside the group replied because the post names them")
	}

	insider := ctxFor(namedViewer(bystanderID, "sam"), root, nil, &entities.ActorRelation{IsLeading: true})
	insider.RootAuthorViewerRel = &entities.ActorRelation{}
	if !CanReply(insider) {
		t.Error("a follower of the group named in the post could not reply")
	}
}

// The older switch still closes a post, whatever the newer value says.
func TestAClosedSwitchReadsAsNobody(t *testing.T) {
	root := limitedThread("followers", "")
	root.CommentStatus = false

	if got := RepliesLimitedTo(root); got != constants.ReplyAccessNobody {
		t.Errorf("RepliesLimitedTo = %q, want nobody", got)
	}

	root.CommentStatus = true
	root.ReplyAccess = ""
	if got := RepliesLimitedTo(root); got != constants.ReplyAccessAnyone {
		t.Errorf("RepliesLimitedTo = %q, want anyone", got)
	}
}
