package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"anahita-libs/data/services"
	"anahita-libs/event"
	"feed/constants"
	"feed/requests"

	"anahita-libs/responses"

	an_entities "anahita-libs/data/entities"

	"github.com/labstack/echo/v4"
)

type repostHandler struct {
	*Env
}

// NewRepostHandler creates a new repost handler
func NewRepostHandler(env *Env) *repostHandler {
	return &repostHandler{
		env,
	}
}

// Browse
func (h *repostHandler) Browse(c echo.Context) error {
	var (
		err    error
		req    = requests.NewRepostRequest().Browse
		gReq   = services.NewRepostRequest().BrowseAuthors
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

	node, err := h.Srv.GraphNodeGRPC.Read(req.ParentID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if ok, err := h.Perms.Reposts.CanBrowse(viewer, node); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	type Response struct {
		Reposters  []*an_entities.AnahitaNode `json:"data"`
		Pagination responses.Pagination       `json:"pagination"`
	}
	resp := &Response{}

	gReq.ParentID = node.ID
	gReq.Viewer = viewer
	resp.Reposters, err = h.Srv.RepostGRPC.BrowseAuthors(gReq)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	for _, item := range resp.Reposters {
		services.PresignAvatar(h.Srv.FilesGRPC, item)
	}

	return c.JSON(http.StatusOK, resp)
}

// Add
func (h *repostHandler) Add(c echo.Context) error {
	var (
		err    error
		req    = requests.NewRepostRequest().Add
		gReq   = services.NewRepostRequest().Add
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

	node, err := h.Srv.GraphNodeGRPC.Read(req.ParentID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if ok, err := h.Perms.Reposts.CanAdd(viewer, node); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	gReq.OwnerID = viewer.ID
	gReq.ParentID = node.ID
	gReq.Service = constants.ConfigServiceName
	gReq.Access = node.Access
	gReq.Viewer = viewer
	repost, err := h.Srv.RepostGRPC.Add(gReq)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// publish event
	go func() {
		entityName := strings.Split(repost.Parent.Type, ".")[3]
		routing_key := fmt.Sprintf(event.EventFormatNodeRepostAdded, entityName)
		payload := &event.EventPayload{
			Name:           routing_key,
			SourceService:  repost.Service,
			SubjectID:      viewer.ID,
			SubjectType:    viewer.Type,
			ObjectID:       repost.ID,
			ObjectType:     repost.Type,
			TargetID:       repost.Owner.ID,
			TargetType:     repost.Owner.Type,
			ViewerID:       viewer.ID,
			ViewerUserType: viewer.UserType,
		}
		log.Println("Publishing event", routing_key)
		if err = h.Publisher.Publish(payload, routing_key); err != nil {
			log.Println(err)
		}
	}()

	return c.JSON(http.StatusCreated, repost)
}

// Delete
func (h *repostHandler) Delete(c echo.Context) error {
	var (
		err     error
		req     = requests.NewRepostRequest().Delete
		readReq = services.NewRepostRequest().Read
		delReq  = services.NewRepostRequest().Delete
		viewer  = getViewer(c)
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	node, err := h.Srv.GraphNodeGRPC.Read(req.ParentID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	readReq.ParentID = node.ID
	readReq.AuthorID = viewer.ID
	repost, err := h.Srv.RepostGRPC.Read(readReq)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if ok, err := h.Perms.Reposts.CanDelete(viewer, repost); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	delReq.ParentID = node.ID
	delReq.AuthorID = viewer.ID
	if err := h.Srv.RepostGRPC.Delete(delReq); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// publish event
	go func() {
		entityName := strings.Split(repost.Parent.Type, ".")[3]
		routing_key := fmt.Sprintf(event.EventFormatNodeRepostDeleted, entityName)
		payload := &event.EventPayload{
			Name:           routing_key,
			SourceService:  repost.Service,
			SubjectID:      viewer.ID,
			SubjectType:    viewer.Type,
			ObjectID:       repost.ID,
			ObjectType:     repost.Type,
			TargetID:       repost.Owner.ID,
			TargetType:     repost.Owner.Type,
			ViewerID:       viewer.ID,
			ViewerUserType: viewer.UserType,
		}
		h.Publisher.Publish(payload, routing_key)
	}()

	return c.NoContent(http.StatusOK)
}
