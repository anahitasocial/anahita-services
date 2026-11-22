package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/feeds"
	an_handlers "anahita-libs/handlers"
	"anahita-libs/responses"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

type CustomFeedItemsRequest struct {
	// The feed's id.
	ID int64 `param:"id" validate:"required,numeric"`
	// Where the page begins: the `next_cursor` of the page before. The
	// latest when left out.
	Cursor string `query:"cursor" validate:"omitempty,max=100"`
	// The most to send: up to 40.
	Limit uint64 `query:"limit" validate:"omitempty,numeric,max=40"`
	// Where the reader is, for a feed of what is near them: its area is
	// around this point for this page, and the point is kept nowhere.
	// Rounded to three decimals. Needed by such a feed, unread by any
	// other.
	NearLatitude  *float64 `query:"near_latitude"`
	NearLongitude *float64 `query:"near_longitude"`
}

type CustomFeedPreviewRequest struct {
	// What it would show, as feeds.md describes it.
	Definition json.RawMessage `json:"definition"`
}

// CustomFeedPage is a page of a custom feed's posts.
type CustomFeedPage struct {
	// Its posts, the latest first.
	Data []*responses.NodeItem `json:"data"`
	// Where the page is.
	Pagination CustomFeedPagination `json:"pagination"`
}

// A feed is paged by what came before, and has no total: counting it would
// be reading all of it.
type CustomFeedPagination struct {
	// The most a page holds.
	Limit uint64 `json:"limit"`
	// What to ask for as `cursor` for the next page.
	NextCursor string `json:"next_cursor"`
	// Whether there is a next page.
	HasMore bool `json:"has_more"`
}

// Items returns a page of a feed: what it shows, as the viewer may see it.
func (h *customFeedHandler) Items(c echo.Context) error {
	viewer := getViewer(c)

	req := &CustomFeedItemsRequest{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	feed := h.read(req.ID, viewer)
	if feed == nil {
		return c.NoContent(http.StatusNotFound)
	}

	def, err := definitionOf(feed)
	if err != nil {
		return refuseFeed(c, http.StatusUnprocessableEntity, string(feeds.RefusedMalformed))
	}

	before, err := feeds.ParseCursor(req.Cursor)
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	// A feed of what is near its reader is read around where they are,
	// which they say with each page.
	if def.NeedsPoint() {
		if req.NearLatitude == nil || req.NearLongitude == nil {
			return refuseFeed(c, http.StatusUnprocessableEntity, "location_needed")
		}
		if !def.AtPoint(*req.NearLatitude, *req.NearLongitude) {
			return refuseFeed(c, http.StatusUnprocessableEntity, string(feeds.RefusedNear))
		}
	}

	// "Whoever I follow" is whoever the feed's owner follows.
	return h.page(c, def, feed.OwnerID, before, req.Limit, viewer)
}

// Preview returns the first page of a feed that has not been saved, so that
// whoever is building it can see what it would show. As they would see it,
// and as theirs.
func (h *customFeedHandler) Preview(c echo.Context) error {
	viewer := getViewer(c)
	if !mayWriteFeeds(viewer) {
		return c.NoContent(http.StatusForbidden)
	}

	req := &CustomFeedPreviewRequest{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	def, err := feeds.Parse(req.Definition)
	if err != nil {
		reason := string(feeds.RefusedMalformed)
		if refusal, ok := feeds.AsRefusal(err); ok {
			reason = string(refusal)
		}
		return refuseFeed(c, http.StatusUnprocessableEntity, reason)
	}

	return h.page(c, def, viewer.ID, feeds.Cursor{}, feeds.PageSize, viewer)
}

func (h *customFeedHandler) page(c echo.Context, def *feeds.Definition, ownerID int64, before feeds.Cursor, limit uint64, viewer *entities.Viewer) error {
	if limit == 0 || limit > feeds.MaxPage {
		limit = feeds.PageSize
	}

	resp := &CustomFeedPage{Data: []*responses.NodeItem{}}
	resp.Pagination.Limit = limit

	// One more than the page, to know whether there is another.
	nodes, err := h.Srv.FeedCustomGRPC.Browse(def, ownerID, before, limit+1, viewer)
	if err != nil {
		if errors.Is(err, services.ErrFeedTooBroad) {
			return refuseFeed(c, http.StatusUnprocessableEntity, string(feeds.RefusedTooBroad))
		}
		log.Println("custom feed: items:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if uint64(len(nodes)) > limit {
		nodes = nodes[:limit]
		resp.Pagination.HasMore = true
	}
	if len(nodes) == 0 {
		return c.JSON(http.StatusOK, resp)
	}
	if resp.Pagination.HasMore {
		last := nodes[len(nodes)-1]
		resp.Pagination.NextCursor = feeds.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}.String()
	}

	if quoted := an_handlers.SetQuotes(h.Srv.MediaGRPC, h.Srv.GraphNodeGRPC, nodes, viewer); len(quoted) > 0 {
		batchPresignPaths(h.Env, quoted)
	}

	batchPresignPaths(h.Env, nodes)
	setFeedFlags(h.Env, nodes, viewer)

	nodeResp := &responses.NodeResponse{}
	resp.Data = nodeResp.MapNodeItems(nodes)

	return c.JSON(http.StatusOK, resp)
}
