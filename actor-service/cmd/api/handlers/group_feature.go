package handlers

import (
	"anahita-libs/requests"
	"anahita-libs/responses"
	"encoding/json"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

// EditFeatures updates a group's features
func (h *groupHandler) EditFeatures(c echo.Context) error {
	var (
		req    = requests.NewActorRequest().EditFeatures
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

	featuresJSON, err := json.Marshal(req.Features)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	actor.FeaturesRaw = featuresJSON
	// The graph service refuses an actor edit without a modifier, and taken
	// from the row it is zero for any group never edited through this API.
	actor.ModifiedBy = viewer.ID

	if actor, err = h.Srv.ActorsGRPC.Edit(actor, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	h.onAfterFetched(c, actor)

	actorResp := &responses.ActorResponse{}
	resp := actorResp.MapNode(actor)

	return c.JSON(http.StatusOK, resp)
}
