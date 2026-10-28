package repositories

import (
	"testing"
	"time"

	"anahita-libs/constants"
	"anahita-libs/feeds"
)

// The tag sweep against a real database.
//
//	ACCESS_TEST_DSN="root@tcp(127.0.0.1:13309)/fresh?parseTime=true" \
//	  go test ./data/repositories/ -run TestTagSweep -v
//
// WHAT THESE GUARD. A hashtag or location a post carries is never removed,
// nor one made in the last day, nor a location a custom feed names; the
// rest go. And a post tagged after the read keeps its tag.
func TestTagSweep(t *testing.T) {
	db := accessTestDB(t)
	repo := NewTagsRepo(db)

	const (
		person   = "node.actor.person-service.person.v1"
		note     = "node.medium.text-service.note.v1"
		hashtag  = constants.NodeBaseHashtagEntityType
		location = constants.NodeBaseLocationEntityType
		old      = "NOW() - INTERVAL 3 DAY"
		recent   = "NOW() - INTERVAL 1 HOUR"
	)

	for _, stmt := range []string{
		`DELETE FROM edges`,
		`DELETE FROM nodes`,
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,created_at,updated_at) VALUES
		 (1,'` + person + `','person-service','Alice','alice','public',1,NOW(),NOW()),
		 (2,'` + note + `','text-service','','','public',1,NOW(),NOW()),
		 (10,'` + hashtag + `','hashtag-service','used','used','public',1,` + old + `,NOW()),
		 (11,'` + hashtag + `','hashtag-service','unused','unused','public',1,` + old + `,NOW()),
		 (12,'` + hashtag + `','hashtag-service','new','new','public',1,` + recent + `,NOW()),
		 (20,'` + location + `','location-service','Used','used-place','public',1,` + old + `,NOW()),
		 (21,'` + location + `','location-service','Unused','unused-place','public',1,` + old + `,NOW()),
		 (22,'` + location + `','location-service','In a feed','feed-place','public',1,` + old + `,NOW())`,
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,owner_id,meta,created_at,updated_at) VALUES
		 (30,'` + feeds.NodeType + `','feed-service','Near','','private',1,1,
		  '{"definition":{"filters":{"locations_any":[22]}}}',NOW(),NOW())`,
		`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,created_at) VALUES
		 ('` + constants.EdgeTagHashtagType + `',10,'` + hashtag + `',2,'` + note + `',NOW()),
		 ('` + constants.EdgeTagLocationType + `',20,'` + location + `',2,'` + note + `',NOW())`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}

	exists := func(id int64) bool {
		t.Helper()
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE id = ?`, id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n == 1
	}

	swept, err := repo.SweepUnused(time.Now().UTC().Add(-24 * time.Hour))
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if swept.Hashtags != 1 || swept.Locations != 1 {
		t.Errorf("swept %+v, want one hashtag and one location", swept)
	}

	for id, want := range map[int64]bool{
		10: true,  // a post carries it
		11: false, // nothing does, and it is old
		12: true,  // made an hour ago
		20: true,  // a post carries it
		21: false, // nothing does, and no feed names it
		22: true,  // a custom feed names it
		2:  true,  // the post itself
		30: true,  // the feed itself
	} {
		if got := exists(id); got != want {
			t.Errorf("node %d exists = %v, want %v", id, got, want)
		}
	}

	// Again, with nothing left to do.
	if swept, err = repo.SweepUnused(time.Now().UTC().Add(-24 * time.Hour)); err != nil || swept != (TagSweep{}) {
		t.Errorf("second sweep: %+v, %v; want nothing", swept, err)
	}
}

// A post tagged between the sweep's read and its delete keeps its tag: the
// delete asks again.
func TestTagSweepDeleteAsksAgain(t *testing.T) {
	db := accessTestDB(t)
	repo := &tagsRepo{db: db, dbTimeout: constants.DBTimeout}

	const (
		note    = "node.medium.text-service.note.v1"
		hashtag = constants.NodeBaseHashtagEntityType
	)

	for _, stmt := range []string{
		`DELETE FROM edges`,
		`DELETE FROM nodes`,
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,created_at,updated_at) VALUES
		 (2,'` + note + `','text-service','','','public',1,NOW(),NOW()),
		 (11,'` + hashtag + `','hashtag-service','late','late','public',1,NOW() - INTERVAL 3 DAY,NOW())`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}

	ids, err := repo.unused(selectUnusedHashtags, hashtag, time.Now().UTC(), constants.EdgeTagHashtagType, tagSweepBatch)
	if err != nil || len(ids) != 1 {
		t.Fatalf("read: %v, %v; want the one hashtag", ids, err)
	}

	// Tagged now, after the read.
	if _, err := db.Exec(`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,created_at) VALUES (?,11,?,2,?,NOW())`,
		constants.EdgeTagHashtagType, hashtag, note); err != nil {
		t.Fatal(err)
	}

	n, err := repo.remove(ids, hashtag, constants.EdgeTagHashtagType)
	if err != nil || n != 0 {
		t.Errorf("remove: %d, %v; want nothing removed", n, err)
	}
}
