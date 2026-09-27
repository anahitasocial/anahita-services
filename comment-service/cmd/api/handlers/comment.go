package handlers

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/event"
	an_responses "anahita-libs/responses"
	"comment/data/validators"
	"comment/requests"
	"comment/responses"
	"fmt"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

type commentHandler struct {
	*Env
}

// NewCommentHandler creates a new comment handler
func NewCommentHandler(env *Env) *commentHandler {
	return &commentHandler{
		env,
	}
}

// Browse returns a list of comments
func (h *commentHandler) Browse(c echo.Context) error {
	var (
		err          error
		req          = requests.NewCommentRequest().Browse
		graphNodeReq = services.NewGraphNodeRequest().Browse
		viewer       = getViewer(c)
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	parent, err := h.readParentWithOwner(req.ParentID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	parentAccessCtx, err := h.getCommentParentAccessContext(parent, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Comments.CanBrowse(parentAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	type Response struct {
		Data       []*responses.CommentNodeItem `json:"data"`
		Pagination an_responses.Pagination      `json:"pagination"`
	}
	resp := &Response{}
	resp.Pagination.Limit = req.Limit
	resp.Pagination.Offset = req.Offset

	graphNodeReq.Types = []string{constants.NodeBaseCommentEntityType}
	graphNodeReq.ParentIDs = []int64{req.ParentID}
	graphNodeReq.Limit = req.Limit
	graphNodeReq.Offset = req.Offset
	graphNodeReq.Ordering = req.Ordering
	graphNodeReq.OrderingDir = req.Direction
	graphNodeReq.AccessCheck = true
	graphNodeReq.Viewer = viewer
	resp.Pagination.Total, err = h.Srv.GraphNodeGRPC.Count(graphNodeReq)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	nodes, err := h.Srv.GraphNodeGRPC.Browse(graphNodeReq)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if len(nodes) == 0 {
		return c.JSON(http.StatusOK, resp)
	}

	targetIds := extractNodeIDs(nodes)
	isLikedStatuses, err := h.Srv.LikesGRPC.IsLikedMany(viewer.ID, targetIds)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	for _, comment := range nodes {
		comment.IsLikedByViewer = isLikedStatuses[comment.ID]
		// One post, read once, for every comment under it — the edit and
		// delete flags need its owner.
		comment.Parent = parent
		if err := h.onAfterFetched(c, comment); err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	batchPresignPaths(h.Env, nodes)

	commentsResp := &responses.CommentNodeResponse{}
	resp.Data = commentsResp.MapCommentNodeItems(nodes)

	return c.JSON(http.StatusOK, resp)
}

// Edit updates a comment
func (h *commentHandler) Edit(c echo.Context) error {
	var (
		err    error
		req    = requests.NewCommentRequest().Edit
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

	// A node read, not a medium read. Comments are node.base.comment.*, and
	// the medium read is scoped to node.medium.* — so it never matched a
	// comment and every edit died on "sql: no rows in result set". Delete has
	// always read the comment this way; Edit was the one that did not.
	comment, err := h.Srv.GraphNodeGRPC.Read(req.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	parent, err := h.readParentWithOwner(comment.ParentID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	comment.Parent = parent

	commentAccessCtx, err := h.getCommentAccessContext(comment, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Comments.CanEdit(commentAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	comment.Body = req.Body
	comment.ModifiedBy = viewer.ID

	v := validators.NewCommentValidator()
	if err := v.ValidateEdit(comment); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	comment, err = h.Srv.GraphNodeGRPC.Edit(comment)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	comment.Parent = parent

	comment.IsLikedByViewer, err = h.Srv.LikesGRPC.IsLiked(viewer.ID, comment.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if err := h.onAfterFetched(c, comment); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	batchPresignPaths(h.Env, []*entities.AnahitaNode{comment})

	// publish event
	go func() {
		routing_key := fmt.Sprintf(event.EventFormatNodeCommentEdited, "comment")
		payload := &event.EventPayload{
			Name:             routing_key,
			SourceService:    comment.Service,
			SubjectID:        viewer.ID,
			SubjectType:      viewer.Type,
			ObjectID:         comment.ID,
			ObjectType:       comment.Type,
			TargetID:         comment.Parent.Author.ID,
			TargetType:       comment.Parent.Author.Type,
			ViewerID:         viewer.ID,
			ViewerPersonType: viewer.PersonType,
		}
		h.Publisher.Publish(payload, routing_key)
	}()

	commentsResp := &responses.CommentNodeResponse{}
	resp := commentsResp.MapCommentNodeItem(comment)

	return c.JSON(http.StatusOK, resp)
}

// Add creates a new comment
func (h *commentHandler) Add(c echo.Context) error {
	var (
		err     error
		req     = requests.NewCommentRequest().Add
		viewer  = getViewer(c)
		comment = entities.NewAnahitaNode()
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	// The post and its owner — see readParentWithOwner. Neither used to reach
	// CanAdd: comment.Parent was the empty placeholder NewAnahitaNode makes,
	// so a stranger could comment on a post they were refused permission to
	// read.
	parent, err := h.readParentWithOwner(req.ParentID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	comment.Parent = parent
	comment.Type = constants.NodeBaseCommentEntityType
	comment.Service = constants.CommentService
	comment.ParentID = parent.ID
	comment.ParentType = parent.Type
	comment.Body = req.Body
	comment.CreatedBy = viewer.ID
	comment.ModifiedBy = viewer.ID
	comment.Enabled = true

	v := validators.NewCommentValidator()
	if err := v.ValidateAdd(comment); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	commentAccessCtx, err := h.getCommentAccessContext(comment, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Comments.CanAdd(commentAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	comment, err = h.Srv.GraphNodeGRPC.Add(comment)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	comment.Parent = parent
	comment.IsLikedByViewer = false

	go func() {
		if err := h.updateParentCommentCount(parent.ID, viewer); err != nil {
			log.Println("Failed to update parent comment count:", err)
		}
	}()

	if err := h.onAfterFetched(c, comment); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	batchPresignPaths(h.Env, []*entities.AnahitaNode{comment})

	// publish event
	go func() {
		routing_key := fmt.Sprintf(event.EventFormatNodeCommentAdded, "comment")
		payload := &event.EventPayload{
			Name:             routing_key,
			SourceService:    comment.Service,
			SubjectID:        viewer.ID,
			SubjectType:      viewer.Type,
			ObjectID:         comment.ID,
			ObjectType:       comment.Type,
			TargetID:         comment.Parent.Author.ID,
			TargetType:       comment.Parent.Author.Type,
			ViewerID:         viewer.ID,
			ViewerPersonType: viewer.PersonType,
		}
		h.Publisher.Publish(payload, routing_key)
	}()

	commentsResp := &responses.CommentNodeResponse{}
	resp := commentsResp.MapCommentNodeItem(comment)

	return c.JSON(http.StatusCreated, resp)
}

// Delete removes a comment
func (h *commentHandler) Delete(c echo.Context) error {
	var (
		err    error
		req    = requests.NewCommentRequest().Delete
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

	comment, err := h.Srv.GraphNodeGRPC.Read(req.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	parent, err := h.readParentWithOwner(comment.ParentID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	comment.Parent = parent

	commentAccessCtx, err := h.getCommentAccessContext(comment, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Comments.CanDelete(commentAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	_, err = h.Srv.GraphNodeGRPC.Delete(comment.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	go func() {
		if err := h.updateParentCommentCount(parent.ID, viewer); err != nil {
			log.Println("Failed to update parent comment count:", err)
		}
	}()

	// publish event
	go func() {
		routing_key := fmt.Sprintf(event.EventFormatNodeCommentDeleted, "comment")
		payload := &event.EventPayload{
			Name:             routing_key,
			SourceService:    comment.Service,
			SubjectID:        viewer.ID,
			SubjectType:      viewer.Type,
			ObjectID:         comment.ID,
			ObjectType:       comment.Type,
			TargetID:         comment.Parent.Author.ID,
			TargetType:       comment.Parent.Author.Type,
			ViewerID:         viewer.ID,
			ViewerPersonType: viewer.PersonType,
		}
		h.Publisher.Publish(payload, routing_key)
	}()

	return c.NoContent(http.StatusOK)
}

// onAfterFetched is called after fetching a comment
func (h *commentHandler) onAfterFetched(c echo.Context, comment *entities.AnahitaNode) error {
	viewer := getViewer(c)

	commentAccessCtx, err := h.getCommentAccessContext(comment, viewer)
	if err != nil {
		return err
	}

	comment.Authorized.Edit, _ = h.Perms.Comments.CanEdit(commentAccessCtx)
	comment.Authorized.Delete, _ = h.Perms.Comments.CanDelete(commentAccessCtx)
	comment.Authorized.Administration, _ = h.Perms.Comments.CanAdministrate(commentAccessCtx)

	return nil
}

func (h *commentHandler) updateParentCommentCount(parentID int64, viewer *entities.Viewer) error {
	parent, err := h.Srv.MediaGRPC.Read(parentID, viewer)
	if err != nil {
		return err
	}

	count, err := h.Srv.GraphNodeGRPC.Count(&services.GraphNodeBrowseRequest{
		ParentIDs:   []int64{parentID},
		Types:       []string{constants.NodeBaseCommentEntityType},
		Viewer:      viewer,
		AccessCheck: false,
	})
	if err != nil {
		return err
	}

	parent.CommentCount = int64(count)

	_, err = h.Srv.MediaGRPC.Edit(parent, viewer)
	if err != nil {
		return err
	}

	return nil
}
