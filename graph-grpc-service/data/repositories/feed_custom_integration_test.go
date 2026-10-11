package repositories

import (
	"database/sql"
	"fmt"
	"testing"

	"anahita-libs/data/entities"
	"anahita-libs/feeds"
	"graph/requests"
)

// Custom feeds against a real database.
//
//	ACCESS_TEST_DSN="root@tcp(127.0.0.1:13309)/fresh?parseTime=true" \
//	  go test ./data/repositories/ -run TestCustomFeed -v
//
// THE RULE THIS GUARDS: a feed never widens what anybody may see. Whatever a
// definition selects, each reader is shown the part of it they could have
// seen anyway, whoever made the feed.

const (
	feedAlice    = 1
	feedBob      = 2
	feedCyclists = 3
	feedCarol    = 4
	feedDave     = 5
	feedSecret   = 6

	feedTagGarden = 100
	feedTagSpam   = 101
	feedPlace     = 110

	feedBobPublic    = 200
	feedBobFollowers = 201
	feedInSecret     = 202
	feedGroupPhoto   = 203
	feedCarolFrench  = 204
	feedCarolReply   = 205
	feedBobDeleted   = 206
	feedCarolPlain   = 207

	feedOnlyMe  = 300
	feedForAll  = 301
	feedOfGroup = 302
)

// seedCustomFeedFixture: Alice follows Bob and a group only its followers can
// read. Carol follows nobody. Bob has blocked Dave. Posts carry #garden, and
// are older the higher their id, a minute apart.
func seedCustomFeedFixture(t *testing.T, db *sql.DB) {
	t.Helper()

	const (
		person  = "'node.actor.person-service.person.v1','person-service'"
		group   = "'node.actor.group-service.group.v1','group-service'"
		note    = "'node.medium.text-service.note.v1','text-service'"
		photo   = "'node.medium.photo-service.photo.v1','photo-service'"
		hashtag = "'node.tag.hashtag-service.hashtag.v1','hashtag-service'"
		place   = "'node.tag.location-service.location.v1','location-service'"
		feed    = "'" + feeds.NodeType + "','" + feeds.Service + "'"
		aPerson = "'node.actor.person-service.person.v1'"
		aGroup  = "'node.actor.group-service.group.v1'"
	)

	// Minutes ago, so that the order is the order of the ids.
	ago := func(minutes int) string {
		return fmt.Sprintf("NOW() - INTERVAL %d MINUTE", minutes)
	}

	for _, stmt := range []string{
		`DELETE FROM edges`,
		`DELETE FROM nodes`,
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,created_at,updated_at) VALUES
		 (1,` + person + `,'Alice','alice','public',1,NOW(),NOW()),
		 (2,` + person + `,'Bob','bob','public',1,NOW(),NOW()),
		 (3,` + group + `,'Cyclists','cyclists','public',1,NOW(),NOW()),
		 (4,` + person + `,'Carol','carol','public',1,NOW(),NOW()),
		 (5,` + person + `,'Dave','dave','public',1,NOW(),NOW()),
		 (6,` + group + `,'Secret','secret','followers',1,NOW(),NOW()),
		 (100,` + hashtag + `,'garden','garden','public',1,NOW(),NOW()),
		 (101,` + hashtag + `,'spam','spam','public',1,NOW(),NOW()),
		 (110,` + place + `,'The allotment','the-allotment','public',1,NOW(),NOW())`,
		`INSERT INTO nodes (id,type,service,name,body,access,language,enabled,owner_id,owner_type,created_by,created_at,updated_at) VALUES
		 (200,` + note + `,'','Compost tips #garden','public','en',1,2,` + aPerson + `,2,` + ago(1) + `,NOW()),
		 (201,` + note + `,'','For followers #garden','followers','en',1,2,` + aPerson + `,2,` + ago(2) + `,NOW()),
		 (202,` + note + `,'','In the secret group #garden','public','en',1,6,` + aGroup + `,1,` + ago(3) + `,NOW()),
		 (203,` + photo + `,'Ride','A ride #garden #spam','public','en',1,3,` + aGroup + `,4,` + ago(4) + `,NOW()),
		 (204,` + note + `,'','Le compost, 100% maison','public','fr',1,4,` + aPerson + `,4,` + ago(5) + `,NOW()),
		 (207,` + note + `,'','Nothing in particular','public','en',1,4,` + aPerson + `,4,` + ago(7) + `,NOW())`,
		`INSERT INTO nodes (id,type,service,body,access,enabled,owner_id,owner_type,created_by,created_at,updated_at,deleted_at) VALUES
		 (206,` + note + `,'Gone #garden','public',1,2,` + aPerson + `,2,` + ago(6) + `,NOW(),NOW())`,
		// A reply, owned by whoever wrote it, in the thread of Bob's post.
		`INSERT INTO nodes (id,type,service,body,access,enabled,parent_id,root_id,owner_id,owner_type,created_by,created_at,updated_at) VALUES
		 (205,` + note + `,'Agreed #garden','',1,200,200,4,` + aPerson + `,4,` + ago(6) + `,NOW())`,
		// Feeds: Alice's own, one of hers for everybody, and one of Carol's.
		`INSERT INTO nodes (id,type,service,name,access,enabled,owner_id,owner_type,created_by,created_at,updated_at) VALUES
		 (300,` + feed + `,'Mine','myself',1,1,` + aPerson + `,1,NOW(),NOW()),
		 (301,` + feed + `,'Gardens','public',1,1,` + aPerson + `,1,NOW(),NOW()),
		 (302,` + feed + `,'For my followers','followers',1,4,` + aPerson + `,4,NOW(),NOW())`,
		`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,created_at) VALUES
		 ('edge.actor.follow.v1',1,` + aPerson + `,2,` + aPerson + `,NOW()),
		 ('edge.actor.follow.v1',1,` + aPerson + `,6,` + aGroup + `,NOW()),
		 ('edge.actor.block.v1',2,` + aPerson + `,5,` + aPerson + `,NOW()),
		 ('edge.tag.hashtag.v1',100,'node.tag.hashtag-service.hashtag.v1',200,'node.medium.text-service.note.v1',NOW()),
		 ('edge.tag.hashtag.v1',100,'node.tag.hashtag-service.hashtag.v1',201,'node.medium.text-service.note.v1',NOW()),
		 ('edge.tag.hashtag.v1',100,'node.tag.hashtag-service.hashtag.v1',202,'node.medium.text-service.note.v1',NOW()),
		 ('edge.tag.hashtag.v1',100,'node.tag.hashtag-service.hashtag.v1',203,'node.medium.photo-service.photo.v1',NOW()),
		 ('edge.tag.hashtag.v1',101,'node.tag.hashtag-service.hashtag.v1',203,'node.medium.photo-service.photo.v1',NOW()),
		 ('edge.tag.hashtag.v1',100,'node.tag.hashtag-service.hashtag.v1',205,'node.medium.text-service.note.v1',NOW()),
		 ('edge.tag.hashtag.v1',100,'node.tag.hashtag-service.hashtag.v1',206,'node.medium.text-service.note.v1',NOW()),
		 ('edge.tag.location.v1',110,'node.tag.location-service.location.v1',204,'node.medium.text-service.note.v1',NOW())`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}
}

func customFeed(t *testing.T, db *sql.DB, definition string, owner, viewer int64) map[int64]bool {
	t.Helper()

	def, err := feeds.Parse([]byte(definition))
	if err != nil {
		t.Fatalf("definition %s: %v", definition, err)
	}

	nodes, err := NewFeedCustomRepo(db).Browse(def, owner, feeds.Cursor{}, feeds.MaxPage, feedViewer(viewer))

	return nodeIDs(t, nodes, err)
}

func TestCustomFeedNeverWidensWhatAReaderMaySee(t *testing.T) {
	db := accessTestDB(t)
	seedCustomFeedFixture(t, db)

	const garden = `{"filters":{"hashtags_any":["garden"]}}`
	const following = `{"sources":{"following":true}}`

	cases := []struct {
		what       string
		definition string
		owner      int64
		viewer     int64
		want       []int64
	}{
		// One hashtag over the whole site, read by four people.
		{"#garden, to somebody who follows nobody", garden, feedCarol, feedCarol, []int64{feedBobPublic, feedGroupPhoto}},
		{"#garden, to a visitor", garden, feedCarol, 0, []int64{feedBobPublic, feedGroupPhoto}},
		{"#garden, to a follower of Bob and of the group", garden, feedCarol, feedAlice, []int64{feedBobPublic, feedBobFollowers, feedInSecret, feedGroupPhoto}},
		{"#garden, to somebody Bob has blocked", garden, feedCarol, feedDave, []int64{feedGroupPhoto}},

		// Alice's feed of whoever she follows. It is hers that is followed,
		// and each reader's own eyes that are read with.
		{"whoever Alice follows, to Alice", following, feedAlice, feedAlice, []int64{feedBobPublic, feedBobFollowers, feedInSecret}},
		{"whoever Alice follows, to Carol", following, feedAlice, feedCarol, []int64{feedBobPublic}},
		{"whoever Alice follows, to a visitor", following, feedAlice, 0, []int64{feedBobPublic}},
		{"whoever Alice follows, to somebody Bob has blocked", following, feedAlice, feedDave, nil},
		{"whoever nobody follows", following, 0, feedAlice, nil},
	}

	for _, c := range cases {
		t.Run(c.what, func(t *testing.T) {
			got := customFeed(t, db, c.definition, c.owner, c.viewer)
			if !sameSet(got, c.want...) {
				t.Errorf("shows %v, want %v", sortedIDs(got), c.want)
			}
		})
	}
}

func TestCustomFeedSourcesFiltersAndExclusions(t *testing.T) {
	db := accessTestDB(t)
	seedCustomFeedFixture(t, db)

	// Read by Alice, who may see everything in the fixture that is there.
	cases := []struct {
		what       string
		definition string
		want       []int64
	}{
		{"profiles chosen by hand", `{"sources":{"actors":[4]}}`, []int64{feedCarolFrench, feedCarolPlain}},
		{"chosen and followed, added together", `{"sources":{"following":true,"actors":[3]}}`, []int64{feedBobPublic, feedBobFollowers, feedInSecret, feedGroupPhoto}},
		{"a keyword, in any case", `{"filters":{"keywords_any":["COMPOST"]}}`, []int64{feedBobPublic, feedCarolFrench}},
		{"a keyword that is LIKE's own character", `{"filters":{"keywords_any":["100%"]}}`, []int64{feedCarolFrench}},
		{"a place", `{"filters":{"locations_any":[110]}}`, []int64{feedCarolFrench}},
		{"a language", `{"sources":{"actors":[4]},"filters":{"language":"fr"}}`, []int64{feedCarolFrench}},
		{"a kind of post", `{"filters":{"hashtags_any":["garden"],"media_types":["photo"]}}`, []int64{feedGroupPhoto}},
		{"a kind of profile", `{"filters":{"hashtags_any":["garden"],"actor_types":["group"]}}`, []int64{feedInSecret, feedGroupPhoto}},
		{"either of two hashtags", `{"filters":{"hashtags_any":["spam","nothing"]}}`, []int64{feedGroupPhoto}},
		{"a hashtag and a keyword, both", `{"filters":{"hashtags_any":["garden"],"keywords_any":["compost"]}}`, []int64{feedBobPublic}},
		{"with replies", `{"filters":{"hashtags_any":["garden"]},"include_replies":true}`, []int64{feedBobPublic, feedBobFollowers, feedInSecret, feedGroupPhoto, feedCarolReply}},
		{"without a hashtag", `{"filters":{"hashtags_any":["garden"]},"exclude":{"hashtags":["spam"]}}`, []int64{feedBobPublic, feedBobFollowers, feedInSecret}},
		{"without a keyword", `{"filters":{"hashtags_any":["garden"]},"exclude":{"keywords":["compost","ride"]}}`, []int64{feedBobFollowers, feedInSecret}},
		{"without a profile", `{"filters":{"hashtags_any":["garden"]},"exclude":{"actors":[2]}}`, []int64{feedInSecret, feedGroupPhoto}},
		{"without somebody's posts wherever they are", `{"filters":{"hashtags_any":["garden"]},"exclude":{"actors":[4]}}`, []int64{feedBobPublic, feedBobFollowers, feedInSecret}},
	}

	for _, c := range cases {
		t.Run(c.what, func(t *testing.T) {
			got := customFeed(t, db, c.definition, feedAlice, feedAlice)
			if !sameSet(got, c.want...) {
				t.Errorf("shows %v, want %v", sortedIDs(got), c.want)
			}
		})
	}
}

// Read a page at a time, a feed is every post once and in order, and a post
// written meanwhile neither repeats one nor hides one.
func TestCustomFeedPagesByWhatCameBefore(t *testing.T) {
	db := accessTestDB(t)
	seedCustomFeedFixture(t, db)

	def, err := feeds.Parse([]byte(`{"filters":{"hashtags_any":["garden"]}}`))
	if err != nil {
		t.Fatal(err)
	}
	repo := NewFeedCustomRepo(db)
	viewer := feedViewer(feedAlice)

	first, err := repo.Browse(def, feedAlice, feeds.Cursor{}, 2, viewer)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 || first[0].ID != feedBobPublic || first[1].ID != feedBobFollowers {
		t.Fatalf("first page %v", pageIDs(first))
	}

	// Somebody posts while the first page is being read.
	if _, err := db.Exec(`INSERT INTO nodes (id,type,service,body,access,enabled,owner_id,owner_type,created_by,created_at,updated_at) VALUES
		(299,'node.medium.text-service.note.v1','text-service','New #garden','public',1,2,'node.actor.person-service.person.v1',2,NOW(),NOW())`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,created_at) VALUES
		('edge.tag.hashtag.v1',100,'node.tag.hashtag-service.hashtag.v1',299,'node.medium.text-service.note.v1',NOW())`); err != nil {
		t.Fatal(err)
	}

	last := first[len(first)-1]
	second, err := repo.Browse(def, feedAlice, feeds.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}, 2, viewer)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 2 || second[0].ID != feedInSecret || second[1].ID != feedGroupPhoto {
		t.Fatalf("second page %v", pageIDs(second))
	}

	last = second[len(second)-1]
	third, err := repo.Browse(def, feedAlice, feeds.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}, 2, viewer)
	if err != nil || len(third) != 0 {
		t.Fatalf("third page %v %v", pageIDs(third), err)
	}
}

// Two posts in the same second are told apart by their ids.
func TestCustomFeedPagesThroughPostsOfTheSameSecond(t *testing.T) {
	db := accessTestDB(t)
	seedCustomFeedFixture(t, db)

	if _, err := db.Exec(`UPDATE nodes SET created_at = '2026-01-01 10:00:00' WHERE id IN (204, 207)`); err != nil {
		t.Fatal(err)
	}

	def, err := feeds.Parse([]byte(`{"sources":{"actors":[4]}}`))
	if err != nil {
		t.Fatal(err)
	}
	repo := NewFeedCustomRepo(db)
	viewer := feedViewer(feedAlice)

	first, err := repo.Browse(def, feedAlice, feeds.Cursor{}, 1, viewer)
	if err != nil || len(first) != 1 || first[0].ID != feedCarolPlain {
		t.Fatalf("first page %v %v", pageIDs(first), err)
	}

	second, err := repo.Browse(def, feedAlice, feeds.Cursor{CreatedAt: first[0].CreatedAt, ID: first[0].ID}, 1, viewer)
	if err != nil || len(second) != 1 || second[0].ID != feedCarolFrench {
		t.Fatalf("second page %v %v", pageIDs(second), err)
	}
}

// A feed is its maker's, and is seen by whoever its audience admits.
func TestACustomFeedIsSeenByItsAudience(t *testing.T) {
	db := accessTestDB(t)
	seedCustomFeedFixture(t, db)
	repo := NewNodesRepo(db)

	all := []int64{feedOnlyMe, feedForAll, feedOfGroup}

	cases := []struct {
		what   string
		viewer int64
		want   []int64
	}{
		{"Alice, who made two of them", feedAlice, []int64{feedOnlyMe, feedForAll}},
		{"Carol, who made the third", feedCarol, []int64{feedForAll, feedOfGroup}},
		{"a visitor", 0, []int64{feedForAll}},
		{"Bob", feedBob, []int64{feedForAll}},
	}

	for _, c := range cases {
		t.Run(c.what, func(t *testing.T) {
			visible, err := repo.FilterVisible(all, feedViewer(c.viewer))
			if err != nil {
				t.Fatal(err)
			}
			got := map[int64]bool{}
			for _, id := range visible {
				got[id] = true
			}
			if !sameSet(got, c.want...) {
				t.Errorf("sees %v, want %v", sortedIDs(got), c.want)
			}
		})
	}
}

func pageIDs(nodes []*entities.AnahitaNode) []int64 {
	ids := make([]int64, 0, len(nodes))
	for _, node := range nodes {
		ids = append(ids, node.ID)
	}
	return ids
}

// A feed is kept and read back by the ordinary node calls feed-service
// makes: what it shows in its meta, its audience, and its owner beside it.
func TestACustomFeedIsKeptAsANode(t *testing.T) {
	db := accessTestDB(t)
	seedCustomFeedFixture(t, db)
	repo := NewNodesRepo(db)

	def, err := feeds.Parse([]byte(`{"filters":{"hashtags_any":["garden"]}}`))
	if err != nil {
		t.Fatal(err)
	}

	node := entities.NewAnahitaNode()
	node.Type = feeds.NodeType
	node.Service = feeds.Service
	node.Name = "Gardens"
	node.Body = "What is growing"
	node.Access = "myself"
	node.Meta = def.JSON()
	node.Enabled = true
	node.OwnerID = feedCarol
	node.OwnerType = "node.actor.person-service.person.v1"
	node.CreatedBy = feedCarol
	node.ModifiedBy = feedCarol

	added, err := repo.Add(node)
	if err != nil || added == nil || added.ID == 0 {
		t.Fatalf("add: %v %v", added, err)
	}

	read, err := repo.Read(added.ID)
	if err != nil {
		t.Fatal(err)
	}
	if read.Type != feeds.NodeType || read.Name != "Gardens" || read.Access != "myself" || read.OwnerID != feedCarol {
		t.Fatalf("read back %q %q %q owner %d", read.Type, read.Name, read.Access, read.OwnerID)
	}
	if read.Owner == nil || read.Owner.Name != "Carol" {
		t.Errorf("read without its owner: %+v", read.Owner)
	}
	again, err := feeds.Parse(read.Meta)
	if err != nil || string(again.JSON()) != string(def.JSON()) {
		t.Fatalf("definition read back as %s (%v)", read.Meta, err)
	}

	// Changed: another name, another audience, something else shown.
	changed, _ := feeds.Parse([]byte(`{"sources":{"following":true}}`))
	read.Name = "Following"
	read.Access = "public"
	read.Meta = changed.JSON()
	read.ModifiedBy = feedCarol
	if _, err := repo.Edit(read); err != nil {
		t.Fatalf("edit: %v", err)
	}

	edited, err := repo.Read(added.ID)
	if err != nil {
		t.Fatal(err)
	}
	if edited.Name != "Following" || edited.Access != "public" || !edited.Enabled {
		t.Errorf("after the edit: %q %q enabled %v", edited.Name, edited.Access, edited.Enabled)
	}
	if after, err := feeds.Parse(edited.Meta); err != nil || !after.Sources.Following {
		t.Errorf("definition after the edit: %s (%v)", edited.Meta, err)
	}

	// Listed among its owner's feeds, with the fixture's one of hers.
	req := requests.NewNodeRequest().Browse
	req.Types = []string{feeds.NodeType}
	req.OwnerIDs = []int64{feedCarol}
	req.Viewer = feedViewer(feedCarol)
	list, err := repo.Browse(req)
	got := nodeIDs(t, list, err)
	if !sameSet(got, feedOfGroup, added.ID) {
		t.Errorf("Carol's feeds are %v", sortedIDs(got))
	}

	// Gone, with whoever kept it on their home page.
	if _, err := db.Exec(`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,ordering,created_by,created_at) VALUES
		(?,?,'node.actor.person-service.person.v1',?,?,1,?,NOW())`, feeds.EdgePinType, feedAlice, added.ID, feeds.NodeType, feedAlice); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Delete(added.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	var kept int
	if err := db.QueryRow(`SELECT COUNT(*) FROM edges WHERE node_b_id = ?`, added.ID).Scan(&kept); err != nil || kept != 0 {
		t.Errorf("%d edges still point at a feed that is gone (%v)", kept, err)
	}
}
