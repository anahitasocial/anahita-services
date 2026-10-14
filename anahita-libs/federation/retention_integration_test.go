package federation

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Against MySQL, with the schema of schema/init.sql:
//
//	FEDERATION_TEST_DSN="root@tcp(127.0.0.1:13309)/fresh?parseTime=true" \
//	  go test ./federation/ -run Retention -p 1 -count=1
//
// It empties nodes, edges, photo_files and abuse_cases: a throwaway
// database only.
func retentionDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("FEDERATION_TEST_DSN")
	if dsn == "" {
		t.Skip("set FEDERATION_TEST_DSN to run against MySQL")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

const (
	bluesky = "node.actor.atproto-service.person.v1"
	local   = "node.actor.person-service.person.v1"
	note    = "node.medium.text-service.note.v1"
	photo   = "node.medium.photo-service.photo.v1"
)

func seedRetention(t *testing.T, db *sql.DB) {
	t.Helper()
	long := "NOW() - INTERVAL 60 DAY"
	lately := "NOW() - INTERVAL 2 DAY"
	for _, stmt := range []string{
		`DELETE FROM abuse_cases`, `DELETE FROM photo_files`, `DELETE FROM edges`, `DELETE FROM nodes`,
		// Ana is here. Alice is followed from here, Bob is not, Carol is
		// nobody's, and Dan is nobody's but blocked by Ana.
		`INSERT INTO nodes (id,type,service,name,alias,access,body,enabled,uri,created_at,updated_at) VALUES
		 (1,'` + local + `','person-service','Ana','ana','public','',1,NULL,NOW(),NOW()),
		 (10,'` + bluesky + `','atproto-service','Alice','alice.test','public','',1,'at://did:plc:alice',` + long + `,` + long + `),
		 (20,'` + bluesky + `','atproto-service','Bob','bob.test','public','',1,'at://did:plc:bob',` + long + `,` + long + `),
		 (30,'` + bluesky + `','atproto-service','Carol','carol.test','public','',1,'at://did:plc:carol',` + long + `,` + long + `),
		 (40,'` + bluesky + `','atproto-service','Dan','dan.test','public','',1,'at://did:plc:dan',` + long + `,` + long + `),
		 (50,'` + bluesky + `','atproto-service','Eve','eve.test','public','',1,'at://did:plc:eve',` + lately + `,` + lately + `),
		 (60,'` + bluesky + `','atproto-service','Fay','fay.test','public','',1,'at://did:plc:fay',` + long + `,` + long + `)`,
		`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,created_at) VALUES
		 ('edge.actor.follow.v1',1,'` + local + `',10,'` + bluesky + `',NOW()),
		 ('edge.actor.block.v1',1,'` + local + `',40,'` + bluesky + `',NOW())`,
		// Posts: owner, how long left alone, and what depends on them.
		`INSERT INTO nodes (id,type,service,name,alias,access,body,enabled,uri,owner_id,created_by,parent_id,root_id,quote_id,created_at,updated_at) VALUES
		 (101,'` + photo + `','photo-service','','','public','old, of Alice',1,'at://a/101',10,10,NULL,NULL,NULL,` + long + `,` + long + `),
		 (102,'` + note + `','text-service','','','public','new, of Alice',1,'at://a/102',10,10,NULL,NULL,NULL,` + lately + `,` + lately + `),
		 (103,'` + note + `','text-service','','','public','newest, of Alice',1,'at://a/103',10,10,NULL,NULL,NULL,NOW(),NOW()),
		 (201,'` + photo + `','photo-service','','','public','old, of Bob',1,'at://b/201',20,20,NULL,NULL,NULL,` + long + `,` + long + `),
		 (202,'` + note + `','text-service','','','public','new, of Bob',1,'at://b/202',20,20,NULL,NULL,NULL,` + lately + `,` + lately + `),
		 (203,'` + note + `','text-service','','','public','old, of Bob, liked here',1,'at://b/203',20,20,NULL,NULL,NULL,` + long + `,` + long + `),
		 (204,'` + note + `','text-service','','','public','old, of Bob, answered here',1,'at://b/204',20,20,NULL,NULL,NULL,` + long + `,` + long + `),
		 (205,'` + note + `','text-service','','','public','old, of Bob, reported',1,'at://b/205',20,20,NULL,NULL,NULL,` + long + `,` + long + `),
		 (206,'` + note + `','text-service','','','public','old, of Bob, reposted by Alice',1,'at://b/206',20,20,NULL,NULL,NULL,` + long + `,` + long + `),
		 (207,'` + note + `','text-service','','','public','old, of Bob, an answer',1,'at://b/207',20,20,204,204,NULL,` + long + `,` + long + `),
		 (301,'` + note + `','text-service','','','public','an answer here to 204',1,NULL,1,1,204,204,NULL,NOW(),NOW()),
		 (302,'node.base.repost-service.repost.v1','repost-service','','','public','',1,NULL,10,10,206,NULL,NULL,NOW(),NOW()),
		 (303,'` + note + `','text-service','','','public','written here, long ago',1,NULL,1,1,NULL,NULL,NULL,` + long + `,` + long + `)`,
		`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,created_at) VALUES
		 ('edge.base.voteup.v1',1,'` + local + `',203,'` + note + `',NOW())`,
		`INSERT INTO photo_files (node_id,position,filename) VALUES (101,0,'a.jpg'),(201,0,'b.jpg')`,
		`INSERT INTO abuse_cases (target_id,target_type,status,first_reported_at,last_reported_at) VALUES (205,'` + note + `','open',NOW(),NOW()),(60,'` + bluesky + `','open',NOW(),NOW())`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%v\n%s", err, stmt)
		}
	}
}

// THE RULE THIS GUARDS: only what came from elsewhere, begins a thread and
// has nothing here depending on it is ever due to go; and with no rule
// set, nothing is.
func TestRetentionSaysWhatIsDue(t *testing.T) {
	db := retentionDB(t)
	seedRetention(t, db)
	now := time.Now()

	due := func(rules Retention) string {
		ids, err := OldPosts(db, bluesky, rules, now, 100)
		if err != nil {
			t.Fatal(err)
		}
		return fmt.Sprint(ids)
	}

	if got := due(Retention{}); got != "[]" {
		t.Errorf("with no rule set: %s", got)
	}
	// Of somebody nobody follows, and left alone: Bob's old photo. Not
	// what is liked, answered, reported or reposted, not an answer, not
	// what is new, and nothing of Alice, who is followed.
	if got := due(Retention{UnfollowedDays: 30}); got != "[201]" {
		t.Errorf("unfollowed: %s", got)
	}
	// Left alone, whoever wrote it: Alice's old one too.
	if got := due(Retention{PostsDays: 30}); got != "[101 201]" {
		t.Errorf("old: %s", got)
	}
	// Past the newest two of each: Alice's oldest. Bob's oldest is 201,
	// 203, 204, 205 and 206; all but 201 are held to.
	if got := due(Retention{PostsKept: 2}); got != "[101 201]" {
		t.Errorf("many: %s", got)
	}
	if got := due(Retention{PostsKept: 3}); got != "[201]" {
		t.Errorf("many, three kept: %s", got)
	}
	// Another kind of server's are not this one's to look at.
	if ids, _ := OldPosts(db, "node.actor.activitypub-service.person.v1", Retention{PostsDays: 30}, now, 100); len(ids) != 0 {
		t.Errorf("of another kind of server: %v", ids)
	}

	// Pictures: of the two old photo posts, both, when nothing holds them.
	pictures, err := OldPicturePosts(db, bluesky, now.AddDate(0, 0, -30), 100)
	if err != nil || fmt.Sprint(pictures) != "[101 201]" {
		t.Errorf("pictures due: %v, %v", pictures, err)
	}

	// People: Carol, of whom nothing is left and at whom nothing points.
	// Not Alice (followed, has posts), Bob (has posts), Dan (blocked by
	// somebody here), Eve (looked up two days ago), nor Fay (reported,
	// and the report not yet looked at).
	people, err := StalePeople(db, bluesky, Retention{UnfollowedDays: 30}, now, 100)
	if err != nil || fmt.Sprint(people) != "[30]" {
		t.Errorf("people due: %v, %v", people, err)
	}
	if people, _ := StalePeople(db, bluesky, Retention{}, now, 100); len(people) != 0 {
		t.Errorf("people due with the rule off: %v", people)
	}
}
