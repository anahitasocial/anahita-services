package handlers

import (
	"anahita-libs/data/services"
	"anahita-libs/responses"
	"feed/data/defaults"
	"feed/requests"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

type feedLeadersHandler struct {
	*Env
}

// NewFeedLeadersHandler creates a new feed leaders handler
func NewFeedLeadersHandler(env *Env) *feedLeadersHandler {
	return &feedLeadersHandler{
		env,
	}
}

// Browse returns a list of feeds
func (h *feedLeadersHandler) Browse(c echo.Context) error {
	var (
		err         error
		req         = requests.NewFeedLeadersRequest().Browse
		gnCountReq  = services.NewFeedLeadersRequest().Count
		gnBrowseReq = services.NewFeedLeadersRequest().Browse
		viewer      = getViewer(c)
	)

	log.Printf("Viewer ID: %v, Username: %v, UserType: %v, Email: %v", viewer.ID, viewer.Username, viewer.UserType, viewer.Email)

	if ok, err := h.Perms.Feeds.CanBrowse(viewer); !ok {
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

	defaultsLoader := defaults.NewDefaultsLoader(h.Config)
	feedDefaults, err := defaultsLoader.Load()
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	gnCountReq.IncludeReposts = req.IncludeReposts
	gnCountReq.IncludeReposts = req.IncludeReposts
	gnCountReq.IncludeComments = req.IncludeComments
	gnCountReq.IncludeLiked = req.IncludeLiked
	gnCountReq.MediumEntityTypes = feedDefaults.MediaEntityTypes
	gnCountReq.LeaderEntityTypes = feedDefaults.ActorEntityTypes
	resp.Pagination.Total, err = h.Srv.FeedLeadersGRPC.Count(gnCountReq, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	gnBrowseReq.IncludeReposts = req.IncludeReposts
	gnBrowseReq.IncludeComments = req.IncludeComments
	gnBrowseReq.IncludeLiked = req.IncludeLiked
	gnBrowseReq.Limit = req.Limit
	gnBrowseReq.Offset = req.Offset
	gnBrowseReq.MediumEntityTypes = feedDefaults.MediaEntityTypes
	gnBrowseReq.LeaderEntityTypes = feedDefaults.ActorEntityTypes
	nodes, err := h.Srv.FeedLeadersGRPC.Browse(gnBrowseReq, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if len(nodes) == 0 {
		return c.JSON(http.StatusOK, resp)
	}

	batchPresignPaths(h.Env, nodes)

	nodeResp := &responses.NodeResponse{}
	resp.Data = nodeResp.MapNodeItems(nodes)

	return c.JSON(http.StatusOK, resp)
}
