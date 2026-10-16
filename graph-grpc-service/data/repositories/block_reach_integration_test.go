package repositories

import (
	"database/sql"
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"

	sq "github.com/Masterminds/squirrel"
)

// What a block takes out of sight for everybody, against a real database:
// what the blocked person left on what is the blocker's. Run as the other
// access tests are (ACCESS_TEST_DSN; see access_integration_test.go).
//
//	1 Ana, who blocks 2 Bob. 4 Cleo reads.
//	30 Ana's note. 31 Bob's reply under it, 32 Cleo's.
//	33 Bob's note on Ana's profile. Bob and Cleo both like 30.
func seedBlockReach(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, stmt := range []string{
		`DELETE FROM edges`,
		`DELETE FROM nodes`,
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,created_at,updated_at) VALUES
		 (1,'node.actor.person-service.person.v1','person-service','Ana','ana','public',1,NOW(),NOW()),
		 (2,'node.actor.person-service.person.v1','person-service','Bob','bob','public',1,NOW(),NOW()),
		 (4,'node.actor.person-service.person.v1','person-service','Cleo','cleo','public',1,NOW(),NOW())`,
		`INSERT INTO nodes (id,type,service,name,body,access,enabled,owner_id,created_by,root_id,parent_id,created_at,updated_at) VALUES
		 (30,'node.medium.text-service.note.v1','text-service','','Beans','public',1,1,1,NULL,NULL,NOW(),NOW()),
		 (31,'node.medium.text-service.note.v1','text-service','','Rude','',1,2,2,30,30,NOW(),NOW()),
		 (32,'node.medium.text-service.note.v1','text-service','','Kind','',1,4,4,30,30,NOW(),NOW()),
		 (33,'node.medium.text-service.note.v1','text-service','','On her wall','public',1,1,2,NULL,NULL,NOW(),NOW())`,
		`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,created_at) VALUES
		 ('edge.base.voteup.v1',2,'node.actor.person-service.person.v1',30,'node.medium.text-service.note.v1',NOW()),
		 ('edge.base.voteup.v1',4,'node.actor.person-service.person.v1',30,'node.medium.text-service.note.v1',NOW()),
		 ('edge.actor.block.v1',1,'node.actor.person-service.person.v1',2,'node.actor.person-service.person.v1',NOW())`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}
}

func count(t *testing.T, db *sql.DB, column string, id int64) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COALESCE("+column+", 0) FROM nodes WHERE id = ?", id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// THE RULE THIS GUARDS: a block takes out of sight, for every reader, the
// replies, likes and posts the blocked person left on what is the blocker's,
// and gives them back when it is lifted.
func TestABlockHidesWhatTheBlockedLeftOnTheBlockers(t *testing.T) {
	db := accessTestDB(t)
	seedBlockReach(t, db)
	replies := NewRepliesRepo(db)
	likes := NewLikesRelRepo(db)
	graph := NewGraphRepo(db)

	cleo := entities.NewViewer()
	cleo.ID = 4
	cleo.PersonType = "registered"

	// Bob's reply is hidden in the thread, Cleo's is not.
	hidden, err := replies.HiddenIDs(30)
	if err != nil {
		t.Fatal(err)
	}
	if len(hidden) != 1 || hidden[0] != 31 {
		t.Errorf("hidden %v", hidden)
	}

	// And counted out.
	if err := replies.Recount(30, 0); err != nil {
		t.Fatal(err)
	}
	if n := count(t, db, "comment_count", 30); n != 1 {
		t.Errorf("replies counted %d", n)
	}
	if err := likes.UpdateLikeCount(30); err != nil {
		t.Fatal(err)
	}
	if n := count(t, db, "vote_up_count", 30); n != 1 {
		t.Errorf("likes counted %d", n)
	}

	// Cleo sees Cleo among the likers, and not Bob.
	likers, err := graph.BrowseNeighbours(&graphpb.BrowseNeighboursRequest{
		NodeId: 30, EdgeTypes: []string{constants.EdgeBaseLikeEntityType},
		Direction: constants.GraphQueryDirectionInbound, Limit: 20,
	}, cleo)
	if err != nil {
		t.Fatal(err)
	}
	if len(likers) != 1 || likers[0].ID != 4 {
		t.Errorf("likers %v", ids(likers))
	}

	// Bob's note on Ana's profile is out of sight; Ana's own is not.
	query, args, err := SetMediumAccessQuery(
		sq.Select("node.id").From("nodes AS node").
			LeftJoin("nodes AS owner ON node.owner_id = owner.id").
			Where(sq.Eq{"node.id": []int64{30, 33}}), cleo).ToSql()
	if err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[int64]bool{}
	for rows.Next() {
		var id int64
		_ = rows.Scan(&id)
		seen[id] = true
	}
	rows.Close()
	if !seen[30] || seen[33] {
		t.Errorf("Cleo sees %v", seen)
	}

	// What a block touches, for counting again.
	roots, liked, err := replies.TouchedByBlock(1, 2)
	if err != nil || len(roots) != 1 || roots[0] != 30 || len(liked) != 1 || liked[0] != 30 {
		t.Errorf("touched %v %v %v", roots, liked, err)
	}

	// Lifted: it all comes back.
	if _, err := db.Exec(`DELETE FROM edges WHERE type = 'edge.actor.block.v1'`); err != nil {
		t.Fatal(err)
	}
	if hidden, _ := replies.HiddenIDs(30); len(hidden) != 0 {
		t.Errorf("hidden after the block was lifted %v", hidden)
	}
	_ = replies.Recount(30, 0)
	_ = likes.UpdateLikeCount(30)
	if count(t, db, "comment_count", 30) != 2 || count(t, db, "vote_up_count", 30) != 2 {
		t.Errorf("counts after the block was lifted: %d replies, %d likes",
			count(t, db, "comment_count", 30), count(t, db, "vote_up_count", 30))
	}
}

func ids(nodes []*entities.AnahitaNode) []int64 {
	out := make([]int64, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, node.ID)
	}
	return out
}
