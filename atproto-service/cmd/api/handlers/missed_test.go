package handlers

import (
	"testing"
	"time"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

type fakeMissed struct {
	found []int64
	since time.Time
}

func (f *fakeMissed) PostsNotWritten(kinds []string, since, olderThan time.Time, limit int) ([]int64, error) {
	f.since = since
	return f.found, nil
}

// THE RULE THIS GUARDS (banu.io, 2026-11-22): a post whose making the bus
// lost is still written to Bluesky, found a while later, and once only.
func TestAPostTheBusLostIsWrittenOnce(t *testing.T) {
	w, _ := writingWorld(t)
	w.connect()
	w.nodes.kept = append(w.nodes.kept, &entities.AnahitaNode{
		ID: 99, Type: constants.NodeMediumNoteEntityType, Enabled: true, OwnerID: anaID, CreatedBy: anaID,
		Access: constants.AnahitaAccessPublic, Body: "Never heard of on the bus",
	})
	missed := &fakeMissed{found: []int64{99}}
	w.h.Missed = missed

	now := time.Now().UTC()
	if written := w.h.writeMissed(t.Context(), now); written != 1 {
		t.Fatalf("written %d, want 1", written)
	}
	if failed := w.drain(); len(failed) != 0 {
		t.Fatal(failed)
	}
	if len(w.writer.records) != 1 {
		t.Fatalf("%d posts on Bluesky, want 1", len(w.writer.records))
	}
	if !missed.since.Equal(now.Add(-missedWindow)) {
		t.Errorf("looked back to %v", missed.since)
	}

	// Found again, as a database that had not yet seen the first would:
	// nothing more is written.
	w.h.writeMissed(t.Context(), now)
	if failed := w.drain(); len(failed) != 0 || len(w.writer.records) != 1 {
		t.Errorf("written again: %v, %d posts", failed, len(w.writer.records))
	}
}
