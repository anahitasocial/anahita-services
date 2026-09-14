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

// EmailConfirm asks a new account to prove control of its address.
//
// Sent INSTEAD of the welcome mail, not alongside it. Two emails arriving
// together, one saying "you're all set" and one saying "confirm to continue",
// is how a confirmation step gets ignored — and this one is not optional,
// since an unverified account cannot sign in. The welcome follows once the
// address is confirmed.
//
// The link goes to auth-service, where the other token-gated pages live.
func (h *signupNotifier) EmailConfirm(user *entities.User, token *entities.Token) error {
	// The username, because an account that has just been created has no
	// display name — signup does not ask for one.
	body := fmt.Sprintf(
		messages.SignupConfirmBody,
		user.Alias,
	)

	actionURL := fmt.Sprintf(
		"%s/signup/confirm?token=%s",
		h.Config.GetAuthURL(),
		token.Body,
	)

	return h.send(
		user.Email,
		messages.SignupConfirmSubject,
		messages.SignupConfirmTitle,
		body,
		messages.SignupConfirmActionLabel,
		actionURL,
	)
}

// EmailSignup welcomes a brand-new self-serve user. Invitees go
// through inviteNotifier.EmailAccept instead.
func (h *signupNotifier) EmailSignup(user *entities.User) error {
	// Falls back to the username: signup does not collect a display name, so
	// a brand-new account has none until onboarding.
	greeting := user.Name
	if greeting == "" {
		greeting = user.Alias
	}

	body := fmt.Sprintf(
		messages.SignupBody,
		greeting,
	)

	// /dashboard, and the frontend URL rather than the "complete domain name".
	//
	// This was "%s/downloads" on GetCompleteDomainName(), and both halves were
	// wrong: Anahita has no downloads route, and that getter builds
	// "https://www." + DOMAIN_NAME, which in development is
	// https://www.localhost:3000 — not a host that exists. GetFrontendURL is
	// the configured address of the web app and the one every other link
	// should use.
	actionURL := fmt.Sprintf(
		"%s/dashboard",
		h.Config.GetFrontendURL(),
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
