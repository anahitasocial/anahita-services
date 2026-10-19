package repositories

import (
	"database/sql"
	"os"
	"testing"

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
