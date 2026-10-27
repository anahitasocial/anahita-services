package handlers

import (
	"anahita-libs/data/entities"
	an_handlers "anahita-libs/handlers"
	"anahita-libs/permissions"
	"anahita-libs/requests"
	"anahita-libs/responses"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

// Publish makes a draft, an article or a topic still being written, a post:
// from now, seen by whoever its audience says, and announced as one made now
// (the graph dates it and tells of it as added). What it shows within itself
// is cited then. Its author's alone to do (the user, 2026-10-16).
func (h *mediumHandler) Publish(c echo.Context) error {
	viewer := getViewer(c)
	req := &requests.MediumPublishRequest{}

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	medium, err := h.Srv.MediaGRPC.Read(req.ID, viewer)
	if err != nil || medium == nil || medium.ID == 0 {
		return c.NoContent(http.StatusNotFound)
	}
	// Somebody else's draft is not there to them.
	if medium.Draft && medium.CreatedBy != viewer.ID {
		return c.NoContent(http.StatusNotFound)
	}
	if !medium.Draft {
		return c.NoContent(http.StatusConflict)
	}

	mediumAccessCtx, err := h.getMediumAccessContext(medium, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Medium.CanEdit(mediumAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	if req.Access != "" {
		if !permissions.IsMediumAudienceAllowed(mediumAccessCtx, req.Access) {
			log.Printf("refused: audience %q is not allowed on this profile", req.Access)
			return an_handlers.RefuseAudience(c, mediumAccessCtx)
		}
		medium.Access = an_handlers.DetermineMediumNodeAccess(req.Access, medium.Owner.Access)
	}

	medium.Draft = false
	medium.ModifiedBy = viewer.ID
	if medium, err = h.Srv.MediaGRPC.Edit(medium, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	h.syncEmbeds(medium)

	if err = h.onAfterFetched(c, medium); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	batchPresignPaths(h.Env, []*entities.AnahitaNode{medium})

	mediaResp := &responses.MediumNodeResponse{}
	return c.JSON(http.StatusOK, mediaResp.MapMediumNode(medium))
}
