package notifiers

import (
	"fmt"

	"auth/data/entities"
	"auth/messages"
)

type reauthNotifier struct {
	*Env
}

func newReauthNotifier(env *Env) *reauthNotifier {
	return &reauthNotifier{env}
}

// EmailLockedOut tells the person that step-up verification is being hammered
// on their account.
//
// Sent ONCE per limiter window, decided by the caller — see the count check in
// reauth.go. Sending on every rejected attempt would turn a guessing attack
// into a mail flood aimed at the person it is meant to warn, which both buries
// the warning and makes the notification itself the more effective attack.
//
// The action points at the sessions list rather than the password form,
// because the step-up limiter is keyed per person: while it is tripped the
// owner cannot change their password either. Signing out a session needs no
// step-up, and the session is the foothold anyway.
func (h *reauthNotifier) EmailLockedOut(user *entities.User) error {
	body := fmt.Sprintf(
		messages.ReauthLockedOutBody,
		user.GivenName,
		user.FamilyName,
	)

	actionURL := fmt.Sprintf(
		"%s/people/%s/settings/security",
		h.Config.GetFrontendURL(),
		user.Username,
	)

	return h.send(
		user.Email,
		messages.ReauthLockedOutSubject,
		messages.ReauthLockedOutTitle,
		body,
		messages.ReauthLockedOutActionLabel,
		actionURL,
	)
}
