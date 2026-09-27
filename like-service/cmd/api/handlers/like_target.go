package handlers

import (
	"anahita-libs/data/entities"
)

// readLikeTarget reads the node being liked together with the actor whose
// permissions decide it: the medium's owner, or — for a comment — the post it
// sits under and that post's owner.
//
// GraphNodeGRPC.Read returns the node's own columns only. Owner and Parent
// come back as the empty placeholders NewAnahitaNode makes, so every like
// permission judged an owner with no id, no access level and no
// add_permissions, and refused everybody but site administrators — followers
// included.
//
// The owner comes from ActorsGRPC.Read because that is where the graph service
// merges the actor_features defaults in. The raw column is NULL for most
// actors, and an owner read any other way would have no like permission at
// all.
func (h *likeHandler) readLikeTarget(id int64, viewer *entities.Viewer) (*entities.AnahitaNode, error) {
	node, err := h.Srv.GraphNodeGRPC.Read(id)
	if err != nil {
		return nil, err
	}

	owned := node
	if node.IsComment() {
		parent, err := h.Srv.GraphNodeGRPC.Read(node.ParentID)
		if err != nil {
			return nil, err
		}
		node.Parent = parent
		owned = parent
	}

	// Nothing to load for a node without an owner. The permissions refuse it
	// on their own, and failing the read here would turn that 403 into a 500.
	if owned.OwnerID <= 0 {
		return node, nil
	}

	owned.Owner, err = h.Srv.ActorsGRPC.Read(owned.OwnerID, viewer)
	if err != nil {
		return nil, err
	}

	return node, nil
}
