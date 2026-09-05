package notifiers

import (
	"fmt"
	"time"

	"auth/constants"
	"auth/data/entities"
	"auth/messages"
)

type accountDeleteNotifier struct {
	*Env
}

func newAccountDeleteNotifier(env *Env) *accountDeleteNotifier {
	return &accountDeleteNotifier{env}
}

// EmailScheduled tells the person their account is on its way out, and when.
//
// The one message that has to arrive. Deletion is gated on a step-up, so an
// unexpected one means somebody got past a passkey or the password — and the
// grace period is the only window in which that is still recoverable. A
// deletion nobody is told about is a deletion nobody can stop.
//
// Sent to the address on the account, which still works: the account is
// disabled, not erased.
func (h *accountDeleteNotifier) EmailScheduled(person *entities.User, deletedAt time.Time) error {
	purgeDate := deletedAt.
		Add(time.Duration(constants.AccountGraceDays) * 24 * time.Hour).
		Format("2 January 2006")

	body := fmt.Sprintf(
		messages.AccountDeleteScheduledBody,
		person.GivenName,
		person.FamilyName,
		purgeDate,
	)

	actionURL := fmt.Sprintf("%s/oauth/login", h.Config.GetAuthURL())

	return h.send(
		person.Email,
		messages.AccountDeleteScheduledSubject,
		messages.AccountDeleteScheduledTitle,
		body,
		messages.AccountDeleteScheduledActionLabel,
		actionURL,
	)
}

// EmailRestored confirms a restore.
//
// Names what did NOT come back — sessions and passkeys were revoked when the
// deletion was requested and restoring does not reinstate them. Somebody
// signing in on their phone and finding their passkey gone would otherwise read
// that as the restore having half-worked.
func (h *accountDeleteNotifier) EmailRestored(person *entities.User) error {
	body := fmt.Sprintf(
		messages.AccountRestoredBody,
		person.GivenName,
		person.FamilyName,
	)

	actionURL := fmt.Sprintf("%s/oauth/login", h.Config.GetAuthURL())

	return h.send(
		person.Email,
		messages.AccountRestoredSubject,
		messages.AccountRestoredTitle,
		body,
		messages.AccountRestoredActionLabel,
		actionURL,
	)
}
