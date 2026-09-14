package repositories

import (
	"database/sql"
	"os"
	"testing"

	"anahita-libs/data/entities"

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
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,created_on,modified_on) VALUES
		 (1,'node.actor.person-service.person.v1','person-service','Alice','alice','public',1,NOW(),NOW()),
		 (2,'node.actor.person-service.person.v1','person-service','Bob','bob','public',1,NOW(),NOW()),
		 (3,'node.actor.group-service.group.v1','group-service','Cyclists','cyclists','public',1,NOW(),NOW())`,
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,owner_id,created_by,created_on,modified_on) VALUES
		 (10,'node.medium.photo-service.photo.v1','photo-service','Bob photo','p1','public',1,2,2,NOW(),NOW()),
		 (11,'node.medium.photo-service.photo.v1','photo-service','Bob in group','p2','public',1,3,2,NOW(),NOW()),
		 (12,'node.medium.photo-service.photo.v1','photo-service','Alice in group','p3','public',1,3,1,NOW(),NOW()),
		 (20,'node.medium.photo-service.photo.v1','photo-service','Alice private','p4','myself',1,1,1,NOW(),NOW())`,
		`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,created_on) VALUES
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
