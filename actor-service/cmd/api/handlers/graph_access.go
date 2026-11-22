package handlers

import (
	"actor/responses"
	an_constants "anahita-libs/constants"
	an_handlers "anahita-libs/handlers"
	"anahita-libs/requests"
	libs_responses "anahita-libs/responses"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

// Who may see a profile's followers and whom it follows (roadmap row 151):
// one of the access levels a profile has, or empty for whoever may see the
// profile, which is what every profile starts with. A route of its own, as
// a profile's access has, so that a form editing something else cannot
// change it. It never widens: the lists are refused to whoever may not see
// the profile, whatever this says (permissions.CanSeeFollows).

// EditGraphAccess sets it for a person: PATCH /people/:id/graph-access.
func (h *personHandler) EditGraphAccess(c echo.Context) error {
	var (
		req    = &requests.ActorEditGraphAccessRequest{}
		viewer = getViewer(c)
	)
	if !an_handlers.Bound(c, req) {
		return c.NoContent(http.StatusBadRequest)
	}

	person, err := h.fetchPerson(c, req.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}
	ctx, err := h.getActorAccessContext(person, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.People.CanEdit(ctx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	person.GraphAccess = req.GraphAccess
	person.ModifiedBy = viewer.ID
	if _, err = h.Srv.ActorsGRPC.Edit(&person.AnahitaNode, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	person, err = h.fetchPerson(c, req.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}
	return c.JSON(http.StatusOK, responses.NewPersonResponse(viewer).MapNode(person))
}

// EditGraphAccess sets it for a group or an event, by its admins:
// PATCH /groups/:id/graph-access and /events/:id/graph-access. A group and
// an event follow nobody, so people it follows and mutuals are not levels
// they have.
func (h *groupHandler) EditGraphAccess(c echo.Context) error {
	var (
		req    = &requests.ActorEditGraphAccessRequest{}
		viewer = getViewer(c)
	)
	if !an_handlers.Bound(c, req) {
		return c.NoContent(http.StatusBadRequest)
	}
	if req.GraphAccess == an_constants.AnahitaAccessLeaders || req.GraphAccess == an_constants.AnahitaAccessMutuals {
		return c.JSON(http.StatusBadRequest, libs_responses.Refusal{Error: "graph_access_invalid"})
	}

	actor, err := h.Srv.ActorsGRPC.ReadOfType(req.ID, h.nodeType, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}
	ctx, err := h.getActorAccessContext(actor, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Actors.CanEdit(ctx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	actor.GraphAccess = req.GraphAccess
	actor.ModifiedBy = viewer.ID
	if actor, err = h.Srv.ActorsGRPC.Edit(actor, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if err = h.onAfterFetched(c, actor); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.JSON(http.StatusOK, (&libs_responses.ActorResponse{}).MapNode(actor))
}
