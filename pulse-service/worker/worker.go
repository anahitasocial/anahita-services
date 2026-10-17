// Package worker reads the feeds of Pulses and posts what is new in them,
// and checks every day that the domains they rest on are still proved.
//
// It runs apart from the API (cmd/worker), as one or more processes: a
// Pulse is taken by one worker at a time (repositories.Claim), so several
// may run. See docs/plans/pulse.md.
package worker

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"pulse/data/repositories"
	"pulse/feeds"

	an_handlers "anahita-libs/cmd/api/handlers"
	an_const "anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/domains"
	"anahita-libs/linkpreviews"

	"github.com/gosimple/slug"
)

const (
	// How often a feed is read when all is well.
	DefaultReadEvery = 30 * time.Minute
	// The most posts a Pulse makes in an hour; the rest wait for the next.
	DefaultHourlyCap = 10
	// A feed that fails is asked less often each time, up to once a day.
	longestWait = 24 * time.Hour
	// A Pulse whose domain's proof has been gone this long is paused.
	proofGrace = 7 * 24 * time.Hour
	// The proofs looked at again on each round.
	proofsPerRound = 20
	// How long a round of nothing to do waits.
	idle = 30 * time.Second
	// How often the proofs are looked at.
	proofRound = time.Hour
)

// Feeds reads feeds. feeds.Reader is the one that does.
type Feeds interface {
	Read(ctx context.Context, feedURL, etag, lastModified string) (*feeds.Read, error)
}

// Checker looks for a domain's token. domains.Checker is the one that does.
type Checker interface {
	Check(ctx context.Context, site domains.Site, token string) (string, error)
}

// Graph is what the worker asks of the graph: the Pulse, who looks after
// it, and its posts.
type Graph interface {
	Pulse(id int64) (*entities.AnahitaNode, error)
	FirstAdmin(pulseID int64) (int64, error)
	Post(article *entities.AnahitaNode, viewer *entities.Viewer) (*entities.AnahitaNode, error)
	Read(id int64, viewer *entities.Viewer) (*entities.AnahitaNode, error)
	Edit(article *entities.AnahitaNode, viewer *entities.Viewer) (*entities.AnahitaNode, error)
}

// Cards finds a link's card; nil when there is none to show.
type Cards interface {
	Preview(ctx context.Context, address string) *linkpreviews.Preview
}

type Worker struct {
	Pulses  repositories.PulsesInterface
	Items   repositories.ItemsInterface
	Domains repositories.DomainsInterface
	Feeds   Feeds
	Checker Checker
	Graph   Graph
	// May be nil: an installation without link-preview-service has no
	// cards, and its articles none.
	Cards Cards

	ReadEvery time.Duration
	HourlyCap int
	Now       func() time.Time

	// Subscribing to the hubs feeds name. Callback is nil where no hub
	// could call back: an installation not reachable from the internet.
	Subscribe func(ctx context.Context, hub, topic, callback string) error
	Callback  func(nodeID int64) string
}

// A subscription is asked for again this long before it runs out, and this
// long after it was asked for when the hub never checked it.
const hubRenew = 24 * time.Hour

// Run reads feeds until it is stopped: whatever is due, then a pause, and
// the proofs once an hour.
func (w *Worker) Run(ctx context.Context) {
	proofs := time.NewTicker(proofRound)
	defer proofs.Stop()
	w.CheckProofs(ctx)
	w.CheckProfiles(ctx)

	for {
		for {
			did, err := w.Once(ctx)
			if err != nil {
				log.Printf("pulse-worker: %v", err)
			}
			if !did || ctx.Err() != nil {
				break
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-proofs.C:
			w.CheckProofs(ctx)
			w.CheckProfiles(ctx)
		case <-time.After(idle):
		}
	}
}

// Once reads the feed most overdue, if any is: did says whether there was
// one.
func (w *Worker) Once(ctx context.Context) (did bool, err error) {
	due, err := w.Pulses.Claim(ctx)
	if err != nil || due == nil {
		return false, err
	}
	return true, w.read(ctx, due)
}

func (w *Worker) now() time.Time {
	if w.Now != nil {
		return w.Now().UTC()
	}
	return time.Now().UTC()
}

func (w *Worker) readEvery() time.Duration {
	if w.ReadEvery > 0 {
		return w.ReadEvery
	}
	return DefaultReadEvery
}

func (w *Worker) hourlyCap() int {
	if w.HourlyCap > 0 {
		return w.HourlyCap
	}
	return DefaultHourlyCap
}

// failed puts the next read off, longer each time: 15 minutes, 30, an hour,
// and so on to a day.
func (w *Worker) failed(ctx context.Context, due *repositories.Due, why error) error {
	failures := due.Failures + 1
	wait := 15 * time.Minute << min(failures-1, 10)
	if wait > longestWait {
		wait = longestWait
	}
	return w.Pulses.Failed(ctx, due.NodeID, failures, w.now().Add(wait), why.Error())
}

func (w *Worker) read(ctx context.Context, due *repositories.Due) error {
	pulse, err := w.Graph.Pulse(due.NodeID)
	if err != nil {
		return w.failed(ctx, due, fmt.Errorf("the Pulse could not be read: %w", err))
	}
	// Switched off, or deleted and waiting out its grace period, archived:
	// nothing is posted on it.
	if pulse == nil || pulse.ID == 0 || !pulse.Enabled || pulse.DeletedAt != nil || pulse.ArchivedAt != nil {
		return w.Pulses.Pause(ctx, due.NodeID, repositories.PausedProfileGone)
	}

	read, err := w.Feeds.Read(ctx, due.FeedURL, due.ETag, due.LastModified)
	if err != nil {
		return w.failed(ctx, due, err)
	}

	next := repositories.Read{
		ETag:         read.ETag,
		LastModified: read.LastModified,
		HourPosts:    due.HourPosts,
		HourBeganAt:  due.HourBeganAt,
		NextReadAt:   w.now().Add(w.readEvery()),
	}
	if !read.NotModified {
		if err := w.take(ctx, pulse, read.Items, &next); err != nil {
			return w.failed(ctx, due, err)
		}
		w.subscribe(ctx, due, read)
	}
	return w.Pulses.Read(ctx, due.NodeID, next)
}

// subscribe asks the hub a feed names to say when it changes: when it has
// not been asked, or its subscription is near its end. A hub that does not
// answer is no reason to stop reading the feed.
func (w *Worker) subscribe(ctx context.Context, due *repositories.Due, read *feeds.Read) {
	if read.Hub == "" || w.Subscribe == nil || w.Callback == nil {
		return
	}
	now := w.now()
	if due.HubURL == read.Hub && due.HubUntil != nil && now.Before(due.HubUntil.Add(-hubRenew)) {
		return
	}
	if err := w.Subscribe(ctx, read.Hub, read.Self, w.Callback(due.NodeID)); err != nil {
		log.Printf("pulse-worker: subscribing to %s: %v", read.Hub, err)
		return
	}
	if err := w.Pulses.Hub(ctx, due.NodeID, read.Hub, now.Add(2*hubRenew)); err != nil {
		log.Printf("pulse-worker: %v", err)
	}
}

// take keeps what is new in a feed, posts as much of what waits as this
// hour allows, and changes the posts of items that changed.
func (w *Worker) take(ctx context.Context, pulse *entities.AnahitaNode, items []feeds.Item, next *repositories.Read) error {
	if len(items) == 0 {
		return nil
	}

	keys := make([]string, len(items))
	for i, item := range items {
		keys[i] = item.Key
	}
	known, err := w.Items.Known(ctx, pulse.ID, keys)
	if err != nil {
		return err
	}

	var fresh []repositories.ItemSeenAs
	var waiting []feeds.Item
	var changed []feeds.Item
	for _, item := range items {
		hash := repositories.ContentHash(item.Title, item.Excerpt)
		kept, ok := known[item.Key]
		switch {
		case !ok:
			fresh = append(fresh, repositories.ItemSeenAs{Key: item.Key, ContentHash: hash})
			waiting = append(waiting, item)
		case kept.State == repositories.ItemWaiting:
			waiting = append(waiting, item)
		case kept.State == repositories.ItemPosted && kept.ContentHash != hash && kept.NodeID > 0:
			changed = append(changed, item)
		}
	}
	if err := w.Items.AddWaiting(ctx, pulse.ID, fresh); err != nil {
		return err
	}
	if len(waiting) == 0 && len(changed) == 0 {
		return nil
	}

	adminID, err := w.Graph.FirstAdmin(pulse.ID)
	if err != nil {
		return err
	}
	viewer := posterFor(adminID)

	// The oldest first, so that they are posted in the order they were
	// published.
	sort.SliceStable(waiting, func(i, j int) bool { return waiting[i].Published.Before(waiting[j].Published) })

	now := w.now()
	if next.HourBeganAt == nil || now.Sub(*next.HourBeganAt) >= time.Hour {
		next.HourBeganAt, next.HourPosts = &now, 0
	}
	for _, item := range waiting {
		if next.HourPosts >= w.hourlyCap() {
			// What is left waits; the feed is read again sooner.
			if after := next.HourBeganAt.Add(time.Hour); after.Before(next.NextReadAt) {
				next.NextReadAt = after
			}
			break
		}
		node, err := w.Graph.Post(w.articleOf(ctx, pulse, adminID, item), viewer)
		if err != nil {
			return fmt.Errorf("the article was not made: %w", err)
		}
		if err := w.Items.Posted(ctx, pulse.ID, item.Key, node.ID); err != nil {
			return err
		}
		next.HourPosts++
	}

	for _, item := range changed {
		if err := w.change(ctx, pulse, known[item.Key].NodeID, item, viewer); err != nil {
			log.Printf("pulse-worker: the article of %s was not changed: %v", item.Link, err)
		}
	}
	return nil
}

// posterFor is the viewer a Pulse's article is made as: its first
// administrator, as the video-worker makes a post as its uploader.
func posterFor(adminID int64) *entities.Viewer {
	viewer := entities.NewViewer()
	viewer.ID = adminID
	viewer.Authenticated = true
	viewer.Enabled = true
	viewer.PersonType = an_const.PersonTypeRegistered
	viewer.Scopes = []string{an_const.OAuthScopeMediaWrite}
	return viewer
}

// ContentOf is what an article of an item says: its excerpt, the address
// of the original on a line of its own (which gives the card), and its
// categories as hashtags.
func ContentOf(item feeds.Item) string {
	var parts []string
	if item.Excerpt != "" && item.Excerpt != item.Title {
		parts = append(parts, item.Excerpt)
	}
	parts = append(parts, item.Link)
	if len(item.Tags) > 0 {
		parts = append(parts, "#"+strings.Join(item.Tags, " #"))
	}
	return strings.Join(parts, "\n\n")
}

func (w *Worker) articleOf(ctx context.Context, pulse *entities.AnahitaNode, adminID int64, item feeds.Item) *entities.AnahitaNode {
	article := entities.NewAnahitaNode()
	article.Type = an_const.NodeMediumArticleEntityType
	article.Service = an_const.TextService
	// The Pulse's own, shown and sent as its, as its replies are; the
	// administrator it was made as is kept for moderators.
	article.OwnerID = pulse.ID
	article.OwnerType = pulse.Type
	article.CreatedBy = pulse.ID
	article.ModifiedBy = adminID
	article.Name = item.Title
	article.Alias = slug.Make(item.Title)
	article.Body = ContentOf(item)
	article.Access = an_const.AnahitaAccessPublic
	article.Enabled = true
	article.CommentStatus = true
	// The graph keeps it unless it is in the future.
	article.CreatedAt = item.Published
	article.Meta = an_handlers.WithWrittenBy(w.cardOf(ctx, item.Link), adminID)
	return article
}

// cardOf is the card of the original, as an article keeps the cards of the
// pages it shows.
func (w *Worker) cardOf(ctx context.Context, link string) []byte {
	if w.Cards == nil {
		return nil
	}
	snapshot := linkpreviews.NewSnapshot(w.Cards.Preview(ctx, link))
	if snapshot == nil {
		return nil
	}
	return linkpreviews.SnapshotsIntoMeta(nil, []*linkpreviews.Snapshot{snapshot})
}

func (w *Worker) change(ctx context.Context, pulse *entities.AnahitaNode, nodeID int64, item feeds.Item, viewer *entities.Viewer) error {
	article, err := w.Graph.Read(nodeID, viewer)
	if err != nil {
		return err
	}
	if article == nil || article.ID == 0 || article.OwnerID != pulse.ID {
		return errors.New("it is no longer the Pulse's")
	}
	article.Name = item.Title
	article.Alias = slug.Make(item.Title)
	article.Body = ContentOf(item)
	article.ModifiedBy = viewer.ID
	if _, err := w.Graph.Edit(article, viewer); err != nil {
		return err
	}
	return w.Items.Changed(ctx, pulse.ID, item.Key, repositories.ContentHash(item.Title, item.Excerpt))
}

// CheckProfiles starts again the Pulses paused because their profile was
// switched off, deleted or archived, once it is back: switched on again, or
// restored within its grace period.
func (w *Worker) CheckProfiles(ctx context.Context) {
	paused, err := w.Pulses.PausedFor(ctx, repositories.PausedProfileGone, proofsPerRound)
	if err != nil {
		log.Printf("pulse-worker: profiles: %v", err)
		return
	}
	for _, id := range paused {
		pulse, err := w.Graph.Pulse(id)
		if err != nil || pulse == nil || pulse.ID == 0 {
			continue
		}
		if pulse.Enabled && pulse.DeletedAt == nil && pulse.ArchivedAt == nil {
			if err := w.Pulses.Resume(ctx, id); err != nil {
				log.Printf("pulse-worker: resuming %d: %v", id, err)
			}
		}
	}
}

// CheckProofs looks again at the proofs not looked at for a day: one found
// holds again, and starts its Pulses again if it had paused them; one gone
// a week pauses them.
func (w *Worker) CheckProofs(ctx context.Context) {
	due, err := w.Domains.Due(ctx, proofsPerRound)
	if err != nil {
		log.Printf("pulse-worker: proofs: %v", err)
		return
	}
	for _, d := range due {
		site := domains.Site{Host: d.Host, Domain: d.Domain}
		method, err := w.Checker.Check(ctx, site, d.Token)
		if err == nil {
			if err := w.Domains.Proved(ctx, d.ID, method); err != nil {
				log.Printf("pulse-worker: proof of %s: %v", d.Domain, err)
				continue
			}
			if d.FailingSince != nil {
				_ = w.Pulses.ResumeAfterProof(ctx, d.ID)
			}
			continue
		}

		since, err := w.Domains.Failing(ctx, d.ID)
		if err != nil {
			log.Printf("pulse-worker: proof of %s: %v", d.Domain, err)
			continue
		}
		if w.now().Sub(since) >= proofGrace {
			if err := w.Pulses.PauseForProof(ctx, d.ID); err != nil {
				log.Printf("pulse-worker: pausing for %s: %v", d.Domain, err)
			}
		}
	}
}
