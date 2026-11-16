package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"syndication/config"
	"syndication/data/repositories"
	"syndication/data/services"
	"syndication/feeds"

	an_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	an_services "anahita-libs/data/services"
	"anahita-libs/domains"
	"anahita-libs/validators"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

type fakeFeeds struct {
	found *feeds.Found
	err   error
	items []feeds.Item
}

func (f *fakeFeeds) Find(context.Context, string) (*feeds.Found, error) { return f.found, f.err }
func (f *fakeFeeds) Read(context.Context, string, string, string) (*feeds.Read, error) {
	return &feeds.Read{Items: f.items}, f.err
}

type fakeChecker struct{ method string }

func (f *fakeChecker) Check(context.Context, domains.Site, string) (string, error) {
	if f.method == "" {
		return "", domains.ErrNotFound
	}
	return f.method, nil
}

type fakeDomains struct {
	repositories.DomainsInterface
	rows map[int64]*repositories.Domain
}

func (f *fakeDomains) Add(_ context.Context, d *repositories.Domain) (int64, error) {
	id := int64(len(f.rows) + 1)
	d.ID = id
	f.rows[id] = d
	return id, nil
}
func (f *fakeDomains) Read(_ context.Context, id int64) (*repositories.Domain, error) {
	if d, ok := f.rows[id]; ok {
		return d, nil
	}
	return nil, repositories.ErrNotFound
}
func (f *fakeDomains) Proved(_ context.Context, id int64, method string) error {
	now := time.Now()
	f.rows[id].VerifiedAt, f.rows[id].Method = &now, method
	return nil
}
func (f *fakeDomains) ForProfile(_ context.Context, id, profileID int64) error {
	f.rows[id].ProfileID = profileID
	return nil
}

type fakeChannels struct {
	repositories.ChannelsInterface
	added   []*repositories.Channel
	removed []int64
}

func (f *fakeChannels) Of(_ context.Context, nodeID int64) (*repositories.Shown, error) {
	for _, p := range f.added {
		if p.NodeID == nodeID {
			return &repositories.Shown{FeedURL: p.FeedURL, SiteURL: p.SiteURL, State: repositories.StateActive}, nil
		}
	}
	return nil, repositories.ErrNotFound
}
func (f *fakeChannels) Remove(_ context.Context, nodeID int64) error {
	kept := f.added[:0]
	for _, p := range f.added {
		if p.NodeID != nodeID {
			kept = append(kept, p)
		}
	}
	f.added = kept
	f.removed = append(f.removed, nodeID)
	return nil
}

func (f *fakeChannels) Add(_ context.Context, p *repositories.Channel) error {
	f.added = append(f.added, p)
	return nil
}
func (f *fakeChannels) FeedTaken(_ context.Context, feedURL string) (bool, error) {
	for _, p := range f.added {
		if p.FeedURL == feedURL {
			return true, nil
		}
	}
	return false, nil
}

type fakeItems struct {
	repositories.ItemsInterface
	seen    []repositories.ItemSeenAs
	waiting []repositories.ItemSeenAs
}

func (f *fakeItems) AddWaiting(_ context.Context, _ int64, items []repositories.ItemSeenAs) error {
	f.waiting = append(f.waiting, items...)
	return nil
}

func (f *fakeItems) MarkSeen(_ context.Context, _ int64, items []repositories.ItemSeenAs) error {
	f.seen = append(f.seen, items...)
	return nil
}

// The graph: only what connecting asks of it. Channel 900 is there, its
// admin person 7.
type fakeActors struct {
	an_services.ActorsGRPCInterface
	edited *entities.AnahitaNode
}

func (f *fakeActors) ReadOfType(id int64, nodeType string, _ *entities.Viewer) (*entities.AnahitaNode, error) {
	if nodeType == an_constants.NodeActorChannelEntityType && id != 900 {
		return nil, errors.New("not found")
	}
	node := entities.NewAnahitaNode()
	node.ID, node.Type = id, nodeType
	return node, nil
}
func (f *fakeActors) Edit(node *entities.AnahitaNode, _ *entities.Viewer) (*entities.AnahitaNode, error) {
	f.edited = node
	return node, nil
}

type fakeAdmins struct {
	an_services.ActorAdminsGRPCInterface
}

func (f *fakeAdmins) Browse(int64) ([]*entities.AnahitaNode, error) {
	admin := entities.NewAnahitaNode()
	admin.ID = 7
	return []*entities.AnahitaNode{admin}, nil
}

type fakeFollows struct {
	an_services.SocialgraphFollowGRPCInterface
	added int
}

func (f *fakeFollows) Add(*an_services.SocialgraphFollowAddRequest) error {
	f.added++
	return nil
}

type counter struct{}

func (counter) Incr(string, time.Duration) (int64, error) { return 1, nil }
func (counter) Del(...string) error                       { return nil }

type fixture struct {
	h        *connectHandler
	feeds    *fakeFeeds
	checker  *fakeChecker
	domains  *fakeDomains
	channels *fakeChannels
	items    *fakeItems
	actors   *fakeActors
	admins   *fakeAdmins
	follows  *fakeFollows
	e        *echo.Echo
}

func newFixture() *fixture {
	f := &fixture{
		feeds: &fakeFeeds{found: &feeds.Found{FeedURL: "https://example.com/feed.xml", Title: "Garden notes", SiteURL: "https://example.com/"},
			items: []feeds.Item{{Key: "k1", Title: "Beans"}, {Key: "k2", Title: "Peas"}}},
		checker:  &fakeChecker{},
		domains:  &fakeDomains{rows: map[int64]*repositories.Domain{}},
		channels: &fakeChannels{},
		items:    &fakeItems{},
		actors:   &fakeActors{},
		admins:   &fakeAdmins{},
		follows:  &fakeFollows{},
		e:        echo.New(),
	}
	f.e.Validator = validators.NewRequestValidator(validator.New())
	f.h = &connectHandler{&Env{
		Config: &config.Config{},
		Srv: &services.Services{
			ActorsGRPC: f.actors, ActorAdminsGRPC: f.admins, SocialgraphFollowGRPC: f.follows,
			Domains: f.domains, Channels: f.channels, Items: f.items,
		},
		Feeds:   f.feeds,
		Domains: f.checker,
		Limiter: counter{},
	}}
	return f
}

func admin(id int64) *entities.Viewer {
	v := entities.NewViewer()
	v.ID, v.Authenticated = id, true
	v.PersonType = an_constants.PersonTypeAdmin
	v.Scopes = []string{an_constants.OAuthScopeActorsWrite}
	return v
}

func (f *fixture) call(handler echo.HandlerFunc, viewer *entities.Viewer, id, body string) *httptest.ResponseRecorder {
	return f.callAt(handler, viewer, "id", id, body)
}

// callAt is call with the path's parameter named: channel_id for a
// channel's feed.
func (f *fixture) callAt(handler echo.HandlerFunc, viewer *entities.Viewer, param, id, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := f.e.NewContext(req, rec)
	if id != "" {
		c.SetParamNames(param)
		c.SetParamValues(id)
	}
	c.Set(an_constants.ViewerContextKey, viewer)
	_ = handler(c)
	return rec
}

func refusedWith(t *testing.T, rec *httptest.ResponseRecorder, status int, reason string) {
	t.Helper()
	var r refusal
	_ = json.Unmarshal(rec.Body.Bytes(), &r)
	if rec.Code != status || r.Error != reason {
		t.Errorf("answered %d %q, want %d %q", rec.Code, r.Error, status, reason)
	}
}

func TestAChannelsFeedIsConnectedInThreeSteps(t *testing.T) {
	f := newFixture()
	ana := admin(7)

	rec := f.call(f.h.Start, ana, "", `{"site_url": "www.example.com"}`)
	var conn Connection
	_ = json.Unmarshal(rec.Body.Bytes(), &conn)
	if rec.Code != http.StatusCreated || conn.Domain != "example.com" || conn.RecordName != "_anahita.example.com" ||
		conn.FileURL != "https://www.example.com/.well-known/anahita-verification.txt" || !strings.HasPrefix(conn.Token, "anahita-") {
		t.Fatalf("started: %d %+v", rec.Code, conn)
	}

	// Connected before the token is anywhere: refused.
	connect := `{"connection_id": 1, "feed_url": "https://example.com/feed.xml"}`
	refusedWith(t, f.callAt(f.h.ConnectFeed, ana, "channel_id", "900", connect), http.StatusConflict, refusedNotProved)

	rec = f.call(f.h.Check, ana, "1", "")
	if !strings.Contains(rec.Body.String(), `"proved":false`) {
		t.Errorf("checked with no token: %s", rec.Body.String())
	}
	f.checker.method = domains.MethodFile
	rec = f.call(f.h.Check, ana, "1", "")
	if !strings.Contains(rec.Body.String(), `"proved":true,"method":"file"`) {
		t.Errorf("checked: %s", rec.Body.String())
	}

	rec = f.callAt(f.h.ConnectFeed, ana, "channel_id", "900", connect)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"state":"active"`) {
		t.Fatalf("connected: %d %s", rec.Code, rec.Body.String())
	}
	if len(f.channels.added) != 1 || f.channels.added[0].NodeID != 900 || len(f.items.waiting) != 2 || len(f.items.seen) != 0 || f.domains.rows[1].ProfileID != 900 {
		t.Errorf("kept: %+v, %d to post, %d seen, domain for %d", f.channels.added, len(f.items.waiting), len(f.items.seen), f.domains.rows[1].ProfileID)
	}
	if f.actors.edited == nil || f.actors.edited.WebsiteURL != "https://www.example.com/" {
		t.Errorf("the site is not its website: %+v", f.actors.edited)
	}

	// One feed a channel.
	refusedWith(t, f.callAt(f.h.ConnectFeed, ana, "channel_id", "900", connect), http.StatusConflict, refusedHasFeed)

	rec = f.callAt(f.h.ShowFeed, ana, "channel_id", "900", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"feed_url":"https://example.com/feed.xml"`) {
		t.Errorf("shown: %d %s", rec.Code, rec.Body.String())
	}

	if rec := f.callAt(f.h.DisconnectFeed, ana, "channel_id", "900", ""); rec.Code != http.StatusNoContent || len(f.channels.removed) != 1 {
		t.Errorf("disconnected: %d, %v", rec.Code, f.channels.removed)
	}
	if rec := f.callAt(f.h.ShowFeed, ana, "channel_id", "900", ""); rec.Code != http.StatusNotFound {
		t.Errorf("shown once disconnected: %d", rec.Code)
	}
}

// Only a channel's admins see, connect and disconnect its feed; a site
// administrator may too.
func TestOnlyAChannelsAdminsLookAfterItsFeed(t *testing.T) {
	f := newFixture()
	f.h.Config.ChannelsFrom = an_constants.RoleLevelRegistered
	f.channels.added = []*repositories.Channel{{NodeID: 900, FeedURL: "https://example.com/feed.xml"}}

	other := admin(8)
	other.PersonType = an_constants.PersonTypeRegistered
	if rec := f.callAt(f.h.ShowFeed, other, "channel_id", "900", ""); rec.Code != http.StatusForbidden {
		t.Errorf("somebody else sees it: %d", rec.Code)
	}
	if rec := f.callAt(f.h.DisconnectFeed, other, "channel_id", "900", ""); rec.Code != http.StatusForbidden {
		t.Errorf("somebody else disconnects it: %d", rec.Code)
	}
	if rec := f.callAt(f.h.ShowFeed, admin(9), "channel_id", "900", ""); rec.Code != http.StatusOK {
		t.Errorf("a site administrator: %d", rec.Code)
	}
	if rec := f.callAt(f.h.ShowFeed, admin(7), "channel_id", "901", ""); rec.Code != http.StatusNotFound {
		t.Errorf("no such channel: %d", rec.Code)
	}
}

func TestWhatIsRefusedOnTheWay(t *testing.T) {
	f := newFixture()
	ana := admin(7)

	refusedWith(t, f.call(f.h.Start, ana, "", `{"site_url": "localhost"}`), http.StatusBadRequest, refusedNotADomain)

	f.feeds.found = &feeds.Found{FeedURL: "https://feeds.elsewhere.example/x"}
	refusedWith(t, f.call(f.h.Start, ana, "", `{"site_url": "example.com"}`), http.StatusBadRequest, refusedFeedOffSite)

	f.feeds.found, f.feeds.err = nil, feeds.ErrNoFeed
	refusedWith(t, f.call(f.h.Start, ana, "", `{"site_url": "example.com"}`), http.StatusBadRequest, refusedNoFeed)

	f.feeds.found, f.feeds.err = nil, feeds.ErrPrivate
	refusedWith(t, f.call(f.h.Start, ana, "", `{"site_url": "example.com"}`), http.StatusBadRequest, refusedPrivateFeed)

	f.feeds.err = nil
	f.feeds.found = &feeds.Found{FeedURL: "https://example.com/feed.xml?token=abc"}
	refusedWith(t, f.call(f.h.Start, ana, "", `{"site_url": "example.com"}`), http.StatusBadRequest, refusedPrivateFeed)

	f.feeds.found = &feeds.Found{FeedURL: "https://example.com/feed.xml"}
	f.channels.added = []*repositories.Channel{{FeedURL: "https://example.com/feed.xml"}}
	refusedWith(t, f.call(f.h.Start, ana, "", `{"site_url": "example.com"}`), http.StatusConflict, refusedFeedTaken)
}

func TestOnlyWhoChannelsFromNamesMayConnect(t *testing.T) {
	f := newFixture()

	member := admin(8)
	member.PersonType = an_constants.PersonTypeRegistered
	if rec := f.call(f.h.Start, member, "", `{"site_url": "example.com"}`); rec.Code != http.StatusForbidden {
		t.Errorf("a member, with CHANNELS_FROM unset: %d", rec.Code)
	}

	f.h.Config.ChannelsFrom = an_constants.RoleLevelRegistered
	if rec := f.call(f.h.Start, member, "", `{"site_url": "example.com"}`); rec.Code != http.StatusCreated {
		t.Errorf("a member, with CHANNELS_FROM=registered: %d", rec.Code)
	}

	// Somebody else's connection is not theirs to check.
	if rec := f.call(f.h.Check, admin(9), "1", ""); rec.Code != http.StatusNotFound {
		t.Errorf("another's connection: %d", rec.Code)
	}

	if rec := f.call(f.h.Start, entities.NewViewer(), "", `{"site_url": "example.com"}`); rec.Code != http.StatusUnauthorized {
		t.Errorf("a visitor: %d", rec.Code)
	}
}

func TestANewChannelPostsItsNewestTen(t *testing.T) {
	var items []feeds.Item
	for n := 1; n <= 13; n++ {
		items = append(items, feeds.Item{Key: fmt.Sprintf("k%d", n), Title: "t", Published: time.Date(2026, 1, n, 0, 0, 0, 0, time.UTC)})
	}
	waiting, seen := firstPosts(items)
	if len(waiting) != FirstPosts || len(seen) != 3 || waiting[0].Key != "k13" || waiting[9].Key != "k4" || seen[0].Key != "k3" {
		t.Errorf("to post %v, seen %v", waiting, seen)
	}
}
