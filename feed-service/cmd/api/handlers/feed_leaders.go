package handlers

import (
	"anahita-libs/data/services"
	an_handlers "anahita-libs/handlers"
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
		gnBrowseReq = services.NewFeedLeadersRequest().Browse
		viewer      = getViewer(c)
	)

	if ok, err := h.Perms.Feeds.CanBrowse(viewer); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	if !an_handlers.Bound(c, req) {
		return c.NoContent(http.StatusBadRequest)
	}

	resp := &responses.List[*responses.NodeItem]{}
	resp.Pagination.Limit = req.Limit
	resp.Pagination.Offset = req.Offset

	defaultsLoader := defaults.NewDefaultsLoader(h.Config)
	feedDefaults, err := defaultsLoader.Load()
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Not counted: counting the home feed is reading all of it, every
	// access rule on every post, for each page (a fiftieth of a second for
	// a page, half a second for its count, on a copy of banu.io; roadmap row
	// 170). One more than the page is asked for instead, which says whether
	// there is a page after it, as a custom feed's does.
	gnBrowseReq.IncludeReposts = req.IncludeReposts
	gnBrowseReq.Limit = req.Limit + 1
	gnBrowseReq.Offset = req.Offset
	gnBrowseReq.MediumEntityTypes = feedDefaults.MediaEntityTypes
	gnBrowseReq.LeaderEntityTypes = feedDefaults.ActorEntityTypes
	nodes, err := h.Srv.FeedLeadersGRPC.Browse(gnBrowseReq, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	more := uint64(len(nodes)) > req.Limit
	if more {
		nodes = nodes[:req.Limit]
	}
	resp.Pagination.HasMore = &more
	if len(nodes) == 0 {
		return c.JSON(http.StatusOK, resp)
	}

	// A note that quotes a post is sent that post, as this viewer may see
	// it. Before the images are made ready, so the quoted posts' are too.
	if quoted := an_handlers.SetQuotes(h.Srv.MediaGRPC, h.Srv.GraphNodeGRPC, nodes, viewer); len(quoted) > 0 {
		batchPresignPaths(h.Env, quoted)
	}

	batchPresignPaths(h.Env, nodes)
	setFeedFlags(h.Env, nodes, viewer)

	nodeResp := &responses.NodeResponse{}
	resp.Data = nodeResp.MapNodeItems(nodes)

	return c.JSON(http.StatusOK, resp)
}
