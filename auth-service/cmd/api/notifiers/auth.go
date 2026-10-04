package notifiers

import (
	"fmt"

	"auth/data/entities"
	"auth/messages"
)

type authNotifier struct {
	*Env
}

func newAuthNotifier(env *Env) *authNotifier {
	return &authNotifier{env}
}

// EmailLogin sends an email notification to the user upon successful login
func (h *authNotifier) EmailLogin(user *entities.User, authLog *entities.AuthLog) error {
	body := fmt.Sprintf(
		messages.AuthLoginBody,
		user.Name,
		authLog.Name,
		authLog.Device,
		authLog.OS,
		authLog.City,
		authLog.Country,
	)

	actionURL := fmt.Sprintf(
		"%s/password/forgot",
		h.Config.GetAuthURL(),
	)

	return h.send(
		user.Email,
		messages.AuthLoginSubject,
		messages.AuthLoginTitle,
		body,
		messages.AuthLoginActionLabel,
		actionURL,
	)
}

// EmailSessionRevoked tells somebody an administrator ended one of their
// sessions.
//
// device and location describe the session that went, because "a session was
// signed out" is not actionable — the person needs to recognise which one, and
// the auth log row already holds both.
//
// Only for administrative revocations. Somebody ending their own session is
// looking at the screen where they did it, and mailing them about it is the
// noise that teaches people to filter these messages.
// EmailLoginCode sends a sign-in code. No action link: see the note on
// messages.AuthLoginCodeBody.
func (h *authNotifier) EmailLoginCode(user *entities.User, code string) error {
	return h.sendCode(
		user.Email,
		fmt.Sprintf(messages.AuthLoginCodeSubject, code),
		messages.AuthLoginCodeTitle,
		fmt.Sprintf(messages.AuthLoginCodeBody, user.Name),
		code,
	)
}

func (h *authNotifier) EmailSessionRevoked(user *entities.User, device, location string) error {
	body := fmt.Sprintf(
		messages.AuthSessionRevokedBody,
		user.Name,
		device,
		location,
	)

	return h.send(
		user.Email,
		messages.AuthSessionRevokedSubject,
		messages.AuthSessionRevokedTitle,
		body,
		messages.AuthSessionRevokedActionLabel,
		fmt.Sprintf("%s/oauth/login", h.Config.GetAuthURL()),
	)
}
