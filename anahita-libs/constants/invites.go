package constants

import "time"

// Invitations: a person asked to follow a group, who then says yes or no.
//
// An invitation is an edge (EdgeActorInviteEntityType) from the person
// invited to the actor, created by whoever invited them. One for each person
// and actor. Its state is in the edge's meta, and when it lapses in ends_at.
const (
	// InviteMetaPending and InviteMetaDeclined are the two things the
	// edge's meta says. Written whole and compared whole, so a query can
	// match them without reading JSON.
	InviteMetaPending  = `{"status":"pending"}`
	InviteMetaDeclined = `{"status":"declined"}`

	InviteStatusPending  = "pending"
	InviteStatusDeclined = "declined"

	// InviteLifetime is how long an invitation waits for an answer.
	InviteLifetime = 30 * 24 * time.Hour
	// InviteCooldown is how long after a refusal the same person cannot be
	// invited to the same actor again. The refused invitation is kept that
	// long, which is what stops the next one.
	InviteCooldown = 90 * 24 * time.Hour

	// InviteMaxPerRequest is how many people one request may invite.
	InviteMaxPerRequest = 50
	// InviteMaxPerDay is how many invitations one person may have made in
	// the last day and still be waiting on.
	InviteMaxPerDay = 200
	// InviteMaxPending is how many invitations to one actor may be waiting.
	InviteMaxPending = 500
)

// What became of each person named in a request to invite.
const (
	InviteResultInvited         = "invited"
	InviteResultAlreadyFollower = "already_follower"
	InviteResultNotFollower     = "not_follower"
	InviteResultBlocked         = "blocked"
	InviteResultCooldown        = "cooldown"
	InviteResultPending         = "pending"
	InviteResultRequested       = "requested"
	InviteResultUnavailable     = "unavailable"
	InviteResultFull            = "full"
)
