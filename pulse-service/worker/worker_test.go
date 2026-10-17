package worker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"pulse/data/repositories"
	"pulse/feeds"

	an_const "anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/domains"
	"anahita-libs/linkpreviews"
)

var start = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

type memPulses struct {
	repositories.PulsesInterface
	due      *repositories.Due
	read     *repositories.Read
	failures int
	nextAt   time.Time
	paused   string
	resumed  []int64
	pausedOn []int64
}

func (m *memPulses) Claim(context.Context) (*repositories.Due, error) {
	due := m.due
	m.due = nil
	return due, nil
}
func (m *memPulses) Read(_ context.Context, _ int64, r repositories.Read) error {
	m.read = &r
	return nil
}
func (m *memPulses) Failed(_ context.Context, _ int64, failures int, next time.Time, _ string) error {
	m.failures, m.nextAt = failures, next
	return nil
}
func (m *memPulses) Pause(_ context.Context, _ int64, why string) error {
	m.paused = why
	return nil
}
func (m *memPulses) PauseForProof(_ context.Context, id int64) error {
	m.pausedOn = append(m.pausedOn, id)
	return nil
}
func (m *memPulses) ResumeAfterProof(_ context.Context, id int64) error {
	m.resumed = append(m.resumed, id)
	return nil
}

type memItems struct {
	repositories.ItemsInterface
	kept map[string]repositories.Known
}

func (m *memItems) Known(_ context.Context, _ int64, keys []string) (map[string]repositories.Known, error) {
	known := map[string]repositories.Known{}
	for _, key := range keys {
		if item, ok := m.kept[key]; ok {
			known[key] = item
		}
	}
	return known, nil
}
func (m *memItems) AddWaiting(_ context.Context, _ int64, items []repositories.ItemSeenAs) error {
	for _, item := range items {
		if _, ok := m.kept[item.Key]; !ok {
			m.kept[item.Key] = repositories.Known{State: repositories.ItemWaiting, ContentHash: item.ContentHash}
		}
	}
	return nil
}
func (m *memItems) Posted(_ context.Context, _ int64, key string, nodeID int64) error {
	item := m.kept[key]
	item.State, item.NodeID = repositories.ItemPosted, nodeID
	m.kept[key] = item
	return nil
}
func (m *memItems) Changed(_ context.Context, _ int64, key, hash string) error {
	item := m.kept[key]
	item.ContentHash = hash
	m.kept[key] = item
	return nil
}

type memFeed struct {
	items []feeds.Item
	err   error
}

func (m *memFeed) Read(context.Context, string, string, string) (*feeds.Read, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &feeds.Read{ETag: `"v2"`, Items: m.items}, nil
}

type memGraph struct {
	pulse  *entities.AnahitaNode
	posted []*entities.AnahitaNode
	edited []*entities.AnahitaNode
	poster int64
}

func (m *memGraph) Pulse(int64) (*entities.AnahitaNode, error) { return m.pulse, nil }
func (m *memGraph) FirstAdmin(int64) (int64, error)            { return 7, nil }
func (m *memGraph) Post(a *entities.AnahitaNode, v *entities.Viewer) (*entities.AnahitaNode, error) {
	a.ID = int64(1000 + len(m.posted))
	m.posted = append(m.posted, a)
	m.poster = v.ID
	return a, nil
}
func (m *memGraph) Read(id int64, _ *entities.Viewer) (*entities.AnahitaNode, error) {
	for _, a := range m.posted {
		if a.ID == id {
			copied := *a
			return &copied, nil
		}
	}
	return nil, errors.New("not found")
}
func (m *memGraph) Edit(a *entities.AnahitaNode, _ *entities.Viewer) (*entities.AnahitaNode, error) {
	m.edited = append(m.edited, a)
	return a, nil
}

type memCards struct{}

func (memCards) Preview(_ context.Context, address string) *linkpreviews.Preview {
	return &linkpreviews.Preview{URL: address, Title: "The original", Status: linkpreviews.StatusOK}
}

type fixture struct {
	w      *Worker
	pulses *memPulses
	items  *memItems
	feed   *memFeed
	graph  *memGraph
	clock  time.Time
}

func newFixture() *fixture {
	pulse := entities.NewAnahitaNode()
	pulse.ID, pulse.Type, pulse.Enabled = 900, an_const.NodeActorPulseEntityType, true
	f := &fixture{
		pulses: &memPulses{},
		items:  &memItems{kept: map[string]repositories.Known{}},
		feed:   &memFeed{},
		graph:  &memGraph{pulse: pulse},
		clock:  start,
	}
	f.w = &Worker{
		Pulses: f.pulses, Items: f.items, Feeds: f.feed, Graph: f.graph, Cards: memCards{},
		Now: func() time.Time { return f.clock },
	}
	return f
}

// round reads the feed once, as it would be when due, carrying over what
// the last round kept of the hour.
func (f *fixture) round(t *testing.T) {
	t.Helper()
	due := &repositories.Due{NodeID: 900, FeedURL: "https://example.com/feed.xml"}
	if f.pulses.read != nil {
		due.HourPosts, due.HourBeganAt = f.pulses.read.HourPosts, f.pulses.read.HourBeganAt
	}
	f.pulses.due = due
	if did, err := f.w.Once(context.Background()); !did || err != nil {
		t.Fatalf("a round: %v, %v", did, err)
	}
}

func item(n int) feeds.Item {
	return feeds.Item{
		Key:       fmt.Sprintf("key-%d", n),
		Title:     fmt.Sprintf("Post %d", n),
		Excerpt:   fmt.Sprintf("What post %d says.", n),
		Link:      fmt.Sprintf("https://example.com/%d", n),
		Tags:      []string{"Gardens", "OpenScience"},
		Published: start.Add(time.Duration(-n) * time.Hour),
	}
}

func TestANewItemIsPostedAsTheArticleOfThePulse(t *testing.T) {
	f := newFixture()
	f.items.kept["key-1"] = repositories.Known{State: repositories.ItemSeen}
	f.feed.items = []feeds.Item{item(1), item(2)}

	f.round(t)

	if len(f.graph.posted) != 1 {
		t.Fatalf("posted %d: what was seen when the Pulse was made is never posted", len(f.graph.posted))
	}
	a := f.graph.posted[0]
	if a.Type != an_const.NodeMediumArticleEntityType || a.Service != an_const.TextService ||
		a.OwnerID != 900 || a.OwnerType != an_const.NodeActorPulseEntityType ||
		a.CreatedBy != 7 || f.graph.poster != 7 || a.Access != an_const.AnahitaAccessPublic {
		t.Errorf("the article: %+v", a)
	}
	if a.Name != "Post 2" || a.Body != "What post 2 says.\n\nhttps://example.com/2\n\n#Gardens #OpenScience" {
		t.Errorf("it says %q: %q", a.Name, a.Body)
	}
	if !a.CreatedAt.Equal(start.Add(-2*time.Hour)) || !strings.Contains(string(a.Meta), "The original") {
		t.Errorf("its date %v, its card %s", a.CreatedAt, a.Meta)
	}
	if f.items.kept["key-2"].State != repositories.ItemPosted || f.items.kept["key-2"].NodeID != a.ID {
		t.Errorf("the item: %+v", f.items.kept["key-2"])
	}
	if f.pulses.read == nil || f.pulses.read.ETag != `"v2"` || !f.pulses.read.NextReadAt.Equal(start.Add(DefaultReadEvery)) {
		t.Errorf("kept: %+v", f.pulses.read)
	}

	f.round(t)
	if len(f.graph.posted) != 1 {
		t.Errorf("an item was posted twice")
	}
}

func TestNoMoreThanTheCapAreMadeInAnHour(t *testing.T) {
	f := newFixture()
	for n := 1; n <= 12; n++ {
		f.feed.items = append(f.feed.items, item(n))
	}

	f.round(t)
	if len(f.graph.posted) != DefaultHourlyCap {
		t.Fatalf("posted %d in the first hour", len(f.graph.posted))
	}
	if f.graph.posted[0].Name != "Post 12" {
		t.Errorf("the oldest first: %s", f.graph.posted[0].Name)
	}

	f.clock = start.Add(30 * time.Minute)
	f.round(t)
	if len(f.graph.posted) != DefaultHourlyCap {
		t.Errorf("posted more in the same hour: %d", len(f.graph.posted))
	}

	f.clock = start.Add(61 * time.Minute)
	f.round(t)
	if len(f.graph.posted) != 12 {
		t.Errorf("what waited was not posted the next hour: %d", len(f.graph.posted))
	}
}

func TestAChangedItemChangesItsArticle(t *testing.T) {
	f := newFixture()
	f.feed.items = []feeds.Item{item(1)}
	f.round(t)

	changed := item(1)
	changed.Title = "Post 1, corrected"
	f.feed.items = []feeds.Item{changed}
	f.round(t)

	if len(f.graph.posted) != 1 || len(f.graph.edited) != 1 || f.graph.edited[0].Name != "Post 1, corrected" {
		t.Errorf("posted %d, edited %+v", len(f.graph.posted), f.graph.edited)
	}
}

func TestAFeedThatFailsIsAskedLessOften(t *testing.T) {
	f := newFixture()
	f.feed.err = errors.New("answered 503")

	f.pulses.due = &repositories.Due{NodeID: 900, Failures: 2}
	_, _ = f.w.Once(context.Background())
	if f.pulses.failures != 3 || !f.pulses.nextAt.Equal(start.Add(time.Hour)) {
		t.Errorf("after the third failure: %d, next at %v", f.pulses.failures, f.pulses.nextAt)
	}

	f.pulses.due = &repositories.Due{NodeID: 900, Failures: 20}
	_, _ = f.w.Once(context.Background())
	if !f.pulses.nextAt.Equal(start.Add(longestWait)) {
		t.Errorf("never less than once a day: next at %v", f.pulses.nextAt)
	}
}

func TestASwitchedOffPulseIsPaused(t *testing.T) {
	f := newFixture()
	f.graph.pulse.Enabled = false
	f.feed.items = []feeds.Item{item(1)}
	f.round(t)
	if f.pulses.paused != repositories.PausedProfileGone || len(f.graph.posted) != 0 {
		t.Errorf("paused %q, posted %d", f.pulses.paused, len(f.graph.posted))
	}
}

type memDomains struct {
	repositories.DomainsInterface
	due     []*repositories.Domain
	since   time.Time
	proved  []int64
	failing []int64
}

func (m *memDomains) Due(context.Context, int) ([]*repositories.Domain, error) { return m.due, nil }
func (m *memDomains) Proved(_ context.Context, id int64, _ string) error {
	m.proved = append(m.proved, id)
	return nil
}
func (m *memDomains) Failing(_ context.Context, id int64) (time.Time, error) {
	m.failing = append(m.failing, id)
	return m.since, nil
}

type tokens map[string]bool

func (t tokens) Check(_ context.Context, site domains.Site, _ string) (string, error) {
	if t[site.Domain] {
		return domains.MethodDNS, nil
	}
	return "", domains.ErrNotFound
}

func TestAProofGoneAWeekPausesItsPulses(t *testing.T) {
	f := newFixture()
	failed := start.Add(-2 * 24 * time.Hour)
	d := &memDomains{
		due: []*repositories.Domain{
			{ID: 1, Domain: "kept.example"},
			{ID: 2, Domain: "back.example", FailingSince: &failed},
			{ID: 3, Domain: "gone.example"},
		},
		since: start.Add(-8 * 24 * time.Hour),
	}
	f.w.Domains, f.w.Checker = d, tokens{"kept.example": true, "back.example": true}

	f.w.CheckProofs(context.Background())

	if fmt.Sprint(d.proved) != "[1 2]" || fmt.Sprint(d.failing) != "[3]" {
		t.Errorf("proved %v, failing %v", d.proved, d.failing)
	}
	if fmt.Sprint(f.pulses.resumed) != "[2]" || fmt.Sprint(f.pulses.pausedOn) != "[3]" {
		t.Errorf("resumed %v, paused %v", f.pulses.resumed, f.pulses.pausedOn)
	}

	d.since, f.pulses.pausedOn = start.Add(-3*24*time.Hour), nil
	d.due = d.due[2:]
	f.w.CheckProofs(context.Background())
	if len(f.pulses.pausedOn) != 0 {
		t.Errorf("paused after three days")
	}
}
