package handlers

import (
	"anahita-libs/constants"
	"anahita-libs/data/services"
	an_responses "anahita-libs/responses"
	"hashtag/requests"
	"hashtag/responses"
	"log"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

type hashtagHandler struct {
	*Env
}

// NewHashtagHandler creates a new hashtag handler
func NewHashtagHandler(env *Env) *hashtagHandler {
	return &hashtagHandler{
		env,
	}
}

// Browse returns a list of hashtag
func (h *hashtagHandler) Browse(c echo.Context) error {
	var (
		err    error
		req    = requests.NewHashtagRequest().Browse
		viewer = getViewer(c)
	)

	if ok, err := h.Perms.Hashtags.CanBrowse(viewer); !ok {
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

	resp := &an_responses.List[*responses.HashtagNodeItem]{}

	resp.Pagination.Limit = req.Limit
	resp.Pagination.Offset = req.Offset

	gReq := services.NewGraphNodeRequest().Browse
	gReq.Types = []string{constants.NodeBaseHashtagEntityType}
	gReq.Limit = req.Limit
	gReq.Offset = req.Offset
	gReq.Ordering = req.Ordering
	gReq.Viewer = viewer
	gReq.AccessCheck = false

	resp.Pagination.Total, err = h.Srv.GraphNodeGRPC.Count(gReq)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	nodes, err := h.Srv.GraphNodeGRPC.Browse(gReq)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if len(nodes) == 0 {
		return c.JSON(http.StatusOK, resp)
	}

	hashtagResp := &responses.HashtagNodeResponse{}
	resp.Data = hashtagResp.MapHashtagNodeItems(nodes)

	return c.JSON(http.StatusOK, resp)
}

// BrowseOrderedByDegree returns a list of popular hashtags
func (h *hashtagHandler) BrowseOrderedByDegree(c echo.Context) error {
	var (
		err    error
		viewer = getViewer(c)
		req    = requests.NewHashtagRequest().BrowseOrderedByDegree
		gReq   = services.NewGraphRequest().BrowseOrderedByDegree
	)

	if ok, err := h.Perms.Hashtags.CanBrowse(viewer); !ok {
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

	resp := &an_responses.List[*responses.HashtagNodeItem]{}

	resp.Pagination.Limit = req.Limit
	resp.Pagination.Offset = req.Offset

	gReq.NodeType = constants.NodeBaseHashtagEntityType
	gReq.EdgeTypes = []string{constants.EdgeTagHashtagType}
	gReq.Direction = constants.GraphQueryDirectionOutbound
	gReq.Limit = req.Limit
	gReq.Offset = req.Offset
	gReq.Viewer = viewer
	gReq.AccessCheck = false
	nodes, total, err := h.Srv.GraphGRPC.BrowseOrderedByDegree(gReq)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	resp.Pagination.Total = total

	if len(nodes) == 0 {
		return c.JSON(http.StatusOK, resp)
	}

	hashtagResp := &responses.HashtagNodeResponse{}
	resp.Data = hashtagResp.MapHashtagNodeItems(nodes)

	return c.JSON(http.StatusOK, resp)
}

// BrowseTrending returns a list of trending hashtags
func (h *hashtagHandler) BrowseTrending(c echo.Context) error {
	var (
		err    error
		viewer = getViewer(c)
		req    = requests.NewHashtagRequest().BrowseTrending
		gReq   = services.NewGraphRequest().Trending
	)

	if ok, err := h.Perms.Hashtags.CanBrowse(viewer); !ok {
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

	gReq.NodeType = constants.NodeBaseHashtagEntityType
	gReq.EdgeTypes = []string{constants.EdgeTagHashtagType}
	gReq.Direction = constants.GraphQueryDirectionOutbound
	gReq.Limit = req.Limit
	gReq.TimeWindow = strings.ToUpper(req.TimeWindow)
	gReq.Viewer = viewer
	gReq.AccessCheck = false
	nodes, err := h.Srv.GraphGRPC.Trending(gReq)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// In the envelope every list comes in, as one page that is all there
	// is. Sent bare, the web app read it as a page with nothing on it, and
	// showed no trending hashtags at all.
	hashtagResp := &responses.HashtagNodeResponse{}
	resp := &an_responses.List[*responses.HashtagNodeItem]{Data: []*responses.HashtagNodeItem{}}
	if len(nodes) > 0 {
		resp.Data = hashtagResp.MapHashtagNodeItems(nodes)
	}
	resp.Pagination.Limit = req.Limit
	resp.Pagination.Total = uint64(len(resp.Data))

	return c.JSON(http.StatusOK, resp)
}

// Read returns a hashtag
func (h *hashtagHandler) Read(c echo.Context) error {
	var (
		req    = requests.NewHashtagRequest().Read
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

	// log.Println(req.Alias)

	gReq := services.NewGraphNodeRequest().ReadByAlias
	gReq.Alias = req.Alias
	gReq.NodeType = constants.NodeBaseHashtagEntityType
	node, err := h.Srv.GraphNodeGRPC.ReadByAlias(gReq)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if ok, err := h.Perms.Hashtags.CanRead(viewer, node); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	hashtagResp := &responses.HashtagNodeResponse{}
	resp := hashtagResp.MapHashtagNodeItem(node)

	return c.JSON(http.StatusOK, resp)
}

// Delete removes a hashtag
func (h *hashtagHandler) Delete(c echo.Context) error {
	var (
		err    error
		req    = requests.NewHashtagRequest().Delete
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

	hashtag, err := h.Srv.GraphNodeGRPC.Read(req.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if ok, err := h.Perms.Hashtags.CanDelete(viewer, hashtag); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	_, err = h.Srv.GraphNodeGRPC.Delete(hashtag.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusOK)
}
