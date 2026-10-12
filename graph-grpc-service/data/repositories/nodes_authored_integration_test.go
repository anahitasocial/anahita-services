package repositories

import (
	"testing"
)

// Run as the other integration tests here are, with ACCESS_TEST_DSN.
//
// THE RULE THIS GUARDS: a person's copy of their data is everything they
// wrote and nothing anybody else did. It includes what only they may see,
// which is why the list applies no access rule, and it is read in pages
// that neither repeat nor miss anything.
func TestWhatAPersonWroteIsAllOfItAndOnlyTheirs(t *testing.T) {
	db := accessTestDB(t)

	for _, statement := range []string{
		`DELETE FROM edges`,
		`DELETE FROM nodes`,
		`INSERT INTO nodes (id,type,service,name,alias,access,enabled,created_at,updated_at) VALUES
			(1,'node.actor.person-service.person.v1','person-service','Ana','ana','public',1,NOW(),NOW()),
			(2,'node.actor.person-service.person.v1','person-service','Bob','bob','public',1,NOW(),NOW()),
			(3,'node.actor.group-service.group.v1','group-service','Growers','growers','followers',1,NOW(),NOW())`,
		`INSERT INTO nodes (id,type,service,name,body,access,enabled,owner_id,owner_type,created_by,parent_id,root_id,deleted_at,created_at,updated_at) VALUES
			(10,'node.medium.text-service.note.v1','text-service','','On her own profile','public',1,1,'node.actor.person-service.person.v1',1,NULL,NULL,NULL,NOW(),NOW()),
			(11,'node.medium.text-service.note.v1','text-service','','For her alone','myself',1,1,'node.actor.person-service.person.v1',1,NULL,NULL,NULL,NOW(),NOW()),
			(12,'node.medium.photo-service.photo.v1','photo-service','In a group','','followers',1,3,'node.actor.group-service.group.v1',1,NULL,NULL,NULL,NOW(),NOW()),
			(13,'node.medium.text-service.note.v1','text-service','','Her reply to Bob','',1,1,'node.actor.person-service.person.v1',1,20,20,NULL,NOW(),NOW()),
			(14,'node.base.repost-service.repost.v1','repost-service','','','public',1,1,'node.actor.person-service.person.v1',1,20,NULL,NULL,NOW(),NOW()),
			(15,'node.feed.feed-service.generator.v1','feed-service','Gardens','','myself',1,1,'node.actor.person-service.person.v1',1,NULL,NULL,NULL,NOW(),NOW()),
			(16,'node.medium.text-service.note.v1','text-service','','One she removed','public',1,1,'node.actor.person-service.person.v1',1,NULL,NULL,NOW(),NOW(),NOW()),
			(20,'node.medium.text-service.note.v1','text-service','','Bob on his profile','public',1,2,'node.actor.person-service.person.v1',2,NULL,NULL,NULL,NOW(),NOW()),
			(21,'node.medium.text-service.note.v1','text-service','','Bob on her profile','public',1,1,'node.actor.person-service.person.v1',2,NULL,NULL,NULL,NOW(),NOW())`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("seed: %v\n%s", err, statement)
		}
	}

	repo := NewNodesRepo(db)

	// In pages of two, each starting after the last of the one before.
	var got []int64
	after := int64(0)
	for {
		page, err := repo.Authored(1, after, 2)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		if len(page) > 2 {
			t.Fatalf("a page of %d", len(page))
		}
		for _, node := range page {
			got = append(got, node.ID)
			after = node.ID
		}
	}

	want := []int64{10, 11, 12, 13, 14, 15}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}

	// What Bob wrote on her profile is Bob's.
	bobs, err := repo.Authored(2, 0, 50)
	if err != nil || len(bobs) != 2 || bobs[0].ID != 20 || bobs[1].ID != 21 {
		t.Errorf("Bob's: %v %v", bobs, err)
	}
	// Nobody in particular wrote nothing.
	if none, _ := repo.Authored(0, 0, 50); len(none) != 0 {
		t.Errorf("nobody wrote %d things", len(none))
	}
}
