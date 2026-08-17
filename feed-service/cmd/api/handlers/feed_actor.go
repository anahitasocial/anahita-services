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

type feedActorHandler struct {
	*Env
}

func NewFeedActorHandler(env *Env) *feedActorHandler {
	return &feedActorHandler{
		Env: env,
	}
}

// Browse returns a list of feeds
func (h *feedActorHandler) Browse(c echo.Context) error {
	var (
		err         error
		req         = requests.NewFeedActorRequest().Browse
		gnCountReq  = services.NewFeedActorRequest().Count
		gnBrowseReq = services.NewFeedActorRequest().Browse
		viewer      = getViewer(c)
	)

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

	gnCountReq.ActorID = req.ActorID
	gnCountReq.IncludeReposts = req.IncludeReposts
	gnCountReq.MediumEntityTypes = feedDefaults.MediaEntityTypes
	resp.Pagination.Total, err = h.Srv.FeedActorGRPC.Count(gnCountReq, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	gnBrowseReq.ActorID = req.ActorID
	gnBrowseReq.IncludeReposts = req.IncludeReposts
	gnBrowseReq.Limit = req.Limit
	gnBrowseReq.Offset = req.Offset
	gnBrowseReq.MediumEntityTypes = feedDefaults.MediaEntityTypes
	nodes, err := h.Srv.FeedActorGRPC.Browse(gnBrowseReq, viewer)
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
