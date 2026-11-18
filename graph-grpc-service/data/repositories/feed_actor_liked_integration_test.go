package repositories

import (
	"database/sql"
	"testing"

	"anahita-libs/proto/graphpb"
)

// What a person liked, as their profile's Likes tab lists it, against a real
// database. Run as the other access tests are (ACCESS_TEST_DSN; see
// access_integration_test.go).
//
//	1 Naomi.  2 Moss.  3 Rose.
//	10 Moss's note, liked by Naomi first.   11 Rose's note, liked by Naomi last.
//	12 Rose's note, liked by Moss only.     13 Naomi's reply under 11, liked by Naomi.
//	14 Moss's note, liked by Naomi and removed.
func seedLiked(t *testing.T, db *sql.DB) {
	t.Helper()
	const (
		person = "'node.actor.person-service.person.v1'"
		note   = "'node.medium.text-service.note.v1'"
		like   = "'edge.base.voteup.v1'"
	)
	for _, stmt := range []string{
		`DELETE FROM edges`,
		`DELETE FROM nodes`,
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,created_at,updated_at) VALUES
		 (1,` + person + `,'person-service','Naomi','naomi','public',1,NOW(),NOW()),
		 (2,` + person + `,'person-service','Moss','moss','public',1,NOW(),NOW()),
		 (3,` + person + `,'person-service','Rose','rose','public',1,NOW(),NOW())`,
		`INSERT INTO nodes (id,type,service,body,access,enabled,owner_id,created_by,created_at,updated_at) VALUES
		 (10,` + note + `,'text-service','Moss on beans','public',1,2,2,NOW() - INTERVAL 1 MINUTE,NOW()),
		 (11,` + note + `,'text-service','Rose on beans','public',1,3,3,NOW() - INTERVAL 9 MINUTE,NOW()),
		 (12,` + note + `,'text-service','Rose on peas','public',1,3,3,NOW() - INTERVAL 8 MINUTE,NOW())`,
		`INSERT INTO nodes (id,type,service,body,access,enabled,owner_id,created_by,root_id,parent_id,created_at,updated_at) VALUES
		 (13,` + note + `,'text-service','Naomi under Rose','public',1,1,1,11,11,NOW() - INTERVAL 7 MINUTE,NOW())`,
		`INSERT INTO nodes (id,type,service,body,access,enabled,owner_id,created_by,deleted_at,created_at,updated_at) VALUES
		 (14,` + note + `,'text-service','Moss, removed','public',1,2,2,NOW(),NOW() - INTERVAL 6 MINUTE,NOW())`,
		`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,created_at) VALUES
		 (` + like + `,1,` + person + `,10,` + note + `,NOW() - INTERVAL 5 MINUTE),
		 (` + like + `,1,` + person + `,11,` + note + `,NOW() - INTERVAL 1 MINUTE),
		 (` + like + `,2,` + person + `,12,` + note + `,NOW()),
		 (` + like + `,1,` + person + `,13,` + note + `,NOW()),
		 (` + like + `,1,` + person + `,14,` + note + `,NOW())`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}
}

// THE RULE THIS GUARDS: a person's liked list holds the posts they liked,
// whoever posted them, the latest liked first; not what others liked, not
// replies, not a removed post.
func TestFeedActorLiked(t *testing.T) {
	db := accessTestDB(t)
	seedLiked(t, db)
	profiles := NewFeedActorRepo(db)

	req := &graphpb.FeedActorBrowseRequest{ActorId: 1, Filter: "liked", MediumEntityTypes: feedNoteTypes, Limit: 50}
	nodes, err := profiles.Browse(req, feedViewer(1))
	if err != nil {
		t.Fatal(err)
	}
	var got []int64
	for _, n := range nodes {
		got = append(got, n.ID)
	}
	if len(got) != 2 || got[0] != 11 || got[1] != 10 {
		t.Errorf("Naomi's liked: %v, want [11 10]", got)
	}

	total, err := profiles.Count(&graphpb.FeedActorCountRequest{ActorId: 1, Filter: "liked", MediumEntityTypes: feedNoteTypes}, feedViewer(1))
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Errorf("counted %d, want 2", total)
	}
}
