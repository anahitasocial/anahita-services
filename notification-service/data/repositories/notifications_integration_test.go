package repositories

import (
	"database/sql"
	"os"
	"testing"

	an_entities "anahita-libs/data/entities"
	"notification/data/entities"

	_ "github.com/go-sql-driver/mysql"
)

// Against a MySQL made from migration-job/schema/init.sql, as graph-grpc-
// service's integration tests are:
//
//	ACCESS_TEST_DSN="root@tcp(127.0.0.1:13309)/fresh?parseTime=true" \
//	  go test ./data/repositories/ -run TestOneNotificationForOneThing -v
//
// WHAT THIS GUARDS (roadmap row 96, part 3): an event heard twice makes one
// row, not two, moved to the top and unread again; a follow, which has no
// object, is one row too, NULLs and all.
func TestOneNotificationForOneThing(t *testing.T) {
	dsn := os.Getenv("ACCESS_TEST_DSN")
	if dsn == "" {
		t.Skip("set ACCESS_TEST_DSN to run against MySQL")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`DELETE FROM notifications`); err != nil {
		t.Fatal(err)
	}
	repo := NewNotification(db)

	like := func(object int64) *entities.Notification {
		n := entities.NewNotification()
		n.Type = "voteup"
		n.Service = "like-service"
		n.RecipientID = 1 // Sisko
		n.Subject.ID, n.Subject.Type = 2, "node.actor.person-service.person.v1"
		n.Object.ID, n.Object.Type = object, "node.medium.text-service.note.v1"
		n.Target.ID, n.Target.Type = 1, "node.actor.person-service.person.v1"
		return n
	}

	first, err := repo.Add(like(100))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE notifications SET is_read = 1, created_at = '2026-01-01' WHERE id = ?`, first); err != nil {
		t.Fatal(err)
	}
	again, err := repo.Add(like(100))
	if err != nil {
		t.Fatal(err)
	}
	if again != first {
		t.Errorf("told again: id %d, want the same row, %d", again, first)
	}
	var rows, isRead int
	if err := db.QueryRow(`SELECT COUNT(*), MAX(is_read) FROM notifications`).Scan(&rows, &isRead); err != nil {
		t.Fatal(err)
	}
	if rows != 1 || isRead != 0 {
		t.Errorf("told again: %d rows, is_read %d; want 1 row, unread", rows, isRead)
	}
	var moved bool
	if err := db.QueryRow(`SELECT created_at > '2026-01-01' FROM notifications WHERE id = ?`, first).Scan(&moved); err != nil {
		t.Fatal(err)
	}
	if !moved {
		t.Error("told again: the row was not moved to the top")
	}

	// Another post is another thing.
	if _, err := repo.Add(like(101)); err != nil {
		t.Fatal(err)
	}

	// A follow has no object: NULL, which a plain unique key would let
	// repeat.
	follow := func() *entities.Notification {
		n := entities.NewNotification()
		n.Type = "actor_follow"
		n.Service = "graph-grpc-service"
		n.RecipientID = 1
		n.Subject.ID, n.Subject.Type = 3, "node.actor.person-service.person.v1" // Dax
		n.Target.ID, n.Target.Type = 1, "node.actor.person-service.person.v1"
		return n
	}
	for i := 0; i < 2; i++ {
		if _, err := repo.Add(follow()); err != nil {
			t.Fatal(err)
		}
	}

	if err := db.QueryRow(`SELECT COUNT(*) FROM notifications`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 3 {
		t.Errorf("%d rows, want 3: one like of each post and one follow", rows)
	}
}

// WHAT THIS GUARDS (roadmap row 96, part 2): a post gone takes every
// notification about it, whoever did what it tells of, and nothing else.
//
//	ACCESS_TEST_DSN=... go test ./data/repositories/ -run TestAPostGoneTakesItsNews -v
func TestAPostGoneTakesItsNews(t *testing.T) {
	dsn := os.Getenv("ACCESS_TEST_DSN")
	if dsn == "" {
		t.Skip("set ACCESS_TEST_DSN to run against MySQL")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`DELETE FROM notifications`); err != nil {
		t.Fatal(err)
	}

	// Post 100 by Sisko (1); 200 is another of his posts; 300 a reply under
	// 100; 400 a repost of 100 by Kira (4).
	if _, err := db.Exec(`INSERT INTO notifications
		(type, service, recipient_id, subject_id, subject_type, object_id, object_type, target_id, target_type, comment_id)
		VALUES
		('voteup', 'like-service', 1, 2, 'p', 100, 'n', 1, 'p', NULL),
		('reply_add', 'text-service', 1, 3, 'p', 100, 'n', 1, 'p', 300),
		('repost_add', 'feed-service', 1, 4, 'p', 100, 'n', 4, 'p', 400),
		('actor_mention', 'actor-service', 5, 1, 'p', 100, 'n', 5, 'p', NULL),
		('voteup', 'like-service', 1, 2, 'p', 200, 'n', 1, 'p', NULL),
		('actor_follow', 'graph-grpc-service', 1, 2, 'p', NULL, '', 1, 'p', NULL)`); err != nil {
		t.Fatal(err)
	}
	repo := NewNotification(db)

	count := func() (n int) {
		if err := db.QueryRow(`SELECT COUNT(*) FROM notifications`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// Kira takes her repost back: only its news goes.
	if err := repo.DeleteAbout(400); err != nil {
		t.Fatal(err)
	}
	if got := count(); got != 5 {
		t.Errorf("after the repost went: %d rows, want 5", got)
	}

	// The post goes, by whoever's hand: its like, reply and mention go too.
	if err := repo.DeleteAbout(100); err != nil {
		t.Fatal(err)
	}
	var left string
	if err := db.QueryRow(`SELECT GROUP_CONCAT(CONCAT(type, ':', IFNULL(object_id, 0)) ORDER BY type) FROM notifications`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != "actor_follow:0,voteup:200" {
		t.Errorf("after the post went, left: %s; want the follow and the other post's like", left)
	}
}

// WHAT THIS GUARDS (roadmap row 96, part 1): a block takes what the two
// were told of each other, a group's administrators' news of the blocked
// person included, and nobody else's.
//
//	ACCESS_TEST_DSN=... go test ./data/repositories/ -run TestABlockTakesWhatTheTwoWereToldOfEachOther -v
func TestABlockTakesWhatTheTwoWereToldOfEachOther(t *testing.T) {
	dsn := os.Getenv("ACCESS_TEST_DSN")
	if dsn == "" {
		t.Skip("set ACCESS_TEST_DSN to run against MySQL")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`DELETE FROM notifications`); err != nil {
		t.Fatal(err)
	}

	// Quark (1) blocks Rom (2); Bajor (10) is a group Kira (3) administers.
	if _, err := db.Exec(`INSERT INTO notifications
		(type, service, recipient_id, subject_id, subject_type, object_id, object_type, target_id, target_type)
		VALUES
		('voteup', 'like-service', 1, 2, 'p', 100, 'n', 1, 'p'),
		('voteup', 'like-service', 2, 1, 'p', 200, 'n', 2, 'p'),
		('actor_follow', 'graph-grpc-service', 3, 2, 'p', NULL, '', 10, 'g'),
		('voteup', 'like-service', 1, 3, 'p', 100, 'n', 1, 'p'),
		('actor_follow', 'graph-grpc-service', 3, 4, 'p', NULL, '', 10, 'g')`); err != nil {
		t.Fatal(err)
	}
	repo := NewNotification(db)

	if err := repo.DeleteBetween(1, 2); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteBetween(10, 2); err != nil { // Bajor blocks Rom
		t.Fatal(err)
	}

	var left string
	if err := db.QueryRow(`SELECT GROUP_CONCAT(CONCAT(recipient_id, '<', subject_id) ORDER BY id) FROM notifications`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != "1<3,3<4" {
		t.Errorf("left %s, want 1<3,3<4: Kira's like and Odo's follow of Bajor", left)
	}
}

// WHAT THIS GUARDS (roadmap row 96, part 4): the bell's question answers the
// unread count and when the newest was told, which moves when something is
// told again although its row and the count stay the same.
func TestTheBellsQuestion(t *testing.T) {
	dsn := os.Getenv("ACCESS_TEST_DSN")
	if dsn == "" {
		t.Skip("set ACCESS_TEST_DSN to run against MySQL")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`DELETE FROM notifications`); err != nil {
		t.Fatal(err)
	}
	repo := NewNotification(db)
	viewer := an_entities.NewViewer()
	viewer.ID = 1

	if count, newest, err := repo.UnreadCount(viewer); err != nil || count != 0 || newest != nil {
		t.Fatalf("nothing yet: %d, %v, %v", count, newest, err)
	}

	if _, err := db.Exec(`INSERT INTO notifications
		(type, service, recipient_id, subject_id, subject_type, object_id, object_type, target_id, target_type, is_read, created_at)
		VALUES
		('voteup', 'like-service', 1, 2, 'p', 100, 'n', 1, 'p', 1, '2026-10-01'),
		('voteup', 'like-service', 1, 3, 'p', 100, 'n', 1, 'p', 0, '2026-10-02')`); err != nil {
		t.Fatal(err)
	}
	count, before, err := repo.UnreadCount(viewer)
	if err != nil || count != 1 || before == nil || before.Format("2006-01-02") != "2026-10-02" {
		t.Fatalf("got %d, %v, %v; want 1 unread, newest 2026-10-02", count, before, err)
	}

	// The read one is told again: unread, and newest.
	n := entities.NewNotification()
	n.Type, n.Service, n.RecipientID = "voteup", "like-service", 1
	n.Subject.ID, n.Subject.Type = 2, "p"
	n.Object.ID, n.Object.Type = 100, "n"
	n.Target.ID, n.Target.Type = 1, "p"
	if _, err := repo.Add(n); err != nil {
		t.Fatal(err)
	}
	count, after, err := repo.UnreadCount(viewer)
	if err != nil || count != 2 || after == nil || !after.After(*before) {
		t.Errorf("told again: %d unread, newest %v (was %v); want 2, and later", count, after, before)
	}
}

// WHAT THIS GUARDS: Mark all read marks the viewer's own, all of them, and
// nobody else's.
func TestMarkAllRead(t *testing.T) {
	dsn := os.Getenv("ACCESS_TEST_DSN")
	if dsn == "" {
		t.Skip("set ACCESS_TEST_DSN to run against MySQL")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`DELETE FROM notifications`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO notifications
		(type, service, recipient_id, subject_id, subject_type, object_id, object_type, target_id, target_type, is_read)
		VALUES
		('voteup', 'like-service', 1, 2, 'p', 100, 'n', 1, 'p', 0),
		('voteup', 'like-service', 1, 3, 'p', 100, 'n', 1, 'p', 0),
		('voteup', 'like-service', 1, 4, 'p', 100, 'n', 1, 'p', 1),
		('voteup', 'like-service', 7, 2, 'p', 200, 'n', 7, 'p', 0)`); err != nil {
		t.Fatal(err)
	}
	viewer := an_entities.NewViewer()
	viewer.ID = 1

	marked, err := NewNotification(db).EditAll(viewer)
	if err != nil || marked != 2 {
		t.Fatalf("marked %d, %v; want 2", marked, err)
	}
	var mine, theirs int
	if err := db.QueryRow(`SELECT SUM(recipient_id = 1 AND is_read = 0), SUM(recipient_id = 7 AND is_read = 0) FROM notifications`).Scan(&mine, &theirs); err != nil {
		t.Fatal(err)
	}
	if mine != 0 || theirs != 1 {
		t.Errorf("unread: %d of mine, %d of somebody else's; want 0 and 1", mine, theirs)
	}
}
