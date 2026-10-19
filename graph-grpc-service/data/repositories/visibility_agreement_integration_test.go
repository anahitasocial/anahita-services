package repositories

import (
	"testing"

	"anahita-libs/cmd/api/permissions"
	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// The rule for who may read a post is written twice: in Go, as
// permissions.CanRead, for a post already loaded, and in SQL, as
// FilterVisible, for many at once (and as permissions.CanReadByID and
// CanReadEach, by id). Two copies can drift apart, and nothing else would
// say so (roadmap row 96, part 1). This asks both, for every post here and
// every viewer, and fails where they disagree.
//
//	ACCESS_TEST_DSN="root@tcp(127.0.0.1:13309)/fresh?parseTime=true" \
//	  go test ./data/repositories/ -run TestCanReadAndFilterVisibleAgree -v
func TestCanReadAndFilterVisibleAgree(t *testing.T) {
	db := accessTestDB(t)
	seedVisibleFixture(t, db)

	const person = "node.actor.person-service.person.v1"
	const note = "'node.medium.text-service.note.v1','text-service'"
	for _, stmt := range []string{
		// Ada's posts for each of the narrower audiences.
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,owner_id,owner_type,created_by,created_at,updated_at) VALUES
		 (40,` + note + `,'leaders','n40','leaders',1,1,'` + person + `',1,NOW(),NOW()),
		 (41,` + note + `,'mutuals','n41','mutuals',1,1,'` + person + `',1,NOW(),NOW()),
		 (42,` + note + `,'admins','n42','admins',1,1,'` + person + `',1,NOW(),NOW()),
		 (43,` + note + `,'registered','n43','registered',1,1,'` + person + `',1,NOW(),NOW())`,
		// Ada follows the stranger and the follower: the follower is then
		// mutual with her, and she leads the stranger.
		`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,created_at) VALUES
		 ('edge.actor.follow.v1',1,'` + person + `',3,'` + person + `',NOW()),
		 ('edge.actor.follow.v1',1,'` + person + `',4,'` + person + `',NOW())`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}

	nodes := NewNodesRepo(db)
	rels := NewSocialgraphRelRepo(db)
	posts := []int64{visPublicPost, visFollowersPost, visPrivatePost, visGracePost, visGroupPost, 40, 41, 42, 43}

	viewers := []*entities.Viewer{entities.NewViewer()}
	for _, id := range []int64{visAda, visGrace, visStranger, visFollower, visBlocked} {
		viewers = append(viewers, visViewer(id, constants.PersonTypeRegistered))
	}

	for _, viewer := range viewers {
		visible, err := nodes.FilterVisible(posts, viewer)
		if err != nil {
			t.Fatal(err)
		}
		seen := map[int64]bool{}
		for _, id := range visible {
			seen[id] = true
		}

		for _, id := range posts {
			post, err := nodes.Read(id)
			if err != nil {
				t.Fatalf("reading %d: %v", id, err)
			}
			owner, err := nodes.Read(post.OwnerID)
			if err != nil {
				t.Fatalf("reading the owner of %d: %v", id, err)
			}
			post.Owner = owner
			rel, err := rels.GetOne(owner.ID, viewer.ID)
			if err != nil {
				t.Fatal(err)
			}

			canRead, _ := permissions.NewMediumPermissions().CanRead(&permissions.MediumAccessContext{
				Viewer:         viewer,
				Medium:         post,
				OwnerViewerRel: rel,
			})
			if canRead != seen[id] {
				t.Errorf("viewer %d, post %d (%s, on %d): CanRead says %v, FilterVisible %v",
					viewer.ID, id, post.Access, owner.ID, canRead, seen[id])
			}
		}
	}
}
