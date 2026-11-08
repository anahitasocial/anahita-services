package repositories

import (
	"database/sql"
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
)

// What a mute takes out of the muter's lists, against a real database
// (roadmap row 133). Run as the other access tests are (ACCESS_TEST_DSN; see
// access_integration_test.go).
//
//	1 Naomi, who follows 2 Moss, 3 Rose and the group 5, and mutes Moss.
//	10 Moss's note.            11 Rose's note.
//	12 Moss's note in the group.
//	13 Rose's repost of 10.   14 Moss's repost of 11.
//	15 Rose's note on Moss's profile.
//	100 a hashtag on 10 and 11.
const (
	muterNaomi = 1
	mutedMoss  = 2
	otherRose  = 3
	groupOfAll = 5
)

func seedMuteLists(t *testing.T, db *sql.DB) {
	t.Helper()
	const (
		person = "'node.actor.person-service.person.v1'"
		group  = "'node.actor.actor-service.group.v1'"
		note   = "'node.medium.text-service.note.v1'"
		repost = "'node.base.repost-service.repost.v1'"
	)
	for _, stmt := range []string{
		`DELETE FROM edges`,
		`DELETE FROM nodes`,
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,created_at,updated_at) VALUES
		 (1,` + person + `,'person-service','Naomi','naomi','public',1,NOW(),NOW()),
		 (2,` + person + `,'person-service','Moss','moss','public',1,NOW(),NOW()),
		 (3,` + person + `,'person-service','Rose','rose','public',1,NOW(),NOW()),
		 (5,` + group + `,'actor-service','Gardeners','gardeners','public',1,NOW(),NOW()),
		 (100,'node.tag.hashtag-service.hashtag.v1','hashtag-service','beans','beans','public',1,NOW(),NOW())`,
		`INSERT INTO nodes (id,type,service,body,access,enabled,owner_id,created_by,created_at,updated_at) VALUES
		 (10,` + note + `,'text-service','Moss on beans','public',1,2,2,NOW() - INTERVAL 6 MINUTE,NOW()),
		 (11,` + note + `,'text-service','Rose on beans','public',1,3,3,NOW() - INTERVAL 5 MINUTE,NOW()),
		 (12,` + note + `,'text-service','Moss in the group','public',1,5,2,NOW() - INTERVAL 4 MINUTE,NOW()),
		 (15,` + note + `,'text-service','Rose on Moss''s wall','public',1,2,3,NOW() - INTERVAL 1 MINUTE,NOW())`,
		`INSERT INTO nodes (id,type,service,access,enabled,owner_id,parent_id,created_by,created_at,updated_at) VALUES
		 (13,` + repost + `,'feed-service','public',1,3,10,3,NOW() - INTERVAL 3 MINUTE,NOW()),
		 (14,` + repost + `,'feed-service','public',1,2,11,2,NOW() - INTERVAL 2 MINUTE,NOW())`,
		`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,created_at) VALUES
		 ('edge.actor.follow.v1',1,` + person + `,2,` + person + `,NOW()),
		 ('edge.actor.follow.v1',1,` + person + `,3,` + person + `,NOW()),
		 ('edge.actor.follow.v1',1,` + person + `,5,` + group + `,NOW()),
		 ('edge.actor.follow.v1',3,` + person + `,2,` + person + `,NOW()),
		 ('edge.actor.mute.v1',1,` + person + `,2,` + person + `,NOW()),
		 ('edge.tag.hashtag.v1',100,'node.tag.hashtag-service.hashtag.v1',10,` + note + `,NOW()),
		 ('edge.tag.hashtag.v1',100,'node.tag.hashtag-service.hashtag.v1',11,` + note + `,NOW())`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}
}

// THE RULE THIS GUARDS: a mute takes out of the muter's lists what the muted
// person wrote, reposted, or had reposted, and what is on their profile;
// leaves the muted person's own profile whole; touches nobody else's lists;
// and ends when it ends.
func TestAMuteTakesThemOutOfTheMutersLists(t *testing.T) {
	db := accessTestDB(t)
	seedMuteLists(t, db)

	leaders := NewFeedLeadersRepo(db)
	profiles := NewFeedActorRepo(db)
	graph := NewGraphRepo(db)
	search := NewSearchRepo(db)

	following := func(viewer int64) map[int64]bool {
		t.Helper()
		req := &graphpb.FeedLeadersBrowseRequest{MediumEntityTypes: feedNoteTypes, IncludeReposts: true, Limit: 50}
		nodes, err := leaders.Browse(req, feedViewer(viewer))
		seen := nodeIDs(t, nodes, err)
		total, err := leaders.Count(&graphpb.FeedLeadersCountRequest{MediumEntityTypes: feedNoteTypes, IncludeReposts: true}, feedViewer(viewer))
		if err != nil {
			t.Fatal(err)
		}
		if int(total) != len(seen) {
			t.Errorf("Following for %d: counted %d, listed %d", viewer, total, len(seen))
		}
		return seen
	}
	profile := func(actor, viewer int64) map[int64]bool {
		t.Helper()
		req := &graphpb.FeedActorBrowseRequest{ActorId: actor, MediumEntityTypes: feedNoteTypes, IncludeReposts: true, Limit: 50}
		nodes, err := profiles.Browse(req, feedViewer(viewer))
		seen := nodeIDs(t, nodes, err)
		total, err := profiles.Count(&graphpb.FeedActorCountRequest{ActorId: actor, MediumEntityTypes: feedNoteTypes, IncludeReposts: true}, feedViewer(viewer))
		if err != nil {
			t.Fatal(err)
		}
		if int(total) != len(seen) {
			t.Errorf("profile %d for %d: counted %d, listed %d", actor, viewer, total, len(seen))
		}
		return seen
	}
	hashtag := func(viewer int64) map[int64]bool {
		t.Helper()
		req := &graphpb.BrowseNeighboursRequest{
			NodeId: 100, EdgeTypes: []string{constants.EdgeTagHashtagType},
			Direction: constants.GraphQueryDirectionOutbound, Limit: 50,
		}
		nodes, err := graph.BrowseNeighbours(req, feedViewer(viewer))
		seen := nodeIDs(t, nodes, err)
		total, err := graph.CountNeighbours(req, feedViewer(viewer))
		if err != nil {
			t.Fatal(err)
		}
		if int(total) != len(seen) {
			t.Errorf("hashtag for %d: counted %d, listed %d", viewer, total, len(seen))
		}
		return seen
	}
	searched := func(scope string, viewer int64) map[int64]bool {
		t.Helper()
		nodes, err := search.Browse(&graphpb.SearchBrowseRequest{Scope: scope, Limit: 50}, feedViewer(viewer))
		seen := nodeIDs(t, nodes, err)
		total, err := search.Count(&graphpb.SearchCountRequest{Scope: scope}, feedViewer(viewer))
		if err != nil {
			t.Fatal(err)
		}
		if int(total) != len(seen) {
			t.Errorf("search %s for %d: counted %d, listed %d", scope, viewer, total, len(seen))
		}
		return seen
	}

	cases := []struct {
		what string
		got  map[int64]bool
		want []int64
	}{
		{"Naomi's Following", following(muterNaomi), []int64{11}},
		{"Moss's own profile, opened by Naomi", profile(mutedMoss, muterNaomi), []int64{10, 14, 15}},
		{"Rose's profile, for Naomi", profile(otherRose, muterNaomi), []int64{11}},
		{"the group's, for Naomi", profile(groupOfAll, muterNaomi), nil},
		{"the hashtag's, for Naomi", hashtag(muterNaomi), []int64{11}},
		{"posts found by Naomi", searched(constants.SearchScopePosts, muterNaomi), []int64{11}},
		{"people found by Naomi, Moss among them", searched(constants.SearchScopeActor, muterNaomi), []int64{1, 2, 3, 5}},
		// One way: Rose has muted nobody, and follows Moss.
		{"Rose's Following, her own repost in it", following(otherRose), []int64{10, 11, 13, 14, 15}},
		{"the hashtag's, for Rose", hashtag(otherRose), []int64{10, 11}},
	}
	for _, c := range cases {
		if !sameSet(c.got, c.want...) {
			t.Errorf("%s: %v, want %v", c.what, sortedIDs(c.got), c.want)
		}
	}

	// Ended an hour ago: everything is back.
	if _, err := db.Exec(`UPDATE edges SET ends_at = UTC_TIMESTAMP() - INTERVAL 1 HOUR WHERE type = 'edge.actor.mute.v1'`); err != nil {
		t.Fatal(err)
	}
	if got := following(muterNaomi); !sameSet(got, 10, 11, 12, 13, 14, 15) {
		t.Errorf("Naomi's Following once the mute ended: %v", sortedIDs(got))
	}
}

// THE RULE THIS GUARDS: a thread loses, for the muter, the replies of whom
// they muted and what is under them, and nothing else.
func TestWithoutMutedTakesTheirBranches(t *testing.T) {
	reply := func(id, parent, by int64) *entities.AnahitaNode {
		return &entities.AnahitaNode{ID: id, ParentID: parent, CreatedBy: by}
	}
	// Under the post 11: Moss's 20, Rose's 21 under it and 23 under that;
	// Naomi's 22, and Rose's 24 under Naomi's.
	thread := []*entities.AnahitaNode{
		reply(20, 11, mutedMoss), reply(21, 20, otherRose), reply(22, 11, muterNaomi),
		reply(23, 21, otherRose), reply(24, 22, otherRose),
	}

	kept := WithoutMuted(thread, map[int64]bool{mutedMoss: true})
	got := map[int64]bool{}
	for _, r := range kept {
		got[r.ID] = true
	}
	if !sameSet(got, 22, 24) {
		t.Errorf("kept %v", sortedIDs(got))
	}

	if all := WithoutMuted(thread, nil); len(all) != len(thread) {
		t.Errorf("nobody muted, kept %d of %d", len(all), len(thread))
	}
}
