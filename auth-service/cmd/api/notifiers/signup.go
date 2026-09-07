package notifiers

import (
	"fmt"
	"log"

	"auth/constants"
	"auth/data/entities"
	"auth/messages"

	an_entities "anahita-libs/data/entities"
	an_event "anahita-libs/event"
)

type signupNotifier struct {
	*Env
}

func newSignupNotifier(env *Env) *signupNotifier {
	return &signupNotifier{env}
}

// NotifySignup publishes the in-app event for a new self-serve org
// signup. The notification-service handles fan-out from there.
func (h *signupNotifier) NotifySignup(person *an_entities.Person) error {
	payload := &an_event.EventPayload{
		Name:          an_event.EventFormatEntityUserSignup,
		SourceService: constants.ConfigServiceName,
		SubjectID:     person.ID,
	}

	log.Println("Publishing event", payload.Name)
	if err := h.Publisher.Publish(payload, payload.Name); err != nil {
		return err
	}

	return nil
}

// EmailSignup welcomes a brand-new self-serve user. Invitees go
// through inviteNotifier.EmailAccept instead.
func (h *signupNotifier) EmailSignup(user *entities.User) error {
	body := fmt.Sprintf(
		messages.SignupBody,
		user.GivenName,
		user.FamilyName,
	)

	actionURL := fmt.Sprintf(
		"%s/downloads",
		h.Config.GetCompleteDomainName(),
	)

	return h.send(
		user.Email,
		messages.SignupSubject,
		messages.SignupTitle,
		body,
		messages.SignupActionLabel,
		actionURL,
	)
}
