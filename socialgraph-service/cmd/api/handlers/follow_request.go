package handlers

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	an_services "anahita-libs/data/services"
	an_requests "anahita-libs/requests"
	"anahita-libs/responses"
	"log"
	"maps"
	"net/http"
	"socialgraph/requests"

	"github.com/labstack/echo/v4"
)

type followRequestHandler struct {
	*Env
}

// NewFollowerHandler creates a new user handler
func NewFollowRequestHandler(env *Env) *followRequestHandler {
	return &followRequestHandler{
		env,
	}
}

// Browse returns a list of follower
func (h *followRequestHandler) Browse(c echo.Context) error {
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

	if ok, err := h.Perms.FollowRequests.CanBrowse(actor, viewer); !ok {
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
	graphReq.EdgeType = constants.EdgeFollowRequestEntityType
	graphReq.Direction = constants.GraphQueryDirectionInbound
	graphReq.Offset = req.Offset
	graphReq.Limit = req.Limit
	graphReq.Viewer = viewer
	followRequests, total, err := h.Srv.GraphGRPC.BrowseConnectionNodes(graphReq)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	resp.Pagination.Total = total

	actorIds := extractNodeIDs(followRequests)
	relationships, err := h.Srv.SocialgraphGRPC.Browse(actorIds, viewer.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	relationshipMap := make(map[int64]*entities.ActorRelation)
	maps.Copy(relationshipMap, relationships)

	for _, followRequest := range followRequests {
		if rel, exists := relationshipMap[followRequest.ID]; exists {
			followRequest.IsLeadingViewer = rel.IsLeading
		}
		h.onAfterFetched(followRequest)
	}

	actorResp := &responses.ActorResponse{}
	resp.Data = actorResp.MapNodeItems(followRequests)

	return c.JSON(http.StatusOK, resp)
}

// Edit changes a follow request to a follower
func (h *followRequestHandler) Edit(c echo.Context) error {
	var (
		err    error
		req    = requests.NewSocialgraphRequest().Edit
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

	if ok, err := h.Perms.FollowRequests.CanEdit(source, target, viewer); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	editReq := services.NewSocialgraphFollowRequest().Edit
	editReq.Source = source
	editReq.Target = target
	editReq.Viewer = viewer
	editReq.AcceptFollowRequest = true
	if err = h.Srv.SocialgraphFollowGRPC.Edit(editReq); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	h.onAfterFetched(target)

	return c.JSON(http.StatusAccepted, target)
}

// Add adds a follower to an actor
func (h *followRequestHandler) Add(c echo.Context) error {
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

	if ok, err := h.Perms.FollowRequests.CanAdd(source, target, viewer); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}
	addReq := services.NewSocialgraphFollowRequest().Add
	addReq.Source = source
	addReq.Target = target
	addReq.Viewer = viewer
	addReq.IsFollowRequest = true
	if err = h.Srv.SocialgraphFollowGRPC.Add(addReq); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	target, err = h.Srv.ActorsGRPC.Read(req.TargetID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	h.onAfterFetched(target)

	return c.JSON(http.StatusCreated, target)
}

// Delete a follower from an actor
func (h *followRequestHandler) Delete(c echo.Context) error {
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

	if ok, err := h.Perms.FollowRequests.CanDelete(source, target, viewer); !ok {
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

	h.onAfterFetched(target)

	return c.JSON(http.StatusOK, target)
}

// onAfterFetched presigns the avatar of the actor
func (h *followRequestHandler) onAfterFetched(actor *entities.AnahitaNode) {
	an_services.PresignAvatar(h.Srv.FilesGRPC, actor)
	an_services.PresignCover(h.Srv.FilesGRPC, actor)
}
