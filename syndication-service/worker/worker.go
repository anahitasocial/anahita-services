// Package worker reads the feeds of websites and posts what is new in them,
// and checks every day that the domains they rest on are still proved.
//
// It runs apart from the API (cmd/worker), as one or more processes: a
// Website is taken by one worker at a time (repositories.Claim), so several
// may run. See docs/plans/done/website.md.
package worker

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"syndication/data/repositories"
	"syndication/feeds"

	an_handlers "anahita-libs/cmd/api/handlers"
	an_const "anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/domains"
	"anahita-libs/linkpreviews"
)

const (
	// How often a feed is read when all is well.
	DefaultReadEvery = 30 * time.Minute
	// The most posts a website makes in an hour; the rest wait for the next.
	DefaultHourlyCap = 10
	// A feed that fails is asked less often each time, up to once a day.
	longestWait = 24 * time.Hour
	// A website whose domain's proof has been gone this long is paused.
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

// Graph is what the worker asks of the graph: the website, who looks after
// it, and its posts.
type Graph interface {
	Website(id int64) (*entities.AnahitaNode, error)
	FirstAdmin(websiteID int64) (int64, error)
	Post(note *entities.AnahitaNode, viewer *entities.Viewer) (*entities.AnahitaNode, error)
	Read(id int64, viewer *entities.Viewer) (*entities.AnahitaNode, error)
	Edit(note *entities.AnahitaNode, viewer *entities.Viewer) (*entities.AnahitaNode, error)
}

// Cards finds a link's card; nil when there is none to show.
type Cards interface {
	Preview(ctx context.Context, address string) *linkpreviews.Preview
}

type Worker struct {
	Websites repositories.WebsitesInterface
	Items    repositories.ItemsInterface
	Domains  repositories.DomainsInterface
	Feeds    Feeds
	Checker  Checker
	Graph    Graph
	// May be nil: an installation without link-preview-service has no
	// cards, and its notes none.
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
				log.Printf("syndication-worker: %v", err)
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
	due, err := w.Websites.Claim(ctx)
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
	return w.Websites.Failed(ctx, due.NodeID, failures, w.now().Add(wait), why.Error())
}

func (w *Worker) read(ctx context.Context, due *repositories.Due) error {
	website, err := w.Graph.Website(due.NodeID)
	if err != nil {
		return w.failed(ctx, due, fmt.Errorf("the Website could not be read: %w", err))
	}
	// Switched off, or deleted and waiting out its grace period, archived:
	// nothing is posted on it.
	if website == nil || website.ID == 0 || !website.Enabled || website.DeletedAt != nil || website.ArchivedAt != nil {
		return w.Websites.Pause(ctx, due.NodeID, repositories.PausedProfileGone)
	}

	// What waits to be posted is posted from the feed as it is read: so
	// while anything waits, the feed is read whole, and not answered "not
	// changed since".
	etag, lastModified := due.ETag, due.LastModified
	if waiting, err := w.Items.Waiting(ctx, due.NodeID); err != nil {
		return w.failed(ctx, due, err)
	} else if waiting > 0 {
		etag, lastModified = "", ""
	}
	read, err := w.Feeds.Read(ctx, due.FeedURL, etag, lastModified)
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
		if err := w.take(ctx, website, read.Items, &next); err != nil {
			return w.failed(ctx, due, err)
		}
		w.subscribe(ctx, due, read)
	}
	return w.Websites.Read(ctx, due.NodeID, next)
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
		log.Printf("syndication-worker: subscribing to %s: %v", read.Hub, err)
		return
	}
	if err := w.Websites.Hub(ctx, due.NodeID, read.Hub, now.Add(2*hubRenew)); err != nil {
		log.Printf("syndication-worker: %v", err)
	}
}

// take keeps what is new in a feed, posts as much of what waits as this
// hour allows, and changes the posts of items that changed.
func (w *Worker) take(ctx context.Context, website *entities.AnahitaNode, items []feeds.Item, next *repositories.Read) error {
	if len(items) == 0 {
		return nil
	}

	keys := make([]string, len(items))
	for i, item := range items {
		keys[i] = item.Key
	}
	known, err := w.Items.Known(ctx, website.ID, keys)
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
	if err := w.Items.AddWaiting(ctx, website.ID, fresh); err != nil {
		return err
	}
	if len(waiting) == 0 && len(changed) == 0 {
		return nil
	}

	adminID, err := w.Graph.FirstAdmin(website.ID)
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
		node, err := w.Graph.Post(w.noteOf(ctx, website, adminID, item), viewer)
		if err != nil {
			return fmt.Errorf("the note was not made: %w", err)
		}
		if err := w.Items.Posted(ctx, website.ID, item.Key, node.ID); err != nil {
			return err
		}
		next.HourPosts++
	}

	for _, item := range changed {
		if err := w.change(ctx, website, known[item.Key].NodeID, item, viewer); err != nil {
			log.Printf("syndication-worker: the note of %s was not changed: %v", item.Link, err)
		}
	}
	return nil
}

// posterFor is the viewer a website's note is made as: its first
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

// ContentOf is what a note of an item says: its title in bold, as a note
// has no title of its own (roadmap row 74); its excerpt; the original's
// address on a line of its own, which gives the card; and its categories
// as hashtags.
func ContentOf(item feeds.Item) string {
	parts := []string{"**" + markdownEscaper.Replace(item.Title) + "**"}
	if item.Excerpt != "" && item.Excerpt != item.Title {
		parts = append(parts, markdownEscaper.Replace(item.Excerpt))
	}
	parts = append(parts, item.Link)
	if len(item.Tags) > 0 {
		parts = append(parts, "#"+strings.Join(item.Tags, " #"))
	}
	return strings.Join(parts, "\n\n")
}

// A title or an excerpt is plain text: what Markdown would read as marks in
// it is kept as it is written.
var markdownEscaper = strings.NewReplacer(`\`, `\\`, `*`, `\*`, `_`, `\_`, "`", "\\`", `[`, `\[`, `#`, `\#`)

// noteOf is the note an item becomes: the website's own, shown and sent as
// its, public, with the original's card. The administrator it is made as is
// kept for moderators (`written_by`), as for its replies.
func (w *Worker) noteOf(ctx context.Context, website *entities.AnahitaNode, adminID int64, item feeds.Item) *entities.AnahitaNode {
	note := entities.NewAnahitaNode()
	note.Type = an_const.NodeMediumNoteEntityType
	note.Service = an_const.TextService
	note.OwnerID = website.ID
	note.OwnerType = website.Type
	note.CreatedBy = website.ID
	note.ModifiedBy = adminID
	note.Body = ContentOf(item)
	note.Access = an_const.AnahitaAccessPublic
	note.Enabled = true
	note.CommentStatus = true
	// The graph keeps it unless it is in the future.
	note.CreatedAt = item.Published
	note.Meta = an_handlers.WithWrittenBy(w.cardOf(ctx, item.Link), adminID)
	return note
}

// cardOf is the card of the original, the one card a note carries.
func (w *Worker) cardOf(ctx context.Context, link string) []byte {
	if w.Cards == nil {
		return nil
	}
	snapshot := linkpreviews.NewSnapshot(w.Cards.Preview(ctx, link))
	if snapshot == nil {
		return nil
	}
	return linkpreviews.IntoMeta(nil, snapshot)
}

func (w *Worker) change(ctx context.Context, website *entities.AnahitaNode, nodeID int64, item feeds.Item, viewer *entities.Viewer) error {
	note, err := w.Graph.Read(nodeID, viewer)
	if err != nil {
		return err
	}
	if note == nil || note.ID == 0 || note.OwnerID != website.ID {
		return errors.New("it is no longer the website's")
	}
	note.Body = ContentOf(item)
	note.ModifiedBy = viewer.ID
	if _, err := w.Graph.Edit(note, viewer); err != nil {
		return err
	}
	return w.Items.Changed(ctx, website.ID, item.Key, repositories.ContentHash(item.Title, item.Excerpt))
}

// CheckProfiles starts again the websites paused because their profile was
// switched off, deleted or archived, once it is back: switched on again, or
// restored within its grace period.
func (w *Worker) CheckProfiles(ctx context.Context) {
	paused, err := w.Websites.PausedFor(ctx, repositories.PausedProfileGone, proofsPerRound)
	if err != nil {
		log.Printf("syndication-worker: profiles: %v", err)
		return
	}
	for _, id := range paused {
		website, err := w.Graph.Website(id)
		if err != nil || website == nil || website.ID == 0 {
			continue
		}
		if website.Enabled && website.DeletedAt == nil && website.ArchivedAt == nil {
			if err := w.Websites.Resume(ctx, id); err != nil {
				log.Printf("syndication-worker: resuming %d: %v", id, err)
			}
		}
	}
}

// CheckProofs looks again at the proofs not looked at for a day: one found
// holds again, and starts its websites again if it had paused them; one gone
// a week pauses them.
func (w *Worker) CheckProofs(ctx context.Context) {
	due, err := w.Domains.Due(ctx, proofsPerRound)
	if err != nil {
		log.Printf("syndication-worker: proofs: %v", err)
		return
	}
	for _, d := range due {
		site := domains.Site{Host: d.Host, Domain: d.Domain}
		method, err := w.Checker.Check(ctx, site, d.Token)
		if err == nil {
			if err := w.Domains.Proved(ctx, d.ID, method); err != nil {
				log.Printf("syndication-worker: proof of %s: %v", d.Domain, err)
				continue
			}
			if d.FailingSince != nil {
				_ = w.Websites.ResumeAfterProof(ctx, d.ID)
			}
			continue
		}

		since, err := w.Domains.Failing(ctx, d.ID)
		if err != nil {
			log.Printf("syndication-worker: proof of %s: %v", d.Domain, err)
			continue
		}
		if w.now().Sub(since) >= proofGrace {
			if err := w.Websites.PauseForProof(ctx, d.ID); err != nil {
				log.Printf("syndication-worker: pausing for %s: %v", d.Domain, err)
			}
		}
	}
}
