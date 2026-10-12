package handlers

import (
	"actor/responses"
	"anahita-libs/requests"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

// EditDownloadDefault says what a person's new photos and videos start
// with: whether whoever can see them may download them.
//
// PATCH /people/:id/download-default
//
// It is where a new post starts, and no more. Each post has its own
// switch, which its author can set either way when posting and afterwards,
// and changing this changes nothing already posted: turning it on does not
// make everything somebody ever posted downloadable.
func (h *personHandler) EditDownloadDefault(c echo.Context) error {
	var (
		req    = &requests.ActorEditDownloadDefaultRequest{}
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

	person.AllowDownload = req.Allow
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
