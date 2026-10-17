package handlers

import (
	"log"
	"net/http"

	an_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/responses"

	"github.com/labstack/echo/v4"
)

// A website is a site's feed as a profile (docs/plans/website.md): followed,
// looked after by administrators, read and deleted as a group is, so it is
// served by the same handler told a different kind.
//
// What differs is here. It is made by syndication-service, which proves its
// domain first, so there is no Add. Its handle was chosen by whoever
// connected it and stays as it was, and its site is the domain proved for
// it: changing one is changing its name and its description only.

type websiteHandler struct {
	*groupHandler
}

// NewWebsiteHandler serves websites.
func NewWebsiteHandler(env *Env) *websiteHandler {
	return &websiteHandler{
		groupHandler: &groupHandler{
			Env:      env,
			nodeType: an_constants.NodeActorWebsiteEntityType,
			service:  an_constants.ActorService,
		},
	}
}

// WebsiteEditRequest is what may be changed of a website.
type WebsiteEditRequest struct {
	// The website's id.
	ID int64 `param:"id" validate:"required,numeric"`
	// Its name.
	Name string `form:"name" json:"name" validate:"required,max=64"`
	// What it is, shown in its About card.
	Body string `form:"body" json:"body" validate:"omitempty,max=500"`
}

// Edit changes a website's name and description.
func (h *websiteHandler) Edit(c echo.Context) error {
	var (
		req    = &WebsiteEditRequest{}
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

	website, err := h.Srv.ActorsGRPC.ReadOfType(req.ID, h.nodeType, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	accessCtx, err := h.getActorAccessContext(website, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Actors.CanEdit(accessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	website.Name = req.Name
	website.Body = req.Body
	website.ModifiedBy = viewer.ID
	if _, err = h.Srv.ActorsGRPC.Edit(website, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	website, err = h.Srv.ActorsGRPC.ReadOfType(req.ID, h.nodeType, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}
	if err = h.onAfterFetched(c, website); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	batchPresignPaths(h.Env, []*entities.AnahitaNode{website})
	batchPresignPaths(h.Env, website.Administrators)

	return c.JSON(http.StatusOK, (&responses.ActorResponse{}).MapNode(website))
}
