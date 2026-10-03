package repositories

import (
	"testing"

	"anahita-libs/data/entities"
	"graph/requests"
)

// A node written through the repository can be edited, read back one at a
// time and in a batch, and tombstoned.
//
// Every one of those statements carries its own hand-written list of column
// names, and the lists are plain strings: a column missing from the table, or
// a list one entry longer than the values scanned from it, compiles and fails
// only when the statement runs. This is the test that runs them.
func TestNodeRoundTripAgainstTheSchema(t *testing.T) {
	db := accessTestDB(t)
	for _, stmt := range []string{`DELETE FROM edges`, `DELETE FROM nodes`} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("clear: %v", err)
		}
	}
	repo := NewNodesRepo(db)

	person := entities.NewAnahitaNode()
	person.Type = "node.actor.person-service.person.v1"
	person.Service = "person-service"
	person.Name = "Alice"
	person.Alias = "alice"
	person.Access = "public"
	person.Enabled = true

	person, err := repo.Add(person)
	if err != nil {
		t.Fatalf("add the person: %v", err)
	}

	note := entities.NewAnahitaNode()
	note.Type = "node.medium.text-service.note.v1"
	note.Service = "text-service"
	note.Body = "hello"
	note.Access = "public"
	note.Enabled = true
	note.OwnerID = person.ID
	note.OwnerType = person.Type
	note.CreatedBy = person.ID

	note, err = repo.Add(note)
	if err != nil {
		t.Fatalf("add the note: %v", err)
	}

	note.Body = "hello again"
	note.ViewCount = 3
	note.Pinned = true
	note.ModifiedBy = person.ID
	if _, err := repo.Edit(note); err != nil {
		t.Fatalf("edit: %v", err)
	}

	got, err := repo.Read(note.ID)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.Body != "hello again" || got.ViewCount != 3 || !got.Pinned {
		t.Errorf("read back body=%q view_count=%d pinned=%v, want %q, 3, true",
			got.Body, got.ViewCount, got.Pinned, "hello again")
	}

	batch, err := repo.BatchRead(&requests.NodeBatchReadRequest{IDs: []int64{person.ID, note.ID}})
	if err != nil {
		t.Fatalf("batch read: %v", err)
	}
	if len(batch) != 2 {
		t.Errorf("batch read returned %d nodes, want 2", len(batch))
	}

	if _, err := repo.SoftDeleteActor(person.ID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if done, err := repo.TombstoneActor(person.ID); err != nil || !done {
		t.Fatalf("tombstone: done=%v err=%v", done, err)
	}

	// A tombstone has to stay readable: it is what reserves the handle. It
	// used to be left with a NULL status, which the scanner could not read.
	tomb, err := repo.Read(person.ID)
	if err != nil {
		t.Fatalf("read the tombstone: %v", err)
	}
	if tomb.Alias != "alice" || tomb.Name != "" {
		t.Errorf("tombstone alias=%q name=%q, want the alias kept and the name gone", tomb.Alias, tomb.Name)
	}
}
