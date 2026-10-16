package handlers

import (
	"actor/constants"
	an_constants "anahita-libs/constants"
	"anahita-libs/data/services"
	an_services "anahita-libs/data/services"
	"anahita-libs/requests"
	"anahita-libs/responses"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

// adminHandler serves the administrators of one kind of actor: a group's,
// or an event's.
type adminHandler struct {
	*Env
	nodeType string
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(env *Env) *adminHandler {
	return &adminHandler{Env: env, nodeType: constants.NodeActorGroupType}
}

// NewEventAdminHandler serves the administrators of events.
func NewEventAdminHandler(env *Env) *adminHandler {
	return &adminHandler{Env: env, nodeType: an_constants.NodeActorEventEntityType}
}

// Browse returns a list of admins
func (h *adminHandler) Browse(c echo.Context) error {
	var (
		err    error
		req    = requests.NewActorAdminsRequest().Browse
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

	actor, err := h.Srv.ActorsGRPC.ReadOfType(req.ActorID, h.nodeType, viewer)
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

	resp := &responses.Unpaged[*responses.ActorNodeItem]{}

	admins, err := h.Srv.ActorAdminsGRPC.Browse(actor.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	batchPresignPaths(h.Env, admins)

	actorResp := &responses.ActorResponse{}
	resp.Data = actorResp.MapNodeItems(admins)

	return c.JSON(http.StatusOK, resp)
}

// Add adds a new admin
func (h *adminHandler) Add(c echo.Context) error {
	var (
		err    error
		req    = requests.NewActorAdminsRequest().Add
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

	actor, err := h.Srv.ActorsGRPC.ReadOfType(req.ActorID, h.nodeType, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	actorAccessCtx, err := h.getActorAccessContext(actor, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Actors.CanAdministrate(actorAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	admin, err := h.Srv.ActorsGRPC.ReadOfType(req.AdminID, an_constants.NodeActorPersonEntityType, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if ok, err := h.Perms.Actors.CanAddAdmin(actorAccessCtx, admin); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	admin, err = h.Srv.ActorAdminsGRPC.Add(actor, admin, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	adminVsActorRel, err := h.Srv.SocialgraphGRPC.Read(admin.ID, actor.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// add the admin as a follower to the group
	if !adminVsActorRel.IsFollowing {
		// Add the admin as a follower to the group
		followReq := services.NewSocialgraphFollowRequest().Add
		followReq.Source = actor
		followReq.Target = admin
		followReq.Viewer = viewer
		if err := h.Srv.SocialgraphFollowGRPC.Add(followReq); err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	an_services.PresignAvatar(h.Srv.FilesGRPC, admin)

	// The same shape Browse answers with. Serialising the entity instead put
	// the node's raw meta column — session_data among it — on the wire, and
	// gave the client a different admin record than the list it belongs to.
	actorResp := &responses.ActorResponse{}

	return c.JSON(http.StatusCreated, actorResp.MapNodeItem(admin))
}

// Delete removes an admin
func (h *adminHandler) Delete(c echo.Context) error {
	var (
		err    error
		req    = requests.NewActorAdminsRequest().Delete
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

	actor, err := h.Srv.ActorsGRPC.ReadOfType(req.ActorID, h.nodeType, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	actorAccessCtx, err := h.getActorAccessContext(actor, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Actors.CanAdministrate(actorAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	admin, err := h.Srv.ActorsGRPC.ReadOfType(req.AdminID, an_constants.NodeActorPersonEntityType, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if ok, err := h.Perms.Actors.CanDeleteAdmin(actorAccessCtx, admin); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	_, err = h.Srv.ActorAdminsGRPC.Delete(actor, admin, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusNoContent)
}
