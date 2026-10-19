package handlers

import (
	"database/sql"
	"os"
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"graph/data/repositories"

	_ "github.com/go-sql-driver/mysql"
)

// A post's locations, tied to it as it is made, against a real database.
//
//	ACCESS_TEST_DSN="root@tcp(127.0.0.1:13309)/fresh?parseTime=true" \
//	  go test ./cmd/server/handlers/ -run TestAPostIsTagged -v
//
// WHAT THIS GUARDS (roadmap row 82, part 3b): up to five, the first chosen
// first, the same one once, and only locations: a group, a deleted location
// and an id with nothing behind it are passed over, and the post is made
// all the same.
func TestAPostIsTaggedWithItsLocationsAsItIsMade(t *testing.T) {
	dsn := os.Getenv("ACCESS_TEST_DSN")
	if dsn == "" {
		t.Skip("set ACCESS_TEST_DSN to run against MySQL")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	const location = "'node.tag.location-service.location.v1','location-service'"
	for _, stmt := range []string{
		`DELETE FROM edges`,
		`DELETE FROM nodes`,
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,created_at,updated_at) VALUES
		 (1,'node.actor.person-service.person.v1','person-service','Kira','kira','public',1,NOW(),NOW()),
		 (2,'node.actor.group-service.group.v1','group-service','Bajor','bajor','public',1,NOW(),NOW()),
		 (100,'node.medium.text-service.note.v1','text-service','','n100','public',1,NOW(),NOW()),
		 (11,` + location + `,'Promenade','promenade','public',1,NOW(),NOW()),
		 (12,` + location + `,'Ops','ops','public',1,NOW(),NOW()),
		 (13,` + location + `,'Quarks','quarks','public',1,NOW(),NOW()),
		 (14,` + location + `,'Infirmary','infirmary','public',1,NOW(),NOW()),
		 (15,` + location + `,'Habitat Ring','habitat-ring','public',1,NOW(),NOW()),
		 (16,` + location + `,'Docking Ring','docking-ring','public',1,NOW(),NOW()),
		 (17,` + location + `,'Gone','gone','public',0,NOW(),NOW())`,
		`UPDATE nodes SET deleted_at = NOW() WHERE id = 17`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}

	h := &mediumHandler{env: &Env{Repos: repositories.NewRepositories(db)}}
	post := entities.NewAnahitaNode()
	post.ID = 100
	post.Type = constants.NodeMediumNoteEntityType

	h.tagLocations(post, []int64{12, 12, 2, 17, 999, 11, 13, 14, 15, 16}, 1)

	rows, err := db.Query(`SELECT node_a_id FROM edges WHERE type = ? AND node_b_id = 100 ORDER BY id`, constants.EdgeTagLocationType)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := []int64{}
	for rows.Next() {
		var id int64
		_ = rows.Scan(&id)
		got = append(got, id)
	}
	want := []int64{12, 11, 13, 14, 15}
	if len(got) != len(want) {
		t.Fatalf("tagged with %v; want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tagged with %v; want %v", got, want)
		}
	}
}
