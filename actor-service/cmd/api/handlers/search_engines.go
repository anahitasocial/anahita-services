package handlers

import (
	"actor/responses"
	an_handlers "anahita-libs/handlers"
	"anahita-libs/requests"
	libs_responses "anahita-libs/responses"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

// Whether search engines may index a profile and its posts (roadmap row
// 174). Off for a person who signs up, who is asked first; on for every
// profile made before, and for a new group, event or channel, which are
// made to be found. web-shell writes `noindex` on the pages of a profile
// that says no, and of its posts. A route of its own, as a profile's access
// has, so that a form editing something else cannot change it. It hides
// nothing: `noindex` asks search engines to leave the pages out of what
// they index, and who may see them is the profile's privacy, unchanged.

// EditSearchEngines sets it for a person: PATCH /people/:id/search-engines.
func (h *personHandler) EditSearchEngines(c echo.Context) error {
	var (
		req    = &requests.ActorEditSearchEnginesRequest{}
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

	person.SearchEngines = req.SearchEngines
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

// EditSearchEngines sets it for a group, an event or a channel, by its
// admins: PATCH /groups/:id/search-engines, and the same under /events and
// /channels.
func (h *groupHandler) EditSearchEngines(c echo.Context) error {
	var (
		req    = &requests.ActorEditSearchEnginesRequest{}
		viewer = getViewer(c)
	)
	if !an_handlers.Bound(c, req) {
		return c.NoContent(http.StatusBadRequest)
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

	actor.SearchEngines = req.SearchEngines
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
