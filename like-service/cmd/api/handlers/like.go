package handlers

import (
	an_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/event"
	"anahita-libs/responses"
	"feed/constants"
	"fmt"
	"like/requests"
	"log"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

type likeHandler struct {
	*Env
}

// NewLikeHandler creates a new like handler
func NewLikeHandler(env *Env) *likeHandler {
	return &likeHandler{
		env,
	}
}

// Browse returns a list of likes
func (h *likeHandler) Browse(c echo.Context) error {
	var (
		err      error
		req      = requests.NewLikeRequest().Browse
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

	node, err := h.Srv.GraphNodeGRPC.Read(req.NodeID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if ok, err := h.Perms.Likes.CanBrowse(viewer, node); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	type Response struct {
		Data       []*responses.ActorNodeItem `json:"data"`
		Pagination responses.Pagination       `json:"pagination"`
	}
	resp := &Response{}
	resp.Pagination.Limit = req.Limit
	resp.Pagination.Offset = req.Offset

	graphReq.NodeId = node.ID
	graphReq.EdgeType = an_constants.EdgeBaseLikeEntityType
	graphReq.Direction = an_constants.GraphQueryDirectionInbound
	graphReq.Offset = req.Offset
	graphReq.Limit = req.Limit
	graphReq.Ordering = req.Ordering
	graphReq.OrderingDir = req.Direction
	graphReq.Viewer = viewer
	graphReq.AccessCheck = true
	nodes, total, err := h.Srv.GraphGRPC.BrowseConnectionNodes(graphReq)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	resp.Pagination.Total = total

	if len(nodes) == 0 {
		return c.JSON(http.StatusOK, resp)
	}

	batchPresignPaths(h.Env, nodes)

	actorsResp := &responses.ActorResponse{}
	resp.Data = actorsResp.MapNodeItems(nodes)

	return c.JSON(http.StatusOK, resp)
}

// Add adds a like
func (h *likeHandler) Add(c echo.Context) error {
	var (
		err    error
		req    = requests.NewLikeRequest().Add
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

	actor, err := h.Srv.ActorsGRPC.Read(viewer.ID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	node, err := h.Srv.GraphNodeGRPC.Read(req.NodeID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if ok, err := h.Perms.Likes.CanAdd(viewer, node); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	alreadyLiked, err := h.Srv.LikesGRPC.IsLiked(actor.ID, node.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if alreadyLiked {
		return c.NoContent(http.StatusConflict)
	}

	if err := h.Srv.LikesGRPC.Add(actor, node, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	node, err = h.Srv.GraphNodeGRPC.Read(req.NodeID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	node.IsLikedByViewer = true
	batchPresignPaths(h.Env, []*entities.AnahitaNode{node})

	// publish event
	go func() {
		entityName := strings.Split(node.Service, "-")[0]
		routing_key := fmt.Sprintf(event.EventFormatEdgeVoteupAdded, entityName)
		if node.IsComment() {
			payload := &event.EventPayload{
				Name:           routing_key,
				SourceService:  constants.ConfigServiceName,
				OwnerID:        node.Author.ID,
				SubjectID:      viewer.ID,
				ObjectID:       node.ID,
				TargetID:       node.Parent.Author.ID,
				ViewerID:       viewer.ID,
				ViewerUserType: viewer.UserType,
			}
			h.Publisher.Publish(payload, routing_key)
		} else {
			payload := &event.EventPayload{
				Name:           routing_key,
				SourceService:  constants.ConfigServiceName,
				OwnerID:        node.Author.ID,
				SubjectID:      viewer.ID,
				ObjectID:       node.ID,
				TargetID:       node.Author.ID,
				ViewerID:       viewer.ID,
				ViewerUserType: viewer.UserType,
			}
			h.Publisher.Publish(payload, routing_key)
		}
	}()

	var resp any
	if node.IsComment() {
		nodeResp := &responses.CommentResponse{}
		resp = nodeResp.MapCommentItem(node)
	} else {
		nodeResp := &responses.MediumNodeResponse{}
		resp = nodeResp.MapMediumNode(node)
	}

	return c.JSON(http.StatusCreated, resp)
}

// Delete removes a like
func (h *likeHandler) Delete(c echo.Context) error {
	var (
		err    error
		req    = requests.NewLikeRequest().Delete
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

	actor, err := h.Srv.ActorsGRPC.Read(viewer.ID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	node, err := h.Srv.GraphNodeGRPC.Read(req.NodeID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	node.IsLikedByViewer, err = h.Srv.LikesGRPC.IsLiked(actor.ID, node.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if ok, err := h.Perms.Likes.CanDelete(viewer, node); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	if err := h.Srv.LikesGRPC.Delete(actor, node); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	node, err = h.Srv.GraphNodeGRPC.Read(req.NodeID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	batchPresignPaths(h.Env, []*entities.AnahitaNode{node})

	// publish event
	go func() {
		entityName := strings.Split(node.Service, "-")[0]
		routing_key := fmt.Sprintf(event.EventFormatEdgeVoteupDeleted, entityName)
		if node.IsComment() {
			payload := &event.EventPayload{
				Name:           routing_key,
				SourceService:  constants.ConfigServiceName,
				OwnerID:        node.Author.ID,
				SubjectID:      viewer.ID,
				ObjectID:       node.ID,
				TargetID:       node.Parent.Author.ID,
				ViewerID:       viewer.ID,
				ViewerUserType: viewer.UserType,
			}
			h.Publisher.Publish(payload, routing_key)
		} else {
			payload := &event.EventPayload{
				Name:           routing_key,
				SourceService:  constants.ConfigServiceName,
				OwnerID:        node.Author.ID,
				SubjectID:      viewer.ID,
				ObjectID:       node.ID,
				TargetID:       node.Author.ID,
				ViewerID:       viewer.ID,
				ViewerUserType: viewer.UserType,
			}
			h.Publisher.Publish(payload, routing_key)
		}
	}()

	var resp any
	if node.IsComment() {
		nodeResp := &responses.CommentResponse{}
		resp = nodeResp.MapCommentItem(node)
	} else {
		nodeResp := &responses.MediumNodeResponse{}
		resp = nodeResp.MapMediumNode(node)
	}

	return c.JSON(http.StatusOK, resp)
}

// onAfterFetched is called after fetching a comment
func (h *likeHandler) onAfterFetched(c echo.Context, node *entities.AnahitaNode) error {
	if node.IsComment() {
		h.onAfterFetchedComment(c, node)
	} else {
		h.onAfterFetchedMedium(c, node)
	}

	return nil
}

// onAfterFetchedComment sets access context for a comment node
func (h *likeHandler) onAfterFetchedComment(c echo.Context, node *entities.AnahitaNode) error {
	var (
		err    error
		viewer = getViewer(c)
	)

	commentAccessCtx, err := h.getCommentAccessContext(node, viewer)
	if err != nil {
		return err
	}

	node.Authorized.Edit, _ = h.Perms.Comments.CanEdit(commentAccessCtx)
	node.Authorized.Delete, _ = h.Perms.Comments.CanDelete(commentAccessCtx)
	node.Authorized.Administration, _ = h.Perms.Comments.CanAdministrate(commentAccessCtx)

	return err
}

// onAfterFetchedMedium sets access context for a medium node
func (h *likeHandler) onAfterFetchedMedium(c echo.Context, node *entities.AnahitaNode) error {
	var (
		err    error
		viewer = getViewer(c)
	)

	mediumAccessCtx, err := h.getMediumAccessContext(node, viewer)
	if err != nil {
		return err
	}

	node.Authorized.Edit, _ = h.Perms.Medium.CanEdit(mediumAccessCtx)
	node.Authorized.Delete, _ = h.Perms.Medium.CanDelete(mediumAccessCtx)
	node.Authorized.Administration, _ = h.Perms.Medium.CanAdministrate(mediumAccessCtx)

	return err
}
