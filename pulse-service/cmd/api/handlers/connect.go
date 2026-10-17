package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"pulse/data/repositories"
	"pulse/feeds"

	an_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/domains"
	"anahita-libs/ratelimit"

	"github.com/labstack/echo/v4"
)

// Connecting a website: "Connect your website", never "create". Three
// calls: the site's address, to find its feed and be given a token; a check
// that the token is where it should be; and the Pulse made, with a name and
// a handle. See docs/plans/pulse.md.
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
	refusedHandleTaken = "handle_taken"
	refusedNotProved   = "not_proved"
)

// Finding a feed fetches somebody else's site: a few times an hour each.
var connectLimit = ratelimit.Limit{Max: 20, Window: time.Hour}

// ConnectStartRequest is a website to connect.
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

// ConnectMakeRequest is the Pulse, once the domain is proved.
type ConnectMakeRequest struct {
	// The connection's id.
	ID int64 `param:"id" validate:"required,min=1"`
	// The feed, as starting the connection found it.
	FeedURL string `json:"feed_url" validate:"required,max=2000"`
	// Its name, shown as a profile's is.
	Name string `json:"name" validate:"required,max=64"`
	// Its handle: `@example`. Letters, digits, `-` and `_`.
	Alias string `json:"alias" validate:"required,username,min=3,max=30"`
	// What it is, shown in its About card.
	Body string `json:"body" validate:"omitempty,max=500"`
}

// Connection is a website being connected: its feed, and the token with
// both places it may be put.
type Connection struct {
	// The connection's id, for checking the proof and making the Pulse.
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
	// A name and a handle to begin with: the feed's title, and the domain's
	// first word.
	SuggestedName  string `json:"suggested_name"`
	SuggestedAlias string `json:"suggested_alias"`
}

// Checked says whether the token was found.
type Checked struct {
	Proved bool `json:"proved"`
	// `dns` or `file`: where it was found.
	Method string `json:"method,omitempty"`
}

// Made is the Pulse as it was made: where to go to see it.
type Made struct {
	ID    int64  `json:"id"`
	Alias string `json:"alias"`
}

type refusal struct {
	Error string `json:"error"`
}

// Start finds a website's feed and gives a token to prove its domain with.
//
// POST /pulses/_connect
func (h *connectHandler) Start(c echo.Context) error {
	viewer := getViewer(c)
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

	if ratelimit.Exceeded(h.Limiter, "pulse_connect", fmt.Sprint(viewer.ID), connectLimit) {
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
	case errors.Is(err, feeds.ErrNoFeed):
		return c.JSON(http.StatusBadRequest, refusal{refusedNoFeed})
	case err != nil:
		return c.JSON(http.StatusBadRequest, refusal{refusedUnreadable})
	}
	if !site.Covers(found.FeedURL) {
		return c.JSON(http.StatusBadRequest, refusal{refusedFeedOffSite})
	}
	if taken, err := h.Srv.Pulses.FeedTaken(ctx, found.FeedURL); err != nil {
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
	name := found.Title
	if name == "" {
		name = site.Domain
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
		SuggestedName:   clipRunes(name, 64),
		SuggestedAlias:  SuggestedAlias(site.Domain),
	})
}

// Check looks for the token in both places.
//
// POST /pulses/_connect/:id/check
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

// Make makes the Pulse, once its domain is proved: whoever proved it is its
// first administrator and its first follower, and what the feed holds now
// is marked seen, not posted.
//
// POST /pulses/_connect/:id
func (h *connectHandler) Make(c echo.Context) error {
	viewer := getViewer(c)
	if status := h.mayConnect(viewer); status != 0 {
		return c.NoContent(status)
	}

	req := &ConnectMakeRequest{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Alias = strings.TrimSpace(req.Alias)
	if err := c.Validate(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	ctx := c.Request().Context()
	domain, status := h.connectionOf(c, req.ID, viewer)
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
	read, err := h.Feeds.Read(ctx, req.FeedURL, "", "")
	switch {
	case errors.Is(err, feeds.ErrNoFeed):
		return c.JSON(http.StatusBadRequest, refusal{refusedNoFeed})
	case err != nil:
		return c.JSON(http.StatusBadRequest, refusal{refusedUnreadable})
	}
	if taken, err := h.Srv.Pulses.FeedTaken(ctx, req.FeedURL); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	} else if taken {
		return c.JSON(http.StatusConflict, refusal{refusedFeedTaken})
	}

	pulse := entities.NewAnahitaNode()
	pulse.Type = an_constants.NodeActorPulseEntityType
	pulse.Service = an_constants.ActorService
	pulse.Name = req.Name
	pulse.Alias = req.Alias
	pulse.Body = req.Body
	pulse.WebsiteURL = "https://" + domain.Host + "/"
	pulse.Access = an_constants.AnahitaAccessPublic
	pulse.Enabled = true
	pulse.CreatedBy = viewer.ID
	pulse.ModifiedBy = viewer.ID

	pulse, err = h.Srv.ActorsGRPC.Add(pulse, viewer)
	if isDuplicateHandle(err) {
		return c.JSON(http.StatusConflict, refusal{refusedHandleTaken})
	}
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if status := h.lookAfter(pulse, viewer); status != 0 {
		return c.NoContent(status)
	}

	err = h.Srv.Pulses.Add(ctx, &repositories.Pulse{
		NodeID:   pulse.ID,
		FeedURL:  req.FeedURL,
		SiteURL:  pulse.WebsiteURL,
		DomainID: domain.ID,
	})
	if errors.Is(err, repositories.ErrFeedTaken) {
		// Connected by somebody else a moment ago. The profile made here
		// stays, with nothing to read, for its administrator to delete.
		return c.JSON(http.StatusConflict, refusal{refusedFeedTaken})
	}
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Its newest few are posted, so that a new Pulse does not open empty;
	// the rest are marked seen, so that its first followers are not sent
	// the site's whole past (the user, 2026-10-16).
	waiting, seen := firstPosts(read.Items)
	if err := h.Srv.Items.AddWaiting(ctx, pulse.ID, waiting); err != nil {
		log.Println(err)
	}
	if err := h.Srv.Items.MarkSeen(ctx, pulse.ID, seen); err != nil {
		log.Println(err)
	}
	if err := h.Srv.Domains.ForProfile(ctx, domain.ID, pulse.ID); err != nil {
		log.Println(err)
	}

	return c.JSON(http.StatusCreated, Made{ID: pulse.ID, Alias: pulse.Alias})
}

// FirstPosts is how many of a feed's items a new Pulse posts: its newest.
const FirstPosts = 10

// firstPosts splits a feed as a new Pulse takes it: its newest FirstPosts
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

// lookAfter makes the viewer the Pulse's first administrator and its first
// follower, as a group's maker is.
func (h *connectHandler) lookAfter(pulse *entities.AnahitaNode, viewer *entities.Viewer) int {
	admin, err := h.Srv.ActorsGRPC.ReadOfType(viewer.ID, an_constants.NodeActorPersonEntityType, viewer)
	if err != nil {
		log.Println(err)
		return http.StatusInternalServerError
	}
	if _, err := h.Srv.ActorAdminsGRPC.Add(pulse, admin, viewer); err != nil {
		log.Println(err)
		return http.StatusInternalServerError
	}

	follow := services.NewSocialgraphFollowRequest().Add
	follow.Source = admin
	follow.Target = pulse
	follow.Viewer = viewer
	if err := h.Srv.SocialgraphFollowGRPC.Add(follow); err != nil {
		log.Println(err)
		return http.StatusInternalServerError
	}
	return 0
}

// mayConnect is whether the viewer may connect a website at all: signed in,
// allowed to write profiles, and of the role PULSES_FROM names.
func (h *connectHandler) mayConnect(viewer *entities.Viewer) int {
	if viewer == nil || !viewer.Authenticated || viewer.ID <= 0 {
		return http.StatusUnauthorized
	}
	if !viewer.HasScope(an_constants.OAuthScopeActorsWrite) || !h.Config.MayConnect(viewer.PersonType) {
		return http.StatusForbidden
	}
	return 0
}

// connectionOf is a connection the viewer began, not yet made into a Pulse.
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

// isDuplicateHandle says the graph refused a profile because its handle is
// taken. gRPC flattens the driver's error to a string, so this matches the
// unique index on handles by name, as auth-service's signup does.
func isDuplicateHandle(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "uq_unique_alias") || strings.Contains(msg, "duplicate entry")
}

var notHandle = regexp.MustCompile(`[^a-z0-9_-]+`)

// SuggestedAlias is a handle to begin with: the domain's first part, as a
// handle may be written (`@example` for example.com), or the whole domain
// when that part is shorter than a handle may be (`@ab-org` for ab.org).
func SuggestedAlias(domain string) string {
	first := strings.SplitN(domain, ".", 2)[0]
	alias := strings.Trim(notHandle.ReplaceAllString(strings.ToLower(first), "-"), "-")
	if len(alias) < 3 {
		alias = strings.Trim(notHandle.ReplaceAllString(strings.ToLower(domain), "-"), "-")
	}
	if len(alias) > 30 {
		alias = strings.Trim(alias[:30], "-")
	}
	return alias
}

func clipRunes(text string, n int) string {
	runes := []rune(text)
	if len(runes) <= n {
		return text
	}
	return strings.TrimSpace(string(runes[:n]))
}
