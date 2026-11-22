package handlers

import (
	"errors"
	"fmt"

	"anahita-libs/data/entities"
)

// errNoTarget is there being no such node to like, or to read the likes
// of: answered 404, as a read that fails is elsewhere, and not 500.
var errNoTarget = errors.New("no such node")

// readLikeTarget reads the node being liked together with the actor whose
// permissions decide it: the post's owner, or, for a reply, the post at the
// top of its thread and that post's owner.
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
		return nil, fmt.Errorf("%w: %v", errNoTarget, err)
	}

	owned := node
	if node.RootID > 0 {
		// A reply. What decides a like on it is the post at the top of its
		// thread, which may be several replies up, so that is what is
		// loaded in the place the permissions look: as its Parent. Here
		// "Parent" means the post whose rules apply, and not what the reply
		// directly answers.
		root, err := h.Srv.GraphNodeGRPC.Read(node.RootID)
		if err != nil {
			return nil, err
		}
		node.Parent = root
		owned = root
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

// refreshLikeTarget reads again a node just liked or unliked, for its new
// count, keeping what readLikeTarget loaded with it: the post at the top of
// a reply's thread, and the owner. Neither changes with a like, and reading
// them again was two more calls for every like (roadmap row 170).
func (h *likeHandler) refreshLikeTarget(read *entities.AnahitaNode) (*entities.AnahitaNode, error) {
	node, err := h.Srv.GraphNodeGRPC.Read(read.ID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errNoTarget, err)
	}
	if node.RootID > 0 {
		node.Parent = read.Parent
	} else {
		node.Owner = read.Owner
	}
	return node, nil
}
