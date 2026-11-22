package repositories

import (
	"testing"

	"anahita-libs/constants"
)

// THE RULE THIS GUARDS: every relation between the viewer and the profiles
// they look at is read, each one only where it holds (roadmap row 170, when
// getRelations was split into a query for each). Run as the other access
// tests are (ACCESS_TEST_DSN; see access_integration_test.go).
//
// Naomi (1) looks at:
//
//	2 Amos, whom she follows and who follows her;
//	3 Alex, who blocks her;
//	4 Bobbie, who mutes her, until unmuted;
//	5 Rocinante, a group she is an admin of;
//	6 Holden, whom she asked to follow;
//	7 Tycho, a group she is invited to follow;
//	8 an event open to the followers of 9 Ceres Station, which hosts it and
//	  which she follows;
//	10 Drummer, on social.example, a server she blocked;
//	11 Chrisjen, nothing at all.
func TestEveryRelationIsReadWhereItHolds(t *testing.T) {
	db := accessTestDB(t)
	const (
		person = "node.actor.person-service.person.v1"
		group  = "node.actor.group-service.group.v1"
	)
	for _, stmt := range []string{
		`DELETE FROM edges`,
		`DELETE FROM events`,
		`DELETE FROM person_domain_blocks`,
		`DELETE FROM nodes`,
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,origin_domain,created_at,updated_at) VALUES
		 (1,'` + person + `','person-service','Naomi','naomi','public',1,NULL,NOW(),NOW()),
		 (2,'` + person + `','person-service','Amos','amos','public',1,NULL,NOW(),NOW()),
		 (3,'` + person + `','person-service','Alex','alex','public',1,NULL,NOW(),NOW()),
		 (4,'` + person + `','person-service','Bobbie','bobbie','public',1,NULL,NOW(),NOW()),
		 (5,'` + group + `','group-service','Rocinante','rocinante','public',1,NULL,NOW(),NOW()),
		 (6,'` + person + `','person-service','Holden','holden','public',1,NULL,NOW(),NOW()),
		 (7,'` + group + `','group-service','Tycho','tycho','public',1,NULL,NOW(),NOW()),
		 (8,'` + constants.NodeActorEventEntityType + `','actor-service','Launch','launch','public',1,NULL,NOW(),NOW()),
		 (9,'` + group + `','group-service','Ceres Station','ceres','public',1,NULL,NOW(),NOW()),
		 (10,'` + constants.NodeActorActivityPubPersonEntityType + `','activitypub-service','Drummer','drummer@social.example','public',1,'social.example',NOW(),NOW()),
		 (11,'` + person + `','person-service','Chrisjen','chrisjen','public',1,NULL,NOW(),NOW())`,
		`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,meta,ends_at,created_at) VALUES
		 ('` + constants.EdgeFollowEntityType + `',1,'',2,'','',NULL,NOW()),
		 ('` + constants.EdgeFollowEntityType + `',2,'',1,'','',NULL,NOW()),
		 ('` + constants.EdgeBlockEntityType + `',3,'',1,'','',NULL,NOW()),
		 ('` + constants.EdgeMuteEntityType + `',4,'',1,'','',NULL,NOW()),
		 ('` + constants.EdgeActorAdminEntityType + `',1,'',5,'','',NULL,NOW()),
		 ('` + constants.EdgeFollowRequestEntityType + `',1,'',6,'','',NULL,NOW()),
		 ('` + constants.EdgeActorInviteEntityType + `',1,'',7,'','` + constants.InviteMetaPending + `',UTC_TIMESTAMP() + INTERVAL 1 DAY,NOW()),
		 ('` + constants.EdgeActorHostEntityType + `',9,'',8,'','',NULL,NOW()),
		 ('` + constants.EdgeFollowEntityType + `',1,'',9,'','',NULL,NOW())`,
		`INSERT INTO events (node_id, open_to_host_followers) VALUES (8, 1)`,
		`INSERT INTO person_domain_blocks (person_id, domain, created_at) VALUES (1, 'social.example', NOW())`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}

	rels, err := NewSocialgraphRelRepo(db).GetMany([]int64{2, 3, 4, 5, 6, 7, 8, 10, 11}, 1)
	if err != nil {
		t.Fatal(err)
	}

	type flags struct {
		leading, following, mutual, blocking, muting, admin, requested, invite, viaHost, serverBlocked bool
	}
	want := map[int64]flags{
		2:  {leading: true, following: true, mutual: true},
		3:  {blocking: true},
		4:  {muting: true},
		5:  {admin: true},
		6:  {requested: true},
		7:  {invite: true},
		8:  {viaHost: true},
		10: {serverBlocked: true},
		11: {},
	}
	for id, w := range want {
		r := rels[id]
		if r == nil {
			t.Errorf("%d: no relation", id)
			continue
		}
		got := flags{r.IsLeading, r.IsFollowing, r.IsMutual, r.IsBlocking, r.IsMuting, r.HasAdmin, r.HasRequested, r.HasInvite, r.ViaHost, r.IsServerBlocked}
		if got != w {
			t.Errorf("%d: %+v, want %+v", id, got, w)
		}
	}
	if !rels[4].MuteHidesNotifications {
		t.Error("an unlimited mute of everything hides notifications")
	}

	// Somebody not signed in is related to nobody, and nothing is asked.
	guest, err := NewSocialgraphRelRepo(db).GetMany([]int64{2}, 0)
	if err != nil || guest[2] == nil || guest[2].IsLeading {
		t.Errorf("a guest: %+v %v", guest[2], err)
	}
}
