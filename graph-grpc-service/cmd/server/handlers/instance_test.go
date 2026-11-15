package handlers

import (
	"os"
	"path/filepath"
	"testing"

	"anahita-libs/data/entities"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Whichever service asks, a post of a kind the instance does not offer is
// not saved; one from another server is, whatever its kind.
func TestAPostTheInstanceDoesNotOfferIsNotSaved(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "instance.json"), []byte(`[{"service":"text-service","node_types":["note"]}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	article := &entities.AnahitaNode{Type: "node.medium.text-service.article.v1"}
	if err := refuseUnoffered(dir, article, false); status.Code(err) != codes.PermissionDenied {
		t.Errorf("an article was let through: %v", err)
	}
	if err := refuseUnoffered(dir, &entities.AnahitaNode{Type: "node.medium.text-service.note.v1"}, false); err != nil {
		t.Errorf("a note was refused: %v", err)
	}
	article.URI = "https://elsewhere.example/articles/1"
	if err := refuseUnoffered(dir, article, false); err != nil {
		t.Errorf("an article from elsewhere was refused: %v", err)
	}
	if err := refuseUnoffered(t.TempDir(), &entities.AnahitaNode{Type: "node.medium.text-service.article.v1"}, false); err != nil {
		t.Errorf("refused with no instance.json: %v", err)
	}
}

// A reply is a kind of its own: a site of photos alone, which lists
// "reply", keeps the conversation under them and takes no note; a site that
// offers notes takes replies with them, as before reply was a kind.
func TestARepliesOnlySiteTakesRepliesAndNoNotes(t *testing.T) {
	note := func() *entities.AnahitaNode { return &entities.AnahitaNode{Type: "node.medium.text-service.note.v1"} }
	gallery := t.TempDir()
	if err := os.WriteFile(filepath.Join(gallery, "instance.json"), []byte(`[{"service":"text-service","node_types":["reply"]},{"service":"photo-service","node_types":["photo"]}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := refuseUnoffered(gallery, note(), true); err != nil {
		t.Errorf("a reply was refused: %v", err)
	}
	if err := refuseUnoffered(gallery, note(), false); status.Code(err) != codes.PermissionDenied {
		t.Errorf("a note was let through: %v", err)
	}
	if err := refuseUnoffered(gallery, &entities.AnahitaNode{Type: "node.medium.text-service.article.v1"}, true); status.Code(err) != codes.PermissionDenied {
		t.Errorf("an article was let through as a reply: %v", err)
	}
	if err := refuseUnoffered(gallery, &entities.AnahitaNode{Type: "node.medium.photo-service.photo.v1"}, false); err != nil {
		t.Errorf("a photo was refused: %v", err)
	}

	notes := t.TempDir()
	if err := os.WriteFile(filepath.Join(notes, "instance.json"), []byte(`[{"service":"text-service","node_types":["note"]}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := refuseUnoffered(notes, note(), true); err != nil {
		t.Errorf("a reply was refused where notes are offered: %v", err)
	}

	none := t.TempDir()
	if err := os.WriteFile(filepath.Join(none, "instance.json"), []byte(`[{"service":"text-service","node_types":["article"]}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := refuseUnoffered(none, note(), true); status.Code(err) != codes.PermissionDenied {
		t.Errorf("a reply was let through where neither replies nor notes are offered: %v", err)
	}
}
