package handlers

import (
	"errors"
	"log"
	"net/http"
	"time"

	"syndication/data/repositories"
	"syndication/feeds"

	an_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/domains"
	an_handlers "anahita-libs/handlers"

	"github.com/labstack/echo/v4"
)

// A channel's feed, in its settings (roadmap row 147, phase 4): shown to
// its admins, connected once its site's domain is proved, and disconnected,
// keeping the notes it brought. One feed a channel.

// ChannelFeedRequest names a channel.
type ChannelFeedRequest struct {
	// The channel's id.
	ChannelID int64 `param:"channel_id" validate:"required,min=1"`
}

// ChannelFeedConnectRequest is a proved site's feed to connect to a channel.
type ChannelFeedConnectRequest struct {
	// The channel's id.
	ChannelID int64 `param:"channel_id" validate:"required,min=1"`
	// The connection begun and proved, as starting it answered.
	ConnectionID int64 `json:"connection_id" validate:"required,min=1"`
	// The feed, as starting the connection found it.
	FeedURL string `json:"feed_url" validate:"required,max=2000"`
}

// ChannelFeed is a channel's feed as its admins see it.
type ChannelFeed struct {
	// The feed read, and the site it is of.
	FeedURL string `json:"feed_url"`
	SiteURL string `json:"site_url"`
	// `active`, or `paused`: its domain's proof gone for a week, or the
	// channel switched off.
	State string `json:"state"`
	// When it was last read, if it has been.
	ReadAt *time.Time `json:"read_at,omitempty"`
}

// ShowFeed is a channel's feed, for its admins.
//
// GET /channels/_connect/feeds/:channel_id
func (h *connectHandler) ShowFeed(c echo.Context) error {
	viewer := getViewer(c)
	req := &ChannelFeedRequest{}
	if err := c.Bind(req); err != nil || c.Validate(req) != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if _, status := h.channelLookedAfter(req.ChannelID, viewer); status != 0 {
		return c.NoContent(status)
	}

	shown, err := h.Srv.Channels.Of(c.Request().Context(), req.ChannelID)
	if errors.Is(err, repositories.ErrNotFound) {
		return c.NoContent(http.StatusNotFound)
	}
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.JSON(http.StatusOK, ChannelFeed{
		FeedURL: shown.FeedURL,
		SiteURL: shown.SiteURL,
		State:   shown.State,
		ReadAt:  shown.ReadAt,
	})
}

// ConnectFeed connects a proved site's feed to a channel: its ten newest
// items are posted, the rest marked seen, and what is published afterwards
// is posted as it comes. An item already seen by the channel, from a feed
// it had before, is not posted again.
//
// PUT /channels/_connect/feeds/:channel_id
func (h *connectHandler) ConnectFeed(c echo.Context) error {
	viewer := getViewer(c)
	// Not where the instance does not offer channels (instance.json,
	// roadmap row 139).
	if !an_handlers.ProfileOffered(h.Srv.GraphGRPC, "channel") {
		return an_handlers.RefuseOff(c, "channel")
	}
	if status := h.mayConnect(viewer); status != 0 {
		return c.NoContent(status)
	}

	req := &ChannelFeedConnectRequest{}
	if err := c.Bind(req); err != nil || c.Validate(req) != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	channel, status := h.channelLookedAfter(req.ChannelID, viewer)
	if status != 0 {
		return c.NoContent(status)
	}

	ctx := c.Request().Context()
	if _, err := h.Srv.Channels.Of(ctx, channel.ID); err == nil {
		return c.JSON(http.StatusConflict, refusal{refusedHasFeed})
	} else if !errors.Is(err, repositories.ErrNotFound) {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	domain, status := h.connectionOf(c, req.ConnectionID, viewer)
	if status != 0 {
		return c.NoContent(status)
	}
	if !domain.Proved() {
		return c.JSON(http.StatusConflict, refusal{refusedNotProved})
	}

	// The feed is read again: what was sent is only an address, and what
	// it holds now is what is marked seen.
	site := domains.Site{Host: domain.Host, Domain: domain.Domain}
	if !site.Covers(req.FeedURL) {
		return c.JSON(http.StatusBadRequest, refusal{refusedFeedOffSite})
	}
	if feeds.IsPrivate(req.FeedURL) {
		return c.JSON(http.StatusBadRequest, refusal{refusedPrivateFeed})
	}
	read, err := h.Feeds.Read(ctx, req.FeedURL, "", "")
	switch {
	case errors.Is(err, feeds.ErrPrivate):
		return c.JSON(http.StatusBadRequest, refusal{refusedPrivateFeed})
	case errors.Is(err, feeds.ErrNoFeed):
		return c.JSON(http.StatusBadRequest, refusal{refusedNoFeed})
	case err != nil:
		return c.JSON(http.StatusBadRequest, refusal{refusedUnreadable})
	}
	if taken, err := h.Srv.Channels.FeedTaken(ctx, req.FeedURL); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	} else if taken {
		return c.JSON(http.StatusConflict, refusal{refusedFeedTaken})
	}

	siteURL := "https://" + domain.Host + "/"
	err = h.Srv.Channels.Add(ctx, &repositories.Channel{
		NodeID:   channel.ID,
		FeedURL:  req.FeedURL,
		SiteURL:  siteURL,
		DomainID: domain.ID,
	})
	if errors.Is(err, repositories.ErrFeedTaken) {
		// Connected to another channel a moment ago, or to this one.
		return c.JSON(http.StatusConflict, refusal{refusedFeedTaken})
	}
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Its newest few are posted, so that its followers see the feed has
	// come; the rest are marked seen, so that they are not sent the site's
	// whole past (the user, 2026-10-16).
	waiting, seen := firstPosts(read.Items)
	if err := h.Srv.Items.AddWaiting(ctx, channel.ID, waiting); err != nil {
		log.Println(err)
	}
	if err := h.Srv.Items.MarkSeen(ctx, channel.ID, seen); err != nil {
		log.Println(err)
	}
	if err := h.Srv.Domains.ForProfile(ctx, domain.ID, channel.ID); err != nil {
		log.Println(err)
	}

	// The site in its About card, where it named none: the one it proved.
	if channel.WebsiteURL == "" {
		channel.WebsiteURL = siteURL
		channel.ModifiedBy = viewer.ID
		if _, err := h.Srv.ActorsGRPC.Edit(channel, viewer); err != nil {
			log.Println(err)
		}
	}

	return c.JSON(http.StatusCreated, ChannelFeed{
		FeedURL: req.FeedURL,
		SiteURL: siteURL,
		State:   repositories.StateActive,
	})
}

// DisconnectFeed stops a channel reading its feed. The notes it made stay,
// as its site wrote them; connecting the same feed again posts none of
// them twice.
//
// DELETE /channels/_connect/feeds/:channel_id
func (h *connectHandler) DisconnectFeed(c echo.Context) error {
	viewer := getViewer(c)
	if viewer == nil || !viewer.Authenticated || viewer.ID <= 0 {
		return c.NoContent(http.StatusUnauthorized)
	}
	if !viewer.HasScope(an_constants.OAuthScopeActorsWrite) {
		return c.NoContent(http.StatusForbidden)
	}

	req := &ChannelFeedRequest{}
	if err := c.Bind(req); err != nil || c.Validate(req) != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if _, status := h.channelLookedAfter(req.ChannelID, viewer); status != 0 {
		return c.NoContent(status)
	}

	ctx := c.Request().Context()
	if _, err := h.Srv.Channels.Of(ctx, req.ChannelID); errors.Is(err, repositories.ErrNotFound) {
		return c.NoContent(http.StatusNotFound)
	} else if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if err := h.Srv.Channels.Remove(ctx, req.ChannelID); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.NoContent(http.StatusNoContent)
}

// channelLookedAfter is a channel the viewer is one of the admins of, or
// any channel for a site administrator. Anybody else is refused, and a
// channel that is not there, or not one, is not found.
func (h *connectHandler) channelLookedAfter(id int64, viewer *entities.Viewer) (*entities.AnahitaNode, int) {
	if viewer == nil || !viewer.Authenticated || viewer.ID <= 0 {
		return nil, http.StatusUnauthorized
	}
	channel, err := h.Srv.ActorsGRPC.ReadOfType(id, an_constants.NodeActorChannelEntityType, viewer)
	if err != nil || channel == nil || channel.ID == 0 {
		return nil, http.StatusNotFound
	}
	if viewer.IsAdmin() {
		return channel, 0
	}
	admins, err := h.Srv.ActorAdminsGRPC.Browse(channel.ID)
	if err != nil {
		log.Println(err)
		return nil, http.StatusInternalServerError
	}
	for _, admin := range admins {
		if admin != nil && admin.ID == viewer.ID {
			return channel, 0
		}
	}
	return nil, http.StatusForbidden
}
