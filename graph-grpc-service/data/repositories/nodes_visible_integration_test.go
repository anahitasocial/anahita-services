package repositories

import (
	"database/sql"
	"sort"
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// The cast. Ada's profile is public, Grace's is for followers only.
const (
	visAda      = 1 // public profile
	visGrace    = 2 // followers-only profile
	visStranger = 3 // follows nobody
	visFollower = 4 // follows Ada and Grace
	visBlocked  = 5 // Ada has blocked them
	visDisabled = 6 // a disabled account
	visDeleted  = 7 // a deleted account
	visGroup    = 8 // a followers-only group, which the follower is in

	visPublicPost    = 10 // Ada, public
	visFollowersPost = 11 // Ada, followers only
	visPrivatePost   = 12 // Ada, only me
	visGracePost     = 13 // public, but on Grace's followers-only profile
	visGroupPost     = 14 // public, in the followers-only group
	visDisabledPost  = 15 // public, by the disabled account

	visReplyOnPublic  = 20 // under Ada's public post
	visReplyOnPrivate = 21 // under Ada's only-me post

	visHashtag  = 30
	visLocation = 31

	visMissing = 999
)

var visAll = []int64{
	visAda, visGrace, visStranger, visFollower, visBlocked, visDisabled, visDeleted, visGroup,
	visPublicPost, visFollowersPost, visPrivatePost, visGracePost, visGroupPost, visDisabledPost,
	visReplyOnPublic, visReplyOnPrivate,
	visHashtag, visLocation,
	visMissing,
}

func seedVisibleFixture(t *testing.T, db *sql.DB) {
	t.Helper()

	const person = "'node.actor.person-service.person.v1','person-service'"
	const note = "'node.medium.text-service.note.v1','text-service'"

	for _, stmt := range []string{
		`DELETE FROM edges`,
		`DELETE FROM nodes`,
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,created_at,updated_at) VALUES
		 (1,` + person + `,'Ada','ada','public',1,NOW(),NOW()),
		 (2,` + person + `,'Grace','grace','followers',1,NOW(),NOW()),
		 (3,` + person + `,'Stranger','stranger','public',1,NOW(),NOW()),
		 (4,` + person + `,'Follower','follower','public',1,NOW(),NOW()),
		 (5,` + person + `,'Blocked','blocked','public',1,NOW(),NOW()),
		 (6,` + person + `,'Disabled','disabled','public',0,NOW(),NOW()),
		 (7,` + person + `,'Deleted','deleted','public',1,NOW(),NOW()),
		 (8,'node.actor.group-service.group.v1','group-service','Circle','circle','followers',1,NOW(),NOW()),
		 (30,'node.tag.hashtag-service.hashtag.v1','hashtag-service','gardens','gardens','',1,NOW(),NOW()),
		 (31,'node.tag.location-service.location.v1','location-service','Vancouver','vancouver','',1,NOW(),NOW())`,
		`UPDATE nodes SET deleted_at = NOW() WHERE id = 7`,
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,owner_id,created_by,created_at,updated_at) VALUES
		 (10,` + note + `,'public','n10','public',1,1,1,NOW(),NOW()),
		 (11,` + note + `,'followers','n11','followers',1,1,1,NOW(),NOW()),
		 (12,` + note + `,'private','n12','myself',1,1,1,NOW(),NOW()),
		 (13,` + note + `,'on grace','n13','public',1,2,2,NOW(),NOW()),
		 (14,` + note + `,'in group','n14','public',1,8,4,NOW(),NOW()),
		 (15,` + note + `,'by disabled','n15','public',1,6,6,NOW(),NOW())`,
		// Replies: notes owned by whoever wrote them, with no audience of
		// their own.
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,parent_id,root_id,owner_id,owner_type,created_by,created_at,updated_at) VALUES
		 (20,` + note + `,'c20','c20','',1,10,10,3,'node.actor.person-service.person.v1',3,NOW(),NOW()),
		 (21,` + note + `,'c21','c21','',1,12,12,1,'node.actor.person-service.person.v1',1,NOW(),NOW())`,
		`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,created_at) VALUES
		 ('edge.actor.follow.v1',4,'node.actor.person-service.person.v1',1,'node.actor.person-service.person.v1',NOW()),
		 ('edge.actor.follow.v1',4,'node.actor.person-service.person.v1',2,'node.actor.person-service.person.v1',NOW()),
		 ('edge.actor.follow.v1',4,'node.actor.person-service.person.v1',8,'node.actor.group-service.group.v1',NOW()),
		 ('edge.actor.block.v1',1,'node.actor.person-service.person.v1',5,'node.actor.person-service.person.v1',NOW())`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}
}

func visViewer(id int64, personType string) *entities.Viewer {
	viewer := entities.NewViewer()
	viewer.ID = id
	viewer.PersonType = personType
	return viewer
}

func sorted(ids []int64) []int64 {
	out := append([]int64{}, ids...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func equalIDs(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// One question, asked of everything at once by six different people. What
// each may see must match what the listings for each kind would show them.
func TestFilterVisibleAgainstRealData(t *testing.T) {
	db := accessTestDB(t)
	seedVisibleFixture(t, db)
	repo := NewNodesRepo(db)

	// What anybody may see: public profiles, Ada's public post and the
	// reply under it, and the tags. Not Grace or her post, not the group
	// or its post, nothing disabled or deleted, nothing that does not exist.
	public := []int64{
		visAda, visStranger, visFollower, visBlocked,
		visPublicPost, visReplyOnPublic,
		visHashtag, visLocation,
	}

	cases := []struct {
		name   string
		viewer *entities.Viewer
		want   []int64
	}{
		{"nobody signed in", nil, public},
		{"a signed-in stranger", visViewer(visStranger, constants.PersonTypeRegistered), public},
		{
			"a follower also sees what is for followers",
			visViewer(visFollower, constants.PersonTypeRegistered),
			append(append([]int64{}, public...),
				visGrace, visGroup, visFollowersPost, visGracePost, visGroupPost),
		},
		{
			// Ada's profile, her posts and the reply under her post all
			// go. The tags and everybody else's profile stay.
			"somebody Ada has blocked sees nothing of hers",
			visViewer(visBlocked, constants.PersonTypeRegistered),
			[]int64{visStranger, visFollower, visBlocked, visHashtag, visLocation},
		},
		{
			"Ada sees her own private post and the reply under it",
			visViewer(visAda, constants.PersonTypeRegistered),
			[]int64{
				visAda, visStranger, visFollower,
				visPublicPost, visFollowersPost, visPrivatePost,
				visReplyOnPublic, visReplyOnPrivate,
				visHashtag, visLocation,
			},
		},
		{
			"an administrator sees everything that exists",
			visViewer(99, constants.PersonTypeAdmin),
			[]int64{
				visAda, visGrace, visStranger, visFollower, visBlocked, visDisabled, visDeleted, visGroup,
				visPublicPost, visFollowersPost, visPrivatePost, visGracePost, visGroupPost, visDisabledPost,
				visReplyOnPublic, visReplyOnPrivate,
				visHashtag, visLocation,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := repo.FilterVisible(visAll, tc.viewer)
			if err != nil {
				t.Fatalf("FilterVisible: %v", err)
			}
			if !equalIDs(sorted(got), sorted(tc.want)) {
				t.Errorf("visible\n  got  %v\n  want %v", sorted(got), sorted(tc.want))
			}
		})
	}

	t.Run("no ids, no query", func(t *testing.T) {
		got, err := repo.FilterVisible(nil, nil)
		if err != nil || len(got) != 0 {
			t.Errorf("got %v, %v", got, err)
		}
	})
}
