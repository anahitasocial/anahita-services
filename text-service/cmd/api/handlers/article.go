package handlers

import (
	an_handlers "anahita-libs/cmd/api/handlers"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/requests"
	"anahita-libs/responses"
	"log"
	"net/http"
	"text/constants"

	"github.com/gosimple/slug"
	"github.com/labstack/echo/v4"
)

type articleHandler struct {
	*Env
	*mediumHandler
}

// NewArticleHandler creates a new article handler
func NewArticleHandler(env *Env) *articleHandler {
	return &articleHandler{
		env,
		NewMediumHandler(env),
	}
}

// Browse returns a list of article
func (h *articleHandler) Browse(c echo.Context) error {
	var (
		err    error
		req    = requests.NewMediumRequest().Browse
		gReq   = services.NewMediaGRPCRequest().Browse
		viewer = getViewer(c)
	)

	if ok, err := h.Perms.Medium.CanBrowse(viewer); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	type Response struct {
		Data       []*responses.MediumNodeItem `json:"data"`
		Pagination responses.Pagination        `json:"pagination"`
	}
	resp := &Response{}
	resp.Pagination.Limit = req.Limit
	resp.Pagination.Offset = req.Offset

	gReq.Types = []string{constants.NodeMediumArticleType}
	gReq.Offset = req.Offset
	gReq.Limit = req.Limit
	gReq.Ordering = req.Ordering
	gReq.Direction = req.Direction
	gReq.Disabled = req.Disabled
	gReq.OwnerID = req.OwnerID
	if err := c.Validate(gReq); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	resp.Pagination.Total, err = h.Srv.MediaGRPC.Count(gReq, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	nodes, err := h.Srv.MediaGRPC.Browse(gReq, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if len(nodes) == 0 {
		return c.JSON(http.StatusOK, resp)
	}

	for _, node := range nodes {
		if err = h.onAfterFetched(c, node); err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	batchPresignPaths(h.Env, nodes)

	mediaResp := &responses.MediumNodeResponse{}
	resp.Data = mediaResp.MapMediumNodeItems(nodes)

	return c.JSON(http.StatusOK, resp)
}

// Edit updates a article
func (h *articleHandler) Edit(c echo.Context) error {
	var (
		req    = requests.NewMediumRequest().Edit
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

	medium, err := h.Srv.MediaGRPC.Read(req.ID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	mediumAccessCtx, err := h.getMediumAccessContext(medium, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Medium.CanEdit(mediumAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	medium.Name = req.Name
	medium.Alias = slug.Make(req.Name)
	medium.Body = req.Body
	medium.ModifiedBy = viewer.ID
	medium, err = h.Srv.MediaGRPC.Edit(medium, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if err = h.onAfterFetched(c, medium); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	batchPresignPaths(h.Env, []*entities.AnahitaNode{medium})

	mediaResp := &responses.MediumNodeResponse{}
	resp := mediaResp.MapMediumNode(medium)

	return c.JSON(http.StatusOK, resp)
}

// Add creates a new article
func (h *articleHandler) Add(c echo.Context) error {
	var (
		req    = requests.NewMediumRequest().Add
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

	owner, err := h.Srv.ActorsGRPC.Read(req.OwnerID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	medium := entities.NewAnahitaNode()
	medium.Type = constants.NodeMediumArticleType
	medium.Service = constants.TextService
	medium.Access = req.Access
	medium.Name = req.Name
	medium.Body = req.Body
	medium.Excerpt = req.Excerpt
	medium.OwnerID = owner.ID
	medium.OwnerType = owner.Type
	medium.Alias = slug.Make(req.Name)
	medium.CreatedBy = viewer.ID
	medium.ModifiedBy = viewer.ID
	medium.Enabled = true
	medium.CommentStatus = true
	medium.Access = an_handlers.DetermineMediumNodeAccess(req.Access, medium.Owner.Access)

	mediumAccessCtx, err := h.getMediumAccessContext(medium, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Medium.CanAdd(mediumAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	medium, err = h.Srv.MediaGRPC.Add(medium, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if err = h.onAfterFetched(c, medium); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	batchPresignPaths(h.Env, []*entities.AnahitaNode{medium})

	mediaResp := &responses.MediumNodeResponse{}
	resp := mediaResp.MapMediumNode(medium)

	return c.JSON(http.StatusCreated, resp)
}
