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
		user.GivenName,
		user.FamilyName,
	)

	// Point at the auth-service form. The token rides as a query
	// param; the handler re-validates it on the GET and again on the
	// POST.
	actionURL := fmt.Sprintf(
		"%s/password/reset?token=%s",
		h.Config.GetAuthURL(),
		token.Body,
	)

	data := map[string]string{
		"body":        body,
		"actionLabel": messages.PasswordActionLabel,
		"actionURL":   actionURL,
		"title":       messages.PasswordTitle,
	}

	return h.Srv.MailGRPC.SendFromTemplate(
		h.Config.SystemEmail,
		[]string{user.Email},
		[]string{},
		[]string{},
		messages.PasswordSubject,
		"message_action",
		data,
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
		user.GivenName,
		user.FamilyName,
	)

	actionURL := fmt.Sprintf(
		"%s/password/forgot",
		h.Config.GetAuthURL(),
	)

	data := map[string]string{
		"body":        body,
		"actionLabel": messages.PasswordResetActionLabel,
		"actionURL":   actionURL,
		"title":       messages.PasswordResetTitle,
	}

	return h.Srv.MailGRPC.SendFromTemplate(
		h.Config.SystemEmail,
		[]string{user.Email},
		[]string{},
		[]string{},
		messages.PasswordResetSubject,
		"message_action",
		data,
	)
}

// EmailUpdateSuccess notifies the user that their password was
// changed via the authenticated settings flow (PATCH /password/change).
//
// Same recovery as EmailResetSuccess: if this email is unexpected,
// the attacker chose the new password and the legitimate user no
// longer knows it. Signing in wouldn't work — they'd need the
// attacker's password. Forgot-password is what takes the account
// back, because it invalidates whatever the attacker set. The
// action button starts that recovery.
func (h *passwordNotifier) EmailUpdateSuccess(user *entities.User) error {
	body := fmt.Sprintf(
		messages.PasswordUpdateBody,
		user.GivenName,
		user.FamilyName,
	)

	actionURL := fmt.Sprintf(
		"%s/password/forgot",
		h.Config.GetAuthURL(),
	)

	data := map[string]string{
		"body":        body,
		"actionLabel": messages.PasswordUpdateActionLabel,
		"actionURL":   actionURL,
		"title":       messages.PasswordUpdateTitle,
	}

	return h.Srv.MailGRPC.SendFromTemplate(
		h.Config.SystemEmail,
		[]string{user.Email},
		[]string{},
		[]string{},
		messages.PasswordUpdateSubject,
		"message_action",
		data,
	)
}
