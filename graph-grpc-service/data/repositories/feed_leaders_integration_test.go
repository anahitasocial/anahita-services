package repositories

import (
	"database/sql"
	"testing"

	"anahita-libs/proto/graphpb"
)

// What the Following feed holds of reposts, against a real database. Run as
// the other access tests are (ACCESS_TEST_DSN; see
// access_integration_test.go).
//
//	1 Naomi, who follows 3 Rose.  2 Moss, whom nobody follows.
//	10 Moss's note.   11 Rose's note.   20 Naomi's note.
//	21 Naomi's repost of 11.   22 Rose's repost of 10.
func seedLeadersReposts(t *testing.T, db *sql.DB) {
	t.Helper()
	const (
		person = "'node.actor.person-service.person.v1'"
		note   = "'node.medium.text-service.note.v1'"
		repost = "'node.base.repost-service.repost.v1'"
	)
	for _, stmt := range []string{
		`DELETE FROM edges`,
		`DELETE FROM nodes`,
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,created_at,updated_at) VALUES
		 (1,` + person + `,'person-service','Naomi','naomi','public',1,NOW(),NOW()),
		 (2,` + person + `,'person-service','Moss','moss','public',1,NOW(),NOW()),
		 (3,` + person + `,'person-service','Rose','rose','public',1,NOW(),NOW())`,
		`INSERT INTO nodes (id,type,service,body,access,enabled,owner_id,created_by,created_at,updated_at) VALUES
		 (10,` + note + `,'text-service','Moss on beans','public',1,2,2,NOW() - INTERVAL 6 MINUTE,NOW()),
		 (11,` + note + `,'text-service','Rose on beans','public',1,3,3,NOW() - INTERVAL 5 MINUTE,NOW()),
		 (20,` + note + `,'text-service','Naomi on beans','public',1,1,1,NOW() - INTERVAL 4 MINUTE,NOW())`,
		`INSERT INTO nodes (id,type,service,access,enabled,owner_id,parent_id,created_by,created_at,updated_at) VALUES
		 (21,` + repost + `,'feed-service','public',1,1,11,1,NOW() - INTERVAL 3 MINUTE,NOW()),
		 (22,` + repost + `,'feed-service','public',1,3,10,3,NOW() - INTERVAL 2 MINUTE,NOW())`,
		`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,created_at) VALUES
		 ('edge.actor.follow.v1',1,` + person + `,3,` + person + `,NOW())`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}
}

// THE RULE THIS GUARDS: the Following feed holds the viewer's own posts but
// not their own reposts, which are for others to see; reposts by those they
// follow stay in it.
func TestFeedLeadersLeavesOutOwnReposts(t *testing.T) {
	db := accessTestDB(t)
	seedLeadersReposts(t, db)
	leaders := NewFeedLeadersRepo(db)

	req := &graphpb.FeedLeadersBrowseRequest{MediumEntityTypes: feedNoteTypes, IncludeReposts: true, Limit: 50}
	nodes, err := leaders.Browse(req, feedViewer(1))
	seen := nodeIDs(t, nodes, err)

	for id, want := range map[int64]bool{11: true, 20: true, 22: true, 21: false, 10: false} {
		if seen[id] != want {
			t.Errorf("node %d in Naomi's Following: %v, want %v", id, seen[id], want)
		}
	}
}
