package repositories

import (
	"database/sql"
	"os"
	"testing"

	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"

	sq "github.com/Masterminds/squirrel"
	_ "github.com/go-sql-driver/mysql"
)

// Behavioural access-control checks against a real database.
//
// Skipped unless ACCESS_TEST_DSN is set, because they need MySQL and a loaded
// schema. The shape tests in access_test.go run everywhere and catch the
// predicate going missing; these catch it being present and WRONG, which is
// the failure that actually happened.
//
// To run:
//
//	docker run -d --name acl -e MYSQL_ALLOW_EMPTY_PASSWORD=1 \
//	  -e MYSQL_DATABASE=anahita -p 13309:3306 mysql:8
//	docker exec -i acl mysql -u root anahita < migration-job/schema/init.sql
//	ACCESS_TEST_DSN="root:@tcp(127.0.0.1:13309)/anahita?parseTime=true" \
//	  go test ./data/repositories/ -run TestAccessAgainstRealData -v

const (
	aliceID = 1
	bobID   = 2
	groupID = 3
)

func accessTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("ACCESS_TEST_DSN")
	if dsn == "" {
		t.Skip("set ACCESS_TEST_DSN to run access-control tests against MySQL")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
	return db
}

// seedAccessFixture builds the smallest graph that distinguishes the bugs:
// a blocked person posting both on their own profile and into a shared group,
// plus content of the viewer's own in that same group.
func seedAccessFixture(t *testing.T, db *sql.DB) {
	t.Helper()

	for _, stmt := range []string{
		`DELETE FROM edges`,
		`DELETE FROM nodes`,
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,created_at,updated_at) VALUES
		 (1,'node.actor.person-service.person.v1','person-service','Alice','alice','public',1,NOW(),NOW()),
		 (2,'node.actor.person-service.person.v1','person-service','Bob','bob','public',1,NOW(),NOW()),
		 (3,'node.actor.group-service.group.v1','group-service','Cyclists','cyclists','public',1,NOW(),NOW())`,
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,owner_id,created_by,created_at,updated_at) VALUES
		 (10,'node.medium.photo-service.photo.v1','photo-service','Bob photo','p1','public',1,2,2,NOW(),NOW()),
		 (11,'node.medium.photo-service.photo.v1','photo-service','Bob in group','p2','public',1,3,2,NOW(),NOW()),
		 (12,'node.medium.photo-service.photo.v1','photo-service','Alice in group','p3','public',1,3,1,NOW(),NOW()),
		 (20,'node.medium.photo-service.photo.v1','photo-service','Alice private','p4','myself',1,1,1,NOW(),NOW())`,
		`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,created_at) VALUES
		 ('edge.actor.block.v1',1,'node.actor.person-service.person.v1',2,'node.actor.person-service.person.v1',NOW())`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}
}

func visibleMedia(t *testing.T, db *sql.DB, viewer *entities.Viewer) map[int64]bool {
	t.Helper()

	base := sq.Select("node.id").From("nodes AS node").
		LeftJoin("nodes AS owner ON node.owner_id = owner.id").
		Where(sq.Eq{"node.type": "node.medium.photo-service.photo.v1"})

	query, args, err := SetMediumAccessQuery(base, viewer).ToSql()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	visible := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		visible[id] = true
	}
	return visible
}

func TestAccessAgainstRealData(t *testing.T) {
	db := accessTestDB(t)
	seedAccessFixture(t, db)

	alice := entities.NewViewer()
	alice.ID = aliceID
	alice.PersonType = "registered"

	visible := visibleMedia(t, db, alice)

	cases := []struct {
		id   int64
		want bool
		what string
	}{
		// Blocking has to reach content, not just the profile.
		{10, false, "a blocked person's photo on their own profile"},
		// The case that matters most: the row's OWNER is the group, so only
		// created_by names the blocked author.
		{11, false, "a blocked person's photo posted into a shared group"},
		// And it must not over-block: the viewer's own content in that same
		// group is unaffected.
		{12, true, "the viewer's own photo in the same group"},
		// "Only me" has to be visible to the me in question.
		{20, true, "the viewer's own private photo"},
	}

	for _, c := range cases {
		t.Run(c.what, func(t *testing.T) {
			if got := visible[c.id]; got != c.want {
				t.Errorf("node %d visible=%v, want %v", c.id, got, c.want)
			}
		})
	}
}

// Blocks are symmetric: it must not matter which way round the edge was
// written. Bob blocking Alice hides Bob from Alice too.
func TestBlockingIsSymmetric(t *testing.T) {
	db := accessTestDB(t)
	seedAccessFixture(t, db)

	// Flip the edge so BOB blocked ALICE instead.
	if _, err := db.Exec(
		`UPDATE edges SET node_a_id = ?, node_b_id = ? WHERE type = 'edge.actor.block.v1'`,
		bobID, aliceID,
	); err != nil {
		t.Fatalf("flip: %v", err)
	}

	alice := entities.NewViewer()
	alice.ID = aliceID
	alice.PersonType = "registered"

	visible := visibleMedia(t, db, alice)

	if visible[10] || visible[11] {
		t.Error("a block written in the other direction did not hide the blocker's content")
	}
	if !visible[12] {
		t.Error("the viewer's own content was hidden")
	}
}

func visibleReplies(t *testing.T, db *sql.DB, viewer *entities.Viewer) map[int64]bool {
	t.Helper()

	base := sq.Select("node.id").From("nodes AS node").
		LeftJoin("nodes AS owner ON node.owner_id = owner.id").
		Where("node.root_id IS NOT NULL")

	query, args, err := SetMediumAccessQuery(base, viewer).ToSql()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	visible := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		visible[id] = true
	}
	return visible
}

// Replies are stored with no audience of their own, owned by whoever wrote
// them, and are visible to whoever can see the post they are under.
//
// These were comments once, and the cases are the ones that guarded them:
// gated like posts, every one failed on its empty access, and nobody but
// site administrators saw any at all.
func TestRepliesFollowTheirPost(t *testing.T) {
	db := accessTestDB(t)
	seedAccessFixture(t, db)

	for _, stmt := range []string{
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,created_at,updated_at) VALUES
		 (4,'node.actor.person-service.person.v1','person-service','Carol','carol','public',1,NOW(),NOW())`,
		`INSERT INTO nodes (id,type,service,body,access,enabled,parent_id,root_id,owner_id,owner_type,created_by,created_at,updated_at) VALUES
		 (30,'node.medium.text-service.note.v1','text-service','on a public post','',1,12,12,1,'node.actor.person-service.person.v1',1,NOW(),NOW()),
		 (31,'node.medium.text-service.note.v1','text-service','on a private post','',1,20,20,1,'node.actor.person-service.person.v1',1,NOW(),NOW()),
		 (32,'node.medium.text-service.note.v1','text-service','by a blocked person','',1,12,12,2,'node.actor.person-service.person.v1',2,NOW(),NOW())`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}

	viewer := func(id int64) *entities.Viewer {
		v := entities.NewViewer()
		v.ID = id
		if id > 0 {
			v.PersonType = "registered"
		}
		return v
	}

	cases := []struct {
		viewer int64
		id     int64
		want   bool
		what   string
	}{
		{4, 30, true, "anybody signed in, on a public post"},
		{4, 31, false, "not on a post only its owner can see"},
		{4, 32, true, "a reply by somebody the viewer has not blocked"},
		{1, 31, true, "the post's owner, on their own private post"},
		{1, 32, false, "a reply by somebody the viewer has blocked"},
		{0, 30, true, "a guest, on a public post"},
		{0, 31, false, "a guest, not on a private post"},
	}

	for _, c := range cases {
		t.Run(c.what, func(t *testing.T) {
			if got := visibleReplies(t, db, viewer(c.viewer))[c.id]; got != c.want {
				t.Errorf("reply %d as viewer %d: visible=%v, want %v", c.id, c.viewer, got, c.want)
			}
		})
	}
}

const (
	carolID = 4
	daveID  = 5

	notePublic    = 40
	noteFollowers = 41
	noteMyself    = 42
	noteMutuals   = 43
	noteInGroup   = 44

	repostOfFollowers = 50
	repostOfPublic    = 51

	replyOnFollowers = 60
	replyOnPublic    = 61
)

// seedFeedFixture: Bob posts at four access levels on his own profile. Carol
// follows Bob, and reposts and replies to his posts. Dave follows Carol but
// not Bob, so anything of Bob's that reaches Dave reaches him through Carol.
// Alice follows nobody and wrote a private post into a group.
func seedFeedFixture(t *testing.T, db *sql.DB) {
	t.Helper()

	const (
		person = "'node.actor.person-service.person.v1','person-service'"
		note   = "'node.medium.text-service.note.v1','text-service'"
	)

	for _, stmt := range []string{
		`DELETE FROM edges`,
		`DELETE FROM nodes`,
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,created_at,updated_at) VALUES
		 (1,` + person + `,'Alice','alice','public',1,NOW(),NOW()),
		 (2,` + person + `,'Bob','bob','public',1,NOW(),NOW()),
		 (3,'node.actor.group-service.group.v1','group-service','Cyclists','cyclists','public',1,NOW(),NOW()),
		 (4,` + person + `,'Carol','carol','public',1,NOW(),NOW()),
		 (5,` + person + `,'Dave','dave','public',1,NOW(),NOW())`,
		`INSERT INTO nodes (id,type,service,body,alias,access,enabled,owner_id,created_by,created_at,updated_at) VALUES
		 (40,` + note + `,'public','n40','public',1,2,2,NOW(),NOW()),
		 (41,` + note + `,'followers','n41','followers',1,2,2,NOW(),NOW()),
		 (42,` + note + `,'only me','n42','myself',1,2,2,NOW(),NOW()),
		 (43,` + note + `,'mutuals','n43','mutuals',1,2,2,NOW(),NOW()),
		 (44,` + note + `,'alice, admins only, in a group','n44','admins',1,3,1,NOW(),NOW())`,
		// Reposts are owned by the reposter and carry a copy of the original's
		// access; replies are owned by whoever wrote them and have none.
		`INSERT INTO nodes (id,type,service,alias,access,enabled,owner_id,parent_id,created_by,created_at,updated_at) VALUES
		 (50,'node.base.repost-service.repost.v1','feed-service','n41','followers',1,4,41,4,NOW(),NOW()),
		 (51,'node.base.repost-service.repost.v1','feed-service','n40','public',1,4,40,4,NOW(),NOW())`,
		`INSERT INTO nodes (id,type,service,body,access,enabled,parent_id,root_id,owner_id,owner_type,created_by,created_at,updated_at) VALUES
		 (60,` + note + `,'on the followers post','',1,41,41,4,'node.actor.person-service.person.v1',4,NOW(),NOW()),
		 (61,` + note + `,'on the public post','',1,40,40,4,'node.actor.person-service.person.v1',4,NOW(),NOW())`,
		`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,created_at) VALUES
		 ('edge.actor.follow.v1',4,` + "'node.actor.person-service.person.v1'" + `,2,` + "'node.actor.person-service.person.v1'" + `,NOW()),
		 ('edge.actor.follow.v1',5,` + "'node.actor.person-service.person.v1'" + `,4,` + "'node.actor.person-service.person.v1'" + `,NOW())`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}
}

func feedViewer(id int64) *entities.Viewer {
	v := entities.NewViewer()
	v.ID = id
	if id > 0 {
		v.PersonType = "registered"
	}
	return v
}

func nodeIDs(t *testing.T, nodes []*entities.AnahitaNode, err error) map[int64]bool {
	t.Helper()
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	ids := map[int64]bool{}
	for _, n := range nodes {
		ids[n.ID] = true
	}
	return ids
}

var feedNoteTypes = []string{"node.medium.text-service.note.v1"}

// A profile feed shows a post only to the people its own access admits.
//
// The feed gate read the access of the row's PARENT. A post has none, so its
// own access was never checked, and every restricted post on a public profile
// was shown to everybody, signed in or not.
func TestActorFeedHonoursAPostsOwnAccess(t *testing.T) {
	db := accessTestDB(t)
	seedFeedFixture(t, db)
	repo := NewFeedActorRepo(db)

	bobsFeed := func(viewer int64) map[int64]bool {
		req := &graphpb.FeedActorBrowseRequest{ActorId: bobID, MediumEntityTypes: feedNoteTypes, Limit: 50}
		nodes, err := repo.Browse(req, feedViewer(viewer))
		return nodeIDs(t, nodes, err)
	}

	cases := []struct {
		viewer int64
		id     int64
		want   bool
		what   string
	}{
		{0, notePublic, true, "a guest sees the public post"},
		{0, noteFollowers, false, "a guest does not see the followers-only post"},
		{0, noteMyself, false, "a guest does not see the only-me post"},
		{0, noteMutuals, false, "a guest does not see the mutuals-only post"},

		{daveID, notePublic, true, "a stranger sees the public post"},
		{daveID, noteFollowers, false, "a stranger does not see the followers-only post"},
		{daveID, noteMyself, false, "a stranger does not see the only-me post"},
		{daveID, noteMutuals, false, "a stranger does not see the mutuals-only post"},

		{carolID, noteFollowers, true, "a follower sees the followers-only post"},
		{carolID, noteMyself, false, "a follower does not see the only-me post"},
		{carolID, noteMutuals, false, "a follower who is not followed back does not see the mutuals-only post"},

		// A person does not follow themselves, so without the owner rule
		// these two would be hidden from the one person who wrote them.
		{bobID, noteFollowers, true, "the owner sees their own followers-only post"},
		{bobID, noteMutuals, true, "the owner sees their own mutuals-only post"},
		{bobID, noteMyself, true, "the owner sees their own only-me post"},
	}

	for _, c := range cases {
		t.Run(c.what, func(t *testing.T) {
			if got := bobsFeed(c.viewer)[c.id]; got != c.want {
				t.Errorf("node %d as viewer %d: visible=%v, want %v", c.id, c.viewer, got, c.want)
			}
		})
	}

	t.Run("the count agrees with the list", func(t *testing.T) {
		req := &graphpb.FeedActorCountRequest{ActorId: bobID, MediumEntityTypes: feedNoteTypes}
		got, err := repo.Count(req, feedViewer(daveID))
		if err != nil {
			t.Fatalf("count: %v", err)
		}
		if got != 1 {
			t.Errorf("a stranger's count of Bob's feed = %d, want 1 (the public post)", got)
		}
	})
}

// The home feed carries the leaders' posts and their reposts, each shown only
// when the viewer may see the post underneath it. It carries no replies:
// those are read in their thread.
//
// A repost used to be judged by the original's access, but with "is the viewer
// a follower" asked of the reposter. Following the reposter was enough to see
// a followers-only post by somebody the viewer did not follow.
func TestLeadersFeedJudgesRepostsByTheirPostAndLeavesRepliesOut(t *testing.T) {
	db := accessTestDB(t)
	seedFeedFixture(t, db)
	repo := NewFeedLeadersRepo(db)

	homeFeed := func(viewer int64) map[int64]bool {
		req := &graphpb.FeedLeadersBrowseRequest{
			MediumEntityTypes: feedNoteTypes,
			IncludeReposts:    true,
			Limit:             50,
		}
		nodes, err := repo.Browse(req, feedViewer(viewer))
		return nodeIDs(t, nodes, err)
	}

	cases := []struct {
		viewer int64
		id     int64
		want   bool
		what   string
	}{
		// Dave follows Carol, not Bob.
		{daveID, repostOfPublic, true, "a repost of a public post reaches the reposter's follower"},
		{daveID, repostOfFollowers, false, "a repost of a followers-only post does not reach somebody who does not follow its author"},
		{daveID, replyOnPublic, false, "a leader's reply, even to a public post"},
		{daveID, replyOnFollowers, false, "a leader's reply to a post the viewer cannot see"},
		{carolID, replyOnPublic, false, "the viewer's own reply"},

		// Carol follows Bob.
		{carolID, notePublic, true, "a leader's public post"},
		{carolID, noteFollowers, true, "a leader's followers-only post, to a follower"},
		{carolID, noteMyself, false, "a leader's only-me post"},
		{carolID, noteMutuals, false, "a leader's mutuals-only post, to a follower who is not followed back"},
		{carolID, repostOfFollowers, true, "the reposter sees their own repost"},

		// The viewer's own posts are part of their home feed.
		{bobID, noteFollowers, true, "the viewer's own followers-only post"},
		{bobID, noteMyself, true, "the viewer's own only-me post"},
	}

	for _, c := range cases {
		t.Run(c.what, func(t *testing.T) {
			if got := homeFeed(c.viewer)[c.id]; got != c.want {
				t.Errorf("node %d as viewer %d: visible=%v, want %v", c.id, c.viewer, got, c.want)
			}
		})
	}
}

// The author of a post in somebody else's space sees it in listings as long
// as they can see that space, whatever the post's own access says. This is
// the rule permissions.MediumPermissions.CanRead applies to a single post.
func TestAuthorSeesTheirOwnPostInAGroup(t *testing.T) {
	db := accessTestDB(t)
	seedFeedFixture(t, db)

	visible := func(viewer int64) map[int64]bool {
		base := sq.Select("node.id").From("nodes AS node").
			LeftJoin("nodes AS owner ON node.owner_id = owner.id").
			Where(sq.Eq{"node.type": feedNoteTypes})
		query, args, err := SetMediumAccessQuery(base, feedViewer(viewer)).ToSql()
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		rows, err := db.Query(query, args...)
		if err != nil {
			t.Fatalf("query: %v", err)
		}
		defer rows.Close()
		ids := map[int64]bool{}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				t.Fatalf("scan: %v", err)
			}
			ids[id] = true
		}
		return ids
	}

	if !visible(aliceID)[noteInGroup] {
		t.Error("the author cannot see their own admins-only post in a public group")
	}
	if visible(daveID)[noteInGroup] {
		t.Error("a stranger sees an admins-only post in a group")
	}
}

// Who the viewer follows that has posted lately: the row of faces on the
// home page. It counts posts, by the actors the viewer follows, that the
// viewer may see, and nothing else.
func TestActiveLeadersCountsOnlyWhatTheViewerMaySee(t *testing.T) {
	db := accessTestDB(t)
	seedFeedFixture(t, db)
	repo := NewFeedLeadersRepo(db)

	active := func(viewer int64, hours uint32) map[int64]uint64 {
		leaders, err := repo.Active(feedViewer(viewer), hours, 30, feedNoteTypes)
		if err != nil {
			t.Fatalf("active: %v", err)
		}
		counts := map[int64]uint64{}
		for _, leader := range leaders {
			counts[leader.ActorID] = leader.Count
		}
		return counts
	}

	// Carol follows Bob. Of his four notes she may see the public one and
	// the one for followers: not the one for himself, and not the one for
	// mutuals, since he does not follow her back.
	if got := active(carolID, 24); len(got) != 1 || got[bobID] != 2 {
		t.Errorf("Carol is shown %v, want Bob with 2", got)
	}

	// Dave follows Carol, who has only reposted and replied. Neither is a
	// post of hers.
	if got := active(daveID, 24); len(got) != 0 {
		t.Errorf("Dave is shown %v, want nobody", got)
	}

	// Bob follows nobody, and his own posts are not news to him. A visitor
	// follows nobody either.
	for _, viewer := range []int64{bobID, 0} {
		if got := active(viewer, 24); len(got) != 0 {
			t.Errorf("viewer %d is shown %v, want nobody", viewer, got)
		}
	}

	// Outside the window, not shown; inside a longer one, shown again.
	if _, err := db.Exec(`UPDATE nodes SET created_at = UTC_TIMESTAMP() - INTERVAL 3 DAY WHERE owner_id = ?`, bobID); err != nil {
		t.Fatal(err)
	}
	if got := active(carolID, 24); len(got) != 0 {
		t.Errorf("after three days Carol is shown %v, want nobody", got)
	}
	if got := active(carolID, 96); got[bobID] != 2 {
		t.Errorf("over four days Carol is shown %v, want Bob with 2", got)
	}
}
