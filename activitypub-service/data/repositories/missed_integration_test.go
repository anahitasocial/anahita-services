package repositories

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"anahita-libs/constants"

	_ "github.com/go-sql-driver/mysql"
)

// Against a real MySQL, never the development one:
//
//	docker run -d --name anahita-acl-test -e MYSQL_ALLOW_EMPTY_PASSWORD=1 \
//	  -e MYSQL_DATABASE=fresh -p 13309:3306 mysql:9.6
//	docker exec -i anahita-acl-test mysql -uroot fresh < migration-job/schema/init.sql
//	ACTIVITYPUB_TEST_DSN="root@tcp(127.0.0.1:13309)/fresh?parseTime=true" go test ./data/repositories/
//	docker rm -fv anahita-acl-test
func missedDB(t *testing.T) *sql.DB {
	dsn := os.Getenv("ACTIVITYPUB_TEST_DSN")
	if dsn == "" {
		t.Skip("ACTIVITYPUB_TEST_DSN is not set")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	for _, table := range []string{"edges", "nodes", "activitypub_follows", "activitypub_objects", "activitypub_accounts"} {
		if _, err := db.Exec("DELETE FROM " + table); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

// THE RULE THIS GUARDS: the sweep finds what other servers were never told,
// and nothing they were told, nothing too new to call lost, and nothing
// from before somebody joined the fediverse.
func TestWhatTheBusLostIsFound(t *testing.T) {
	db := missedDB(t)
	repo := NewMissed(db)

	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	hour := func(n int) time.Time { return now.Add(time.Duration(-n) * time.Hour) }
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.Exec(query, args...); err != nil {
			t.Fatal(err)
		}
	}

	const (
		ana, ben     = 1, 2 // here: Ana is on the fediverse, Ben is not
		alice, carol = 10, 11
		person       = constants.NodeActorPersonEntityType
		remote       = constants.NodeActorActivityPubPersonEntityType
	)
	node := func(id int64, kind string, owner, author int64, access string, enabled int, made time.Time, uri any) {
		exec(`INSERT INTO nodes (id, type, service, name, alias, body, owner_id, created_by, access, enabled, created_at, uri)
VALUES (?, ?, 'text-service', 'n', ?, '', ?, ?, ?, ?, ?, ?)`, id, kind, fmt.Sprintf("n%d", id), owner, author, access, enabled, made, uri)
	}
	follow := func(a int64, aType string, b int64, made time.Time) {
		exec(`INSERT INTO edges (type, node_a_id, node_a_type, node_b_id, node_b_type, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
			constants.EdgeFollowEntityType, a, aType, b, remote, made)
	}
	asked := func(local, far int64, direction string) {
		exec(`INSERT INTO activitypub_follows (local_id, remote_id, direction, activity_id, state, created_at, updated_at)
VALUES (?, ?, ?, ?, 'accepted', ?, ?)`, local, far, direction, fmt.Sprintf("a-%d-%d-%s", local, far, direction), now, now)
	}

	exec(`INSERT INTO activitypub_accounts (person_id, enabled, enabled_at, created_at, updated_at) VALUES (?, 1, ?, ?, ?), (?, 0, NULL, ?, ?)`,
		ana, hour(10), now, now, ben, now, now)

	// Follows. Ana > Alice was never asked about; Ana > Carol was. Ben >
	// Alice is found too (he is not on the fediverse, and whoever handles
	// it takes the follow away). Alice > Ana is hers, not ours to ask. One
	// made a moment ago is not yet lost.
	follow(ana, person, alice, hour(1))
	follow(ana, person, carol, hour(1))
	asked(ana, carol, "out")
	follow(ben, person, alice, hour(1))
	exec(`INSERT INTO edges (type, node_a_id, node_a_type, node_b_id, node_b_type, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		constants.EdgeFollowEntityType, alice, remote, ana, person, hour(1))
	follow(3, person, alice, now)

	found, err := repo.FollowsNotAsked(now.Add(-2*time.Minute), 50)
	if err != nil || fmt.Sprint(found) != "[{1 10} {2 10}]" {
		t.Errorf("follows not asked: %v (%v)", found, err)
	}

	// A follow on record with no follow in the graph. One the other way is
	// not looked at.
	asked(ana, 12, "out")
	asked(ana, 13, "in")
	gone, err := repo.FollowsGone(50)
	if err != nil || fmt.Sprint(gone) != "[{1 12}]" {
		t.Errorf("follows gone: %v (%v)", gone, err)
	}

	// Notes. 100 is the one that was lost.
	note := constants.NodeMediumNoteEntityType
	node(100, note, ana, ana, "public", 1, hour(3), nil)
	node(101, note, ana, ana, "public", 1, hour(3), nil) // sent
	exec(`INSERT INTO activitypub_objects (node_id, uuid, actor_id, created_at) VALUES (101, 'u-101', ?, ?)`, ana, now)
	node(102, note, ana, ana, "followers", 1, hour(3), nil)                               // not for everybody
	node(103, note, ana, ana, "public", 1, hour(20), nil)                                 // before she joined
	node(104, note, ana, ana, "public", 1, now, nil)                                      // a moment ago
	node(105, note, ben, ben, "public", 1, hour(3), nil)                                  // not on the fediverse
	node(106, note, 50, ana, "public", 1, hour(3), nil)                                   // on somebody else's profile
	node(107, note, ana, ana, "public", 1, hour(72), nil)                                 // too long ago to send as new
	node(108, "node.medium.text-service.article.v1", ana, ana, "public", 1, hour(3), nil) // not a note
	node(109, note, ana, ana, "public", 0, hour(3), nil)                                  // disabled

	notes, err := repo.NotesNotSent(now.Add(-48*time.Hour), now.Add(-2*time.Minute), 50)
	if err != nil || fmt.Sprint(notes) != "[100]" {
		t.Errorf("notes not sent: %v (%v)", notes, err)
	}

	// Sent, and since gone, closed or disabled.
	exec(`INSERT INTO activitypub_objects (node_id, uuid, actor_id, created_at) VALUES (102, 'u-102', ?, ?), (109, 'u-109', ?, ?), (999, 'u-999', ?, ?)`,
		ana, now, ana, now, ana, now)
	back, err := repo.NotesGone(50)
	if err != nil || fmt.Sprint(back) != "[102 109 999]" {
		t.Errorf("notes gone: %v (%v)", back, err)
	}

	people, err := repo.People(50)
	if err != nil || fmt.Sprint(people) != "[1]" {
		t.Errorf("people: %v (%v)", people, err)
	}
}
