package handlers

import (
	"log"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// tagLocations ties a post, as it is made, to the locations its author chose
// for it, before it is announced, so that it goes to other servers with them
// (the user, 2026-10-19; roadmap row 82, part 3b). Up to five, as a post may
// have, the first chosen first; the same one twice once; what is not a
// location, or was deleted, is passed over. A tag that cannot be made does
// not undo the post: it is logged, and can be added from the post's page.
func (h *mediumHandler) tagLocations(post *entities.AnahitaNode, locationIDs []int64, by int64) {
	seen := map[int64]bool{}
	tagged := 0
	for _, id := range locationIDs {
		if tagged >= constants.MaxLocationsOfAPost {
			return
		}
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true

		location, err := h.env.Repos.Nodes.Read(id)
		if err != nil || location == nil || location.Type != constants.NodeBaseLocationEntityType || location.DeletedAt != nil {
			log.Printf("post %d: %d is not a location to tag it with", post.ID, id)
			continue
		}

		edge := entities.NewAnahitaEdge()
		edge.Type = constants.EdgeTagLocationType
		edge.NodeAID = location.ID
		edge.NodeAType = location.Type
		edge.NodeBID = post.ID
		edge.NodeBType = post.Type
		edge.CreatedBy = by
		edge.ModifiedBy = by
		if _, err := h.env.Repos.Edges.Add(edge); err != nil {
			log.Printf("post %d: tagging it with location %d failed: %v", post.ID, id, err)
			continue
		}
		tagged++
	}
}
