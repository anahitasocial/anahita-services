package handlers

import (
	"actor/responses"
	"anahita-libs/requests"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

// EditRepostsOnProfile says whether what a person reposts is kept out of the
// posts on their profile.
//
// PATCH /people/:id/reposts-on-profile
//
// A route of its own, as a profile's access has: a form that edits something
// else and does not send this must not change it. The choice is about the
// profile and nothing else. Followers are still sent the reposts in their
// own feeds, and the profile's list of reposts still has them.
func (h *personHandler) EditRepostsOnProfile(c echo.Context) error {
	var (
		req    = &requests.ActorEditRepostsOnProfileRequest{}
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

	person.HideRepostsOnProfile = req.Hide
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

	personResp := &responses.PersonResponse{}

	return c.JSON(http.StatusOK, personResp.MapNode(person))
}
