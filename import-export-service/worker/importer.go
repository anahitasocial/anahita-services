package worker

import (
	"context"
	"log"
	"strings"
	"time"

	"anahita-libs/data/entities"

	"importexport/data/repositories"
)

// Imports (anahita-web roadmap row 124): the worker goes through each
// started import a line at a time, as its person would by hand, slowly
// enough that a long list troubles neither this site nor any other.

// Acts are what a line asks for, as its person. Each answers what came of
// it: done, already (there was nothing to do), or failed, with why in a
// word.
type Acts interface {
	// FindPerson is who a line names, here or found on their own server
	// and kept.
	FindPerson(ctx context.Context, person *entities.Viewer, line *repositories.Line) (*entities.AnahitaNode, string)
	// FindPost is the post a line names, the same way.
	FindPost(ctx context.Context, person *entities.Viewer, line *repositories.Line) (*entities.AnahitaNode, string)
	Follow(person *entities.Viewer, target *entities.AnahitaNode) (string, string)
	Block(person *entities.Viewer, target *entities.AnahitaNode) (string, string)
	Mute(person *entities.Viewer, target *entities.AnahitaNode, hideNotifications bool, endsAt *time.Time) (string, string)
	Save(person *entities.Viewer, post *entities.AnahitaNode) (string, string)
}

// Why a line could not be done, in a word, for the list the person keeps.
const (
	// Nobody, or nothing, at that address.
	ReasonNotFound = "not_found"
	// A group or an event on another Anahita: not on the fediverse.
	ReasonNotOnFediverse = "not_on_fediverse"
	// On a server, or an account, this site refuses.
	ReasonRefused = "refused"
	// This site does not take part in that network.
	ReasonNotFederating = "not_federating"
	// Their server could not be reached; trying again later may do.
	ReasonUnreachable = "unreachable"
	// One of them blocks the other.
	ReasonBlocked = "blocked"
	// Not theirs to follow, or to see.
	ReasonNotAllowed = "not_allowed"
	// A group or an event whose rules are to be agreed first, by hand.
	ReasonRulesNotAgreed = "rules_not_agreed"
	// A mute that has ended since the file was made.
	ReasonEnded = "ended"
	// Something here went wrong.
	ReasonFailed = "failed"
)

// Importer goes through the started imports.
type Importer struct {
	Imports repositories.ImportsInterface
	Acts    Acts
	// The time between one line and the next: a second.
	Pace time.Duration
	// How long to wait when there is nothing to do.
	Idle time.Duration
	Now  func() time.Time
}

// Run goes through lines until ctx is done.
func (im *Importer) Run(ctx context.Context) {
	people := map[string]int64{}
	for {
		wait := im.Pace
		done, err := im.Next(ctx, people)
		if err != nil {
			log.Println("import worker:", err)
		}
		if !done {
			wait = im.Idle
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

// Next does the next line of any import, and says whether there was one.
// With none, the imports with nothing left are marked done.
func (im *Importer) Next(ctx context.Context, people map[string]int64) (bool, error) {
	line, err := im.Imports.NextLine()
	if err != nil {
		return false, err
	}
	if line == nil {
		return false, im.Imports.FinishImports()
	}

	personID, ok := people[line.ImportID]
	if !ok {
		imp, err := im.Imports.Read(line.ImportID)
		if err != nil || imp == nil {
			return true, im.Imports.Finish(line.ID, repositories.LineFailed, ReasonFailed)
		}
		personID = imp.PersonID
		people[line.ImportID] = personID
	}

	status, reason := im.Do(ctx, as(personID), line)
	return true, im.Imports.Finish(line.ID, status, reason)
}

// Do does one line as its person.
func (im *Importer) Do(ctx context.Context, person *entities.Viewer, line *repositories.Line) (string, string) {
	if line.Kind == repositories.KindSave {
		post, reason := im.Acts.FindPost(ctx, person, line)
		if post == nil {
			return repositories.LineFailed, reason
		}
		return im.Acts.Save(person, post)
	}

	if line.Kind == repositories.KindMute && line.EndsAt != nil && !line.EndsAt.After(im.now()) {
		return repositories.LineAlready, ReasonEnded
	}

	// A group or an event on another Anahita has no address to be found by:
	// groups are not on the fediverse.
	if line.Address == "" && line.URI == "" {
		return repositories.LineFailed, ReasonNotOnFediverse
	}

	target, reason := im.Acts.FindPerson(ctx, person, line)
	if target == nil {
		return repositories.LineFailed, reason
	}
	if target.ID == person.ID {
		return repositories.LineAlready, ""
	}

	switch line.Kind {
	case repositories.KindFollow:
		return im.Acts.Follow(person, target)
	case repositories.KindBlock:
		return im.Acts.Block(person, target)
	case repositories.KindMute:
		return im.Acts.Mute(person, target, line.HideNotifications, line.EndsAt)
	}
	return repositories.LineFailed, ReasonFailed
}

func (im *Importer) now() time.Time {
	if im.Now != nil {
		return im.Now()
	}
	return time.Now()
}

// handleOf splits a fediverse address, alice@mastodon.example, into its
// user and its domain; a Bluesky handle, alice.bsky.social, has no user.
func handleOf(address string) (string, string) {
	address = strings.TrimPrefix(strings.TrimSpace(address), "@")
	if user, domain, ok := strings.Cut(address, "@"); ok {
		return user, strings.ToLower(domain)
	}
	return "", strings.ToLower(address)
}
