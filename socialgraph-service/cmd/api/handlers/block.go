package handlers

import (
	"anahita-libs/constants"
	an_entities "anahita-libs/data/entities"
	"anahita-libs/data/services"
	an_requests "anahita-libs/requests"
	"anahita-libs/responses"
	"log"
	"net/http"
	"socialgraph/requests"

	"github.com/labstack/echo/v4"
)

type blockHandler struct {
	*Env
}

// NewBlockedHandler creates a new user handler
func NewBlockHandler(env *Env) *blockHandler {
	return &blockHandler{
		env,
	}
}

// Browse returns a list of blocked
func (h *blockHandler) Browse(c echo.Context) error {
	var (
		err      error
		req      = requests.NewSocialgraphRequest().Browse
		graphReq = services.NewGraphRequest().BrowseNeighbours
		actorReq = an_requests.NewActorRequest().BatchRead
		viewer   = getViewer(c)
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Bind(actorReq); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(actorReq); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	actor, err := h.Srv.ActorsGRPC.Read(req.ActorID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if ok, err := h.Perms.Blocks.CanBrowse(actor, viewer); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	resp := &struct {
		Data       []*responses.ActorNodeItem `json:"data"`
		Pagination responses.Pagination       `json:"pagination"`
	}{
		Data: []*responses.ActorNodeItem{},
		Pagination: responses.Pagination{
			Limit:  req.Limit,
			Offset: req.Offset,
			Total:  0,
		},
	}

	graphReq.NodeId = actor.ID
	graphReq.EdgeTypes = []string{constants.EdgeBlockEntityType}
	graphReq.Direction = constants.GraphQueryDirectionOutbound
	graphReq.Limit = constants.MaxFetchSize
	graphReq.Viewer = viewer
	graphReq.AccessCheck = true

	nodes, total, err := h.Srv.GraphGRPC.BrowseNeighbours(graphReq)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	resp.Pagination.Total = total

	onAfterActorFetched(h.Env, nodes)

	actorResp := &responses.ActorResponse{}
	resp.Data = actorResp.MapNodeItems(nodes)

	return c.JSON(http.StatusOK, resp)
}

// Add adds a block relationship to an actor
func (h *blockHandler) Add(c echo.Context) error {
	var (
		err    error
		req    = requests.NewSocialgraphRequest().Add
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

	target, err := h.Srv.ActorsGRPC.Read(req.TargetID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	source, err := h.Srv.ActorsGRPC.Read(req.SourceID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if ok, err := h.Perms.Blocks.CanAdd(source, target, viewer); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	if err = h.Srv.SocialgraphBlockGRPC.Add(source, target, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	onAfterActorFetched(h.Env, []*an_entities.AnahitaNode{target})

	actorResp := &responses.ActorResponse{}

	return c.JSON(http.StatusCreated, actorResp.MapNodeItem(target))
}

// Delete a block relationship from an actor
func (h *blockHandler) Delete(c echo.Context) error {
	var (
		err    error
		req    = requests.NewSocialgraphRequest().Delete
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

	target, err := h.Srv.ActorsGRPC.Read(req.TargetID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	source, err := h.Srv.ActorsGRPC.Read(req.SourceID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if ok, err := h.Perms.Blocks.CanDelete(source, target, viewer); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	if err := h.Srv.SocialgraphBlockGRPC.Delete(source, target, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	onAfterActorFetched(h.Env, []*an_entities.AnahitaNode{target})

	actorResp := &responses.ActorResponse{}

	return c.JSON(http.StatusOK, actorResp.MapNodeItem(target))
}
