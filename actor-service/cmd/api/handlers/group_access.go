package handlers

import (
	an_handlers "anahita-libs/cmd/api/handlers"
	"anahita-libs/cmd/api/permissions"
	an_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/policy"
	"anahita-libs/requests"
	"anahita-libs/responses"
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

	actor, err := h.Srv.ActorsGRPC.ReadOfType(req.ID, h.nodeType, viewer)
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

	// An installation that allows nothing new to be public does not let a
	// profile be made public either. One that already is may stay so.
	if !policy.MayChangeAccess(actor.Access, req.Access) {
		log.Printf("refused: this installation does not allow a profile to be made %q", req.Access)
		return an_handlers.RefuseAccess(c, policy.Allowed([]string{
			an_constants.AnahitaAccessPublic, an_constants.AnahitaAccessRegistered,
			an_constants.AnahitaAccessFollowers, an_constants.AnahitaAccessAdmins,
		}))
	}

	actor.AllowFollowRequest = req.AllowFollowRequest
	actor.Access = an_handlers.DetermineActorNodeAccess(req.Access)

	// Changing access is a modification, and the edit validator requires a
	// modifier. See person_access.go for what its absence did.
	actor.ModifiedBy = viewer.ID
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

		// Set here rather than at the one call site that needs it, so no
		// future caller of CanAdd can forget: left empty the level ranks
		// above every person type and nobody may create anything. Resolved
		// through the config getter, so the default is already applied.
		CreateLevel: h.Config.GetGroupsFrom(),
	}

	ctx.ActorViewerRel, err = h.Srv.SocialgraphGRPC.Read(actor.ID, viewer.ID)
	if err != nil {
		return nil, err
	}

	return ctx, nil
}
