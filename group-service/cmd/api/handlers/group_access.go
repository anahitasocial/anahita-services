package handlers

import (
	an_handlers "anahita-libs/cmd/api/handlers"
	"anahita-libs/cmd/api/permissions"
	"anahita-libs/data/entities"
	"anahita-libs/requests"
	"anahita-libs/responses"
	"group/constants"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

// Edit updates the access of a group
func (h *groupHandler) EditAccess(c echo.Context) error {
	var (
		req    = requests.NewActorRequest().EditAccess
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

	actor, err := h.Srv.ActorsGRPC.ReadOfType(req.ID, constants.NodeActorGroupType, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	actorAccessCtx, err := h.getActorAccessContext(actor, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Actors.CanEdit(actorAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	actor.AllowFollowRequest = req.AllowFollowRequest
	actor.Access = an_handlers.DetermineActorNodeAccess(req.Access)
	if actor, err = h.Srv.ActorsGRPC.Edit(actor, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if err = h.onAfterFetched(c, actor); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	actorResp := &responses.ActorResponse{}
	resp := actorResp.MapNode(actor)

	return c.JSON(http.StatusOK, resp)
}

// getActorAccessContext retrieves the ActorAccessContext for the given Viewer
func (h *groupHandler) getActorAccessContext(actor *entities.AnahitaNode, viewer *entities.Viewer) (*permissions.ActorAccessContext, error) {
	var (
		err error
		ctx *permissions.ActorAccessContext
	)

	ctx = &permissions.ActorAccessContext{
		Viewer: viewer,
		Actor:  actor,
	}

	ctx.ActorViewerRel, err = h.Srv.SocialgraphGRPC.Read(actor.ID, viewer.ID)
	if err != nil {
		return nil, err
	}

	return ctx, nil
}
