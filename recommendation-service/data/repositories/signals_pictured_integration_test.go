package repositories

import (
	"fmt"
	"testing"
	"time"

	"recommendation/constants"
)

// People in photos and videos (roadmap row 104), on top of seed: the viewer
// posted a public photo with 30 in it, and 31 waiting; leader B posted one
// with 32 and 30 in it; leader C posted one for followers with 31 in it.
func TestPeopleInPhotosWithTheViewerAndTheirLeaders(t *testing.T) {
	db := testDB(t)
	seed(t, db)

	const (
		photo   = "'node.medium.photo-service.photo.v1','photo-service'"
		aPerson = "'node.actor.person-service.person.v1'"
		aPhoto  = "'node.medium.photo-service.photo.v1'"
	)
	tag := func(person, post int, meta string) string {
		return fmt.Sprintf("('edge.tag.person.v1',%d,%s,%d,%s,'%s',NOW())", person, aPerson, post, aPhoto, meta)
	}
	for _, stmt := range []string{
		`INSERT INTO nodes (id,type,service,body,access,enabled,owner_id,owner_type,created_by,created_at,updated_at) VALUES
		 (300,` + photo + `,'At the lake','public',1,1,` + aPerson + `,1,NOW(),NOW()),
		 (301,` + photo + `,'On the bridge','public',1,2,` + aPerson + `,2,NOW(),NOW()),
		 (302,` + photo + `,'For my followers','followers',1,3,` + aPerson + `,3,NOW(),NOW())`,
		`INSERT INTO edges (type,node_a_id,node_a_type,node_b_id,node_b_type,meta,created_at) VALUES ` +
			tag(30, 300, "approved") + "," + tag(31, 300, "pending") + "," +
			tag(32, 301, "approved") + "," + tag(30, 301, "approved") + "," +
			tag(31, 302, "approved"),
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}

	found, err := NewSignals(db).Find(viewer, constants.KindPerson, true, false, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	// 30 is in the viewer's photo; 31's tag there still waits.
	if got := fmt.Sprint(idsOf(found[constants.ReasonPicturedWithYou])); got != "[30]" {
		t.Errorf("pictured with you: %s", got)
	}
	// 32 and 30 are in B's public photo; 31 only in C's for followers.
	if got := fmt.Sprint(idsOf(found[constants.ReasonPicturedWithLeaders])); got != "[30 32]" {
		t.Errorf("pictured with leaders: %s", got)
	}
	if hit := hitFor(found[constants.ReasonPicturedWithLeaders], 32); hit == nil || fmt.Sprint(hit.IDs) != "[2]" {
		t.Errorf("with whom 32 is pictured: %+v", hit)
	}

	// Not when the viewer said what they do is not to be used.
	found, err = NewSignals(db).Find(viewer, constants.KindPerson, false, false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(found[constants.ReasonPicturedWithYou]) > 0 || len(found[constants.ReasonPicturedWithLeaders]) > 0 {
		t.Errorf("found without leave: %v", found)
	}
}
