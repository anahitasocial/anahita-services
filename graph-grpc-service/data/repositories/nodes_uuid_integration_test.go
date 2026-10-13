package repositories

import (
	"regexp"
	"testing"
)

// Run as the other integration tests here are, with ACCESS_TEST_DSN.
//
// THE RULE THIS GUARDS: every node has a UUID without anything having asked
// for one, no two the same, and it is what the node is found by. The address
// a person or a post has on other servers is made of it, so one that was
// missing, shared or changed would be somebody else's address.
func TestEveryNodeHasAUUIDOfItsOwn(t *testing.T) {
	db := accessTestDB(t)

	for _, statement := range []string{
		`DELETE FROM edges`,
		`DELETE FROM nodes`,
		// Added the way everything adds one: without a word about a UUID.
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,created_at,updated_at) VALUES
			(1,'node.actor.person-service.person.v1','person-service','Ana','ana','public',1,NOW(),NOW()),
			(2,'node.actor.person-service.person.v1','person-service','Bob','bob','public',1,NOW(),NOW())`,
		`INSERT INTO nodes (id,type,service,body,access,enabled,owner_id,owner_type,created_by,created_at,updated_at) VALUES
			(10,'node.medium.text-service.note.v1','text-service','A note','public',1,1,'node.actor.person-service.person.v1',1,NOW(),NOW())`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("seed: %v\n%s", err, statement)
		}
	}

	repo := NewNodesRepo(db)

	// Version 4, and the variant every reader of one expects.
	shape := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

	seen := map[string]int64{}
	for _, id := range []int64{1, 2, 10} {
		node, err := repo.Read(id)
		if err != nil || node == nil {
			t.Fatalf("reading %d: %v", id, err)
		}
		if !shape.MatchString(node.UUID) {
			t.Errorf("node %d has %q for a UUID", id, node.UUID)
		}
		if other, taken := seen[node.UUID]; taken {
			t.Errorf("nodes %d and %d share %s", other, id, node.UUID)
		}
		seen[node.UUID] = id

		// Found by it, whatever kind of node it is.
		found, err := repo.ReadByUUID(node.UUID)
		if err != nil || found == nil || found.ID != id {
			t.Errorf("node %d was not found by its UUID: %+v (%v)", id, found, err)
		}

		// Changing a node leaves what it is called alone.
		if _, err := db.Exec(`UPDATE nodes SET name = CONCAT(COALESCE(name, ''), '!'), updated_at = NOW() WHERE id = ?`, id); err != nil {
			t.Fatal(err)
		}
		again, err := repo.Read(id)
		if err != nil || again == nil || again.UUID != node.UUID {
			t.Errorf("node %d was %s and is not now (%v)", id, node.UUID, err)
		}
	}

	// One that is nobody's finds nothing, and neither does something that is
	// no UUID at all.
	for _, uuid := range []string{"00000000-0000-4000-8000-000000000000", "1", "' OR 1=1 -- "} {
		if found, _ := repo.ReadByUUID(uuid); found != nil {
			t.Errorf("%q found node %d", uuid, found.ID)
		}
	}

	// The same one cannot be given twice.
	var first string
	if err := db.QueryRow(`SELECT uuid FROM nodes WHERE id = 1`).Scan(&first); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE nodes SET uuid = ? WHERE id = 2`, first); err == nil {
		t.Error("two nodes were given one UUID")
	}
}
