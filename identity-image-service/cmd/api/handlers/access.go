package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/permissions"
	"net/http"
)

// editable reads the node an image is for, as the viewer, and says whether
// they may change it.
//
// Returns the node as its own service would read it, an actor or a post, so
// that saving it afterwards goes through the same rules as any other edit of
// it. When the answer is no, the node is nil and the status says why:
//
//	404  there is no such node, or the viewer may not see it
//	403  it cannot have this kind of image, or the viewer may not edit it
func (h *imageHandler) editable(id int64, viewer *entities.Viewer, k kind) (*entities.AnahitaNode, int) {
	// What sort of node it is, before anything else: that decides who is
	// asked about it.
	node, err := h.Srv.GraphNodeGRPC.Read(id)
	if err != nil || node == nil {
		return nil, http.StatusNotFound
	}

	if !k.allowed(node) {
		return nil, http.StatusForbidden
	}

	if node.IsActor() {
		actor, err := h.Srv.ActorsGRPC.Read(id, viewer)
		if err != nil || actor == nil {
			return nil, http.StatusNotFound
		}

		rel, err := h.Srv.SocialgraphGRPC.Read(actor.ID, viewer.ID)
		if err != nil {
			return nil, http.StatusInternalServerError
		}

		if ok, _ := h.Perms.Actors.CanEdit(&permissions.ActorAccessContext{
			Viewer:         viewer,
			Actor:          actor,
			ActorViewerRel: rel,
		}); !ok {
			return nil, http.StatusForbidden
		}

		return actor, http.StatusOK
	}

	medium, err := h.Srv.MediaGRPC.Read(id, viewer)
	if err != nil || medium == nil {
		return nil, http.StatusNotFound
	}

	rels, err := h.Srv.SocialgraphGRPC.Browse(
		[]int64{medium.GetOwnerID(), medium.GetAuthorID()}, viewer.ID,
	)
	if err != nil {
		return nil, http.StatusInternalServerError
	}

	if ok, _ := h.Perms.Media.CanEdit(&permissions.MediumAccessContext{
		Viewer:          viewer,
		Medium:          medium,
		OwnerViewerRel:  rels[medium.GetOwnerID()],
		AuthorViewerRel: rels[medium.GetAuthorID()],
	}); !ok {
		return nil, http.StatusForbidden
	}

	return medium, http.StatusOK
}
