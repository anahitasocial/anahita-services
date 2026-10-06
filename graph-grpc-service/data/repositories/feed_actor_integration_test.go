package repositories

import (
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
)

// A profile's three lists against a real database, on the fixture the thread
// tests use.
//
//	ACCESS_TEST_DSN="root@tcp(127.0.0.1:13309)/fresh?parseTime=true" \
//	  go test ./data/repositories/ -run TestAProfiles -v
//
// THE RULE THIS GUARDS: a profile's list of replies shows a reply to exactly
// the people who can see the post at the top of its thread. The list is of
// one person's replies, taken from threads all over the site, so it is the
// easiest place there is to show somebody what was said in a private group.

func profileList(t *testing.T, repo FeedActorRepoInterface, actor int64, filter string, viewer int64) map[int64]bool {
	t.Helper()

	req := &graphpb.FeedActorBrowseRequest{
		ActorId:           actor,
		Filter:            filter,
		MediumEntityTypes: []string{noteType, "node.medium.photo-service.photo.v1"},
		IncludeReposts:    true,
		Limit:             50,
	}

	nodes, err := repo.Browse(req, replyViewer(viewer))
	seen := nodeIDs(t, nodes, err)

	// The count is a second statement over the same rows, and has to agree.
	total, err := repo.Count(&graphpb.FeedActorCountRequest{
		ActorId:           req.ActorId,
		Filter:            req.Filter,
		MediumEntityTypes: req.MediumEntityTypes,
		IncludeReposts:    req.IncludeReposts,
	}, replyViewer(viewer))
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if int(total) != len(seen) {
		t.Errorf("counted %d, listed %d", total, len(seen))
	}

	return seen
}

func TestAProfilesRepliesAreShownOnlyToThoseWhoCanSeeTheirThreads(t *testing.T) {
	db := accessTestDB(t)
	seedRepliesFixture(t, db)
	repo := NewFeedActorRepo(db)

	cases := []struct {
		what   string
		actor  int64
		viewer int64
		want   []int64
	}{
		// Alice replied in the private group and under her own "only me"
		// note. Her profile is public, and neither reply is anybody's to see
		// who cannot see the thread it is in.
		{"Alice's, to somebody outside the group", replyAlice, replyBob, nil},
		{"Alice's, to a visitor", replyAlice, 0, nil},
		{"Alice's, to a follower of the group", replyAlice, replyCarol, []int64{replyInGroup}},
		{"Alice's, to Alice", replyAlice, replyAlice, []int64{replyInGroup, replyOnlyMe}},

		// Carol's reply in the group, and not the one that was removed.
		{"Carol's, to Carol", replyCarol, replyCarol, []int64{replyNested}},
		{"Carol's, to somebody outside the group", replyCarol, replyBob, nil},

		// Bob's are under a public post: anybody's to see.
		{"Bob's, to a visitor", replyBob, 0, []int64{replyByBob, replyToGone}},

		// Alice blocked Dave.
		{"Dave's, to somebody he is blocked by", replyDave, replyAlice, nil},
		{"Dave's, to anybody else", replyDave, replyBob, []int64{replyByDave}},
	}

	for _, c := range cases {
		t.Run(c.what, func(t *testing.T) {
			got := profileList(t, repo, c.actor, constants.FeedFilterReplies, c.viewer)
			if !sameSet(got, c.want...) {
				t.Errorf("lists %v, want %v", sortedIDs(got), c.want)
			}
		})
	}
}

// A reply hidden from its thread is hidden for everybody. Its author's
// profile is not a way round that.
func TestAHiddenReplyLeavesItsAuthorsProfileToo(t *testing.T) {
	db := accessTestDB(t)
	seedRepliesFixture(t, db)
	repo := NewFeedActorRepo(db)

	dave := &entities.AnahitaNode{}
	dave.ID = replyByDave
	dave.Type = noteType

	if err := NewRepliesRepo(db).SetHidden(rootPublic, dave, true, replyAlice); err != nil {
		t.Fatal(err)
	}

	for _, viewer := range []int64{replyBob, replyDave, 0} {
		if got := profileList(t, repo, replyDave, constants.FeedFilterReplies, viewer); len(got) != 0 {
			t.Errorf("viewer %d is listed %v", viewer, sortedIDs(got))
		}
	}
}

// The other two lists hold no replies, and the list of replies holds nothing
// else.
func TestAProfilesPostsAndRepostsHoldNoReplies(t *testing.T) {
	db := accessTestDB(t)
	seedRepliesFixture(t, db)
	repo := NewFeedActorRepo(db)

	// Alice's own profile: her public note, her "only me" note and her
	// photo. Her note in the group is the group's.
	posts := profileList(t, repo, replyAlice, constants.FeedFilterPosts, replyAlice)
	if !sameSet(posts, rootPublic, rootOnlyMe, rootPhoto) {
		t.Errorf("posts: %v", sortedIDs(posts))
	}

	// Asked for nothing in particular, it is the posts.
	if unnamed := profileList(t, repo, replyAlice, "", replyAlice); !sameSet(unnamed, rootPublic, rootOnlyMe, rootPhoto) {
		t.Errorf("with no filter: %v", sortedIDs(unnamed))
	}

	if reposts := profileList(t, repo, replyAlice, constants.FeedFilterReposts, replyAlice); len(reposts) != 0 {
		t.Errorf("reposts: %v", sortedIDs(reposts))
	}

	// A repost is in the list of reposts and, when asked for, among the
	// posts. It is never a reply.
	if _, err := db.Exec(`INSERT INTO nodes (id,type,service,alias,access,enabled,owner_id,owner_type,parent_id,created_by,created_at,updated_at) VALUES
		(400,'node.base.repost-service.repost.v1','feed-service','n101','public',1,2,'node.actor.person-service.person.v1',101,2,NOW(),NOW())`); err != nil {
		t.Fatal(err)
	}

	if reposts := profileList(t, repo, replyBob, constants.FeedFilterReposts, 0); !sameSet(reposts, 400) {
		t.Errorf("Bob's reposts: %v", sortedIDs(reposts))
	}
	if posts := profileList(t, repo, replyBob, constants.FeedFilterPosts, 0); !sameSet(posts, 400) {
		t.Errorf("Bob's posts, with his repost: %v", sortedIDs(posts))
	}
	if replies := profileList(t, repo, replyBob, constants.FeedFilterReplies, 0); !sameSet(replies, replyByBob, replyToGone) {
		t.Errorf("Bob's replies: %v", sortedIDs(replies))
	}
}
