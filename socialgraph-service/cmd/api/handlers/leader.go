package handlers

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	an_requests "anahita-libs/requests"
	"anahita-libs/responses"
	"log"
	"net/http"
	"socialgraph/requests"

	"github.com/labstack/echo/v4"
)

type leaderHandler struct {
	*Env
}

// NewLeaderHandler creates a new user handler
func NewLeaderHandler(env *Env) *leaderHandler {
	return &leaderHandler{
		env,
	}
}

// Browse returns a list of leader
func (h *leaderHandler) Browse(c echo.Context) error {
	var (
		err      error
		req      = requests.NewSocialgraphRequest().Browse
		graphReq = services.NewGraphRequest().BrowseConnectionNodes
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

	if ok, err := h.Perms.Leaders.CanBrowse(actor, viewer); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	resp := &struct {
		Data       []*responses.ActorNodeItem `json:"data"`
		Pagination responses.Pagination       `json:"pagination"`
	}{
		Pagination: responses.Pagination{
			Limit:  req.Limit,
			Offset: req.Offset,
			Total:  0,
		},
	}

	graphReq.NodeId = actor.ID
	graphReq.EdgeType = constants.EdgeFollowEntityType
	graphReq.Direction = constants.GraphQueryDirectionOutbound
	graphReq.Offset = req.Offset
	graphReq.Limit = req.Limit
	graphReq.Viewer = viewer
	graphReq.AccessCheck = true

	leaders, total, err := h.Srv.GraphGRPC.BrowseConnectionNodes(graphReq)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	resp.Pagination.Total = total

	actorIds := extractNodeIDs(leaders)
	relationships, err := h.Srv.SocialgraphGRPC.Browse(actorIds, viewer.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	relationshipMap := make(map[int64]*entities.ActorRelation)
	for id, rel := range relationships {
		relationshipMap[id] = rel
	}

	for _, leader := range leaders {
		if rel, exists := relationshipMap[leader.ID]; exists {
			leader.IsLeadingViewer = rel.IsLeading
		}
	}

	onAfterActorFetched(h.Env, leaders)

	actorResp := &responses.ActorResponse{}
	resp.Data = actorResp.MapNodeItems(leaders)

	return c.JSON(http.StatusOK, resp)
}

// Add adds a leader to an actor
func (h *leaderHandler) Add(c echo.Context) error {
	return c.NoContent(http.StatusNotImplemented)
}

// Delete a leader from an actor
func (h *leaderHandler) Delete(c echo.Context) error {
	return c.NoContent(http.StatusNotImplemented)
}
