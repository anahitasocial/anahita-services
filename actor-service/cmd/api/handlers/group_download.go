package handlers

import (
	"anahita-libs/requests"
	"anahita-libs/responses"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

// EditDownloadDefault says what new photos and videos posted on a group
// start with: whether whoever can see them may download them.
//
// PATCH /groups/:id/download-default
//
// A group that shares what is posted in it says so once. As for a person,
// it is where a new post starts: each post keeps its own switch, and
// nothing already posted changes.
func (h *groupHandler) EditDownloadDefault(c echo.Context) error {
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

	actor.AllowDownload = req.Allow
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

	return c.JSON(http.StatusOK, actorResp.MapNode(actor))
}
