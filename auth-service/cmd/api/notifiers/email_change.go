package notifiers

import (
	"fmt"

	"auth/data/entities"
	"auth/messages"
)

type emailChangeNotifier struct {
	*Env
}

func newEmailChangeNotifier(env *Env) *emailChangeNotifier {
	return &emailChangeNotifier{env}
}

// EmailConfirm sends the confirmation link for a pending email change.
//
// SENT TO THE PERSON'S CURRENT ADDRESS, NOT THE NEW ONE. That is the
// whole security property of this flow and the easiest thing to get
// backwards when copying the password-reset notifier: the current inbox
// is what an attacker with a stolen session does not have, so it is the
// thing that must approve the move. Sending to the new address instead
// would let anyone inside a live session repoint the account to
// themselves and confirm it.
//
// token.RecipientEmail is that current address, and token.NewEmail is
// the destination — named in the body so the person can see what they
// are approving before they click.
func (h *emailChangeNotifier) EmailConfirm(person *entities.User, token *entities.Token) error {
	body := fmt.Sprintf(
		messages.EmailChangeBody,
		person.GivenName,
		person.FamilyName,
		token.NewEmail,
	)

	actionURL := fmt.Sprintf(
		"%s/email/change/confirm?token=%s",
		h.Config.GetAuthURL(),
		token.Body,
	)

	return h.send(
		token.RecipientEmail,
		messages.EmailChangeSubject,
		messages.EmailChangeTitle,
		body,
		messages.EmailChangeActionLabel,
		actionURL,
	)
}

// EmailChangedOld tells the previous address that the change completed.
//
// This is the tripwire. If the change was not authorised, this is the
// last message that address will ever receive about the account, so it
// has to say plainly that password reset no longer reaches them and
// that support is the only way back. Pointing them at a self-service
// reset would be worse than useless — the link would go to the
// attacker.
//
// Takes oldEmail explicitly rather than reading person.Email, because
// by the time this is called the person record already carries the new
// address.
func (h *emailChangeNotifier) EmailChangedOld(person *entities.User, oldEmail string) error {
	body := fmt.Sprintf(
		messages.EmailChangedOldBody,
		person.GivenName,
		person.FamilyName,
		oldEmail,
		person.Email,
	)

	// GetFrontendURL, not GetCompleteDomainName — the support page lives
	// inside the app, and the two differ in production only. See the
	// note on GetCompleteDomainName.
	actionURL := fmt.Sprintf(
		"%s/support",
		h.Config.GetFrontendURL(),
	)

	return h.send(
		oldEmail,
		messages.EmailChangedOldSubject,
		messages.EmailChangedOldTitle,
		body,
		messages.EmailChangedOldActionLabel,
		actionURL,
	)
}

// EmailChangedNew confirms to the new address that it is now the
// account's address, and that sessions were revoked.
func (h *emailChangeNotifier) EmailChangedNew(person *entities.User) error {
	body := fmt.Sprintf(
		messages.EmailChangedNewBody,
		person.GivenName,
		person.FamilyName,
	)

	actionURL := fmt.Sprintf(
		"%s/oauth/login",
		h.Config.GetAuthURL(),
	)

	return h.send(
		person.Email,
		messages.EmailChangedNewSubject,
		messages.EmailChangedNewTitle,
		body,
		messages.EmailChangedNewActionLabel,
		actionURL,
	)
}
