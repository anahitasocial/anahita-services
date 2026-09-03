package notifiers

import (
	"fmt"

	"auth/data/entities"
	"auth/messages"
)

type usernameChangeNotifier struct {
	*Env
}

func newUsernameChangeNotifier(env *Env) *usernameChangeNotifier {
	return &usernameChangeNotifier{env}
}

// EmailChanged tells the person their username moved.
//
// This is the whole detection story for the flow, which is why it is
// sent even though the change was already gated on a step-up. The gate
// establishes that somebody proved a passkey or the password; it does
// not establish that the person owns the hands that did it.
//
// Takes oldUsername explicitly, because by the time this is called the
// person record already carries the new one.
//
// The action points at the password settings rather than a reset link.
// Whoever made this change passed a step-up, so the credential to
// assume compromised is the password itself — and unlike an email
// change, the person still controls the mailbox, so an ordinary signed-
// in change is available to them and is the faster path.
func (h *usernameChangeNotifier) EmailChanged(person *entities.User, oldUsername string) error {
	body := fmt.Sprintf(
		messages.UsernameChangedBody,
		person.GivenName,
		person.FamilyName,
		oldUsername,
		person.Username,
	)

	// Built from the NEW handle, which is also the new profile alias —
	// EditUsername writes both together, so this resolves. The old one
	// would 404 by the time the mail is read.
	actionURL := fmt.Sprintf(
		"%s/people/%s/settings/password",
		h.Config.GetFrontendURL(),
		person.Username,
	)

	data := map[string]string{
		"body":        body,
		"actionLabel": messages.UsernameChangedActionLabel,
		"actionURL":   actionURL,
		"title":       messages.UsernameChangedTitle,
	}

	return h.Srv.MailGRPC.SendFromTemplate(
		h.Config.SystemEmail,
		[]string{person.Email},
		[]string{},
		[]string{},
		messages.UsernameChangedSubject,
		"message_action",
		data,
	)
}
