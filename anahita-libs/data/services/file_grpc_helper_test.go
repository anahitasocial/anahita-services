package services

import (
	"strings"
	"testing"

	"anahita-libs/data/entities"
)

// A list's actors, their covers, and its posts' owners and authors are all
// signed; a nil entry is skipped, and a post is not mistaken for an actor.
func TestBatchPresignNodesSignsEveryPicture(t *testing.T) {
	worf := &entities.AnahitaNode{ID: 2, Type: "node.actor.person.person", AvatarFilename: "worf.jpg", CoverFilename: "bridge.jpg"}
	dax := &entities.AnahitaNode{ID: 3, Type: "node.actor.person.person", AvatarFilename: "dax.jpg"}
	note := &entities.AnahitaNode{ID: 4, Type: "node.medium.note", AvatarFilename: "not-an-actor.jpg", Owner: worf, Author: dax}

	files := &fakePresigner{}
	BatchPresignNodes(files, []*entities.AnahitaNode{worf, nil, note})

	for _, size := range worf.GetAvatarSizes() {
		if url := worf.Avatar[size].Url; !strings.HasPrefix(url, "https://media.example/") {
			t.Errorf("Worf's avatar, %s: %q not signed", size, url)
		}
		if url := dax.Avatar[size].Url; !strings.HasPrefix(url, "https://media.example/") {
			t.Errorf("Dax's avatar, as the note's author, %s: %q not signed", size, url)
		}
	}
	for size, cover := range worf.Cover {
		if !strings.HasPrefix(cover.Url, "https://media.example/") {
			t.Errorf("Worf's cover, %s: %q not signed", size, cover.Url)
		}
	}
	for _, path := range files.asked {
		if strings.Contains(path, "not-an-actor") {
			t.Errorf("a note's avatar was signed: %q", path)
		}
	}
}
