package handlers

import (
	"actor/responses"
	"anahita-libs/requests"
	"encoding/json"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

// EditFeatures updates a person's features
func (h *personHandler) EditFeatures(c echo.Context) error {
	var (
		req    = requests.NewPersonRequest().EditFeatures
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

	featuresJSON, err := json.Marshal(req.Features)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	person.FeaturesRaw = featuresJSON
	// The graph service refuses an actor edit without a modifier. Taken from
	// the row it was nearly always zero — 692 of 822 people had never been
	// edited through this API — so the save answered 500 for most of them.
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
