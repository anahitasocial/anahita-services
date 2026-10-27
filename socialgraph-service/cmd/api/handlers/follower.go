package handlers

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	an_handlers "anahita-libs/handlers"
	an_requests "anahita-libs/requests"
	"anahita-libs/responses"
	"log"
	"maps"
	"net/http"
	"socialgraph/requests"

	"github.com/labstack/echo/v4"
)

type followerHandler struct {
	*Env
}

// NewFollowerHandler creates a new user handler
func NewFollowerHandler(env *Env) *followerHandler {
	return &followerHandler{
		env,
	}
}

// Browse returns a list of follower
func (h *followerHandler) Browse(c echo.Context) error {
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

	if ok, err := h.Perms.Followers.CanBrowse(actor, viewer); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	resp := &responses.List[*responses.ActorNodeItem]{
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
	// By when each followed: the latest first, or the oldest when asked
	// (?dir=asc).
	graphReq.Ordering = constants.OrderingConnected
	graphReq.OrderingDir = req.Direction
	graphReq.Viewer = viewer
	graphReq.AccessCheck = true
	followers, total, err := h.Srv.GraphGRPC.BrowseConnectionNodes(graphReq)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	resp.Pagination.Total = total

	actorIds := extractNodeIDs(followers)
	relationships, err := h.Srv.SocialgraphGRPC.Browse(actorIds, viewer.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	relationshipMap := make(map[int64]*entities.ActorRelation)
	maps.Copy(relationshipMap, relationships)

	for _, follower := range followers {
		if rel, exists := relationshipMap[follower.ID]; exists {
			follower.IsLeadingViewer = rel.IsLeading
			// And whether they follow the viewer, for "Follows you".
			follower.IsFollowingViewer = rel.IsFollowing
		}
	}

	onAfterActorFetched(h.Env, followers)

	actorResp := &responses.ActorResponse{}
	resp.Data = actorResp.MapNodeItems(followers)

	return c.JSON(http.StatusOK, resp)
}

// Add adds a follower to an actor
func (h *followerHandler) Add(c echo.Context) error {
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

	if ok, err := h.Perms.Followers.CanAdd(source, target, viewer); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	addReq := services.NewSocialgraphFollowRequest().Add
	addReq.Source = source
	addReq.Target = target
	addReq.Viewer = viewer
	if err = h.Srv.SocialgraphFollowGRPC.Add(addReq); err != nil {
		// Its rules, ticked first (roadmap row 102, part 5b).
		if an_handlers.IsRulesNotAgreed(err) {
			return an_handlers.RefuseRulesNotAgreed(c)
		}
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Re-read so follower_count describes the graph after the edge, not
	// before it. The client stores whatever comes back.
	target, err = h.Srv.ActorsGRPC.Read(req.TargetID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	onAfterActorFetched(h.Env, []*entities.AnahitaNode{target})

	// The viewer now leads the target when the viewer is the one who followed;
	// an admin acting for a group leaves their own relationship untouched. The
	// read above happened before the edge existed, and an actor read does not
	// compute the relationship anyway, so it has to be stated here.
	if source.ID == viewer.ID {
		target.IsLeadingViewer = true
	}

	actorResp := &responses.ActorResponse{}

	return c.JSON(http.StatusCreated, actorResp.MapNodeItem(target))
}

// Delete a follower from an actor
func (h *followerHandler) Delete(c echo.Context) error {
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

	if ok, err := h.Perms.Followers.CanDelete(source, target, viewer); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	delReq := services.NewSocialgraphFollowRequest().Delete
	delReq.Source = source
	delReq.Target = target
	delReq.Viewer = viewer
	if err := h.Srv.SocialgraphFollowGRPC.Delete(delReq); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Re-read so follower_count describes the graph after the edge was
	// removed, not before it.
	target, err = h.Srv.ActorsGRPC.Read(req.TargetID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	onAfterActorFetched(h.Env, []*entities.AnahitaNode{target})

	if source.ID == viewer.ID {
		target.IsLeadingViewer = false
	}

	actorResp := &responses.ActorResponse{}

	return c.JSON(http.StatusOK, actorResp.MapNodeItem(target))
}
