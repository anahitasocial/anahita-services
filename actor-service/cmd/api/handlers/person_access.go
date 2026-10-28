package handlers

import (
	"actor/responses"
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	an_handlers "anahita-libs/handlers"
	"anahita-libs/permissions"
	"anahita-libs/policy"
	"anahita-libs/requests"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

// EditAccess updates a person's access
func (h *personHandler) EditAccess(c echo.Context) error {
	var (
		req    = requests.NewPersonRequest().EditAccess
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

	person, err := h.fetchPerson(c, req.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	actorAccessCtx, err := h.getActorAccessContext(person, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.People.CanEdit(actorAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	// An installation that allows nothing new to be public does not let a
	// profile be made public either. One that already is may stay so.
	if !policy.MayChangeAccess(person.Access, req.Access) {
		log.Printf("refused: this installation does not allow a profile to be made %q", req.Access)
		return an_handlers.RefuseAccess(c, policy.Allowed([]string{
			constants.AnahitaAccessPublic, constants.AnahitaAccessRegistered, constants.AnahitaAccessFollowers,
			constants.AnahitaAccessLeaders, constants.AnahitaAccessMutuals, constants.AnahitaAccessMyself,
		}))
	}

	person.AllowFollowRequest = req.AllowFollowRequest
	person.Access = an_handlers.DeterminePersonNodeAccess(req.Access)

	// Record who changed it. Changing access IS a modification, and the edit
	// validator requires a modifier — so without this the whole edit was
	// rejected for any node that had never been edited before.
	//
	// 344 media had modified_by NULL, which scans as 0, so their access could
	// not be changed at all: the request came back a 500 with "invalid
	// modifier ID" and nothing in front of the person to explain it. The actor
	// handlers only escaped it because most profiles had been edited at least
	// once already.
	person.ModifiedBy = viewer.ID

	_, err = h.Srv.ActorsGRPC.Edit(&person.AnahitaNode, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	person, err = h.fetchPerson(c, req.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	personResp := responses.NewPersonResponse(viewer)
	resp := personResp.MapNode(person)

	return c.JSON(http.StatusOK, resp)
}

// getActorAccessContext retrieves the ActorAccessContext for the given Viewer
func (h *personHandler) getActorAccessContext(person *entities.Person, viewer *entities.Viewer) (*permissions.ActorAccessContext, error) {
	var (
		err error
		ctx *permissions.ActorAccessContext
	)

	ctx = &permissions.ActorAccessContext{
		Viewer: viewer,
		Actor:  person,
	}

	ctx.ActorViewerRel, err = h.Srv.SocialgraphGRPC.Read(person.ID, viewer.ID)
	if err != nil {
		return nil, err
	}

	return ctx, nil
}
