package handlers

import (
	"anahita-libs/data/entities"
)

// restrictUnseen replaces each actor the viewer may not see with what a
// stranger is shown of it, and says whether the viewer has asked to follow
// it. The actors the viewer may see are returned as they came.
//
// For a list read through SetActorAccessOrAskableQuery, which lets both
// kinds in.
func (h *actorHandler) restrictUnseen(actors []*entities.AnahitaNode, viewer *entities.Viewer) ([]*entities.AnahitaNode, error) {
	if len(actors) == 0 {
		return actors, nil
	}

	ids := extractNodeIDs(actors)

	visible, err := h.env.Repos.Nodes.FilterVisible(ids, viewer)
	if err != nil {
		return nil, err
	}
	seen := make(map[int64]bool, len(visible))
	for _, id := range visible {
		seen[id] = true
	}

	rels, err := h.env.Repos.SocialgraphRel.GetMany(ids, viewer.ID)
	if err != nil {
		return nil, err
	}

	out := make([]*entities.AnahitaNode, 0, len(actors))
	for _, actor := range actors {
		if seen[actor.ID] {
			out = append(out, actor)
			continue
		}

		limited := restrictedActor(actor)
		if rel := rels[actor.ID]; rel != nil {
			limited.IsFollowRequestedByViewer = rel.HasRequested
		}
		out = append(out, limited)
	}

	return out, nil
}

// restrictedActor is an actor cut down to what somebody who may not see it
// is shown: whose it is, and that it may be asked.
//
// A new node with the few fields copied in, so that a field added to the
// node later is left out until somebody decides otherwise.
func restrictedActor(actor *entities.AnahitaNode) *entities.AnahitaNode {
	limited := entities.NewAnahitaNode()
	limited.ID = actor.ID
	limited.Type = actor.Type
	limited.Service = actor.Service
	limited.Name = actor.Name
	limited.Alias = actor.Alias
	limited.Verified = actor.Verified
	limited.Enabled = actor.Enabled
	limited.AvatarFilename = actor.AvatarFilename
	limited.AllowFollowRequest = actor.AllowFollowRequest
	limited.CreatedAt = actor.CreatedAt
	limited.UpdatedAt = actor.UpdatedAt
	limited.Restricted = true

	return limited
}
