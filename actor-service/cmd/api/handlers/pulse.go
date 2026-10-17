package handlers

import (
	"log"
	"net/http"

	an_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/responses"

	"github.com/labstack/echo/v4"
)

// A Pulse is a website's feed as a profile (docs/plans/pulse.md): followed,
// looked after by administrators, read and deleted as a group is, so it is
// served by the same handler told a different kind.
//
// What differs is here. It is made by pulse-service, which proves its
// domain first, so there is no Add. Its handle was chosen by whoever
// connected it and stays as it was, and its site is the domain proved for
// it: changing one is changing its name and its description only.

type pulseHandler struct {
	*groupHandler
}

// NewPulseHandler serves Pulses.
func NewPulseHandler(env *Env) *pulseHandler {
	return &pulseHandler{
		groupHandler: &groupHandler{
			Env:      env,
			nodeType: an_constants.NodeActorPulseEntityType,
			service:  an_constants.ActorService,
		},
	}
}

// PulseEditRequest is what may be changed of a Pulse.
type PulseEditRequest struct {
	// The Pulse's id.
	ID int64 `param:"id" validate:"required,numeric"`
	// Its name.
	Name string `form:"name" json:"name" validate:"required,max=64"`
	// What it is, shown in its About card.
	Body string `form:"body" json:"body" validate:"omitempty,max=500"`
}

// Edit changes a Pulse's name and description.
func (h *pulseHandler) Edit(c echo.Context) error {
	var (
		req    = &PulseEditRequest{}
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

	pulse, err := h.Srv.ActorsGRPC.ReadOfType(req.ID, h.nodeType, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	accessCtx, err := h.getActorAccessContext(pulse, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Actors.CanEdit(accessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	pulse.Name = req.Name
	pulse.Body = req.Body
	pulse.ModifiedBy = viewer.ID
	if _, err = h.Srv.ActorsGRPC.Edit(pulse, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	pulse, err = h.Srv.ActorsGRPC.ReadOfType(req.ID, h.nodeType, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}
	if err = h.onAfterFetched(c, pulse); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	batchPresignPaths(h.Env, []*entities.AnahitaNode{pulse})
	batchPresignPaths(h.Env, pulse.Administrators)

	return c.JSON(http.StatusOK, (&responses.ActorResponse{}).MapNode(pulse))
}
