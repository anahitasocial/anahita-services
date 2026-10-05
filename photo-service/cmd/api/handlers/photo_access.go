package handlers

import (
	"anahita-libs/cmd/api/handlers"
	"anahita-libs/cmd/api/permissions"
	"anahita-libs/data/entities"
	"anahita-libs/requests"
	"anahita-libs/responses"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

// EditAccess updates the access of a photo
func (h *photoHandler) EditAccess(c echo.Context) error {
	var (
		req    = requests.NewMediumRequest().EditAccess
		viewer = getViewer(c)
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	medium, err := h.Srv.MediaGRPC.Read(req.ID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	mediumAccessCtx, err := h.getMediumAccessContext(medium, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Media.CanEdit(mediumAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	// The same rule as when a post is created, so it cannot be given by
	// editing an audience it could not have been posted with.
	if !permissions.IsMediumAudienceAllowed(mediumAccessCtx, req.Access) {
		log.Printf("refused: audience %q is not allowed on this profile", req.Access)
		return handlers.RefuseAudience(c, mediumAccessCtx)
	}

	medium.Access = handlers.DetermineMediumNodeAccess(req.Access, medium.Owner.Access)

	// Changing access is a modification, and the edit validator requires a
	// modifier. See person_access.go for what its absence did.
	medium.ModifiedBy = viewer.ID

	if medium, err = h.Srv.MediaGRPC.Edit(medium, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if err = h.onAfterFetched(c, medium); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	batchPresignPaths(h.Env, []*entities.AnahitaNode{medium})

	mediaResp := &responses.MediumNodeResponse{}
	resp := mediaResp.MapMediumNode(medium)

	return c.JSON(http.StatusOK, resp)
}

// getMediumAccessContext retrieves the MediumAccessContext for the given CommentAccessContext
func (h *photoHandler) getMediumAccessContext(medium *entities.AnahitaNode, viewer *entities.Viewer) (*permissions.MediumAccessContext, error) {
	var (
		err error
		ctx *permissions.MediumAccessContext
	)

	ctx = &permissions.MediumAccessContext{
		Viewer: viewer,
		Medium: medium,
	}

	actorIds := []int64{medium.GetOwnerID(), medium.GetAuthorID()}
	relationships, err := h.Srv.SocialgraphGRPC.Browse(actorIds, viewer.ID)
	if err != nil {
		return nil, err
	}

	ctx.OwnerViewerRel = relationships[medium.GetOwnerID()]
	ctx.AuthorViewerRel = relationships[medium.GetAuthorID()]

	return ctx, nil
}
