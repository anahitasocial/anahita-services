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
	if err := refuseUnoffered(dir, article); status.Code(err) != codes.PermissionDenied {
		t.Errorf("an article was let through: %v", err)
	}
	if err := refuseUnoffered(dir, &entities.AnahitaNode{Type: "node.medium.text-service.note.v1"}); err != nil {
		t.Errorf("a note was refused: %v", err)
	}
	article.URI = "https://elsewhere.example/articles/1"
	if err := refuseUnoffered(dir, article); err != nil {
		t.Errorf("an article from elsewhere was refused: %v", err)
	}
	if err := refuseUnoffered(t.TempDir(), &entities.AnahitaNode{Type: "node.medium.text-service.article.v1"}); err != nil {
		t.Errorf("refused with no instance.json: %v", err)
	}
}
