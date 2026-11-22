package handlers

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/feeds"
	"anahita-libs/responses"
	"log"
	"net/http"
	"sort"

	"github.com/labstack/echo/v4"
)

type CustomFeedPinsRequest struct {
	// The feeds kept on the viewer's home page, in order: up to 10.
	IDs []int64 `json:"ids" validate:"max=10,dive,numeric,min=1"`
}

// pinEdges reads the feeds a person keeps on their home page, in order.
func (h *customFeedHandler) pinEdges(personID int64) ([]*entities.AnahitaEdge, error) {
	if personID <= 0 {
		return nil, nil
	}

	req := services.NewGraphEdgeRequest().BatchReadByFilters
	req.NodeAID = personID
	req.Types = []string{feeds.EdgePinType}
	req.Ordering = "ordering"
	req.OrderingDir = constants.DirAsc

	edges, err := h.Srv.GraphEdgeGRPC.BatchReadByFilters(req)
	if err != nil {
		return nil, err
	}

	sort.SliceStable(edges, func(i, j int) bool { return edges[i].Ordering < edges[j].Ordering })

	return edges, nil
}

func (h *customFeedHandler) pinnedIDs(personID int64) ([]int64, map[int64]bool, error) {
	edges, err := h.pinEdges(personID)
	if err != nil {
		return nil, nil, err
	}

	ids := make([]int64, 0, len(edges))
	pinned := make(map[int64]bool, len(edges))
	for _, edge := range edges {
		if !pinned[edge.NodeBID] {
			pinned[edge.NodeBID] = true
			ids = append(ids, edge.NodeBID)
		}
	}

	return ids, pinned, nil
}

// writePins replaces a person's pinned feeds with these, in this order.
func (h *customFeedHandler) writePins(viewer *entities.Viewer, ids []int64) error {
	clear := services.NewGraphEdgeRequest().BatchDeleteByFilters
	clear.NodeAID = viewer.ID
	clear.Types = []string{feeds.EdgePinType}
	// Narrowed three ways. Left to the person alone, this would be every
	// edge they have: whoever they follow, what they like.
	clear.NodeAType = constants.NodeActorPersonEntityType
	clear.NodeBType = feeds.NodeType
	if err := h.Srv.GraphEdgeGRPC.BatchDeleteByFilters(clear); err != nil {
		return err
	}

	if len(ids) == 0 {
		return nil
	}

	edges := make([]*entities.AnahitaEdge, 0, len(ids))
	for place, id := range ids {
		edges = append(edges, &entities.AnahitaEdge{
			Type:      feeds.EdgePinType,
			NodeAID:   viewer.ID,
			NodeAType: constants.NodeActorPersonEntityType,
			NodeBID:   id,
			NodeBType: feeds.NodeType,
			Ordering:  int32(place + 1),
			CreatedBy: viewer.ID,
		})
	}

	_, err := h.Srv.GraphEdgeGRPC.BatchAdd(edges)

	return err
}

// Pinned lists the feeds the viewer keeps on their home page, in order:
// their own and other people's. One they may no longer see is left out.
func (h *customFeedHandler) Pinned(c echo.Context) error {
	viewer := getViewer(c)
	if !viewer.Authenticated || viewer.ID <= 0 {
		return c.NoContent(http.StatusUnauthorized)
	}

	list, pinned, err := h.pinnedFeeds(viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, responses.Unpaged[*CustomFeedResponse]{Data: h.respondMany(list, viewer, pinned)})
}

func (h *customFeedHandler) pinnedFeeds(viewer *entities.Viewer) ([]*entities.AnahitaNode, map[int64]bool, error) {
	ids, pinned, err := h.pinnedIDs(viewer.ID)
	if err != nil {
		return nil, nil, err
	}
	if len(ids) == 0 {
		return []*entities.AnahitaNode{}, pinned, nil
	}

	req := services.NewGraphNodeRequest().BatchRead
	req.IDs = ids
	req.Viewer = viewer
	nodes, err := h.Srv.GraphNodeGRPC.BatchRead(req)
	if err != nil {
		return nil, nil, err
	}

	visible, err := h.Srv.GraphNodeGRPC.FilterVisible(ids, viewer)
	if err != nil {
		return nil, nil, err
	}
	seen := make(map[int64]bool, len(visible))
	for _, id := range visible {
		seen[id] = true
	}

	byID := make(map[int64]*entities.AnahitaNode, len(nodes))
	for _, node := range nodes {
		if node.Type != feeds.NodeType {
			continue
		}
		if ownsFeed(viewer, node) || viewer.IsAdmin() || seen[node.ID] {
			byID[node.ID] = node
		}
	}

	list := make([]*entities.AnahitaNode, 0, len(ids))
	for _, id := range ids {
		if node, ok := byID[id]; ok {
			list = append(list, node)
		}
	}

	return list, pinned, nil
}

// SetPins replaces the feeds on the viewer's home page with these, in this
// order. An empty list takes them all off.
func (h *customFeedHandler) SetPins(c echo.Context) error {
	viewer := getViewer(c)
	if !viewer.Authenticated || viewer.ID <= 0 {
		return c.NoContent(http.StatusUnauthorized)
	}

	req := &CustomFeedPinsRequest{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		return refuseFeed(c, http.StatusUnprocessableEntity, "too_many_pins")
	}

	ids := make([]int64, 0, len(req.IDs))
	seen := map[int64]bool{}
	for _, id := range req.IDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		if h.read(id, viewer) == nil {
			return refuseFeed(c, http.StatusUnprocessableEntity, "feed_not_found")
		}
		ids = append(ids, id)
	}

	if err := h.writePins(viewer, ids); err != nil {
		log.Println("custom feed: pins:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return h.Pinned(c)
}

// Pin keeps one feed on the viewer's home page, after the ones there.
func (h *customFeedHandler) Pin(c echo.Context) error {
	return h.setPinned(c, true)
}

// Unpin takes one feed off the viewer's home page.
func (h *customFeedHandler) Unpin(c echo.Context) error {
	return h.setPinned(c, false)
}

func (h *customFeedHandler) setPinned(c echo.Context, pin bool) error {
	viewer := getViewer(c)
	if !viewer.Authenticated || viewer.ID <= 0 {
		return c.NoContent(http.StatusUnauthorized)
	}

	req := &CustomFeedIDRequest{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	current, pinned, err := h.pinnedIDs(viewer.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Taking one off needs nothing of the feed: it may be gone, or no
	// longer the viewer's to see.
	if !pin {
		if !pinned[req.ID] {
			return c.NoContent(http.StatusNoContent)
		}
		kept := make([]int64, 0, len(current))
		for _, id := range current {
			if id != req.ID {
				kept = append(kept, id)
			}
		}
		if err := h.writePins(viewer, kept); err != nil {
			log.Println("custom feed: unpin:", err)
			return c.NoContent(http.StatusInternalServerError)
		}
		return c.NoContent(http.StatusNoContent)
	}

	if h.read(req.ID, viewer) == nil {
		return c.NoContent(http.StatusNotFound)
	}
	if pinned[req.ID] {
		return c.NoContent(http.StatusNoContent)
	}
	if len(current) >= feeds.MaxPinned {
		return refuseFeed(c, http.StatusConflict, "too_many_pins")
	}

	if err := h.writePins(viewer, append(current, req.ID)); err != nil {
		log.Println("custom feed: pin:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusNoContent)
}
