package repositories

import (
	"fmt"
	"testing"

	"anahita-libs/constants"
	"anahita-libs/proto/graphpb"

	"graph/requests"
)

// Pinning against a real database.
//
//	ACCESS_TEST_DSN="root@tcp(127.0.0.1:13309)/fresh?parseTime=true" \
//	  go test ./data/repositories/ -run TestPin -v

func TestPinningAPostToItsProfile(t *testing.T) {
	db := accessTestDB(t)
	for _, stmt := range []string{`DELETE FROM edges`, `DELETE FROM nodes`} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("clear: %v", err)
		}
	}

	const person = "'node.actor.person-service.person.v1','person-service'"
	const personType = "'node.actor.person-service.person.v1'"
	if _, err := db.Exec(`INSERT INTO nodes (id,type,service,name,alias,access,enabled,created_at,updated_at) VALUES
		(1,` + person + `,'Ada','ada','public',1,NOW(),NOW()),
		(2,` + person + `,'Grace','grace','public',1,NOW(),NOW())`); err != nil {
		t.Fatal(err)
	}

	// Seven notes on Ada's profile, oldest first, one of them for her
	// followers only; a reply under the first; and one on Grace's.
	for i := 1; i <= 7; i++ {
		access := "public"
		if i == 7 {
			access = "followers"
		}
		if _, err := db.Exec(fmt.Sprintf(`INSERT INTO nodes (id,type,service,body,alias,access,enabled,owner_id,owner_type,created_by,created_at,updated_at) VALUES
			(%d,'%s','text-service','note %d','n%d','%s',1,1,%s,1,NOW() - INTERVAL %d MINUTE,NOW())`,
			10+i, noteType, i, i, access, personType, 100-i)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO nodes (id,type,service,body,alias,access,enabled,owner_id,owner_type,created_by,parent_id,root_id,created_at,updated_at) VALUES
		(30,'` + noteType + `','text-service','a reply','r30','',1,1,` + personType + `,1,11,11,NOW(),NOW())`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO nodes (id,type,service,body,alias,access,enabled,owner_id,owner_type,created_by,created_at,updated_at) VALUES
		(40,'` + noteType + `','text-service','grace','n40','public',1,2,` + personType + `,2,NOW(),NOW())`); err != nil {
		t.Fatal(err)
	}

	pins := NewPinsRepo(db)
	nodes := NewNodesRepo(db)

	// Pinned one at a time, each a moment after the last, so the order is
	// the order they were pinned in.
	pin := func(id int64) error {
		err := pins.Pin(id, 1, 1)
		if err == nil {
			if _, err := db.Exec(`UPDATE nodes SET pinned_at = pinned_at + INTERVAL ? SECOND WHERE id = ?`, id, id); err != nil {
				t.Fatal(err)
			}
		}
		return err
	}

	feed := NewFeedActorRepo(db)
	listed := func(viewer int64, offset uint64) []int64 {
		rows, err := feed.Browse(&graphpb.FeedActorBrowseRequest{
			ActorId:           1,
			Filter:            constants.FeedFilterPosts,
			MediumEntityTypes: []string{noteType},
			Limit:             4,
			Offset:            offset,
		}, replyViewer(viewer))
		if err != nil {
			t.Fatalf("browse: %v", err)
		}
		ids := []int64{}
		for _, row := range rows {
			ids = append(ids, row.ID)
		}
		return ids
	}

	// By date, newest first, the notes are 17 down to 11. 17 is for
	// followers only.
	if err := pin(17); err != nil {
		t.Fatal(err)
	}
	// Pinning one that is pinned already changes nothing.
	if err := pins.Pin(17, 1, 1); err != nil {
		t.Errorf("pinning a pinned post again: %v", err)
	}
	if pinned, _ := pins.PinnedIDs(1); fmt.Sprint(pinned) != "[17]" {
		t.Errorf("pinned: %v", pinned)
	}

	// Pinning does not show a post to anybody who could not see it.
	if got := fmt.Sprint(listed(0, 0), listed(0, 4)); got != "[16 15 14 13] [12 11]" {
		t.Errorf("Ada's posts as a visitor sees them, with 17 pinned: %s", got)
	}

	// A reply is not pinned, and neither is a post on somebody else's
	// profile named as if it were on this one. Asking for either moves
	// nothing: the pin stays where it was.
	for _, id := range []int64{30, 40} {
		if err := pins.Pin(id, 1, 1); err != nil {
			t.Fatalf("pin %d: %v", id, err)
		}
		if got, _ := nodes.Read(id); got.Pinned {
			t.Errorf("%d was pinned", id)
		}
		if pinned, _ := pins.PinnedIDs(1); fmt.Sprint(pinned) != "[17]" {
			t.Errorf("asking to pin %d moved the pin: %v", id, pinned)
		}
	}

	// A profile has as many pins as constants.MaxPinnedPosts. Pinning
	// another moves the pin: the oldest gives way.
	if err := pin(11); err != nil {
		t.Fatal(err)
	}
	pinned, err := pins.PinnedIDs(1)
	if err != nil || len(pinned) != constants.MaxPinnedPosts || pinned[0] != 11 {
		t.Errorf("pinned: %v, %v; want %d, led by 11", pinned, err, constants.MaxPinnedPosts)
	}
	if got, _ := nodes.Read(17); got.Pinned {
		t.Error("the post that gave up the pin is still pinned")
	}

	// The profile's posts: the pinned one first, though it is the oldest,
	// then the rest by date. Each once.
	if got := fmt.Sprint(listed(1, 0), listed(1, 4)); got != "[11 17 16 15] [14 13 12]" {
		t.Errorf("Ada's posts as Ada sees them: %s", got)
	}
	if got := fmt.Sprint(listed(0, 0), listed(0, 4)); got != "[11 16 15 14] [13 12]" {
		t.Errorf("Ada's posts as a visitor sees them: %s", got)
	}

	// The same lead in the list of one kind of post.
	browsed, err := nodes.Browse(&requests.NodeBrowseRequest{
		Types:        []string{noteType},
		OwnerIDs:     []int64{1},
		Ordering:     constants.OrderingRecent,
		Limit:        4,
		Enabled:      true,
		TopLevelOnly: true,
		Viewer:       replyViewer(1),
	})
	if err != nil {
		t.Fatalf("browse notes: %v", err)
	}
	first := []int64{}
	for _, row := range browsed {
		first = append(first, row.ID)
	}
	if got := fmt.Sprint(first); got != "[11 17 16 15]" {
		t.Errorf("Ada's notes: %s", got)
	}

	// An edit of a pinned post leaves it pinned.
	note, _ := nodes.Read(11)
	note.Body = "edited"
	note.ModifiedBy = 1
	if _, err := nodes.Edit(note); err != nil {
		t.Fatal(err)
	}
	if got, _ := nodes.Read(11); !got.Pinned || got.Body != "edited" {
		t.Errorf("after an edit: pinned %v, body %q", got.Pinned, got.Body)
	}

	// Unpinned, it goes back to its place by date.
	if err := pins.Unpin(11, 1); err != nil {
		t.Fatal(err)
	}
	if got, _ := nodes.Read(11); got.Pinned {
		t.Error("still pinned after unpinning")
	}
	if got := fmt.Sprint(listed(1, 0), listed(1, 4)); got != "[17 16 15 14] [13 12 11]" {
		t.Errorf("after unpinning: %s", got)
	}
}
