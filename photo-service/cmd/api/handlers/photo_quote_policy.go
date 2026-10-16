package handlers

import (
	"anahita-libs/cmd/api/handlers"
	"anahita-libs/data/entities"
	"anahita-libs/requests"
	"anahita-libs/responses"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

// EditQuotePolicy changes who may quote a post: anyone, the people who
// follow its author, or nobody. Whoever may edit the post may change it.
//
// PATCH /{articles,notes,topics,photos}/:id/quote-policy
//
// It decides who may quote from now on. A note that already quotes the post
// stays as it is: the post's author takes theirs out of it one at a time.
func (h *photoHandler) EditQuotePolicy(c echo.Context) error {
	var (
		req    = &requests.MediumEditQuotePolicyRequest{}
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

	quotePolicy, valid := handlers.RequestQuotePolicy(req.QuotePolicy)
	if !valid || quotePolicy == "" {
		return handlers.RefuseQuotePolicy(c)
	}

	medium, err := h.Srv.MediaGRPC.Read(req.ID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	// A reply has none of its own: its thread decides.
	if medium.RootID > 0 {
		return c.JSON(http.StatusUnprocessableEntity, responses.Refusal{Error: "reply_follows_its_thread"})
	}

	mediumAccessCtx, err := h.getMediumAccessContext(medium, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Media.CanEdit(mediumAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	medium.QuotePolicy = quotePolicy
	medium.ModifiedBy = viewer.ID

	if medium, err = h.Srv.MediaGRPC.Edit(medium, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if err = h.onAfterFetched(c, medium); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	batchPresignPaths(h.Env, []*entities.AnahitaNode{medium})

	mediaResp := &responses.MediumNodeResponse{}

	return c.JSON(http.StatusOK, mediaResp.MapMediumNode(medium))
}
