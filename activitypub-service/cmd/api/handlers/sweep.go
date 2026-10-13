package handlers

import (
	"context"
	"log"
	"time"

	"activitypub/data/repositories"
)

// The sweep for what the bus lost. See data/repositories/missed.go.
//
// Each thing found is given to what handles the event it never heard, and
// every one of those does its work once however often it is asked: a
// follow is begun once in the table, a post is recorded as sent once, a
// post that is gone is forgotten once. So a sweep that finds nothing new
// sends nothing, any number of copies of this service may sweep at the
// same time, and a sweep that overlaps an event that was merely late does
// no harm.
//
// A change to something other servers already have, a note edited or a
// profile edited, is found by what is recorded of what they were last sent
// (told_updated_at): a change made later than that was never told.

const (
	// How long the bus is given before something is taken for lost.
	sweepGrace = 2 * time.Minute
	// How far back a post is looked for. One that went unsent for longer
	// is not sent late: followers would be shown it as new.
	sweepNotesWindow = 48 * time.Hour
	// The most of each kind put right in one sweep. The rest wait for the
	// next.
	sweepBatch = 200
)

// Swept is what one sweep found and put right, for the log.
type Swept struct {
	Follows, Unfollows, Notes, NotesGone, NotesEdited, PeopleEdited, Left int
}

func (s Swept) any() bool {
	return s.Follows+s.Unfollows+s.Notes+s.NotesGone+s.NotesEdited+s.PeopleEdited+s.Left > 0
}

// Sweep looks once for what was lost and puts it right.
func (h *Handlers) Sweep(ctx context.Context, now time.Time) Swept {
	var swept Swept
	if h.Missed == nil {
		return swept
	}
	if ok, _ := h.siteFederates(); !ok {
		return swept
	}

	each := func(what string, found []repositories.Pair, err error, do func(repositories.Pair) error) int {
		if err != nil {
			log.Printf("sweep: looking for %s: %v", what, err)
			return 0
		}
		done := 0
		for _, pair := range found {
			if ctx.Err() != nil {
				break
			}
			if err := do(pair); err != nil {
				log.Printf("sweep: %s (%d > %d): %v", what, pair.LocalID, pair.RemoteID, err)
				continue
			}
			done++
		}
		return done
	}
	ids := func(found []int64) []repositories.Pair {
		pairs := make([]repositories.Pair, 0, len(found))
		for _, id := range found {
			pairs = append(pairs, repositories.Pair{LocalID: id})
		}
		return pairs
	}

	// Somebody here follows somebody elsewhere, and that server was never
	// asked. It is asked now; or, where the follow is one that cannot be
	// (from the lab to a real server, or by somebody not on the
	// fediverse), it is taken away, as it would have been at the time.
	found, err := h.Missed.FollowsNotAsked(now.Add(-sweepGrace), sweepBatch)
	swept.Follows = each("follows not asked", found, err, func(pair repositories.Pair) error {
		return h.localFollowed(ctx, pair.LocalID, pair.RemoteID)
	})

	// Somebody here stopped following somebody elsewhere, and that server
	// still thinks they follow.
	found, err = h.Missed.FollowsGone(sweepBatch)
	swept.Unfollows = each("follows gone", found, err, func(pair repositories.Pair) error {
		return h.localUnfollowed(ctx, pair.LocalID, pair.RemoteID)
	})

	// A post that should have gone out and did not.
	notes, err := h.Missed.NotesNotSent(now.Add(-sweepNotesWindow), now.Add(-sweepGrace), sweepBatch)
	swept.Notes = each("notes not sent", ids(notes), err, func(pair repositories.Pair) error {
		return h.posted(ctx, pair.LocalID)
	})

	// A post other servers have that is gone here, or closed.
	notes, err = h.Missed.NotesGone(sweepBatch)
	swept.NotesGone = each("notes gone", ids(notes), err, func(pair repositories.Pair) error {
		return h.posted(ctx, pair.LocalID)
	})

	// A post other servers have that was changed here, with nobody told.
	notes, err = h.Missed.NotesEdited(now.Add(-sweepGrace), sweepBatch)
	swept.NotesEdited = each("notes changed", ids(notes), err, func(pair repositories.Pair) error {
		return h.posted(ctx, pair.LocalID)
	})

	// Somebody whose profile was changed, with their followers' servers
	// not told.
	edited, err := h.Missed.PeopleEdited(now.Add(-sweepGrace), sweepBatch)
	swept.PeopleEdited = each("profiles changed", ids(edited), err, func(pair repositories.Pair) error {
		return h.profileChanged(ctx, pair.LocalID)
	})

	// Somebody on the fediverse who may no longer be: their profile was
	// closed, disabled or deleted, and the word of it was lost.
	people, err := h.Missed.People(sweepBatch * 10)
	if err != nil {
		log.Printf("sweep: looking for who is on the fediverse: %v", err)
	}
	for _, id := range people {
		if ctx.Err() != nil {
			break
		}
		person, err := h.Nodes.Read(id)
		if err != nil {
			log.Printf("sweep: reading %d: %v", id, err)
			continue
		}
		if person != nil && person.ID != 0 && h.mayFederate(person) {
			continue
		}
		if person == nil || person.ID == 0 {
			// Gone altogether, with nothing left to describe them by.
			// Their choice is put out, so that they are not looked at
			// again.
			err = h.Accounts.Set(id, false)
		} else {
			err = h.leave(ctx, person)
		}
		if err != nil {
			log.Printf("sweep: %d leaving: %v", id, err)
			continue
		}
		swept.Left++
	}

	if swept.any() {
		log.Printf("sweep: put right %d follows, %d unfollows, %d notes not sent, %d notes gone, %d notes changed, %d profiles changed, %d people who left",
			swept.Follows, swept.Unfollows, swept.Notes, swept.NotesGone, swept.NotesEdited, swept.PeopleEdited, swept.Left)
	}
	return swept
}

// RunSweep sweeps now and then, for as long as the service runs.
func (h *Handlers) RunSweep(ctx context.Context, every time.Duration) {
	// Not at once: the service has only just begun to listen, and what
	// happened while it was down is exactly what the first sweep is for.
	timer := time.NewTimer(time.Minute)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		pass, cancel := context.WithTimeout(ctx, every/2)
		h.Sweep(pass, time.Now().UTC())
		cancel()

		timer.Reset(every)
	}
}
