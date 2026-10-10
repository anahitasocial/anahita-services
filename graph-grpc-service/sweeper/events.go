package sweeper

import (
	"fmt"
	"log"
	"time"

	"anahita-libs/constants"
	"anahita-libs/event"
	"graph/data/repositories"
)

// How many events one run archives, and reminds people of. What is left
// waits an hour for the next run.
const eventBatch = 200

// Events does what time does to events.
//
//   - Whoever is going to one that starts within a day is reminded, once.
//   - Invitations still waiting to one that is over, or was called off, are
//     removed: there is nothing left to say yes to.
//   - One that ended thirty days ago is archived. It stays where it is and
//     can be read, and nothing more can be added to it.
//
// Each part is logged and skipped when it fails, so one does not hold up
// the others. Without a publisher the invitations still go and the events
// are still archived; nobody is reminded, and nobody is marked as having
// been, so the reminders go out when there is one again.
func Events(repos *repositories.Repositories, publisher *event.Publisher) {
	remind(repos, publisher)
	dropInvitesToClosedEvents(repos, publisher)
	archiveFinished(repos)
}

func remind(repos *repositories.Repositories, publisher *event.Publisher) {
	if publisher == nil {
		return
	}

	due, err := repos.Events.ClaimReminders(constants.EventReminderBefore, eventBatch)
	if err != nil {
		log.Printf("sweeper: event reminders: %v", err)
	}

	reminded := 0
	for _, eventID := range due {
		people, err := repos.Events.AttendeeIDs(eventID, true)
		if err != nil {
			log.Printf("sweeper: who is going to %d: %v", eventID, err)
			continue
		}

		routingKey := fmt.Sprintf(event.EventFormatEventReminded, "actor")
		for _, personID := range people {
			publisher.Publish(&event.EventPayload{
				Name:          routingKey,
				SourceService: "graph-grpc-service",
				ObjectID:      personID,
				TargetID:      eventID,
			}, routingKey)
			reminded++
		}
	}

	if len(due) > 0 {
		log.Printf("sweeper: %d reminder(s) for %d event(s)", reminded, len(due))
	}
}

func dropInvitesToClosedEvents(repos *repositories.Repositories, publisher *event.Publisher) {
	gone, err := repos.Invites.SweepClosedEvents()
	if err != nil {
		log.Printf("sweeper: invitations to closed events: %v", err)
		return
	}
	if len(gone) == 0 {
		return
	}

	log.Printf("sweeper: %d invitation(s) to events that are over or called off", len(gone))

	if publisher == nil {
		return
	}

	routingKey := fmt.Sprintf(event.EventFormatEdgeInviteDeleted, "socialgraph")
	for _, invite := range gone {
		publisher.Publish(&event.EventPayload{
			Name:          routingKey,
			SourceService: "graph-grpc-service",
			ObjectID:      invite.InviteeID,
			TargetID:      invite.ActorID,
			OwnerID:       invite.InviterID,
		}, routingKey)
	}
}

func archiveFinished(repos *repositories.Repositories) {
	finished, err := repos.Events.OverSince(time.Now().Add(-constants.EventArchiveAfter), eventBatch)
	if err != nil {
		log.Printf("sweeper: finished events: %v", err)
		return
	}

	archived := 0
	for _, id := range finished {
		changed, err := repos.Nodes.ArchiveActor(id)
		if err != nil {
			log.Printf("sweeper: archiving event %d: %v", id, err)
			continue
		}
		if changed {
			archived++
		}
	}

	if archived > 0 {
		log.Printf("sweeper: %d event(s) archived", archived)
	}
}
