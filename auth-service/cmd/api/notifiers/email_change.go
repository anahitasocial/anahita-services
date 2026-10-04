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

// EmailCode sends the code that approves a pending email change. It goes to
// the CURRENT address, and names the new one so the owner can see where the
// account is being moved to. No link: the settings page that wants the code
// is already open.
func (h *emailChangeNotifier) EmailCode(person *entities.User, newEmail, code string) error {
	return h.sendCode(
		person.Email,
		fmt.Sprintf(messages.EmailChangeCodeSubject, code),
		messages.EmailChangeCodeTitle,
		fmt.Sprintf(messages.EmailChangeCodeBody, person.Name, newEmail),
		code,
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
		person.Name,
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
		person.Name,
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
