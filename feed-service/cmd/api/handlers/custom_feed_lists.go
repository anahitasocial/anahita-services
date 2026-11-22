package handlers

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/feeds"
	"anahita-libs/responses"
	"log"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

type CustomFeedDiscoverRequest struct {
	// Part of a name, to find a feed by.
	Search string `query:"q" validate:"omitempty,max=100"`
	// How many to pass over.
	Start uint64 `query:"start" validate:"omitempty,numeric"`
	// The most to send: up to 40.
	Limit uint64 `query:"limit" validate:"omitempty,numeric,max=40"`
	// Whose feeds: everyone's the viewer may see, theirs among them;
	// those of the people they follow; or theirs alone. Not given, the
	// feeds other people have shared, as Recommendations lists them.
	Whose string `query:"whose" validate:"omitempty,oneof=everyone following mine"`
}

// The lists Discover makes, by whose feeds they are (anahita-web row 125).
const (
	WhoseEveryone  = "everyone"
	WhoseFollowing = "following"
	WhoseMine      = "mine"
)

// DiscoverPage is a page of the feeds people have shared.
type DiscoverPage struct {
	// The feeds.
	Data []*CustomFeedResponse `json:"data"`
	// Where the page is.
	Pagination DiscoverPagination `json:"pagination"`
}

// DiscoverPagination is where a page of shared feeds is: by how many came
// before it, and with no total.
type DiscoverPagination struct {
	// The most a page holds.
	Limit uint64 `json:"limit"`
	// How many came before this page.
	Start uint64 `json:"start"`
	// What to ask for as `start` for the next page.
	NextStart uint64 `json:"next_start"`
	// Whether there is a next page.
	HasMore bool `json:"has_more"`
}

// Mine lists the feeds the viewer has made, newest first.
func (h *customFeedHandler) Mine(c echo.Context) error {
	viewer := getViewer(c)
	if !viewer.Authenticated || viewer.ID <= 0 {
		return c.NoContent(http.StatusUnauthorized)
	}

	req := services.NewGraphNodeRequest().Browse
	req.Types = []string{feeds.NodeType}
	req.OwnerIDs = []int64{viewer.ID}
	req.Limit = feeds.MaxOwned
	req.Viewer = viewer

	list, err := h.Srv.GraphNodeGRPC.Browse(req)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	_, pinned, err := h.pinnedIDs(viewer.ID)
	if err != nil {
		log.Println(err)
	}

	return c.JSON(http.StatusOK, responses.Unpaged[*CustomFeedResponse]{Data: h.respondMany(list, viewer, pinned)})
}

// Discover lists the feeds people have shared, the ones the viewer may see,
// newest first: public ones, ones for anyone signed in, and ones for the
// followers of somebody the viewer follows. A feed kept to its owner is in
// nobody's list here, its owner's included: theirs are under their own.
//
// `whose` makes the Feeds page's list of it (anahita-web row 125), for
// somebody signed in: `everyone`, the same with the viewer's own kept to
// themselves among them; `following`, the feeds of the people they follow;
// `mine`, theirs, all of them. Never ranked: newest first.
//
// The graph's list of nodes gates each feed by its own audience and its
// owner's, for the viewer, as it does a post. The viewer's own private
// feeds pass that gate, being theirs, and are taken out here. So a page can
// come back a little short, and where the next one starts is said by
// `next_start`, counted in what the graph returned and not in what was
// sent.
func (h *customFeedHandler) Discover(c echo.Context) error {
	viewer := getViewer(c)

	req := &CustomFeedDiscoverRequest{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if req.Limit == 0 {
		req.Limit = feeds.PageSize
	}
	signedIn := viewer.Authenticated && viewer.ID > 0
	if req.Whose != "" && !signedIn {
		return c.NoContent(http.StatusUnauthorized)
	}

	browse := services.NewGraphNodeRequest().Browse
	browse.Types = []string{feeds.NodeType}
	browse.Search = strings.TrimSpace(req.Search)
	browse.Offset = req.Start
	// One more than the page, to know whether there is another.
	browse.Limit = req.Limit + 1
	browse.Viewer = viewer
	browse.AccessCheck = true
	// Newest first, said, not left to the graph: one owner's list is led
	// by what is pinned to their profile otherwise.
	browse.Ordering = constants.OrderingRecent
	switch req.Whose {
	case WhoseFollowing:
		browse.OwnedByFollowed = true
	case WhoseMine:
		browse.OwnerIDs = []int64{viewer.ID}
	}

	found, err := h.Srv.GraphNodeGRPC.Browse(browse)
	if err != nil {
		log.Println("custom feed: discover:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	hasMore := uint64(len(found)) > req.Limit
	if hasMore {
		found = found[:req.Limit]
	}

	shared := make([]*entities.AnahitaNode, 0, len(found))
	for _, feed := range found {
		if feed.Type != feeds.NodeType {
			continue
		}
		// One kept to its owner only in their own lists: everyone's and
		// theirs.
		own := signedIn && feed.OwnerID == viewer.ID
		if feed.Access != constants.AnahitaAccessMyself ||
			(own && (req.Whose == WhoseEveryone || req.Whose == WhoseMine)) {
			shared = append(shared, feed)
		}
	}

	_, pinned, err := h.pinnedIDs(viewer.ID)
	if err != nil {
		log.Println(err)
	}

	return c.JSON(http.StatusOK, DiscoverPage{
		Data: h.respondMany(shared, viewer, pinned),
		Pagination: DiscoverPagination{
			Limit:     req.Limit,
			Start:     req.Start,
			NextStart: req.Start + uint64(len(found)),
			HasMore:   hasMore,
		},
	})
}
