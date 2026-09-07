package repositories

import (
	"database/sql"
	"os"
	"testing"
	"time"

	"anahita-libs/constants"

	_ "github.com/go-sql-driver/mysql"
)

// HasAdmin, against a real database.
//
// A shape test cannot check this one. The four relation queries are built from
// the same template, and after the fix two of them are byte-identical apart
// from their bound arguments — which sqlmock's matcher does not expose. An
// assertion on the SQL text matched the wrong statement and passed while the
// direction was reversed, which is precisely the failure being guarded against.
//
// So this asserts the answer instead of the query.
//
// Skipped unless ACCESS_TEST_DSN is set. To run against the dev stack:
//
//	ACCESS_TEST_DSN="root:root@tcp(127.0.0.1:3306)/anahita?parseTime=true" \
//	  go test ./data/repositories/ -run TestHasAdmin -v
func TestHasAdminIsTrueWhenTheViewerAdministersTheActor(t *testing.T) {
	dsn := os.Getenv("ACCESS_TEST_DSN")
	if dsn == "" {
		t.Skip("set ACCESS_TEST_DSN to run")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	// Ids far above anything the fixtures use, so a failure cannot collide
	// with real rows.
	const (
		adminPerson = 990001
		theGroup    = 990002
		bystander   = 990003
	)

	cleanup := func() {
		_, _ = db.Exec(`DELETE FROM edges WHERE node_a_id IN (?,?,?) OR node_b_id IN (?,?,?)`,
			adminPerson, theGroup, bystander, adminPerson, theGroup, bystander)
	}
	cleanup()
	t.Cleanup(cleanup)

	// The edge runs administrator -> administered.
	if _, err := db.Exec(`
		INSERT INTO edges (type, node_a_id, node_a_type, node_b_id, node_b_type, created_on)
		VALUES (?, ?, 'node.actor.person-service.person.v1', ?, 'node.actor.group-service.group.v1', NOW())`,
		constants.EdgeActorAdminEntityType, adminPerson, theGroup); err != nil {
		t.Fatalf("seed: %v", err)
	}

	repo := &socialgraphRelRepo{db: db, dbTimeout: 5 * time.Second}

	// The question every consumer asks: may this VIEWER act on this ACTOR
	// because they administer it. Actor first, viewer second — the same
	// argument order group_access.go and person_access.go use.
	rel, err := repo.GetOne(theGroup, adminPerson)
	if err != nil {
		t.Fatalf("GetOne: %v", err)
	}
	if !rel.HasAdmin {
		t.Error("HasAdmin is false for the person who administers the group. " +
			"Every permission resting on it fails: a group administrator " +
			"cannot edit or delete their own group, and admins-only content " +
			"is invisible to the people it is restricted to.")
	}

	// And it must not be true for somebody unrelated, or it grants everything
	// to everybody rather than nothing to anybody.
	other, err := repo.GetOne(theGroup, bystander)
	if err != nil {
		t.Fatalf("GetOne bystander: %v", err)
	}
	if other.HasAdmin {
		t.Error("HasAdmin is true for somebody who administers nothing")
	}

	// The reverse relation must NOT read as true. A group does not administer
	// a person, and reading the edge backwards is the bug this replaced.
	reversed, err := repo.GetOne(adminPerson, theGroup)
	if err != nil {
		t.Fatalf("GetOne reversed: %v", err)
	}
	if reversed.HasAdmin {
		t.Error("HasAdmin is true with the arguments reversed, so the edge is " +
			"being read symmetrically when it has a direction")
	}
}
