package handlers

import (
	an_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/requests"
	"anahita-libs/responses"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

type inboundHandler struct {
	*Env
}

// NewInboundHandler creates a new inbound handler
func NewInboundHandler(env *Env) *inboundHandler {
	return &inboundHandler{
		env,
	}
}

// Browse returns a list of tagged nodes
func (h *inboundHandler) Browse(c echo.Context) error {
	var (
		err      error
		req      = requests.NewInboundRequest().Browse
		graphReq = services.NewGraphRequest().BrowseConnectionNodes
		viewer   = getViewer(c)
	)

	if ok, err := h.Perms.Taggables.CanBrowse(viewer); !ok {
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

	source, err := h.Srv.GraphNodeGRPC.Read(req.SourceID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	resp := &responses.List[*responses.NodeItem]{}
	resp.Pagination.Limit = req.Limit
	resp.Pagination.Offset = req.Offset

	graphReq.NodeId = source.ID
	graphReq.EdgeType = an_constants.EdgeTagLocationType
	graphReq.Direction = an_constants.GraphQueryDirectionOutbound
	graphReq.Offset = req.Offset
	graphReq.Limit = req.Limit
	graphReq.Ordering = req.Ordering
	graphReq.OrderingDir = req.OrderingDir
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

	nodeResp := &responses.NodeResponse{}
	resp.Data = nodeResp.MapNodeItems(nodes)

	return c.JSON(http.StatusOK, resp)
}

// Add adds a new association to a location
func (h *inboundHandler) Add(c echo.Context) error {
	var (
		err    error
		req    = requests.NewInboundRequest().Add
		viewer = getViewer(c)
	)

	if err = c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err = c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	location, err := h.Srv.GraphNodeGRPC.Read(req.TargetID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	source, err := h.Srv.GraphNodeGRPC.Read(req.SourceID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if ok, err := h.Perms.Taggables.CanAdd(viewer, source); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	edge := entities.NewAnahitaEdge()
	edge.Type = an_constants.EdgeTagLocationType
	edge.NodeAID = location.ID
	edge.NodeAType = location.Type
	edge.NodeBID = source.ID
	edge.NodeBType = source.Type
	edge.CreatedBy = viewer.ID
	edge, err = h.Srv.GraphEdgeGRPC.Add(edge)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusCreated)
}

// Delete removes a tagged node from location
func (h *inboundHandler) Delete(c echo.Context) error {
	var (
		err    error
		req    = requests.NewInboundRequest().Delete
		viewer = getViewer(c)
	)

	if err = c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err = c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	location, err := h.Srv.GraphNodeGRPC.Read(req.TargetID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	source, err := h.Srv.GraphNodeGRPC.Read(req.SourceID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if ok, err := h.Perms.Taggables.CanDelete(viewer, source); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	gReq := &services.GraphEdgeBatchReadByFiltersRequest{
		NodeAID:   location.ID,
		NodeAType: location.Type,
		NodeBID:   source.ID,
		NodeBType: source.Type,
		Types:     []string{an_constants.EdgeTagLocationType},
	}
	edges, err := h.Srv.GraphEdgeGRPC.BatchReadByFilters(gReq)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	for _, edge := range edges {
		err = h.Srv.GraphEdgeGRPC.Delete(edge.ID)
		if err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	return c.NoContent(http.StatusOK)
}
