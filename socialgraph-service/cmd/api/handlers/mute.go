package handlers

import (
	"anahita-libs/constants"
	an_entities "anahita-libs/data/entities"
	"anahita-libs/data/services"
	an_requests "anahita-libs/requests"
	"anahita-libs/responses"
	"log"
	"net/http"
	"socialgraph/requests"
	"time"

	"github.com/labstack/echo/v4"
)

// Mutes (roadmap row 133): whom a person would rather not see, their own
// alone. Nothing is told to the person muted, nor to their server.
type muteHandler struct {
	*Env
}

func NewMuteHandler(env *Env) *muteHandler {
	return &muteHandler{env}
}

// The most muted people sent at once.
const mutesPageMax = 100

// How long each choice of `for` lasts.
var muteLengths = map[string]time.Duration{
	"day":   24 * time.Hour,
	"week":  7 * 24 * time.Hour,
	"month": 30 * 24 * time.Hour,
}

// Browse sends a page of whom the person muted, most lately first, with
// until when and whether their notifications are hidden. A mute that has
// ended is not in it.
func (h *muteHandler) Browse(c echo.Context) error {
	var (
		req      = requests.NewSocialgraphRequest().Browse
		graphReq = services.NewGraphRequest().BrowseNeighbours
		actorReq = an_requests.NewActorRequest().BatchRead
		viewer   = getViewer(c)
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Bind(actorReq); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	actor, err := h.Srv.ActorsGRPC.Read(req.ActorID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if ok, err := h.Perms.Mutes.CanBrowse(actor, viewer); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	graphReq.NodeId = actor.ID
	graphReq.EdgeTypes = []string{constants.EdgeMuteEntityType}
	graphReq.Direction = constants.GraphQueryDirectionOutbound
	graphReq.Offset = req.Offset
	graphReq.Limit = req.Limit
	if graphReq.Limit == 0 || graphReq.Limit > mutesPageMax {
		graphReq.Limit = mutesPageMax
	}
	graphReq.Search = req.Search
	graphReq.Ordering = constants.OrderingConnected
	graphReq.OrderingDir = req.Direction
	graphReq.Viewer = viewer
	graphReq.AccessCheck = true

	nodes, total, err := h.Srv.GraphGRPC.BrowseNeighbours(graphReq)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if err := h.withMutes(actor.ID, nodes); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	onAfterActorFetched(h.Env, nodes)

	actorResp := &responses.ActorResponse{}
	return c.JSON(http.StatusOK, &responses.List[*responses.ActorNodeItem]{
		Data: actorResp.MapNodeItems(nodes),
		Pagination: responses.Pagination{
			Limit:  req.Limit,
			Offset: req.Offset,
			Total:  total,
		},
	})
}

// withMutes fills in, on each person in a page, the mute made of them: one
// read of the person's mutes, which are few, rather than one a row.
func (h *muteHandler) withMutes(muterID int64, nodes []*an_entities.AnahitaNode) error {
	if len(nodes) == 0 {
		return nil
	}
	edges, err := h.Srv.GraphEdgeGRPC.BatchReadByFilters(&services.GraphEdgeBatchReadByFiltersRequest{
		NodeAID: muterID,
		Types:   []string{constants.EdgeMuteEntityType},
	})
	if err != nil {
		return err
	}
	byMuted := make(map[int64]*an_entities.AnahitaEdge, len(edges))
	for _, e := range edges {
		byMuted[e.NodeBID] = e
	}
	for _, node := range nodes {
		e, ok := byMuted[node.ID]
		if !ok {
			continue
		}
		node.IsMutedByViewer = true
		node.MuteHidesNotifications = e.Meta != constants.MuteMetaNotifications
		if e.EndsAt.Unix() > 0 {
			node.MuteEndsAt = e.EndsAt
		}
	}
	return nil
}

// Add mutes, or changes a mute: it is made again from now.
func (h *muteHandler) Add(c echo.Context) error {
	var (
		req    = &requests.MuteAddRequest{}
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

	target, err := h.Srv.ActorsGRPC.Read(req.TargetID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}
	source, err := h.Srv.ActorsGRPC.Read(req.SourceID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if ok, err := h.Perms.Mutes.CanAdd(source, target, viewer); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	hide := req.HideNotifications == nil || *req.HideNotifications
	var endsAt time.Time
	if length, ok := muteLengths[req.For]; ok {
		endsAt = time.Now().UTC().Add(length).Truncate(time.Second)
	}

	if err := h.Srv.SocialgraphMuteGRPC.Add(source, target, viewer, hide, endsAt); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	target.IsMutedByViewer = true
	target.MuteHidesNotifications = hide
	target.MuteEndsAt = endsAt
	onAfterActorFetched(h.Env, []*an_entities.AnahitaNode{target})

	actorResp := &responses.ActorResponse{}
	return c.JSON(http.StatusCreated, actorResp.MapNodeItem(target))
}

// Delete unmutes.
func (h *muteHandler) Delete(c echo.Context) error {
	var (
		req    = requests.NewSocialgraphRequest().Delete
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

	target, err := h.Srv.ActorsGRPC.Read(req.TargetID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}
	source, err := h.Srv.ActorsGRPC.Read(req.SourceID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if ok, err := h.Perms.Mutes.CanDelete(source, target, viewer); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	if err := h.Srv.SocialgraphMuteGRPC.Delete(source, target, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	target.IsMutedByViewer = false
	onAfterActorFetched(h.Env, []*an_entities.AnahitaNode{target})

	actorResp := &responses.ActorResponse{}
	return c.JSON(http.StatusOK, actorResp.MapNodeItem(target))
}
