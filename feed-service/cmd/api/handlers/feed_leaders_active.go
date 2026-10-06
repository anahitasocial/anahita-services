package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/responses"
	"feed/data/defaults"
	"log"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

// activeRequest asks who has posted lately. Both are capped by the graph.
type activeRequest struct {
	Hours uint32 `query:"hours" validate:"omitempty,max=168"`
	Limit uint32 `query:"limit" validate:"omitempty,max=50"`
}

// activeLeader is one actor the viewer follows who has posted lately.
type activeLeader struct {
	Actor *responses.ActorNodeItem `json:"actor"`
	// When the newest of their posts in the window was made. The app keeps
	// the time it last looked at each actor, on the device, and compares.
	LatestPostAt time.Time `json:"latest_post_at"`
	// How many posts in the window this viewer may see.
	Count uint64 `json:"count"`
}

// Active returns the actors the viewer follows that have posted lately, most
// recent first: the row of faces at the top of the home page.
//
// GET /feeds/leaders/active?hours=24
//
// It says who posted and when, and nothing about who has looked. Whether the
// viewer has seen an actor's posts is kept by the app on the viewer's own
// device and is never sent here: nobody, the poster included, can find out
// who looked at what.
//
// Only posts the viewer may see are counted, so an actor whose only recent
// post is one the viewer cannot see is not in the list.
func (h *feedLeadersHandler) Active(c echo.Context) error {
	var (
		req    = &activeRequest{}
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

	type Response struct {
		Data []*activeLeader `json:"data"`
	}
	resp := &Response{Data: []*activeLeader{}}

	feedDefaults, err := defaults.NewDefaultsLoader(h.Config).Load()
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	leaders, err := h.Srv.FeedLeadersGRPC.Active(req.Hours, req.Limit, feedDefaults.MediaEntityTypes, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if len(leaders) == 0 {
		return c.JSON(http.StatusOK, resp)
	}

	ids := make([]int64, 0, len(leaders))
	for _, leader := range leaders {
		ids = append(ids, leader.ActorID)
	}

	actorReq := services.NewActorGRPCRequest().BatchRead
	actorReq.IDs = ids
	actors, err := h.Srv.ActorsGRPC.BatchRead(actorReq, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	byID := make(map[int64]*entities.AnahitaNode, len(actors))
	avatars := make([]entities.AnahitaNodeAvatarInterface, 0, len(actors))
	for _, actor := range actors {
		byID[actor.ID] = actor
		avatars = append(avatars, actor)
	}
	services.BatchPresignAvatar(h.Srv.FilesGRPC, avatars)

	// In the order the graph gave: most recent first. An actor the viewer
	// may not read as an actor is left out.
	actorResp := &responses.ActorResponse{}
	for _, leader := range leaders {
		actor, ok := byID[leader.ActorID]
		if !ok {
			continue
		}
		resp.Data = append(resp.Data, &activeLeader{
			Actor:        actorResp.MapNodeItem(actor),
			LatestPostAt: leader.LatestAt,
			Count:        leader.Count,
		})
	}

	return c.JSON(http.StatusOK, resp)
}
