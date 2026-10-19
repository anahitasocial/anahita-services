package handlers

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	an_responses "anahita-libs/responses"
	"anahita-libs/trending"
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
	gReq.Limit = req.Limit
	gReq.Viewer = viewer
	gReq.AccessCheck = false

	// From the window asked for, or the installation's, wider while it
	// holds too few (anahita-libs/trending).
	start := trending.Normal(req.TimeWindow)
	if start == "" {
		start = trending.DefaultWindow()
	}
	found, window, err := trending.Widen(start, req.Limit, func(window string) ([]*services.TrendingItem, error) {
		gReq.TimeWindow = window
		return h.Srv.GraphGRPC.TrendingItems(gReq)
	})
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// In the envelope every list comes in, as one page that is all there
	// is, and with the window it is of.
	hashtagResp := &responses.HashtagNodeResponse{}
	resp := &an_responses.Trending[*responses.HashtagNodeItem]{Window: strings.ToLower(window)}
	resp.Data = []*responses.HashtagNodeItem{}
	for _, each := range found {
		mapped := hashtagResp.MapHashtagNodeItems([]*entities.AnahitaNode{each.Node})
		if len(mapped) == 1 {
			mapped[0].Profiles = each.Profiles
			resp.Data = append(resp.Data, mapped[0])
		}
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
