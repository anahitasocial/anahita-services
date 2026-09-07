package messages

const (
	// Sent to each administrator of a group that has been scheduled for
	// deletion.
	//
	// This is the notification the plan recorded as missing entirely:
	// "group deletion notifies nobody". A group can have several
	// administrators, and any one of them can delete it — so the others could
	// lose a shared space and find out by visiting it.
	//
	// It names the group, and it names the window, because during the window
	// this is reversible and afterwards it is not. Somebody reading this the
	// next morning needs to know which of those they are in.
	//
	// Sent to the person who did it as well as to the others. They know, but a
	// deletion that mails everyone except the actor is a deletion where the
	// record of who was told depends on who did it — and the copy reads the
	// same either way, so there is nothing to get wrong.
	//
	// Three verbs: given name, family name, group name.
	GroupDeletedSubject     = "The {{site}} group \"%s\" is scheduled for deletion"
	GroupDeletedTitle       = "A group you administer is scheduled for deletion"
	GroupDeletedBody        = "Hello %s %s, the group \"%s\" that you administer on {{site}} has been scheduled for deletion. It is no longer visible to members, and its posts, photos and discussions will be permanently removed when the grace period ends. Until then this can still be undone by any of its administrators. If this was not intended, restore it now."
	GroupDeletedActionLabel = "Restore the group"

	// Sent to each administrator when a group comes back.
	//
	// The counterpart matters as much as the deletion notice. Somebody who
	// received the message above and did nothing needs to know the outcome,
	// and somebody who did not delete it needs to know that a space they
	// administer is live again.
	//
	// Three verbs: given name, family name, group name.
	GroupRestoredSubject     = "The {{site}} group \"%s\" has been restored"
	GroupRestoredTitle       = "A group you administer has been restored"
	GroupRestoredBody        = "Hello %s %s, the group \"%s\" that you administer on {{site}} has been restored before its grace period ended. It is visible to members again, and nothing was lost. If you did not expect this, review who administers the group."
	GroupRestoredActionLabel = "Open the group"
)
