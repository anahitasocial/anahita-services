package repositories

import (
	"encoding/json"
	"testing"
)

// Making everything public registered-only, against a real MySQL.
//
//	ACCESS_TEST_DSN="root@tcp(127.0.0.1:13309)/fresh?parseTime=true" \
//	  go test ./data/repositories/ -run TestPublicAccess -v
func TestPublicAccessIsCountedAndMadeRegistered(t *testing.T) {
	db := accessTestDB(t)

	const person = "'node.actor.person-service.person.v1','person-service'"
	const group = "'node.actor.group-service.group.v1','group-service'"
	const note = "'node.medium.text-service.note.v1','text-service'"

	for _, stmt := range []string{
		`DELETE FROM edges`,
		`DELETE FROM nodes`,
		`DELETE FROM site_actions`,
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,created_at,updated_at) VALUES
		 (1,` + person + `,'Ada','ada','public',1,NOW(),NOW()),
		 (2,` + person + `,'Grace','grace','public',1,NOW(),NOW()),
		 (3,` + person + `,'Private','private','followers',1,NOW(),NOW()),
		 (8,` + group + `,'Circle','circle','public',1,NOW(),NOW()),
		 (9,` + group + `,'Draft','draft','admins',1,NOW(),NOW()),
		 (10,` + note + `,'n10','n10','public',1,NOW(),NOW()),
		 (11,` + note + `,'n11','n11','public',1,NOW(),NOW()),
		 (12,` + note + `,'n12','n12','public',1,NOW(),NOW()),
		 (13,` + note + `,'n13','n13','myself',1,NOW(),NOW()),
		 (14,'node.base.text-service.article-revision.v1','text-service','r14','r14','public',1,NOW(),NOW()),
		 (20,` + note + `,'c20','c20','',1,NOW(),NOW()),
		 (30,'node.tag.hashtag-service.hashtag.v1','hashtag-service','seeds','seeds','public',1,NOW(),NOW())`,
		// A reply to the first note.
		`UPDATE nodes SET root_id = 10, parent_id = 10 WHERE id = 20`,
		`UPDATE nodes SET updated_at = '2020-01-01 00:00:00'`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}

	repo := NewPublicAccessRepo(db)

	before, err := repo.Count()
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	// Two people, one group, three notes and a revision. Not the tag.
	if before.People != 2 || before.Groups != 1 || before.Posts != 4 || before.Total() != 7 {
		t.Fatalf("counted %+v", before)
	}

	const superAdmin = 99
	changed, err := repo.MakeRegistered(superAdmin)
	if err != nil {
		t.Fatalf("make registered: %v", err)
	}
	if *changed != *before {
		t.Errorf("changed %+v, counted %+v", changed, before)
	}

	access := func(id int64) string {
		var value string
		if err := db.QueryRow(`SELECT COALESCE(access, '') FROM nodes WHERE id = ?`, id).Scan(&value); err != nil {
			t.Fatalf("reading %d: %v", id, err)
		}
		return value
	}

	for id, want := range map[int64]string{
		1: "registered", 8: "registered", 10: "registered", 14: "registered",
		// What was narrower stays as it was.
		3: "followers", 9: "admins", 13: "myself",
		// A reply says nothing of its own; it follows its post.
		20: "",
		// A tag is nobody's content.
		30: "public",
	} {
		if got := access(id); got != want {
			t.Errorf("node %d is %q, want %q", id, got, want)
		}
	}

	// Nobody edited the posts.
	var touched int
	if err := db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE updated_at > '2021-01-01'`).Scan(&touched); err != nil {
		t.Fatal(err)
	}
	if touched != 0 {
		t.Errorf("%d rows had their updated_at moved", touched)
	}

	// Recorded once, with who and how much.
	var action, detail string
	var by int64
	if err := db.QueryRow(`SELECT action, performed_by, detail FROM site_actions`).Scan(&action, &by, &detail); err != nil {
		t.Fatalf("reading the record: %v", err)
	}
	recorded := &PublicAccessCounts{}
	if err := json.Unmarshal([]byte(detail), recorded); err != nil {
		t.Fatalf("the record's detail is not JSON: %q", detail)
	}
	if action != SiteActionMadePublicRegistered || by != superAdmin || *recorded != *before {
		t.Errorf("recorded %q by %d: %+v", action, by, recorded)
	}

	// A second time changes nothing.
	again, err := repo.MakeRegistered(superAdmin)
	if err != nil || again.Total() != 0 {
		t.Errorf("the second time: %+v, %v", again, err)
	}
}
