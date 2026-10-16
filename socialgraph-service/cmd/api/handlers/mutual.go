package handlers

import (
	"anahita-libs/constants"
	"anahita-libs/data/services"
	"anahita-libs/responses"
	"log"
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

// Browse returns who the viewer has in common with an actor: the people
// the viewer follows who follow that actor.
//
// GET /socialgraph/:actor_id/mutuals/
//
// One query in the graph, so the list is whole, paged and counted. It used
// to compare a page of the actor's followers with a page of who the viewer
// follows, and so found nobody past the first page of either.
func (h *mutualHandler) Browse(c echo.Context) error {
	var (
		req      = requests.NewSocialgraphRequest().Browse
		graphReq = services.NewGraphRequest().BrowseConnectionNodes
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

	resp := &responses.List[*responses.ActorNodeItem]{
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
	graphReq.Search = req.Search
	// Among the actor's followers, the ones the viewer follows.
	graphReq.FollowedBy = viewer.ID
	// By when each followed: the latest first, or the oldest when asked
	// (?dir=asc).
	graphReq.Ordering = constants.OrderingConnected
	graphReq.OrderingDir = req.Direction
	graphReq.Viewer = viewer
	graphReq.AccessCheck = true

	common, total, err := h.Srv.GraphGRPC.BrowseConnectionNodes(graphReq)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	resp.Pagination.Total = total

	if len(common) == 0 {
		return c.JSON(http.StatusOK, resp)
	}

	relationships, err := h.Srv.SocialgraphGRPC.Browse(extractNodeIDs(common), viewer.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	for _, node := range common {
		if rel, exists := relationships[node.ID]; exists {
			node.IsLeadingViewer = rel.IsLeading
			node.IsFollowingViewer = rel.IsFollowing
		}
	}

	onAfterActorFetched(h.Env, common)

	actorResp := &responses.ActorResponse{}
	resp.Data = actorResp.MapNodeItems(common)

	return c.JSON(http.StatusOK, resp)
}
