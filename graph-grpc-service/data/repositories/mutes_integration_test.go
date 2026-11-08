package repositories

import (
	"database/sql"
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
)

// Mutes against a real database (roadmap row 133). Run as the other access
// tests are (ACCESS_TEST_DSN; see access_integration_test.go).
//
//	1 Ana mutes 2 Bob until unmuted, notifications hidden;
//	    3 Cleo for a day, notifications kept;
//	    4 Dev, a mute that ended an hour ago.
func seedMutes(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, stmt := range []string{
		`DELETE FROM edges`,
		`DELETE FROM nodes`,
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,created_at,updated_at) VALUES
		 (1,'node.actor.person-service.person.v1','person-service','Ana','ana','public',1,NOW(),NOW()),
		 (2,'node.actor.person-service.person.v1','person-service','Bob','bob','followers',1,NOW(),NOW()),
		 (3,'node.actor.person-service.person.v1','person-service','Cleo','cleo','public',1,NOW(),NOW()),
		 (4,'node.actor.person-service.person.v1','person-service','Dev','dev','public',1,NOW(),NOW())`,
		`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,meta,ends_at,created_at) VALUES
		 ('edge.actor.mute.v1',1,'node.actor.person-service.person.v1',2,'node.actor.person-service.person.v1','',NULL,NOW()),
		 ('edge.actor.mute.v1',1,'node.actor.person-service.person.v1',3,'node.actor.person-service.person.v1','notifications',UTC_TIMESTAMP() + INTERVAL 1 DAY,NOW()),
		 ('edge.actor.mute.v1',1,'node.actor.person-service.person.v1',4,'node.actor.person-service.person.v1','',UTC_TIMESTAMP() - INTERVAL 1 HOUR,NOW())`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}
}

// THE RULE THIS GUARDS: the relationship says who mutes whom, while the mute
// lasts, and whether their notifications are hidden.
func TestTheRelationshipSaysWhoIsMuted(t *testing.T) {
	db := accessTestDB(t)
	seedMutes(t, db)
	rels := NewSocialgraphRelRepo(db)

	bob, err := rels.GetOne(1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !bob.IsMuting || !bob.MuteHidesNotifications || !bob.MuteEndsAt.IsZero() {
		t.Errorf("Ana of Bob %+v", bob)
	}

	cleo, _ := rels.GetOne(1, 3)
	if !cleo.IsMuting || cleo.MuteHidesNotifications || cleo.MuteEndsAt.IsZero() {
		t.Errorf("Ana of Cleo %+v", cleo)
	}

	if dev, _ := rels.GetOne(1, 4); dev.IsMuting {
		t.Errorf("a mute that ended still counts: %+v", dev)
	}

	// One way only: Bob has not muted Ana.
	if back, _ := rels.GetOne(2, 1); back.IsMuting {
		t.Errorf("Bob of Ana %+v", back)
	}

	// Many at once, as notification-service asks: who among 1 and 3 mutes 2.
	many, err := rels.GetMany([]int64{1, 3}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !many[1].IsMuting || many[3].IsMuting {
		t.Errorf("many %+v %+v", many[1], many[3])
	}
}

// THE RULE THIS GUARDS: a person's list of mutes holds the mutes that last,
// whoever they are and whatever their profile's access, and no others.
func TestOnesOwnMutesAreListed(t *testing.T) {
	db := accessTestDB(t)
	seedMutes(t, db)
	graph := NewGraphRepo(db)

	ana := entities.NewViewer()
	ana.ID = 1
	ana.PersonType = "registered"

	req := &graphpb.BrowseNeighboursRequest{
		NodeId: 1, EdgeTypes: []string{constants.EdgeMuteEntityType},
		Direction: constants.GraphQueryDirectionOutbound, Limit: 20,
	}
	muted, err := graph.BrowseNeighbours(req, ana)
	if err != nil {
		t.Fatal(err)
	}
	got := map[int64]bool{}
	for _, n := range muted {
		got[n.ID] = true
	}
	if len(muted) != 2 || !got[2] || !got[3] {
		t.Errorf("Ana's mutes %v", ids(muted))
	}
	total, err := graph.CountNeighbours(req, ana)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Errorf("Ana's mutes counted %d", total)
	}
}
