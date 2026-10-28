package handlers

import (
	"actor/responses"
	an_handlers "anahita-libs/handlers"
	"anahita-libs/requests"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

// EditQuotePolicy says who may quote a person's posts, for the posts that do
// not say themselves: anyone, the people who follow them, or nobody.
//
// PATCH /people/:id/quote-policy
//
// A route of its own, like a profile's access. People only: a post's author
// is a person, and it is the author's setting a post falls back on.
func (h *personHandler) EditQuotePolicy(c echo.Context) error {
	var (
		req    = &requests.PersonEditQuotePolicyRequest{}
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

	quotePolicy, valid := an_handlers.RequestQuotePolicy(req.QuotePolicy)
	if !valid || quotePolicy == "" {
		return an_handlers.RefuseQuotePolicy(c)
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

	person.QuotePolicy = quotePolicy
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
