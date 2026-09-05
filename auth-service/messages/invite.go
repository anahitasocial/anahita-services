package messages

// An invitation is the one message here sent to somebody who does not yet have
// an account, so it is the one with the least context to lean on. Naming the
// site is not decoration: "%s invited you to join" gives the reader nothing to
// decide with, and an invitation to an unnamed thing reads exactly like spam.
//
// These constants previously carried the site as a `%s`, and the notifier
// passed the subject and title to the mailer without formatting them — so
// invitees were sent "You're invited to %s". SiteToken is substituted in one
// place for every message, which is what stops that recurring; see
// messages.go.
const (
	// Two verbs on the body: inviter's given name, inviter's family name.
	InviteSubject     = "You're invited to {{site}}"
	InviteTitle       = "You're invited to {{site}}"
	InviteBody        = "%s %s invited you to join {{site}}. Click the button below to set up your account."
	InviteActionLabel = "Accept invitation"

	// Sent to administrators when somebody accepts. One subject and title
	// serve two bodies, because the inviter and everyone else need different
	// sentences — the inviter is being told their invitation landed, the rest
	// are being told who joined and on whose invitation.
	InviteAcceptSubject     = "A new member joined {{site}}"
	InviteAcceptTitle       = "New member joined"
	InviteAcceptActionLabel = "View member"

	// Three verbs: recipient's given name, joiner's given name, joiner's
	// family name.
	InviteAcceptBodyToInviter = "Hi %s, %s %s accepted your invitation and now has an account."

	// Five verbs: recipient's given name, joiner's given name, joiner's family
	// name, inviter's given name, inviter's family name.
	InviteAcceptBody = "Hi %s, %s %s has joined {{site}} after accepting an invitation from %s %s."

	// Sent to somebody whose pending invitation was withdrawn before they
	// used it.
	//
	// Neutral by design. The ordinary reason is administrative — the wrong
	// address, or a change of plan — and an invitation that simply stops
	// working with no word at all is worse than a short note saying so.
	//
	// Two verbs: revoking administrator's given name, family name.
	InviteRevokedSubject = "Your invitation to {{site}} was withdrawn"
	InviteRevokedTitle   = "Invitation withdrawn"
	InviteRevokedBody    = "%s %s has withdrawn your invitation to join {{site}}. The link you were sent no longer works. If you were expecting to join, contact the person who invited you."

	// There is nothing for this recipient to do on the site — they have no
	// account — so the button goes to the front page rather than inventing a
	// destination. Kept because every message in this system carries an
	// action, and a template with an empty one renders a dead button.
	InviteRevokedActionLabel = "Go to {{site}}"
)
