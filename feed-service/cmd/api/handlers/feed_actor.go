package handlers

import (
	"anahita-libs/constants"
	"anahita-libs/data/services"
	"anahita-libs/policy"
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

	// Replies are not part of a preview. A visitor to a site that shows
	// one is sent a profile's posts and not what its owner said under
	// other people's.
	if req.Filter == constants.FeedFilterReplies && policy.PreviewOnly() && viewer.ID == 0 {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "members_only"})
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

	// Whether reposts are among a profile's posts is the profile's choice,
	// and not the caller's: asking for them does not bring them back. The
	// profile is read as the viewer, so one they may not see answers as
	// one that is not there.
	actor, err := h.Srv.ActorsGRPC.Read(req.ActorID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}
	includeReposts := !actor.HideRepostsOnProfile

	gnCountReq.ActorID = req.ActorID
	gnCountReq.Filter = req.Filter
	gnCountReq.IncludeReposts = includeReposts
	gnCountReq.MediumEntityTypes = feedDefaults.MediaEntityTypes
	resp.Pagination.Total, err = h.Srv.FeedActorGRPC.Count(gnCountReq, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	gnBrowseReq.ActorID = req.ActorID
	gnBrowseReq.Filter = req.Filter
	gnBrowseReq.IncludeReposts = includeReposts
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
	if req.Filter == constants.FeedFilterReplies {
		setReplyFeedFlags(h.Env, nodes, viewer)
	} else {
		setFeedFlags(h.Env, nodes, viewer)
	}

	nodeResp := &responses.NodeResponse{}
	resp.Data = nodeResp.MapNodeItems(nodes)

	return c.JSON(http.StatusOK, resp)
}
