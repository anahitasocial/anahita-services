package handlers

import (
	"errors"
	"testing"
	"time"

	"anahita-libs/data/entities"
	"anahita-libs/data/services"
)

type fakeThreadMedia struct {
	services.MediaGRPCInterface
	nodes map[int64]*entities.AnahitaNode
}

func (f *fakeThreadMedia) Read(id int64, _ *entities.Viewer) (*entities.AnahitaNode, error) {
	if node, ok := f.nodes[id]; ok {
		return node, nil
	}
	return nil, errors.New("no such node")
}

func TestARemovedReplyShowsNothingOfWhatItWas(t *testing.T) {
	gone := time.Now()
	reply := entities.NewAnahitaNode()
	reply.DeletedAt = &gone
	reply.Body = "words"
	reply.Filename = "one.jpg"
	reply.Files = []*entities.PhotoFile{{Filename: "one.jpg"}}
	reply.Meta = []byte(`{"video":{"poster":"p.jpg"}}`)

	BlankRemoved(reply)

	if reply.Body != "" || reply.Filename != "" || reply.Files != nil || reply.Meta != nil {
		t.Errorf("left %+v", reply)
	}

	// One that is there is left alone.
	kept := entities.NewAnahitaNode()
	kept.Filename = "two.jpg"
	BlankRemoved(kept)
	if kept.Filename != "two.jpg" {
		t.Error("a reply that is there was emptied")
	}
}

func TestAReplyMadeLaterAsksAgain(t *testing.T) {
	gone := time.Now()
	removed := entities.NewAnahitaNode()
	removed.ID, removed.RootID, removed.DeletedAt = 7, 1, &gone
	readers := ThreadReaders{Media: &fakeThreadMedia{nodes: map[int64]*entities.AnahitaNode{7: removed}}}
	viewer := entities.NewViewer()
	viewer.ID = 5

	// What it answers was removed: no, and no error, since that will not
	// pass.
	if may, err := MayReplyTo(readers, 7, viewer); may || err != nil {
		t.Errorf("a removed reply: %v %v", may, err)
	}
	// What it answers cannot be read: an error, which may pass.
	if may, err := MayReplyTo(readers, 8, viewer); may || err == nil {
		t.Errorf("nothing to read: %v %v", may, err)
	}
}
