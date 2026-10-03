package event

const (
	AmqpConnCounts    = 15
	EventExchangeName = "anahita_events"
	QueueMessageTTL   = 86400000 // 1 day

	// Auth events
	EventFormatEntityUserSignup   = "entity.auth-service.user.signup.v1"
	EventFormatEntityUserRegister = "entity.auth-service.user.register.v1"

	// Node events
	// Node Format: node.<type>.<service>.<entity>.<action>.<version>
	// Edge Format: edge.<type>.<service>.<action>.<version>

	EventFormatNodeActorAdded   = "node.actor.%s.%s.added.v1"
	EventFormatNodeActorEdited  = "node.actor.%s.%s.edited.v1"
	EventFormatNodeActorDeleted = "node.actor.%s.%s.deleted.v1"

	// Purged is the second half of deletion, published by the sweeper once the
	// grace period has expired and the content is gone for good.
	//
	// Distinct from "deleted", which now means "scheduled": consumers react to
	// the two differently. Deleted revokes credentials but leaves the account
	// restorable; purged removes what is left of it and cannot be undone.
	EventFormatNodeActorPurged = "node.actor.%s.%s.purged.v1"

	// Restored is the third outcome of a deletion: the grace period ended in
	// the account coming back rather than being purged.
	//
	// A consumer that reacted to "deleted" needs this to undo whatever it did.
	// Without it the only way to notice a restore is to poll, and anything
	// that revoked access on the way down stays revoked on the way up.
	EventFormatNodeActorRestored = "node.actor.%s.%s.restored.v1"

	// The reversible switch and the permanent one. Consumers react
	// differently: disabled is temporary and enabled undoes it, while archived
	// is forever and nothing undoes it.
	EventFormatNodeActorArchived = "node.actor.%s.%s.archived.v1"
	EventFormatNodeActorDisabled = "node.actor.%s.%s.disabled.v1"
	EventFormatNodeActorEnabled  = "node.actor.%s.%s.enabled.v1"

	EventFormatNodeMediumAdded   = "node.medium.%s.%s.added.v1"
	EventFormatNodeMediumEdited  = "node.medium.%s.%s.edited.v1"
	EventFormatNodeMediumDeleted = "node.medium.%s.%s.deleted.v1"

	EventFormatNodeCommentAdded   = "node.comment.comment.%s.added.v1"
	EventFormatNodeCommentEdited  = "node.comment.comment.%s.edited.v1"
	EventFormatNodeCommentDeleted = "node.comment.comment.%s.deleted.v1"

	EventFormatNodeRepostAdded   = "node.repost.repost.%s.added.v1"
	EventFormatNodeRepostDeleted = "node.repost.repost.%s.deleted.v1"

	// Edge events
	EventFormatEdgeVoteupAdded   = "edge.voteup.%s.added.v1"
	EventFormatEdgeVoteupDeleted = "edge.voteup.%s.deleted.v1"

	EventFormatEdgeFollowAdded   = "edge.follow.%s.added.v1"
	EventFormatEdgeFollowDeleted = "edge.follow.%s.deleted.v1"

	EventFormatEdgeFollowRequestAdded   = "edge.follow-request.%s.added.v1"
	EventFormatEdgeFollowRequestDeleted = "edge.follow-request.%s.deleted.v1"

	EventFormatEdgeBlockAdded   = "edge.block.%s.added.v1"
	EventFormatEdgeBlockDeleted = "edge.block.%s.deleted.v1"

	EventFormatEdgeSubscribeAdded   = "edge.subscribe.%s.added.v1"
	EventFormatEdgeSubscribeDeleted = "edge.subscribe.%s.deleted.v1"

	EventFormatEdgeAdminAdded   = "edge.admin.%s.added.v1"
	EventFormatEdgeAdminDeleted = "edge.admin.%s.deleted.v1"

	EventFormatEdgeLocationAdded   = "edge.location.%s.added.v1"
	EventFormatEdgeLocationDeleted = "edge.location.%s.deleted.v1"
)
