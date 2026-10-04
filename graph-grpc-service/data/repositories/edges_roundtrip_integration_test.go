package repositories

import (
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// An edge added the way the service adds one, against the schema a fresh
// installation is built from.
//
// The other real-database tests set up their edges with SQL of their own,
// naming only the columns they need. So when `ordering` went missing from
// schema/init.sql they all kept passing, on a schema where nobody could
// follow, like or subscribe: Add names every column, and that one was gone.
//
//	ACCESS_TEST_DSN="root@tcp(127.0.0.1:13309)/fresh?parseTime=true" \
//	  go test ./data/repositories/ -run TestEdgeRoundTrip -v
func TestEdgeRoundTripAgainstTheSchema(t *testing.T) {
	db := accessTestDB(t)

	const follower, leader = 9401, 9402

	clean := func() {
		if _, err := db.Exec(
			`DELETE FROM edges WHERE node_a_id = ? OR node_b_id = ?`, follower, leader); err != nil {
			t.Fatalf("clean: %v", err)
		}
	}
	clean()
	t.Cleanup(clean)

	repo := NewEdgesRepo(db)

	edge := entities.NewAnahitaEdge()
	edge.Type = constants.EdgeFollowEntityType
	edge.NodeAID, edge.NodeAType = follower, constants.NodeActorPersonEntityType
	edge.NodeBID, edge.NodeBType = leader, constants.NodeActorPersonEntityType
	edge.CreatedBy = follower

	added, err := repo.Add(edge)
	if err != nil {
		t.Fatalf("adding an edge: %v", err)
	}
	if added == nil || added.ID == 0 {
		t.Fatalf("the added edge came back without an id: %+v", added)
	}

	read, err := repo.Read(added.ID)
	if err != nil {
		t.Fatalf("reading it back: %v", err)
	}
	if read.NodeAID != follower || read.NodeBID != leader || read.Type != constants.EdgeFollowEntityType {
		t.Errorf("read back as %+v", read)
	}
	if read.CreatedAt.IsZero() {
		t.Error("the edge has no created_at")
	}

	exists, err := repo.Exists(edge)
	if err != nil || !exists {
		t.Errorf("Exists = %v, %v", exists, err)
	}
}
