package repositories

import (
	"errors"
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// Invitations against a real database, on the fixture the thread tests use.
//
//	ACCESS_TEST_DSN="root@tcp(127.0.0.1:13309)/fresh?parseTime=true" \
//	  go test ./data/repositories/ -run TestInvit -v
//
// WHAT THESE GUARD. Nobody is made a follower without saying yes. Only the
// inviter's own followers can be invited, and never across a block. One
// invitation for each person and group, and a refusal holds for the
// cooldown.
func TestInvitations(t *testing.T) {
	db := accessTestDB(t)
	seedRepliesFixture(t, db)
	repo := NewInvitesRepo(db)

	const (
		personType = "node.actor.person-service.person.v1"
		groupType  = "node.actor.group-service.group.v1"
		dave       = 5 // Alice blocked him
		eve        = 6 // follows nobody
		frank      = 7 // follows Alice, and has asked to follow the group
		grace      = 8 // follows Alice
	)

	for _, stmt := range []string{
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,created_at,updated_at) VALUES
		 (6,'` + personType + `','person-service','Eve','eve','public',1,NOW(),NOW()),
		 (7,'` + personType + `','person-service','Frank','frank','public',1,NOW(),NOW()),
		 (8,'` + personType + `','person-service','Grace','grace','public',1,NOW(),NOW())`,
		`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,created_at) VALUES
		 ('edge.actor.follow.v1',2,'` + personType + `',1,'` + personType + `',NOW()),
		 ('edge.actor.follow.v1',4,'` + personType + `',1,'` + personType + `',NOW()),
		 ('edge.actor.follow.v1',5,'` + personType + `',1,'` + personType + `',NOW()),
		 ('edge.actor.follow.v1',7,'` + personType + `',1,'` + personType + `',NOW()),
		 ('edge.actor.follow.v1',8,'` + personType + `',1,'` + personType + `',NOW()),
		 ('edge.actor.follow-request.v1',7,'` + personType + `',3,'` + groupType + `',NOW())`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}

	group := &entities.AnahitaNode{}
	group.ID = replyGroup
	group.Type = groupType

	follows := func(person int64) bool {
		t.Helper()
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM edges WHERE type = 'edge.actor.follow.v1' AND node_a_id = ? AND node_b_id = ?`, person, replyGroup).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n > 0
	}

	// Alice invites everybody she can think of, herself and nobody included.
	got, err := repo.Add(group, replyAlice, []int64{replyBob, replyCarol, dave, eve, frank, 99, replyAlice, replyBob})
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	want := map[int64]string{
		replyBob:   constants.InviteResultInvited,
		replyCarol: constants.InviteResultAlreadyFollower,
		dave:       constants.InviteResultBlocked,
		eve:        constants.InviteResultNotFollower,
		frank:      constants.InviteResultRequested,
		99:         constants.InviteResultUnavailable,
		replyAlice: constants.InviteResultUnavailable,
	}
	for id, result := range want {
		if got[id] != result {
			t.Errorf("person %d: %q, want %q", id, got[id], result)
		}
	}

	// Invited is not following.
	if follows(replyBob) {
		t.Fatal("Bob follows the group without having said yes")
	}

	// It shows in what waits for Bob, in what waits to the group, and in
	// how Bob stands with the group.
	if invite, err := repo.Read(replyGroup, replyBob); err != nil || invite == nil || invite.InviterID != replyAlice {
		t.Fatalf("read: %+v (err %v), want one from Alice", invite, err)
	}
	if list, total, err := repo.ListForActor(replyGroup, 10, 0); err != nil || total != 1 || len(list) != 1 || list[0].InviteeID != replyBob {
		t.Errorf("waiting to the group: %d of %+v (err %v), want Bob alone", total, list, err)
	}
	if list, total, err := repo.ListForInvitee(replyBob, 10, 0); err != nil || total != 1 || len(list) != 1 || list[0].ActorID != replyGroup {
		t.Errorf("waiting for Bob: %d of %+v (err %v), want the group alone", total, list, err)
	}
	if made, err := repo.CountMadeLately(replyAlice); err != nil || made != 1 {
		t.Errorf("made by Alice lately: %d (err %v), want 1", made, err)
	}
	rels, err := NewSocialgraphRelRepo(db).GetMany([]int64{replyGroup}, replyBob)
	if err != nil || !rels[replyGroup].HasInvite || rels[replyGroup].IsLeading {
		t.Errorf("Bob and the group: %+v (err %v), want invited and not following", rels[replyGroup], err)
	}

	// Inviting him again changes nothing.
	if got, _ := repo.Add(group, replyAlice, []int64{replyBob}); got[replyBob] != constants.InviteResultPending {
		t.Errorf("invited twice: %q, want pending", got[replyBob])
	}

	// He says no. It holds: he cannot be invited again for the cooldown.
	if _, err := repo.Decline(replyGroup, replyBob); err != nil {
		t.Fatalf("decline: %v", err)
	}
	if invite, _ := repo.Read(replyGroup, replyBob); invite != nil {
		t.Error("a refused invitation is still waiting")
	}
	if _, err := repo.Decline(replyGroup, replyBob); !errors.Is(err, ErrNoInvite) {
		t.Errorf("declining twice: %v, want ErrNoInvite", err)
	}
	if _, err := repo.Accept(replyGroup, replyBob); !errors.Is(err, ErrNoInvite) {
		t.Errorf("accepting what was refused: %v, want ErrNoInvite", err)
	}
	if got, _ := repo.Add(group, replyAlice, []int64{replyBob}); got[replyBob] != constants.InviteResultCooldown {
		t.Errorf("invited after refusing: %q, want cooldown", got[replyBob])
	}

	// The sweep leaves a refusal alone until its cooldown is over.
	if _, err := repo.Sweep(); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.Add(group, replyAlice, []int64{replyBob}); got[replyBob] != constants.InviteResultCooldown {
		t.Errorf("after a sweep inside the cooldown: %q, want cooldown", got[replyBob])
	}
	if _, err := db.Exec(`UPDATE edges SET updated_at = NOW() - INTERVAL 91 DAY WHERE type = ? AND node_a_id = ?`, constants.EdgeActorInviteEntityType, replyBob); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.Add(group, replyAlice, []int64{replyBob}); got[replyBob] != constants.InviteResultInvited {
		t.Errorf("invited after the cooldown: %q, want invited", got[replyBob])
	}

	// This time he says yes, and only now does he follow.
	invite, err := repo.Accept(replyGroup, replyBob)
	if err != nil || invite.InviterID != replyAlice {
		t.Fatalf("accept: %+v (err %v)", invite, err)
	}
	if !follows(replyBob) {
		t.Error("Bob accepted and does not follow the group")
	}
	if left, _ := repo.Read(replyGroup, replyBob); left != nil {
		t.Error("an accepted invitation is still waiting")
	}
	if _, err := repo.Accept(replyGroup, replyBob); !errors.Is(err, ErrNoInvite) {
		t.Errorf("accepting twice: %v, want ErrNoInvite", err)
	}

	// Grace is invited and the invitation is taken back.
	if got, _ := repo.Add(group, replyAlice, []int64{grace}); got[grace] != constants.InviteResultInvited {
		t.Fatalf("Grace: %q, want invited", got[grace])
	}
	if _, err := repo.Delete(replyGroup, grace); err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	if _, err := repo.Accept(replyGroup, grace); !errors.Is(err, ErrNoInvite) || follows(grace) {
		t.Errorf("accepting what was withdrawn: %v, follows %v", err, follows(grace))
	}

	// Invited again and never answered: it lapses, and the sweep says so.
	if got, _ := repo.Add(group, replyAlice, []int64{grace}); got[grace] != constants.InviteResultInvited {
		t.Fatalf("Grace again: %q, want invited", got[grace])
	}
	if _, err := db.Exec(`UPDATE edges SET ends_at = NOW() - INTERVAL 1 MINUTE WHERE type = ? AND node_a_id = ?`, constants.EdgeActorInviteEntityType, grace); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Accept(replyGroup, grace); !errors.Is(err, ErrNoInvite) {
		t.Errorf("accepting what lapsed: %v, want ErrNoInvite", err)
	}
	lapsed, err := repo.Sweep()
	if err != nil || len(lapsed) != 1 || lapsed[0].InviteeID != grace {
		t.Errorf("swept: %+v (err %v), want Grace's alone", lapsed, err)
	}
	var left int
	if err := db.QueryRow(`SELECT COUNT(*) FROM edges WHERE type = ?`, constants.EdgeActorInviteEntityType).Scan(&left); err != nil || left != 0 {
		t.Errorf("%d invitation edges left (err %v), want none", left, err)
	}
}
