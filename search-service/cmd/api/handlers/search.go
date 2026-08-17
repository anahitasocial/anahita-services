package handlers

import (
	"anahita-libs/data/services"
	"anahita-libs/responses"
	"log"
	"net/http"
	"search/requests"

	"github.com/labstack/echo/v4"
)

type searchHandler struct {
	*Env
}

// NewSearchHandler creates a new search handler
func NewSearchHandler(env *Env) *searchHandler {
	return &searchHandler{
		env,
	}
}

// Browse returns a list of search
func (h *searchHandler) Browse(c echo.Context) error {
	var (
		err    error
		req    = requests.NewSearchRequest().Browse
		gReq   = services.NewSearchRequest().Browse
		viewer = getViewer(c)
	)

	if ok, err := h.Perms.Search.CanBrowse(viewer); !ok {
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
		Data       []*responses.NodeItem `json:"data"`
		Pagination responses.Pagination  `json:"pagination"`
	}
	resp := &Response{}

	resp.Pagination.Limit = req.Limit
	resp.Pagination.Offset = req.Offset

	gReq.Limit = req.Limit
	gReq.Offset = req.Offset
	gReq.Keywords = req.Keywords
	gReq.Scope = req.Scope
	gReq.Ordering = req.Ordering
	gReq.OrderingDir = req.OrderingDir
	gReq.CoordLat = req.CoordLat
	gReq.CoordLng = req.CoordLng
	gReq.CoordRadius = req.CoordRadius
	gReq.IncludeComments = req.IncludeComments

	resp.Pagination.Total, err = h.Srv.SearchGRPC.Count(gReq, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	nodes, err := h.Srv.SearchGRPC.Browse(gReq, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	batchPresignPaths(h.Env, nodes)

	nodeResp := &responses.NodeResponse{}
	resp.Data = nodeResp.MapNodeItems(nodes)

	return c.JSON(http.StatusOK, resp)
}
