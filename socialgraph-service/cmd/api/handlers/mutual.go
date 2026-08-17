package handlers

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/responses"
	"log"
	"maps"
	"net/http"
	"socialgraph/requests"

	"github.com/labstack/echo/v4"
)

type mutualHandler struct {
	*Env
}

// NewMutualHandler creates a new user handler
func NewMutualHandler(env *Env) *mutualHandler {
	return &mutualHandler{
		env,
	}
}

// Browse returns a list of mutual
func (h *mutualHandler) Browse(c echo.Context) error {
	var (
		err      error
		req      = requests.NewSocialgraphRequest().Browse
		graphReq = services.NewGraphRequest().BrowseConnectionNodes
		actorReq = services.NewActorGRPCRequest().BatchRead
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

	actor, err := h.Srv.ActorsGRPC.Read(req.ActorID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if ok, err := h.Perms.Mutuals.CanBrowse(actor, viewer); !ok {
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
	graphReq.EdgeType = constants.EdgeFollowEntityType
	graphReq.Direction = constants.GraphQueryDirectionInbound
	graphReq.Offset = req.Offset
	graphReq.Limit = req.Limit
	graphReq.Viewer = viewer
	graphReq.AccessCheck = true
	if err := c.Validate(graphReq); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	followers, _, err := h.Srv.GraphGRPC.BrowseConnectionNodes(graphReq)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	graphReq.NodeId = viewer.ID
	graphReq.EdgeType = constants.EdgeFollowEntityType
	graphReq.Direction = constants.GraphQueryDirectionOutbound
	graphReq.Offset = req.Offset
	graphReq.Limit = req.Limit
	graphReq.Viewer = viewer
	if err := c.Validate(graphReq); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	leaders, _, err := h.Srv.GraphGRPC.BrowseConnectionNodes(graphReq)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Find mutual IDs
	actorIdSet := make(map[int64]struct{})
	for _, follower := range followers {
		actorIdSet[follower.ID] = struct{}{}
	}

	var actorIds []int64
	for _, leader := range leaders {
		if _, exists := actorIdSet[leader.ID]; exists {
			actorIds = append(actorIds, leader.ID)
		}
	}

	if len(actorIds) == 0 {
		return c.JSON(http.StatusOK, resp)
	}

	count := len(actorIds)
	resp.Pagination.Total = uint64(count)

	actorReq.IDs = actorIds
	actorReq.Ordering = req.Ordering
	actorReq.Direction = req.Direction
	if err := c.Validate(actorReq); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	nodes, err := h.Srv.ActorsGRPC.BatchRead(actorReq, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	mutualRelationships, err := h.Srv.SocialgraphGRPC.Browse(actorIds, viewer.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	relationshipsMap := make(map[int64]*entities.ActorRelation)
	maps.Copy(relationshipsMap, mutualRelationships)

	for _, mutual := range nodes {
		if rel, exists := relationshipsMap[mutual.ID]; exists {
			mutual.IsLeadingViewer = rel.IsLeading
		}
	}

	onAfterActorFetched(h.Env, nodes)

	actorResp := &responses.ActorResponse{}
	resp.Data = actorResp.MapNodeItems(nodes)

	return c.JSON(http.StatusOK, resp)
}

// Add adds a mutual to an actor
func (h *mutualHandler) Add(c echo.Context) error {
	return c.NoContent(http.StatusNotImplemented)
}

// Delete a mutual from an actor
func (h *mutualHandler) Delete(c echo.Context) error {
	return c.NoContent(http.StatusNotImplemented)
}
