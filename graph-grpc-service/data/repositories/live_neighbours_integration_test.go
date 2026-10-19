package repositories

import (
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
)

// Who somebody follows, against a real database.
//
//	ACCESS_TEST_DSN="root@tcp(127.0.0.1:13309)/fresh?parseTime=true" \
//	  go test ./data/repositories/ -run TestFollowingKeeps -v
//
// WHAT THIS GUARDS (the user, 2026-10-18). A profile scheduled for deletion
// stays in the list for its followers and administrators, who see it
// labelled and can find their way back to restore it, and for super
// administrators, and for nobody else; once purged it is gone. A disabled profile
// leaves the list; an archived one stays. The total says the same as the list.
func TestFollowingKeepsTheDeletedForTheirOwnOnly(t *testing.T) {
	db := accessTestDB(t)
	graph := NewGraphRepo(db)

	const person = "'node.actor.person-service.person.v1','person-service'"
	const event = "'node.actor.actor-service.event.v1','actor-service'"
	const personType = "'node.actor.person-service.person.v1'"
	const eventType = "'node.actor.actor-service.event.v1'"

	for _, stmt := range []string{
		`DELETE FROM edges`,
		`DELETE FROM nodes`,
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,created_at,updated_at) VALUES
		 (1,` + person + `,'Kira','kira','public',1,NOW(),NOW()),
		 (2,` + person + `,'Odo','odo','public',1,NOW(),NOW()),
		 (3,` + event + `,'Founders Day','founders-day','public',0,NOW(),NOW()),
		 (4,` + person + `,'Dukat','dukat','public',0,NOW(),NOW()),
		 (5,` + person + `,'Nog','nog','public',1,NOW(),NOW()),
		 (6,` + event + `,'Gone Day','gone-day','public',0,NOW(),NOW()),
		 (7,` + person + `,'Quark','quark','public',1,NOW(),NOW())`,
		`UPDATE nodes SET deleted_at = NOW() WHERE id IN (3, 6)`,
		`UPDATE nodes SET purged_at = NOW() WHERE id = 6`,
		`UPDATE nodes SET archived_at = NOW() WHERE id = 5`,
		`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,created_at) VALUES
		 ('edge.actor.follow.v1',1,` + personType + `,2,` + personType + `,NOW()),
		 ('edge.actor.follow.v1',1,` + personType + `,3,` + eventType + `,NOW()),
		 ('edge.actor.follow.v1',1,` + personType + `,4,` + personType + `,NOW()),
		 ('edge.actor.follow.v1',1,` + personType + `,5,` + personType + `,NOW()),
		 ('edge.actor.follow.v1',1,` + personType + `,6,` + eventType + `,NOW()),
		 ('edge.actor.admin.v1',7,` + personType + `,3,` + eventType + `,NOW())`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}

	super := entities.NewViewer()
	super.ID = 99
	super.PersonType = constants.PersonTypeSuperAdmin

	for _, each := range []struct {
		name   string
		viewer *entities.Viewer
		want   []int64
	}{
		{"Kira, who follows the event", replyViewer(1), []int64{2, 3, 5}},
		{"Quark, who administers it", replyViewer(7), []int64{2, 3, 5}},
		{"a visitor", entities.NewViewer(), []int64{2, 5}},
		{"Odo, who has no edge to it", replyViewer(2), []int64{2, 5}},
		{"a super administrator, who sees every one", super, []int64{2, 3, 5}},
	} {
		req := &graphpb.BrowseNeighboursRequest{
			NodeId:    1,
			EdgeTypes: []string{constants.EdgeFollowEntityType},
			Direction: constants.GraphQueryDirectionOutbound,
			Limit:     20,
		}
		nodes, err := graph.BrowseNeighbours(req, each.viewer)
		got := sortedIDs(nodeIDs(t, nodes, err))
		if len(got) != len(each.want) {
			t.Errorf("%s: Kira follows %v; want %v", each.name, got, each.want)
			continue
		}
		for i := range got {
			if got[i] != each.want[i] {
				t.Errorf("%s: Kira follows %v; want %v", each.name, got, each.want)
				break
			}
		}
		total, err := graph.CountNeighbours(req, each.viewer)
		if err != nil || int(total) != len(each.want) {
			t.Errorf("%s: counted %d (%v); want %d", each.name, total, err, len(each.want))
		}
	}
}
