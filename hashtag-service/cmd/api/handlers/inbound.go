package handlers

import (
	libs_constants "anahita-libs/constants"
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

// Browse returns a list of nodes
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
	graphReq.EdgeType = libs_constants.EdgeTagHashtagType
	graphReq.Direction = libs_constants.GraphQueryDirectionOutbound
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
