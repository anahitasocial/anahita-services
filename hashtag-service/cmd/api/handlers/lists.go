package handlers

import (
	"log"
	"net/http"
	"strings"

	"anahita-libs/constants"
	"anahita-libs/data/services"
	an_responses "anahita-libs/responses"
	"hashtag/responses"

	"github.com/labstack/echo/v4"
)

// The lists of the Hashtags page beside what is trending (the user,
// 2026-10-17): the hashtags somebody follows, and every hashtag, from A to
// Z or the most used first, to search. A hashtag is counted by the posts it
// is on that anybody may see, and the viewer's own; one on none of those
// is listed only to somebody who follows it.

// ListRequest is a page of one of the lists.
type ListRequest struct {
	// Only hashtags beginning with this, its # left out.
	Q string `query:"q" validate:"max=100"`
	// The order: `name`, A to Z (the default), or `uses`, the most used
	// first.
	Sort string `query:"sort" validate:"omitempty,oneof=name uses"`
	// How many to pass over.
	Offset uint64 `query:"start"`
	// The most to send: up to 20.
	Limit uint32 `query:"limit" validate:"omitempty,max=20"`
}

// Following is the hashtags the viewer follows: those in their custom
// feeds, which following a hashtag makes.
//
// GET /hashtags/_browse/following
func (h *hashtagHandler) Following(c echo.Context) error {
	return h.list(c, true)
}

// All is every hashtag on a post anybody may see.
//
// GET /hashtags/_browse/all
func (h *hashtagHandler) All(c echo.Context) error {
	return h.list(c, false)
}

func (h *hashtagHandler) list(c echo.Context, followedOnly bool) error {
	req := &ListRequest{Sort: "name", Limit: 20}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if req.Sort == "" {
		req.Sort = "name"
	}
	if req.Limit == 0 {
		req.Limit = 20
	}

	found, total, err := h.Srv.HashtagGRPC.List(&services.HashtagSuggestRequest{
		Prefix:       strings.TrimPrefix(strings.TrimSpace(req.Q), "#"),
		Limit:        req.Limit,
		List:         true,
		FollowedOnly: followedOnly,
		Order:        req.Sort,
		Offset:       req.Offset,
	}, getViewer(c))
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	resp := &an_responses.List[*responses.HashtagNodeItem]{Data: []*responses.HashtagNodeItem{}}
	resp.Pagination.Offset = req.Offset
	resp.Pagination.Limit = uint64(req.Limit)
	resp.Pagination.Total = total
	for _, each := range found {
		resp.Data = append(resp.Data, &responses.HashtagNodeItem{
			ID:       each.ID,
			Type:     constants.NodeBaseHashtagEntityType,
			Name:     each.Name,
			Alias:    each.Alias,
			Uses:     uint64(each.Uses),
			Followed: each.Followed,
		})
	}
	return c.JSON(http.StatusOK, resp)
}
