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
//	1 Naomi mutes 2 Moss until unmuted, notifications hidden;
//	    3 Rose for a day, notifications kept;
//	    4 Teal'c, a mute that ended an hour ago.
func seedMutes(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, stmt := range []string{
		`DELETE FROM edges`,
		`DELETE FROM nodes`,
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,created_at,updated_at) VALUES
		 (1,'node.actor.person-service.person.v1','person-service','Naomi','naomi','public',1,NOW(),NOW()),
		 (2,'node.actor.person-service.person.v1','person-service','Moss','moss','followers',1,NOW(),NOW()),
		 (3,'node.actor.person-service.person.v1','person-service','Rose','rose','public',1,NOW(),NOW()),
		 (4,'node.actor.person-service.person.v1','person-service','Teal''c','tealc','public',1,NOW(),NOW())`,
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

	moss, err := rels.GetOne(1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !moss.IsMuting || !moss.MuteHidesNotifications || !moss.MuteEndsAt.IsZero() {
		t.Errorf("Naomi of Moss %+v", moss)
	}

	rose, _ := rels.GetOne(1, 3)
	if !rose.IsMuting || rose.MuteHidesNotifications || rose.MuteEndsAt.IsZero() {
		t.Errorf("Naomi of Rose %+v", rose)
	}

	if dev, _ := rels.GetOne(1, 4); dev.IsMuting {
		t.Errorf("a mute that ended still counts: %+v", dev)
	}

	// One way only: Moss has not muted Naomi.
	if back, _ := rels.GetOne(2, 1); back.IsMuting {
		t.Errorf("Moss of Naomi %+v", back)
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
		t.Errorf("Naomi's mutes %v", ids(muted))
	}
	total, err := graph.CountNeighbours(req, ana)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Errorf("Naomi's mutes counted %d", total)
	}
}

// THE RULE THIS GUARDS: the sweeper deletes the mutes that have ended, and
// no other.
func TestTheSweeperDeletesEndedMutes(t *testing.T) {
	db := accessTestDB(t)
	seedMutes(t, db)

	n, err := NewSocialgraphRelRepo(db).DeleteEndedMutes()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("deleted %d, want Teal'c's alone", n)
	}

	var left []int64
	rows, err := db.Query(`SELECT node_b_id FROM edges WHERE type = 'edge.actor.mute.v1' ORDER BY node_b_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		_ = rows.Scan(&id)
		left = append(left, id)
	}
	if len(left) != 2 || left[0] != 2 || left[1] != 3 {
		t.Errorf("left %v, want Moss's and Rose's", left)
	}
}
