package repositories

import (
	"testing"

	"anahita-libs/data/entities"
)

// An edit needs the edge and who makes it, nothing more. It used to call
// itself, and an edge edited for the first time (a tag approved, roadmap row
// 103) took the service down.
func TestAnEdgeEditIsCheckedOnce(t *testing.T) {
	r := &edgesRepo{}
	edge := entities.NewAnahitaEdge()
	// As the graph's Edit handler builds it: no created_by.
	edge.ID, edge.ModifiedBy, edge.Meta = 1, 2, "approved"
	if err := r.validateEdit(edge); err != nil {
		t.Fatalf("an edit: %v", err)
	}
	edge.ModifiedBy = 0
	if err := r.validateEdit(edge); err == nil {
		t.Error("an edit with nobody making it passed")
	}
	edge.ModifiedBy, edge.ID = 2, 0
	if err := r.validateEdit(edge); err == nil {
		t.Error("an edit of no edge passed")
	}
}
