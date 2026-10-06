package handlers

import (
	"actor/constants"
	"anahita-libs/requests"
	"anahita-libs/responses"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

// EditRepostsOnProfile says whether what a group reposts is kept out of the
// posts on its profile. Changed by whoever may edit the group: its
// administrators.
//
// PATCH /groups/:id/reposts-on-profile
//
// See the same route for a person.
func (h *groupHandler) EditRepostsOnProfile(c echo.Context) error {
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

	actor.HideRepostsOnProfile = req.Hide
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
