package sweeper

import (
	"fmt"
	"log"

	"anahita-libs/event"
	"graph/data/repositories"
)

// Invites removes the invitations that have run out: the ones nobody
// answered within their lifetime, and the refusals whose cooldown is over.
//
// Each one that lapsed unanswered is announced as deleted, so the invited
// person's notification goes with it. Without a publisher the invitations
// still go; the notification stays, and its buttons answer that there is
// nothing to accept.
func Invites(repos *repositories.Repositories, publisher *event.Publisher) int {
	lapsed, err := repos.Invites.Sweep()
	if err != nil {
		log.Printf("sweeper: invitations: %v", err)
		return 0
	}

	if len(lapsed) > 0 {
		log.Printf("sweeper: %d invitation(s) lapsed", len(lapsed))
	}

	if publisher == nil {
		return len(lapsed)
	}

	for _, invite := range lapsed {
		routingKey := fmt.Sprintf(event.EventFormatEdgeInviteDeleted, "socialgraph")
		publisher.Publish(&event.EventPayload{
			Name:          routingKey,
			SourceService: "graph-grpc-service",
			ObjectID:      invite.InviteeID,
			TargetID:      invite.ActorID,
			OwnerID:       invite.InviterID,
		}, routingKey)
	}

	return len(lapsed)
}
