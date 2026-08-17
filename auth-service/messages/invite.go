package messages

// NOTE: these carried %s verbs for a site name that is not available to
// the invite notifier — Env has no site-name config and nothing plumbs
// one in. InviteSubject/InviteTitle/InviteAcceptSubject were also passed
// through raw rather than through Sprintf, so they rendered the literal
// text "%s", and the two Sprintf'd bodies emitted "%!s(MISSING)".
//
// The unfillable verbs are removed rather than guessed at. If a site
// name should appear here, plumb one into notifiers.Env and add the
// verbs back with arguments.
const (
	InviteSubject     = "You're invited"
	InviteTitle       = "You're invited"
	InviteBody        = "%s %s invited you to join. Click the button below to set up your account."
	InviteActionLabel = "Accept invitation"

	InviteAcceptSubject     = "A new member joined"
	InviteAcceptTitle       = "New member joined"
	InviteAcceptActionLabel = "View member"

	InviteAcceptBodyToInviter = "Hi %s, %s %s accepted your invitation and now has an account."

	InviteAcceptBody = "Hi %s, %s %s has joined after accepting an invitation from %s %s."
)
