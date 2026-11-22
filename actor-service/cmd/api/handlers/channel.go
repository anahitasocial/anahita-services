package handlers

import (
	an_handlers "anahita-libs/handlers"
	"log"
	"net/http"

	an_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/responses"

	"github.com/labstack/echo/v4"
)

// A channel is a site's feed as a profile (docs/plans/done/channel.md): followed,
// looked after by administrators, read and deleted as a group is, so it is
// served by the same handler told a different kind.
//
// What differs is here. It is made by syndication-service, which proves its
// domain first, so there is no Add. Its handle was chosen by whoever
// connected it and stays as it was, and its site is the domain proved for
// it: changing one is changing its name and its description only.

type channelHandler struct {
	*groupHandler
}

// NewChannelHandler serves channels.
func NewChannelHandler(env *Env) *channelHandler {
	return &channelHandler{
		groupHandler: &groupHandler{
			Env:      env,
			nodeType: an_constants.NodeActorChannelEntityType,
			service:  an_constants.ActorService,
		},
	}
}

// ChannelEditRequest is what may be changed of a channel.
type ChannelEditRequest struct {
	// The channel's id.
	ID int64 `param:"id" validate:"required,numeric"`
	// Its name.
	Name string `form:"name" json:"name" validate:"required,max=64"`
	// What it is, shown in its About card.
	Body string `form:"body" json:"body" validate:"omitempty,max=500"`
}

// Edit changes a channel's name and description.
func (h *channelHandler) Edit(c echo.Context) error {
	var (
		req    = &ChannelEditRequest{}
		viewer = getViewer(c)
	)

	if !an_handlers.Bound(c, req) {
		return c.NoContent(http.StatusBadRequest)
	}

	channel, err := h.Srv.ActorsGRPC.ReadOfType(req.ID, h.nodeType, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	accessCtx, err := h.getActorAccessContext(channel, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Actors.CanEdit(accessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	channel.Name = req.Name
	channel.Body = req.Body
	channel.ModifiedBy = viewer.ID
	if _, err = h.Srv.ActorsGRPC.Edit(channel, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	channel, err = h.Srv.ActorsGRPC.ReadOfType(req.ID, h.nodeType, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}
	if err = h.onAfterFetched(c, channel); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	batchPresignPaths(h.Env, []*entities.AnahitaNode{channel})
	batchPresignPaths(h.Env, channel.Administrators)

	return c.JSON(http.StatusOK, (&responses.ActorResponse{}).MapNode(channel))
}
