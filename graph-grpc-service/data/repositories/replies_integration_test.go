package repositories

import (
	"database/sql"
	"sort"
	"testing"

	"anahita-libs/data/entities"

	"graph/requests"

	sq "github.com/Masterminds/squirrel"
)

// The thread rules against a real database.
//
//	ACCESS_TEST_DSN="root@tcp(127.0.0.1:13309)/fresh?parseTime=true" \
//	  go test ./data/repositories/ -run TestReplies -v
//
// Never the development database: the fixture starts by emptying nodes.
//
// THE RULE THESE GUARD: a reply is visible to exactly the people who can see
// the post at the top of its thread. A reply is owned by its author and has
// no audience of its own, so any listing that judges it as an ordinary post
// shows it wherever its author's profile is visible. Each listing that can
// return a note has a case here, and a new one needs one.

const (
	replyAlice = 1 // posts in the private group; follows it
	replyBob   = 2 // outside the group
	replyGroup = 3 // followers only
	replyCarol = 4 // follows the group
	replyDave  = 5 // blocked by Alice

	rootInGroup  = 100 // Alice's note in the private group
	rootPublic   = 101 // a public note on Alice's profile
	rootOnlyMe   = 102 // Alice's "only me" note
	rootPhoto    = 103 // a public photo on Alice's profile, with nothing under it
	replyInGroup = 200 // Alice answers her note in the group
	replyNested  = 201 // Carol answers that
	replyByBob   = 210 // Bob answers the public note
	replyByDave  = 211 // Dave answers the public note
	replyRemoved = 212 // a removed reply, kept as a placeholder
	replyToGone  = 213 // Bob's answer to the removed reply
	replyOnlyMe  = 220 // Alice answers her own private note
)

const noteType = "node.medium.text-service.note.v1"

func seedRepliesFixture(t *testing.T, db *sql.DB) {
	t.Helper()

	const person = "'node.actor.person-service.person.v1','person-service'"
	const note = "'" + noteType + "','text-service'"
	const personType = "'node.actor.person-service.person.v1'"
	const groupType = "'node.actor.group-service.group.v1'"

	for _, stmt := range []string{
		`DELETE FROM edges`,
		`DELETE FROM nodes`,

		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,created_at,updated_at) VALUES
		 (1,` + person + `,'Alice','alice','public',1,NOW(),NOW()),
		 (2,` + person + `,'Bob','bob','public',1,NOW(),NOW()),
		 (3,'node.actor.group-service.group.v1','group-service','Circle','circle','followers',1,NOW(),NOW()),
		 (4,` + person + `,'Carol','carol','public',1,NOW(),NOW()),
		 (5,` + person + `,'Dave','dave','public',1,NOW(),NOW())`,

		// The posts threads hang from.
		`INSERT INTO nodes (id,type,service,name,body,alias,access,enabled,owner_id,owner_type,created_by,created_at,updated_at) VALUES
		 (100,` + note + `,'','in the group','n100','public',1,3,` + groupType + `,1,NOW(),NOW()),
		 (101,` + note + `,'','public','n101','public',1,1,` + personType + `,1,NOW(),NOW()),
		 (102,` + note + `,'','only me','n102','myself',1,1,` + personType + `,1,NOW(),NOW()),
		 (103,'node.medium.photo-service.photo.v1','photo-service','a photo','','n103','public',1,1,` + personType + `,1,NOW(),NOW())`,

		// Replies, as the graph stores them: owned by their author, with no
		// audience of their own, and naming their thread's root.
		`INSERT INTO nodes (id,type,service,name,body,alias,access,enabled,owner_id,owner_type,created_by,parent_id,parent_type,root_id,created_at,updated_at) VALUES
		 (200,` + note + `,'','alice answers in the group','r200','',1,1,` + personType + `,1,100,'` + noteType + `',100,NOW() + INTERVAL 1 SECOND,NOW()),
		 (201,` + note + `,'','carol answers alice','r201','',1,4,` + personType + `,4,200,'` + noteType + `',100,NOW() + INTERVAL 2 SECOND,NOW()),
		 (210,` + note + `,'','bob answers','r210','',1,2,` + personType + `,2,101,'` + noteType + `',101,NOW() + INTERVAL 1 SECOND,NOW()),
		 (211,` + note + `,'','dave answers','r211','',1,5,` + personType + `,5,101,'` + noteType + `',101,NOW() + INTERVAL 2 SECOND,NOW()),
		 (212,` + note + `,'','','r212','',1,4,` + personType + `,4,101,'` + noteType + `',101,NOW() + INTERVAL 3 SECOND,NOW()),
		 (213,` + note + `,'','bob answers what is gone','r213','',1,2,` + personType + `,2,212,'` + noteType + `',101,NOW() + INTERVAL 4 SECOND,NOW()),
		 (220,` + note + `,'','alice to herself','r220','',1,1,` + personType + `,1,102,'` + noteType + `',102,NOW() + INTERVAL 1 SECOND,NOW())`,

		`UPDATE nodes SET deleted_at = NOW() WHERE id = 212`,

		`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,created_at) VALUES
		 ('edge.actor.follow.v1',1,` + personType + `,3,` + groupType + `,NOW()),
		 ('edge.actor.follow.v1',4,` + personType + `,3,` + groupType + `,NOW()),
		 ('edge.actor.block.v1',1,` + personType + `,5,` + personType + `,NOW())`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}
}

func replyViewer(id int64) *entities.Viewer {
	viewer := entities.NewViewer()
	viewer.ID = id
	if id > 0 {
		viewer.PersonType = "registered"
	}
	return viewer
}

func sortedIDs(seen map[int64]bool) []int64 {
	var ids []int64
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func sameSet(got map[int64]bool, want ...int64) bool {
	if len(got) != len(want) {
		return false
	}
	for _, id := range want {
		if !got[id] {
			return false
		}
	}
	return true
}

// notesThrough runs a listing of notes through one of the gates.
func notesThrough(t *testing.T, db *sql.DB, gate func(sq.SelectBuilder) sq.SelectBuilder) map[int64]bool {
	t.Helper()

	base := sq.Select("node.id").From("nodes AS node").
		LeftJoin("nodes AS owner ON node.owner_id = owner.id").
		Where(sq.Eq{"node.type": noteType})

	query, args, err := gate(base).ToSql()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		t.Fatalf("query: %v\n%s", err, query)
	}
	defer rows.Close()

	seen := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		seen[id] = true
	}
	return seen
}

// The listing every list of posts goes through.
func TestRepliesAreListedOnlyToThoseWhoCanSeeTheirRoot(t *testing.T) {
	db := accessTestDB(t)
	seedRepliesFixture(t, db)

	cases := []struct {
		who    string
		viewer int64
		want   []int64
	}{
		// Bob is outside the group. Alice's answer in the group is owned by
		// Alice, whose profile is public, and that must not show it to him:
		// this is the case the whole design exists for. Nor Carol's answer
		// under it, nor the answer to Alice's private note. The removed
		// reply is nowhere but in its thread.
		{"somebody outside the group", replyBob,
			[]int64{rootPublic, replyByBob, replyByDave, replyToGone}},
		{"a visitor", 0,
			[]int64{rootPublic, replyByBob, replyByDave, replyToGone}},
		// Carol follows the group, so she sees its thread. Not Alice's
		// private note or the answer under it.
		{"a follower of the group", replyCarol,
			[]int64{rootInGroup, rootPublic, replyInGroup, replyNested, replyByBob, replyByDave, replyToGone}},
		// Alice sees everything of hers, and not what Dave wrote: she
		// blocked him, and a block reaches replies as it reaches posts.
		{"the author", replyAlice,
			[]int64{rootInGroup, rootPublic, rootOnlyMe, replyInGroup, replyNested, replyByBob, replyToGone, replyOnlyMe}},
	}

	for _, c := range cases {
		t.Run(c.who, func(t *testing.T) {
			viewer := replyViewer(c.viewer)
			got := notesThrough(t, db, func(sb sq.SelectBuilder) sq.SelectBuilder {
				return SetMediumAccessQuery(sb, viewer)
			})
			if !sameSet(got, c.want...) {
				t.Errorf("sees %v, want %v", sortedIDs(got), c.want)
			}
		})
	}
}

// Search and the pages of a hashtag or a place write their gates out by
// hand. A reply to something the viewer may not see must not get through
// either of them.
func TestRepliesDoNotLeakThroughSearchOrTagPages(t *testing.T) {
	db := accessTestDB(t)
	seedRepliesFixture(t, db)
	bob := replyViewer(replyBob)

	gates := map[string]func(sq.SelectBuilder) sq.SelectBuilder{
		"search": func(sb sq.SelectBuilder) sq.SelectBuilder {
			return SetSearchAccessQuery(sb, bob)
		},
		"a hashtag or place page": func(sb sq.SelectBuilder) sq.SelectBuilder {
			return SetPolymorphicAccessQuery(sb, bob)
		},
	}

	for name, gate := range gates {
		t.Run(name, func(t *testing.T) {
			got := notesThrough(t, db, gate)

			for _, hidden := range []int64{rootInGroup, replyInGroup, replyNested, rootOnlyMe, replyOnlyMe, replyRemoved} {
				if got[hidden] {
					t.Errorf("%d is shown to somebody who cannot see its thread; sees %v", hidden, sortedIDs(got))
				}
			}
			for _, shown := range []int64{rootPublic, replyByBob} {
				if !got[shown] {
					t.Errorf("%d is hidden from somebody who can see its thread; sees %v", shown, sortedIDs(got))
				}
			}
		})
	}
}

// Asking whether particular nodes are visible, which abuse reports and
// recommendations do.
func TestRepliesAreVisibleByTheirRootWhenAskedAboutOneByOne(t *testing.T) {
	db := accessTestDB(t)
	seedRepliesFixture(t, db)
	repo := newRepo(db)

	got, err := repo.FilterVisible([]int64{replyInGroup, replyNested, replyByBob, replyOnlyMe, replyRemoved}, replyViewer(replyBob))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != replyByBob {
		t.Errorf("visible to somebody outside the group: %v, want only %d", got, replyByBob)
	}
}

// A list of somebody's notes is their posts, not their answers to others.
func TestAListOfNotesLeavesRepliesOutUnlessAsked(t *testing.T) {
	db := accessTestDB(t)
	seedRepliesFixture(t, db)
	repo := NewNodesRepo(db)

	list := func(viewer int64, topLevelOnly bool) map[int64]bool {
		req := requests.NewNodeRequest().Browse
		req.Types = []string{noteType}
		req.OwnerIDs = []int64{replyAlice}
		req.Viewer = replyViewer(viewer)
		req.Enabled = true
		req.TopLevelOnly = topLevelOnly

		nodes, err := repo.Browse(req)
		return nodeIDs(t, nodes, err)
	}

	if got := list(replyBob, true); !sameSet(got, rootPublic) {
		t.Errorf("Alice's notes, to Bob: %v, want only her public note", sortedIDs(got))
	}

	// With replies asked for, still nothing from the group or from her
	// private thread: the root decides, not the owner.
	if got := list(replyBob, false); !sameSet(got, rootPublic) {
		t.Errorf("Alice's posts and replies, to Bob: %v, want only her public note", sortedIDs(got))
	}

	if got := list(replyAlice, false); !sameSet(got, rootPublic, rootOnlyMe, replyInGroup, replyOnlyMe) {
		t.Errorf("Alice's posts and replies, to herself: %v", sortedIDs(got))
	}
}

func threadIDs(t *testing.T, db *sql.DB, root int64, viewer int64) []int64 {
	t.Helper()

	replies, err := NewRepliesRepo(db).Thread(root, replyViewer(viewer))
	if err != nil {
		t.Fatalf("thread: %v", err)
	}
	var ids []int64
	for _, reply := range replies {
		ids = append(ids, reply.ID)
	}
	return ids
}

func sameOrder(got []int64, want ...int64) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestAThreadIsReadWholeOldestFirstByThoseWhoCanSeeItsRoot(t *testing.T) {
	db := accessTestDB(t)
	seedRepliesFixture(t, db)

	// Every depth, in the order written. The removed reply is there, as
	// the placeholder Bob's answer hangs from.
	if got := threadIDs(t, db, rootPublic, replyBob); !sameOrder(got, replyByBob, replyByDave, replyRemoved, replyToGone) {
		t.Errorf("the public thread, to Bob: %v", got)
	}

	// Alice blocked Dave, so his reply is not in the thread she reads.
	if got := threadIDs(t, db, rootPublic, replyAlice); !sameOrder(got, replyByBob, replyRemoved, replyToGone) {
		t.Errorf("the public thread, to Alice: %v", got)
	}

	if got := threadIDs(t, db, rootInGroup, replyBob); len(got) != 0 {
		t.Errorf("the group's thread, to somebody outside it: %v", got)
	}
	if got := threadIDs(t, db, rootInGroup, 0); len(got) != 0 {
		t.Errorf("the group's thread, to a visitor: %v", got)
	}
	if got := threadIDs(t, db, rootInGroup, replyCarol); !sameOrder(got, replyInGroup, replyNested) {
		t.Errorf("the group's thread, to a follower of the group: %v", got)
	}
}

func commentCount(t *testing.T, db *sql.DB, id int64) int {
	t.Helper()

	var count int
	if err := db.QueryRow(`SELECT COALESCE(comment_count, 0) FROM nodes WHERE id = ?`, id).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestCountsAreOfWhatIsThereAndNotHidden(t *testing.T) {
	db := accessTestDB(t)
	seedRepliesFixture(t, db)
	repo := NewRepliesRepo(db)

	if err := repo.Recount(rootPublic, replyRemoved); err != nil {
		t.Fatal(err)
	}
	// Two: Bob's, and Bob's answer to the removed reply. Not the removed
	// reply itself, and not Dave's: Alice, whose note it is, blocks him.
	if got := commentCount(t, db, rootPublic); got != 2 {
		t.Errorf("the public note counts %d replies, want 2", got)
	}
	// The removed reply still has the one answer under it.
	if got := commentCount(t, db, replyRemoved); got != 1 {
		t.Errorf("the removed reply counts %d direct replies, want 1", got)
	}

	// Hiding one takes it out of the count; showing it puts it back; and
	// hiding it twice is hiding it once.
	bob := &entities.AnahitaNode{}
	bob.ID = replyByBob
	bob.Type = noteType

	for _, step := range []struct {
		hidden bool
		want   int
	}{{true, 1}, {true, 1}, {false, 2}} {
		if err := repo.SetHidden(rootPublic, bob, step.hidden, replyAlice); err != nil {
			t.Fatal(err)
		}
		if err := repo.Recount(rootPublic, 0); err != nil {
			t.Fatal(err)
		}
		if got := commentCount(t, db, rootPublic); got != step.want {
			t.Errorf("hidden=%v: counts %d, want %d", step.hidden, got, step.want)
		}
	}

	// What is hidden is what was hidden and what the blocked person wrote.
	if err := repo.SetHidden(rootPublic, bob, true, replyAlice); err != nil {
		t.Fatal(err)
	}
	hidden, err := repo.HiddenIDs(rootPublic)
	sort.Slice(hidden, func(i, j int) bool { return hidden[i] < hidden[j] })
	if err != nil || len(hidden) != 2 || hidden[0] != replyByBob || hidden[1] != replyByDave {
		t.Errorf("hidden replies: %v, %v", hidden, err)
	}

	// A post with nothing under it counts nothing.
	if err := repo.Recount(rootPhoto, 0); err != nil {
		t.Fatal(err)
	}
	if got := commentCount(t, db, rootPhoto); got != 0 {
		t.Errorf("the photo counts %d, want 0", got)
	}
}

func TestARemovedReplyIsEmptiedAndKeptOnlyWhileItHoldsAThreadTogether(t *testing.T) {
	db := accessTestDB(t)
	seedRepliesFixture(t, db)
	repo := NewRepliesRepo(db)

	answered, err := repo.HasReplies(replyInGroup)
	if err != nil || !answered {
		t.Fatalf("Alice's reply has Carol's under it: %v, %v", answered, err)
	}
	if answered, _ := repo.HasReplies(replyNested); answered {
		t.Error("Carol's reply has nothing under it")
	}
	if answered, _ := repo.HasReplies(rootPhoto); answered {
		t.Error("the photo has nothing under it")
	}

	if err := repo.Tombstone(replyInGroup, replyAlice); err != nil {
		t.Fatal(err)
	}

	var body string
	var removed bool
	if err := db.QueryRow(`SELECT COALESCE(body, ''), deleted_at IS NOT NULL FROM nodes WHERE id = ?`, replyInGroup).
		Scan(&body, &removed); err != nil {
		t.Fatal(err)
	}
	if body != "" || !removed {
		t.Errorf("after removal: body %q, marked removed %v", body, removed)
	}

	// Still in its thread, with Carol's answer after it, and gone from
	// every other listing.
	if got := threadIDs(t, db, rootInGroup, replyCarol); !sameOrder(got, replyInGroup, replyNested) {
		t.Errorf("the thread after removal: %v", got)
	}
	carol := replyViewer(replyCarol)
	listed := notesThrough(t, db, func(sb sq.SelectBuilder) sq.SelectBuilder {
		return SetMediumAccessQuery(sb, carol)
	})
	if listed[replyInGroup] || !listed[replyNested] {
		t.Errorf("listed to Carol after removal: %v", sortedIDs(listed))
	}

	// A post is never emptied this way, whatever is asked.
	if err := repo.Tombstone(rootPublic, replyAlice); err != nil {
		t.Fatal(err)
	}
	if n := 0; db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE id = ? AND deleted_at IS NULL AND body = 'public'`, rootPublic).Scan(&n) != nil || n != 1 {
		t.Error("a post at the top of a thread was emptied as if it were a reply")
	}
}
