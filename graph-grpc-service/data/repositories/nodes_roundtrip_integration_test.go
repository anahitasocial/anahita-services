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

	// Whether reposts show among the profile's posts can be turned on and, unlike
	// a setting that is only written when it is true, off again.
	for _, show := range []bool{true, false} {
		person.ShowRepostsOnProfile = show
		person.ModifiedBy = person.ID
		if _, err := repo.Edit(person); err != nil {
			t.Fatalf("edit the person: %v", err)
		}
		got, err := repo.Read(person.ID)
		if err != nil || got.ShowRepostsOnProfile != show {
			t.Fatalf("show_reposts_on_profile is %v (err %v), want %v", got.ShowRepostsOnProfile, err, show)
		}
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
	note.CommentStatus = true
	note.ReplyAccess = "followers,mentioned"

	note, err = repo.Add(note)
	if err != nil {
		t.Fatalf("add the note: %v", err)
	}
	if note.ReplyAccess != "followers,mentioned" {
		t.Errorf("added with reply_access %q, want followers,mentioned", note.ReplyAccess)
	}

	note.Body = "hello again"
	note.ViewCount = 3
	note.ModifiedBy = person.ID
	if _, err := repo.Edit(note); err != nil {
		t.Fatalf("edit: %v", err)
	}

	got, err := repo.Read(note.ID)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.Body != "hello again" || got.ViewCount != 3 {
		t.Errorf("read back body=%q view_count=%d, want %q, 3",
			got.Body, got.ViewCount, "hello again")
	}

	// Who may reply was not part of that edit, and is as it was. An edit
	// that does not say leaves it alone; one that says changes it, and
	// "anyone" is how it is set back.
	if got.ReplyAccess != "followers,mentioned" {
		t.Errorf("reply_access %q after an edit that did not change it, want followers,mentioned", got.ReplyAccess)
	}
	got.ReplyAccess = ""
	got.ModifiedBy = person.ID
	if _, err := repo.Edit(got); err != nil {
		t.Fatalf("edit without reply_access: %v", err)
	}
	if got, err = repo.Read(note.ID); err != nil || got.ReplyAccess != "followers,mentioned" {
		t.Errorf("reply_access %q (err %v) after an edit that carried none, want followers,mentioned", got.ReplyAccess, err)
	}
	got.ReplyAccess = "anyone"
	got.ModifiedBy = person.ID
	if _, err := repo.Edit(got); err != nil {
		t.Fatalf("edit reply_access: %v", err)
	}
	if got, err = repo.Read(note.ID); err != nil || got.ReplyAccess != "anyone" {
		t.Errorf("reply_access %q (err %v), want anyone", got.ReplyAccess, err)
	}

	// A note that quotes the first. What it quotes is set when it is made
	// and read back; the quoted note is counted, and counted again when its
	// author takes it out of the quote.
	got.QuotePolicy = "followers"
	got.ModifiedBy = person.ID
	if _, err := repo.Edit(got); err != nil {
		t.Fatalf("edit quote_policy: %v", err)
	}

	quoting := entities.NewAnahitaNode()
	quoting.Type = note.Type
	quoting.Service = note.Service
	quoting.Body = "look at this"
	quoting.Access = "public"
	quoting.Enabled = true
	quoting.OwnerID = person.ID
	quoting.OwnerType = person.Type
	quoting.CreatedBy = person.ID
	quoting.QuoteID = note.ID
	if quoting, err = repo.Add(quoting); err != nil {
		t.Fatalf("add the quoting note: %v", err)
	}
	if quoting.QuoteID != note.ID || quoting.QuoteDetached {
		t.Errorf("quoting note: quote_id %d, detached %v", quoting.QuoteID, quoting.QuoteDetached)
	}

	quotes := NewQuotesRepo(db)
	quoteCount := func() int64 {
		quoted, err := repo.Read(note.ID)
		if err != nil {
			t.Fatalf("read the quoted note: %v", err)
		}
		if quoted.QuotePolicy != "followers" {
			t.Errorf("quote_policy %q, want followers", quoted.QuotePolicy)
		}
		return quoted.QuoteCount
	}

	if err := quotes.Recount(note.ID); err != nil {
		t.Fatal(err)
	}
	if n := quoteCount(); n != 1 {
		t.Errorf("quote_count %d, want 1", n)
	}

	// Listed as a quote of the first, to somebody who may see it.
	visitor := entities.NewViewer()
	if listed, err := quotes.List(note.ID, visitor, 0); err != nil || len(listed) != 1 || listed[0].ID != quoting.ID {
		t.Errorf("quotes listed to a visitor: %v, %v", listed, err)
	}

	// An edit of the quoting note leaves what it quotes alone.
	quoting.Body = "look at this again"
	quoting.ModifiedBy = person.ID
	if _, err := repo.Edit(quoting); err != nil {
		t.Fatal(err)
	}

	for step, wantChanged := range []bool{true, false} {
		changed, err := quotes.Detach(quoting.ID, person.ID)
		if err != nil || changed != wantChanged {
			t.Errorf("detach %d: changed %v, err %v; want %v", step, changed, err, wantChanged)
		}
	}
	if err := quotes.Recount(note.ID); err != nil {
		t.Fatal(err)
	}
	if n := quoteCount(); n != 0 {
		t.Errorf("quote_count %d after the post was taken out, want 0", n)
	}
	if listed, err := quotes.List(note.ID, visitor, 0); err != nil || len(listed) != 0 {
		t.Errorf("a note the post was taken out of is still listed: %v, %v", listed, err)
	}
	if after, err := repo.Read(quoting.ID); err != nil || !after.QuoteDetached || after.QuoteID != note.ID || after.Body != "look at this again" {
		t.Errorf("the quoting note after: %+v, %v", after, err)
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
