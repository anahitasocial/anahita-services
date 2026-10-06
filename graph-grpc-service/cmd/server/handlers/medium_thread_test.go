package handlers

import (
	"testing"
	"time"

	"anahita-libs/data/entities"
)

const threadNote = "node.medium.text-service.note.v1"

func post(id int64, nodeType string, root int64) *entities.AnahitaNode {
	node := entities.NewAnahitaNode()
	node.ID = id
	node.Type = nodeType
	node.RootID = root
	return node
}

// Every reply in a thread names the same root, however deep it is.
func TestAReplyNamesTheRootOfItsThread(t *testing.T) {
	// To a post at the top level: the post is the root.
	direct := entities.NewAnahitaNode()
	direct.Access = "public"
	if err := placeInThread(direct, post(100, "node.medium.photo-service.photo.v1", 0)); err != nil {
		t.Fatal(err)
	}
	if direct.ParentID != 100 || direct.RootID != 100 || direct.ParentType != "node.medium.photo-service.photo.v1" {
		t.Errorf("a direct reply: parent %d (%s), root %d", direct.ParentID, direct.ParentType, direct.RootID)
	}
	// It has no audience of its own, whatever it was sent with.
	if direct.Access != "" {
		t.Errorf("a reply kept an audience of its own: %q", direct.Access)
	}

	// To a reply: that reply's root, not the reply.
	nested := entities.NewAnahitaNode()
	if err := placeInThread(nested, post(200, threadNote, 100)); err != nil {
		t.Fatal(err)
	}
	if nested.ParentID != 200 || nested.RootID != 100 {
		t.Errorf("a reply to a reply: parent %d, root %d; want 200 and 100", nested.ParentID, nested.RootID)
	}
}

func TestOnlyAPostThatIsStillThereCanBeAnswered(t *testing.T) {
	for what, parent := range map[string]*entities.AnahitaNode{
		"a profile": post(1, "node.actor.person-service.person.v1", 0),
		"a hashtag": post(30, "node.tag.hashtag-service.hashtag.v1", 0),
		"a repost":  post(300, "node.base.repost-service.repost.v1", 0),
		"nothing":   nil,
	} {
		if err := placeInThread(entities.NewAnahitaNode(), parent); err == nil {
			t.Errorf("a reply to %s was accepted", what)
		}
	}

	removed := post(212, threadNote, 101)
	when := time.Now()
	removed.DeletedAt = &when
	if err := placeInThread(entities.NewAnahitaNode(), removed); err == nil {
		t.Error("a reply to a removed reply was accepted")
	}
}
