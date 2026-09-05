package notifiers

import (
	"fmt"

	"auth/data/entities"
	"auth/messages"
)

type webAuthnNotifier struct {
	*Env
}

func newWebAuthnNotifier(env *Env) *webAuthnNotifier {
	return &webAuthnNotifier{env}
}

// EmailPasskeyAdded notifies a user that a new passkey was enrolled.
//
// Neutral tone, matching TotpEnabled. Enrolling a passkey strengthens
// the account; an alarmist alert on a routine security improvement
// trains people to ignore the alerts that matter. The recovery
// instruction is still there for the case where it wasn't them.
func (h *webAuthnNotifier) EmailPasskeyAdded(user *entities.User, nickname string) error {
	body := fmt.Sprintf(
		messages.WebAuthnAddedBody,
		user.GivenName,
		user.FamilyName,
		nickname,
	)

	return h.sendPasskeyEmail(
		user,
		messages.WebAuthnAddedSubject,
		messages.WebAuthnAddedTitle,
		body,
		messages.WebAuthnAddedActionLabel,
		h.passwordResetURL(),
	)
}

// EmailPasskeyRemoved notifies a person that a passkey was revoked.
//
// Firmer tone than the added case. Removing a sign-in method is the
// direction an account takeover moves in — an attacker who has
// enrolled their own passkey will often remove the real owner's.
// Unlike TOTP removal this is not a lockout: password and TOTP remain,
// so the message points at password reset rather than panic.
func (h *webAuthnNotifier) EmailPasskeyRemoved(user *entities.User, nickname string) error {
	body := fmt.Sprintf(
		messages.WebAuthnRemovedBody,
		user.GivenName,
		user.FamilyName,
		nickname,
	)

	return h.sendPasskeyEmail(
		user,
		messages.WebAuthnRemovedSubject,
		messages.WebAuthnRemovedTitle,
		body,
		messages.WebAuthnRemovedActionLabel,
		h.passwordResetURL(),
	)
}

// EmailPasskeysRevoked notifies a person that an administrator removed
// their passkeys.
//
// count is included because "your passkeys were removed" reads very
// differently depending on whether that was one or five, and the
// person has no way to check what was there before.
//
// Two message constants rather than one with "passkey(s)". Go has no
// plural machinery and this is a security email — the kind people
// actually read, and the kind where a sloppy "1 passkey(s)" makes the
// whole thing look automated and ignorable.
//
// The caller gates on count > 0. There is nothing to tell someone
// about a revocation that removed nothing.
//
// The action points at support rather than password reset: unlike the
// other two, the person did not do this and has nothing to fix — they
// want to know why it happened.
func (h *webAuthnNotifier) EmailPasskeysRevoked(user *entities.User, count int64) error {
	template := messages.WebAuthnRevokedAllBody
	if count == 1 {
		template = messages.WebAuthnRevokedAllBodyOne
	}

	body := fmt.Sprintf(
		template,
		user.GivenName,
		user.FamilyName,
		count,
	)

	return h.sendPasskeyEmail(
		user,
		messages.WebAuthnRevokedAllSubject,
		messages.WebAuthnRevokedAllTitle,
		body,
		messages.WebAuthnRevokedAllActionLabel,
		h.supportURL(),
	)
}

// sendPasskeyEmail is the single dispatch point for all three
// notifications.
//
// actionURL is passed in rather than derived here because the three
// messages send people to different places — two to password reset,
// one to support — and a switch on the subject line to work that out
// would be worse than a parameter.
func (h *webAuthnNotifier) sendPasskeyEmail(
	user *entities.User,
	subject string,
	title string,
	body string,
	actionLabel string,
	actionURL string,
) error {
	return h.send(
		user.Email,
		subject,
		title,
		body,
		actionLabel,
		actionURL,
	)
}

// passwordResetURL is where the "this wasn't me" path leads for the
// added and removed notifications. Same target EmailLogin uses.
func (h *webAuthnNotifier) passwordResetURL() string {
	return fmt.Sprintf("%s/password/forgot", h.Config.GetAuthURL())
}

// supportURL is where the admin-revocation notification leads.
//
// Support rather than password reset: unlike the added and removed
// notifications, the person did not do this and has nothing to fix.
// What they want is to know why it happened.
//
// The support page is a public route in the Anahita web app, which
// matters here — this email exists partly for the case where someone
// has lost control of their account, and a link that demanded a
// login first would be useless at exactly that moment.
func (h *webAuthnNotifier) supportURL() string {
	return fmt.Sprintf("%s/support", h.Config.GetFrontendURL())
}
