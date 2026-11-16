package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"syndication/data/repositories"
	"syndication/feeds"

	an_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/domains"
	an_handlers "anahita-libs/handlers"
	"anahita-libs/ratelimit"

	"github.com/labstack/echo/v4"
)

// Connecting a channel's feed (roadmap row 147, phase 4): the channel is
// made first, as a group is, and its admins connect a website's feed to it
// in its settings. Three calls: the site's address, to find its feed and be
// given a token; a check that the token is where it should be; and the feed
// connected to the channel (channel_feed.go). See docs/plans/channels.md.
type connectHandler struct {
	*Env
}

// Why a connection is refused, said in the answer as `{"error": "…"}`.
const (
	refusedNotADomain  = "not_a_domain"
	refusedNoFeed      = "no_feed"
	refusedUnreadable  = "unreadable"
	refusedFeedOffSite = "feed_not_on_domain"
	refusedFeedTaken   = "feed_taken"
	refusedNotProved   = "not_proved"
	refusedPrivateFeed = "private_feed"
	// The channel reads a feed already: one a channel.
	refusedHasFeed = "has_feed"
)

// Finding a feed fetches somebody else's site: a few times an hour each.
var connectLimit = ratelimit.Limit{Max: 20, Window: time.Hour}

// ConnectStartRequest is a channel to connect.
type ConnectStartRequest struct {
	// The site's address, `example.com` or `https://example.com/blog/`, or
	// its feed's.
	SiteURL string `json:"site_url" validate:"required,max=2000"`
}

// ConnectIDRequest names a connection begun by the viewer.
type ConnectIDRequest struct {
	// The connection's id, as starting it answered.
	ID int64 `param:"id" validate:"required,min=1"`
}

// Connection is a channel being connected: its feed, and the token with
// both places it may be put.
type Connection struct {
	// The connection's id, for checking the proof and making the channel.
	ID int64 `json:"id"`
	// The domain to prove, and the host the file is read from.
	Domain string `json:"domain"`
	Host   string `json:"host"`
	// The site and its feed, as found.
	SiteURL         string `json:"site_url"`
	FeedURL         string `json:"feed_url"`
	FeedTitle       string `json:"feed_title,omitempty"`
	FeedDescription string `json:"feed_description,omitempty"`
	// What to put in either place.
	Token string `json:"token"`
	// A TXT record of this name, holding the token.
	RecordName string `json:"record_name"`
	// Or a file at this address, holding the token.
	FileURL string `json:"file_url"`
}

// Checked says whether the token was found.
type Checked struct {
	Proved bool `json:"proved"`
	// `dns` or `file`: where it was found.
	Method string `json:"method,omitempty"`
}

type refusal struct {
	Error string `json:"error"`
}

// Start finds a channel's feed and gives a token to prove its domain with.
//
// POST /channels/_connect
func (h *connectHandler) Start(c echo.Context) error {
	viewer := getViewer(c)
	// Not where the instance does not offer channels (instance.json,
	// roadmap row 139).
	if !an_handlers.ProfileOffered(h.Srv.GraphGRPC, "channel") {
		return an_handlers.RefuseOff(c, "channel")
	}
	if status := h.mayConnect(viewer); status != 0 {
		return c.NoContent(status)
	}

	req := &ConnectStartRequest{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	if ratelimit.Exceeded(h.Limiter, "channel_connect", fmt.Sprint(viewer.ID), connectLimit) {
		return c.NoContent(http.StatusTooManyRequests)
	}

	site, err := domains.SiteOf(req.SiteURL)
	if err != nil {
		return c.JSON(http.StatusBadRequest, refusal{refusedNotADomain})
	}

	address := strings.TrimSpace(req.SiteURL)
	if !strings.Contains(address, "://") {
		address = "https://" + address
	}
	ctx := c.Request().Context()
	found, err := h.Feeds.Find(ctx, address)
	switch {
	case errors.Is(err, feeds.ErrPrivate) || (err == nil && feeds.IsPrivate(found.FeedURL)):
		return c.JSON(http.StatusBadRequest, refusal{refusedPrivateFeed})
	case errors.Is(err, feeds.ErrNoFeed):
		return c.JSON(http.StatusBadRequest, refusal{refusedNoFeed})
	case err != nil:
		return c.JSON(http.StatusBadRequest, refusal{refusedUnreadable})
	}
	if !site.Covers(found.FeedURL) {
		return c.JSON(http.StatusBadRequest, refusal{refusedFeedOffSite})
	}
	if taken, err := h.Srv.Channels.FeedTaken(ctx, found.FeedURL); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	} else if taken {
		return c.JSON(http.StatusConflict, refusal{refusedFeedTaken})
	}

	token, err := domains.NewToken()
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	id, err := h.Srv.Domains.Add(ctx, &repositories.Domain{
		Domain:   site.Domain,
		Host:     site.Host,
		Token:    token,
		PersonID: viewer.ID,
	})
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	siteURL := found.SiteURL
	if siteURL == "" || !site.Covers(siteURL) {
		siteURL = "https://" + site.Host + "/"
	}

	return c.JSON(http.StatusCreated, Connection{
		ID:              id,
		Domain:          site.Domain,
		Host:            site.Host,
		SiteURL:         siteURL,
		FeedURL:         found.FeedURL,
		FeedTitle:       found.Title,
		FeedDescription: found.Description,
		Token:           token,
		RecordName:      site.RecordName(),
		FileURL:         site.FileURL(),
	})
}

// Check looks for the token in both places.
//
// POST /channels/_connect/:id/check
func (h *connectHandler) Check(c echo.Context) error {
	viewer := getViewer(c)
	if status := h.mayConnect(viewer); status != 0 {
		return c.NoContent(status)
	}

	req := &ConnectIDRequest{}
	if err := c.Bind(req); err != nil || c.Validate(req) != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	ctx := c.Request().Context()
	domain, status := h.connectionOf(c, req.ID, viewer)
	if status != 0 {
		return c.NoContent(status)
	}

	method, err := h.Domains.Check(ctx, domains.Site{Host: domain.Host, Domain: domain.Domain}, domain.Token)
	if errors.Is(err, domains.ErrNotFound) {
		return c.JSON(http.StatusOK, Checked{Proved: false})
	}
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if err := h.Srv.Domains.Proved(ctx, domain.ID, method); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.JSON(http.StatusOK, Checked{Proved: true, Method: method})
}

// FirstPosts is how many of a feed's items a new channel posts: its newest.
const FirstPosts = 10

// firstPosts splits a feed as a new channel takes it: its newest FirstPosts
// items to post, and the rest seen.
func firstPosts(items []feeds.Item) (waiting, seen []repositories.ItemSeenAs) {
	newest := append([]feeds.Item(nil), items...)
	sort.SliceStable(newest, func(i, j int) bool { return newest[i].Published.After(newest[j].Published) })
	for i, item := range newest {
		kept := repositories.ItemSeenAs{Key: item.Key, ContentHash: repositories.ContentHash(item.Title, item.Excerpt)}
		if i < FirstPosts {
			waiting = append(waiting, kept)
		} else {
			seen = append(seen, kept)
		}
	}
	return waiting, seen
}

// mayConnect is whether the viewer may connect a channel at all: signed in,
// allowed to write profiles, and of the role CHANNELS_FROM names.
func (h *connectHandler) mayConnect(viewer *entities.Viewer) int {
	if viewer == nil || !viewer.Authenticated || viewer.ID <= 0 {
		return http.StatusUnauthorized
	}
	if !viewer.HasScope(an_constants.OAuthScopeActorsWrite) || !h.Config.MayConnect(viewer.PersonType) {
		return http.StatusForbidden
	}
	return 0
}

// connectionOf is a connection the viewer began, not yet made into a channel.
// Anybody else's is not there, as far as they are told.
func (h *connectHandler) connectionOf(c echo.Context, id int64, viewer *entities.Viewer) (*repositories.Domain, int) {
	domain, err := h.Srv.Domains.Read(c.Request().Context(), id)
	if errors.Is(err, repositories.ErrNotFound) {
		return nil, http.StatusNotFound
	}
	if err != nil {
		log.Println(err)
		return nil, http.StatusInternalServerError
	}
	if domain.PersonID != viewer.ID || domain.ProfileID != 0 {
		return nil, http.StatusNotFound
	}
	return domain, 0
}
