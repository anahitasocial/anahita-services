package notifiers

import (
	"errors"
	"fmt"
	"log"

	"auth/constants"
	"auth/data/entities"
	"auth/messages"
	"auth/requests"

	an_constants "anahita-libs/constants"
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

// EmailSignupRequestReceived tells the administrators somebody is waiting.
//
// Sent once the address is CONFIRMED, not when the request is submitted —
// otherwise this is the thing a spam wave turns into a mail flood aimed at the
// people who would otherwise be dealing with it.
//
// Errors are accumulated rather than short-circuiting, so one administrator
// with a dead address does not stop the others being told.
func (h *signupNotifier) EmailSignupRequestReceived(sr *entities.SignupRequest) error {
	personReq := requests.NewUserRequest().Browse
	personReq.PersonTypes = []string{
		an_constants.PersonTypeAdmin,
		an_constants.PersonTypeSuperAdmin,
	}

	admins, err := h.Repos.Users.Browse(personReq)
	if err != nil {
		return err
	}

	// Straight to the queue rather than the front page. An administrator
	// clicking through from this mail is coming to do one thing.
	actionURL := fmt.Sprintf("%s/settings/signup-requests", h.Config.GetFrontendURL())

	var sendErrors []error
	for _, admin := range admins {
		body := fmt.Sprintf(
			messages.SignupRequestReceivedBody,
			admin.Name,
			sr.Name,
			sr.Username,
			sr.Body,
		)

		if err := h.send(
			admin.Email,
			messages.SignupRequestReceivedSubject,
			messages.SignupRequestReceivedTitle,
			body,
			messages.SignupRequestReceivedActionLabel,
			actionURL,
		); err != nil {
			sendErrors = append(sendErrors,
				fmt.Errorf("signup request notice to admin %d failed: %w", admin.ID, err))
		}
	}

	return errors.Join(sendErrors...)
}

// EmailSignupRequestApproved tells somebody they are in.
func (h *signupNotifier) EmailSignupRequestApproved(sr *entities.SignupRequest) error {
	greeting := sr.Name
	if greeting == "" {
		greeting = sr.Username
	}

	return h.send(
		sr.Email,
		messages.SignupRequestApprovedSubject,
		messages.SignupRequestApprovedTitle,
		fmt.Sprintf(messages.SignupRequestApprovedBody, greeting),
		messages.SignupRequestApprovedActionLabel,
		fmt.Sprintf("%s/oauth/login", h.Config.GetAuthURL()),
	)
}

// EmailSignupRequestRejected tells somebody they are not.
//
// Sent, rather than silence. Somebody who is never coming should be told so
// instead of waiting — silence reads as a broken site, and the person keeps
// checking. No reason is given: a rejection note written for a moderator's
// records is not written to be read by its subject, and passing it on turns
// every decision into an argument.
func (h *signupNotifier) EmailSignupRequestRejected(sr *entities.SignupRequest) error {
	greeting := sr.Name
	if greeting == "" {
		greeting = sr.Username
	}

	return h.send(
		sr.Email,
		messages.SignupRequestRejectedSubject,
		messages.SignupRequestRejectedTitle,
		fmt.Sprintf(messages.SignupRequestRejectedBody, greeting),
		messages.SignupRequestRejectedActionLabel,
		h.Config.GetFrontendURL(),
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
