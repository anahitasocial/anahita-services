package repositories

import (
	"testing"

	"anahita-libs/constants"
	"anahita-libs/proto/graphpb"
)

// What is trending, against a real database.
//
//	ACCESS_TEST_DSN="root@tcp(127.0.0.1:13309)/fresh?parseTime=true" \
//	  go test ./data/repositories/ -run TestTrending -v
//
// WHAT THIS GUARDS. A tag trends by the profiles that used it (the user,
// 2026-10-18): the writers of posts carrying it and the profiles tagged with
// it, of every kind, each once however many times. A post counts by when it
// was written and a profile by when it was tagged, not by when the profile
// was made; and only what anybody may see counts. The same for hashtags and
// for locations, which share the query.
func TestTrendingCountsEachProfileOnce(t *testing.T) {
	db := accessTestDB(t)
	repo := NewGraphRepo(db)

	const person = "'node.actor.person-service.person.v1','person-service'"
	const remote = "'node.actor.activitypub-service.person.v1','activitypub-service'"
	const website = "'node.actor.actor-service.website.v1','actor-service'"
	const group = "'node.actor.group-service.group.v1','group-service'"
	const note = "'node.medium.text-service.note.v1','text-service'"
	const noteType = "'node.medium.text-service.note.v1'"
	const personType = "'node.actor.person-service.person.v1'"
	const groupType = "'node.actor.group-service.group.v1'"

	for _, tag := range []struct {
		name     string
		nodeType string
		edgeType string
	}{
		{"hashtags", constants.NodeBaseHashtagEntityType, constants.EdgeTagHashtagType},
		{"locations", constants.NodeBaseLocationEntityType, constants.EdgeTagLocationType},
	} {
		t.Run(tag.name, func(t *testing.T) {
			tagType := "'" + tag.nodeType + "'"
			tagNode := tagType + ",'tag-service'"
			edge := "'" + tag.edgeType + "'"

			for _, stmt := range []string{
				`DELETE FROM edges`,
				`DELETE FROM nodes`,

				// Kira and Odo here, Garak on another server, Quark's
				// website, the Bajor group, and Section 31, a group nobody
				// may see.
				`INSERT INTO nodes (id,type,service,name,alias,access,enabled,created_by,created_at,updated_at) VALUES
				 (1,` + person + `,'Kira','kira','public',1,NULL,NOW() - INTERVAL 3 YEAR,NOW()),
				 (2,` + person + `,'Odo','odo','public',1,NULL,NOW() - INTERVAL 3 YEAR,NOW()),
				 (3,` + remote + `,'Garak','garak','public',1,NULL,NOW(),NOW()),
				 (4,` + website + `,'Quark''s','quarks','public',1,1,NOW(),NOW()),
				 (5,` + group + `,'Bajor','bajor','public',1,1,NOW() - INTERVAL 3 YEAR,NOW()),
				 (6,` + group + `,'Section 31','section-31','followers',1,1,NOW(),NOW())`,

				`INSERT INTO nodes (id,type,service,name,alias,access,enabled,created_at,updated_at) VALUES
				 (10,` + tagNode + `,'Promenade','promenade','public',1,NOW(),NOW()),
				 (11,` + tagNode + `,'Ops','ops','public',1,NOW(),NOW()),
				 (12,` + tagNode + `,'Deep Space Nine','deep-space-nine','public',1,NOW(),NOW()),
				 (13,` + tagNode + `,'Terok Nor','terok-nor','public',1,NOW(),NOW())`,

				`INSERT INTO nodes (id,type,service,name,body,alias,access,enabled,owner_id,owner_type,created_by,created_at,updated_at) VALUES
				 (100,` + note + `,'','kira','p100','public',1,1,` + personType + `,1,NOW(),NOW()),
				 (101,` + note + `,'','kira again','p101','public',1,1,` + personType + `,1,NOW(),NOW()),
				 (102,` + note + `,'','odo','p102','public',1,2,` + personType + `,2,NOW(),NOW()),
				 (103,` + note + `,'','garak','p103','public',1,3,` + personType + `,3,NOW(),NOW()),
				 (104,` + note + `,'','from the feed','p104','public',1,4,'node.actor.actor-service.website.v1',4,NOW(),NOW()),
				 (105,` + note + `,'','odo again','p105','public',1,2,` + personType + `,2,NOW(),NOW()),
				 (106,` + note + `,'','only odo','p106','myself',1,2,` + personType + `,2,NOW(),NOW()),
				 (107,` + note + `,'','long ago','p107','public',1,1,` + personType + `,1,NOW() - INTERVAL 2 YEAR,NOW())`,

				// Promenade: three people's posts, Kira's twice. Ops: a
				// website's post and the Bajor group, made long ago but
				// tagged today. Deep Space Nine: Odo's profile and Odo's
				// post, one profile. Terok Nor: only what does not count,
				// a post nobody else may see, a post from long ago, a
				// profile tagged long ago, and a group nobody may see.
				`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,created_at) VALUES
				 (` + edge + `,10,` + tagType + `,100,` + noteType + `,NOW()),
				 (` + edge + `,10,` + tagType + `,101,` + noteType + `,NOW()),
				 (` + edge + `,10,` + tagType + `,102,` + noteType + `,NOW()),
				 (` + edge + `,10,` + tagType + `,103,` + noteType + `,NOW()),
				 (` + edge + `,11,` + tagType + `,104,` + noteType + `,NOW()),
				 (` + edge + `,11,` + tagType + `,5,` + groupType + `,NOW()),
				 (` + edge + `,12,` + tagType + `,2,` + personType + `,NOW()),
				 (` + edge + `,12,` + tagType + `,105,` + noteType + `,NOW()),
				 (` + edge + `,13,` + tagType + `,106,` + noteType + `,NOW()),
				 (` + edge + `,13,` + tagType + `,107,` + noteType + `,NOW()),
				 (` + edge + `,13,` + tagType + `,1,` + personType + `,NOW() - INTERVAL 2 YEAR),
				 (` + edge + `,13,` + tagType + `,6,` + groupType + `,NOW())`,
			} {
				if _, err := db.Exec(stmt); err != nil {
					t.Fatalf("seed: %v\n%s", err, stmt)
				}
			}

			req := &graphpb.TrendingRequest{
				NodeType:   tag.nodeType,
				EdgeTypes:  []string{tag.edgeType},
				TimeWindow: constants.TimeWindowYearly,
				Limit:      10,
			}
			found, err := repo.Trending(req, nil)
			if err != nil {
				t.Fatal(err)
			}
			got := []int64{}
			for _, node := range found {
				got = append(got, node.ID)
			}
			want := []int64{10, 11, 12}
			if len(got) != len(want) || got[0] != 10 || got[1] != 11 || got[2] != 12 {
				t.Fatalf("trending %v; want %v, most profiles first, and Terok Nor not at all", got, want)
			}

			profiles, err := repo.TrendingProfiles(req, []int64{10, 11, 12, 13})
			if err != nil {
				t.Fatal(err)
			}
			if profiles[10] != 3 {
				t.Errorf("Promenade: %d profiles; want Kira once, Odo and Garak", profiles[10])
			}
			if profiles[11] != 2 {
				t.Errorf("Ops: %d profiles; want the website and the group", profiles[11])
			}
			if profiles[12] != 1 {
				t.Errorf("Deep Space Nine: %d profiles; want Odo once, for his profile and his post", profiles[12])
			}
			if profiles[13] != 0 {
				t.Errorf("Terok Nor: %d profiles; nothing on it counts", profiles[13])
			}
		})
	}
}
