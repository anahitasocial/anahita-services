package handlers

import (
	"log"

	"anahita-libs/data/entities"
)

// RelationReader is the one question SetViewerRelations asks: how each of
// these actors stands with the viewer.
type RelationReader interface {
	Browse(actorAIds []int64, actorBId int64) (map[int64]*entities.ActorRelation, error)
}

// SetViewerRelations says, on each actor in a list, whether the viewer
// follows it and whether it follows the viewer.
//
// A list of people with a Follow button beside each needs both: without the
// first the button says "Follow" to somebody already followed, and without
// the second it cannot say "Follow back". Lists that came with neither were
// search results and the people who liked or reposted a post.
//
// Nodes that are not actors are left alone. For a visitor nothing is asked.
// A failure to read is logged and the list goes out without the marks: the
// buttons are then merely less exact, which is better than no list.
func SetViewerRelations(relations RelationReader, nodes []*entities.AnahitaNode, viewer *entities.Viewer) {
	if relations == nil || viewer == nil || viewer.ID <= 0 {
		return
	}

	ids := make([]int64, 0, len(nodes))
	for _, node := range nodes {
		if node != nil && node.IsActor() {
			ids = append(ids, node.ID)
		}
	}
	if len(ids) == 0 {
		return
	}

	rels, err := relations.Browse(ids, viewer.ID)
	if err != nil {
		log.Println("viewer relations:", err)
		return
	}

	for _, node := range nodes {
		if node == nil || !node.IsActor() {
			continue
		}
		if rel, ok := rels[node.ID]; ok && rel != nil {
			// IsLeading: the viewer follows the actor. IsFollowing: the
			// actor follows the viewer.
			node.IsLeadingViewer = rel.IsLeading
			node.IsFollowingViewer = rel.IsFollowing
		}
	}
}
