package handlers

import (
	"actor/responses"
	"anahita-libs/requests"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

// EditRepostsOnProfile says whether what a person reposts is also shown among the
// posts on their profile.
//
// PATCH /people/:id/reposts-on-profile
//
// People only. A repost is made by a person, as a reply is; a group or any
// other kind of actor has none to show.
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

	person.ShowRepostsOnProfile = req.Show
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

	personResp := responses.NewPersonResponse(viewer)

	return c.JSON(http.StatusOK, personResp.MapNode(person))
}
