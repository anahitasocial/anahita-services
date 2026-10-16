package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/responses"
	"log"
	"net/http"
	"notification/requests"

	"github.com/labstack/echo/v4"
)

type subscriptionHandler struct {
	*Env
}

// NewSubscriptionHandler creates a new subscription handler
func NewSubscriptionHandler(env *Env) *subscriptionHandler {
	return &subscriptionHandler{
		env,
	}
}

// Add adds a new subscription
func (h *subscriptionHandler) Add(c echo.Context) error {
	var (
		err    error
		req    = requests.NewSubscriptionRequest().Add
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

	target, err := h.Srv.GraphNodeGRPC.Read(req.TargetID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	source, err := h.Srv.ActorsGRPC.Read(viewer.ID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	// Check if the viewer has permission to subscribe to the target
	if target.IsActor() {
		actor, err := h.Srv.ActorsGRPC.Read(target.ID, viewer)
		if err != nil {
			log.Println(err)
			return c.NoContent(http.StatusNotFound)
		}

		actorAccessCtx, err := h.getActorAccessContext(actor, viewer)
		if err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
		if ok, err := h.Perms.Actors.CanRead(actorAccessCtx); !ok {
			log.Println(err)
			return c.NoContent(http.StatusForbidden)
		}
	} else if target.IsMedium() {
		medium, err := h.Srv.MediaGRPC.Read(target.ID, viewer)
		if err != nil {
			log.Println(err)
			return c.NoContent(http.StatusNotFound)
		}

		mediumAccessCtx, err := h.getMediumAccessContext(medium, viewer)
		if err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
		if ok, err := h.Perms.Media.CanRead(mediumAccessCtx); !ok {
			log.Println(err)
			return c.NoContent(http.StatusForbidden)
		}
	} else {
		return c.NoContent(http.StatusForbidden)
	}

	if err := h.Srv.SubscriptionsGRPC.Add(source, target, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	target, err = h.Srv.GraphNodeGRPC.Read(req.TargetID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	target.IsSubscribedByViewer = true

	nodeResp := &responses.NodeResponse{}

	return c.JSON(http.StatusCreated, nodeResp.MapNodeItem(target))
}

// Delete deletes a subscription
func (h *subscriptionHandler) Delete(c echo.Context) error {
	var (
		err    error
		req    = requests.NewSubscriptionRequest().Delete
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

	target, err := h.Srv.GraphNodeGRPC.Read(req.TargetID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	source, err := h.Srv.ActorsGRPC.Read(viewer.ID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	// Check if the viewer has permission to read the target
	if target.IsActor() {
		actor, err := h.Srv.ActorsGRPC.Read(target.ID, viewer)
		if err != nil {
			log.Println(err)
			return c.NoContent(http.StatusNotFound)
		}

		actorAccessCtx, err := h.getActorAccessContext(actor, viewer)
		if err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
		if ok, err := h.Perms.Actors.CanRead(actorAccessCtx); !ok {
			log.Println(err)
			return c.NoContent(http.StatusForbidden)
		}
	} else if target.IsMedium() {
		medium, err := h.Srv.MediaGRPC.Read(target.ID, viewer)
		if err != nil {
			log.Println(err)
			return c.NoContent(http.StatusNotFound)
		}

		mediumAccessCtx, err := h.getMediumAccessContext(medium, viewer)
		if err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
		if ok, err := h.Perms.Media.CanRead(mediumAccessCtx); !ok {
			log.Println(err)
			return c.NoContent(http.StatusForbidden)
		}
	} else {
		return c.NoContent(http.StatusForbidden)
	}

	if err := h.Srv.SubscriptionsGRPC.Delete(source, target); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	target, err = h.Srv.GraphNodeGRPC.Read(req.TargetID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	target.IsSubscribedByViewer = false

	if target.IsMedium() {
		if err := h.onAfterFetchedMedium(c, target); err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	nodeResp := &responses.NodeResponse{}

	return c.JSON(http.StatusOK, nodeResp.MapNodeItem(target))
}

// onAfterFetched is called after fetching a note
func (h *subscriptionHandler) onAfterFetchedMedium(c echo.Context, item *entities.AnahitaNode) error {
	return nil
}
