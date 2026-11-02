package handlers

import (
	"testing"
	"time"

	"anahita-libs/data/entities"
)

func citedAt(id int64, minutes int) *entities.AnahitaNode {
	node := &entities.AnahitaNode{}
	node.ID = id
	node.CreatedAt = time.Date(2026, 11, 2, 0, minutes, 0, 0, time.UTC)
	return node
}

func idsOf(nodes []*entities.AnahitaNode) []int64 {
	ids := []int64{}
	for _, node := range nodes {
		ids = append(ids, node.ID)
	}
	return ids
}

func TestCitedByListsCitationsAndQuotesNewestFirst(t *testing.T) {
	// Articles citing a post, and notes quoting it, each newest first.
	citing := []*entities.AnahitaNode{citedAt(1, 50), citedAt(2, 30), citedAt(3, 10)}
	quoting := []*entities.AnahitaNode{citedAt(11, 40), citedAt(12, 20)}

	cases := []struct {
		name          string
		offset, limit uint64
		want          []int64
	}{
		{"the first page", 0, 3, []int64{1, 11, 2}},
		{"the next takes up after it", 3, 3, []int64{12, 3}},
		{"past the end there is nothing", 5, 3, []int64{}},
		{"all of them", 0, 10, []int64{1, 11, 2, 12, 3}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := idsOf(newestFirst(citing, quoting, c.offset, c.limit))
			if len(got) != len(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("got %v, want %v", got, c.want)
				}
			}
		})
	}
}

func TestCitedByListsAPostThatBothCitesAndQuotesOnce(t *testing.T) {
	got := idsOf(newestFirst(
		[]*entities.AnahitaNode{citedAt(1, 50)},
		[]*entities.AnahitaNode{citedAt(1, 50), citedAt(2, 10)},
		0, 10,
	))
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Errorf("got %v, want [1 2]", got)
	}
}
