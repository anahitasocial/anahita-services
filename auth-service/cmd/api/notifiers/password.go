package notifiers

import (
	"fmt"

	"auth/data/entities"
	"auth/messages"
)

type passwordNotifier struct {
	*Env
}

func newPasswordNotifier(env *Env) *passwordNotifier {
	return &passwordNotifier{env}
}

// EmailToken sends the password reset email with a link to the
// server-rendered reset form on the auth-service host.
func (h *passwordNotifier) EmailToken(user *entities.User, token *entities.Token) error {
	body := fmt.Sprintf(
		messages.PasswordBody,
		user.Name,
	)

	// Point at the auth-service form. The token rides as a query
	// param; the handler re-validates it on the GET and again on the
	// POST.
	actionURL := fmt.Sprintf(
		"%s/password/reset?token=%s",
		h.Config.GetAuthURL(),
		token.Body,
	)

	return h.send(
		user.Email,
		messages.PasswordSubject,
		messages.PasswordTitle,
		body,
		messages.PasswordActionLabel,
		actionURL,
	)
}

// EmailResetSuccess notifies the user that their password was reset
// via the forgot-password email-link flow.
//
// If this email arrives unexpectedly, an attacker has gained access
// to the user's email account and reset the password through the
// email link. The right recovery is to request a fresh reset (which
// invalidates the attacker's password) and then secure the email
// account before contacting support. The action button starts that
// recovery — back to the forgot-password page.
func (h *passwordNotifier) EmailResetSuccess(user *entities.User) error {
	body := fmt.Sprintf(
		messages.PasswordResetBody,
		user.Name,
	)

	actionURL := fmt.Sprintf(
		"%s/password/forgot",
		h.Config.GetAuthURL(),
	)

	return h.send(
		user.Email,
		messages.PasswordResetSubject,
		messages.PasswordResetTitle,
		body,
		messages.PasswordResetActionLabel,
		actionURL,
	)
}

// EmailUpdateSuccess notifies the user that their password was
// changed via the authenticated settings flow (PATCH /password/change).
//
// revokedSessions is how many other signed-in sessions the change
// ended, and is reported in the body — see the switch below.
//
// Same recovery as EmailResetSuccess: if this email is unexpected,
// the attacker chose the new password and the legitimate user no
// longer knows it. Signing in wouldn't work — they'd need the
// attacker's password. Forgot-password is what takes the account
// back, because it invalidates whatever the attacker set. The
// action button starts that recovery.
func (h *passwordNotifier) EmailUpdateSuccess(user *entities.User, revokedSessions int) error {
	body := fmt.Sprintf(
		messages.PasswordUpdateBody,
		user.Name,
	)

	// Say what else the change did. Someone reading this to work out
	// whether they have been attacked is served by knowing that four
	// other sessions just ended — it either matches what they did or
	// tells them how far in the attacker already was.
	switch {
	case revokedSessions == 1:
		body += messages.PasswordUpdateSessionsOne
	case revokedSessions > 1:
		body += fmt.Sprintf(messages.PasswordUpdateSessionsMany, revokedSessions)
	}

	actionURL := fmt.Sprintf(
		"%s/password/forgot",
		h.Config.GetAuthURL(),
	)

	return h.send(
		user.Email,
		messages.PasswordUpdateSubject,
		messages.PasswordUpdateTitle,
		body,
		messages.PasswordUpdateActionLabel,
		actionURL,
	)
}
