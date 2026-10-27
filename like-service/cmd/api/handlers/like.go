package handlers

import (
	an_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/event"
	an_handlers "anahita-libs/handlers"
	"anahita-libs/permissions"
	"anahita-libs/responses"
	"errors"
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

	node, err := h.readLikeTarget(req.NodeID, viewer)
	if err != nil {
		log.Println(err)
		if errors.Is(err, errNoTarget) {
			return c.NoContent(http.StatusNotFound)
		}
		return c.NoContent(http.StatusInternalServerError)
	}

	if ok, err := h.Perms.Likes.CanBrowse(viewer, node); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	resp := &responses.List[*responses.ActorNodeItem]{}
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

	services.BatchPresignNodes(h.Env.Srv.FilesGRPC, nodes)

	// Whether the viewer follows each of them, and is followed by them.
	an_handlers.SetViewerRelations(h.Srv.SocialgraphGRPC, nodes, viewer)

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

	node, err := h.readLikeTarget(req.NodeID, viewer)
	if err != nil {
		log.Println(err)
		if errors.Is(err, errNoTarget) {
			return c.NoContent(http.StatusNotFound)
		}
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

	node, err = h.readLikeTarget(req.NodeID, viewer)
	if err != nil {
		log.Println(err)
		if errors.Is(err, errNoTarget) {
			return c.NoContent(http.StatusNotFound)
		}
		return c.NoContent(http.StatusInternalServerError)
	}

	node.IsLikedByViewer = true
	services.BatchPresignNodes(h.Env.Srv.FilesGRPC, []*entities.AnahitaNode{node})

	// publish event
	go func() {
		entityName := strings.Split(node.Service, "-")[0]
		routing_key := fmt.Sprintf(event.EventFormatEdgeVoteupAdded, entityName)
		payload := &event.EventPayload{
			Name:             routing_key,
			SourceService:    constants.ConfigServiceName,
			OwnerID:          node.Author.ID,
			SubjectID:        viewer.ID,
			ObjectID:         node.ID,
			TargetID:         node.Author.ID,
			ViewerID:         viewer.ID,
			ViewerPersonType: viewer.PersonType,
		}
		h.Publisher.Publish(payload, routing_key)
	}()

	nodeResp := &responses.MediumNodeResponse{}

	return c.JSON(http.StatusCreated, nodeResp.MapMediumNode(node))
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

	node, err := h.readLikeTarget(req.NodeID, viewer)
	if err != nil {
		log.Println(err)
		if errors.Is(err, errNoTarget) {
			return c.NoContent(http.StatusNotFound)
		}
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

	node, err = h.readLikeTarget(req.NodeID, viewer)
	if err != nil {
		log.Println(err)
		if errors.Is(err, errNoTarget) {
			return c.NoContent(http.StatusNotFound)
		}
		return c.NoContent(http.StatusInternalServerError)
	}

	services.BatchPresignNodes(h.Env.Srv.FilesGRPC, []*entities.AnahitaNode{node})

	// publish event
	go func() {
		entityName := strings.Split(node.Service, "-")[0]
		routing_key := fmt.Sprintf(event.EventFormatEdgeVoteupDeleted, entityName)
		payload := &event.EventPayload{
			Name:             routing_key,
			SourceService:    constants.ConfigServiceName,
			OwnerID:          node.Author.ID,
			SubjectID:        viewer.ID,
			ObjectID:         node.ID,
			TargetID:         node.Author.ID,
			ViewerID:         viewer.ID,
			ViewerPersonType: viewer.PersonType,
		}
		h.Publisher.Publish(payload, routing_key)
	}()

	nodeResp := &responses.MediumNodeResponse{}

	return c.JSON(http.StatusOK, nodeResp.MapMediumNode(node))
}

// onAfterFetched answers what the viewer may do with what they liked.
func (h *likeHandler) onAfterFetched(c echo.Context, node *entities.AnahitaNode) error {
	var (
		err    error
		viewer = getViewer(c)
	)

	// A reply's answers come from its thread, as text-service gives them.
	// readLikeTarget has loaded the post at the top as its Parent.
	if node.RootID > 0 {
		return h.onAfterFetchedReply(node, viewer)
	}

	mediumAccessCtx, err := h.getMediumAccessContext(node, viewer)
	if err != nil {
		return err
	}

	node.Authorized.Edit, _ = h.Perms.Medium.CanEdit(mediumAccessCtx)
	node.Authorized.Delete, _ = h.Perms.Medium.CanDelete(mediumAccessCtx)
	node.Authorized.Administration, _ = h.Perms.Medium.CanAdministrate(mediumAccessCtx)

	// The web app replaces the post with this response, so it carries the
	// same answers the media services give.
	postFlags(node, viewer, mediumAccessCtx.OwnerViewerRel)

	return err
}

// onAfterFetchedReply answers what the viewer may do with a reply, by the
// rules for the thread it is in.
func (h *likeHandler) onAfterFetchedReply(reply *entities.AnahitaNode, viewer *entities.Viewer) error {
	root := reply.GetParent()
	if root == nil || root.GetID() == 0 {
		return nil
	}

	rels, err := h.Srv.SocialgraphGRPC.Browse([]int64{root.GetOwnerID()}, viewer.ID)
	if err != nil {
		return err
	}

	ctx := &permissions.ReplyAccessContext{
		Viewer:             viewer,
		Reply:              reply,
		Root:               root,
		RootOwnerViewerRel: rels[root.GetOwnerID()],
	}

	hide := permissions.CanHideReply(ctx)
	canReply := permissions.CanReply(ctx)
	like := permissions.CanLikeReply(ctx)

	reply.Authorized.Edit = permissions.CanEditReply(ctx)
	reply.Authorized.Delete = permissions.CanDeleteReply(ctx)
	reply.Authorized.Hide = &hide
	reply.Authorized.Comment = &canReply
	reply.Authorized.Like = &like

	return nil
}
