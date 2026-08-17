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
		user.GivenName,
		user.FamilyName,
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

	data := map[string]string{
		"body":        body,
		"actionLabel": messages.AuthLoginActionLabel,
		"actionURL":   actionURL,
		"title":       messages.AuthLoginTitle,
	}

	err := h.Srv.MailGRPC.SendFromTemplate(
		h.Config.SystemEmail,
		[]string{user.Email},
		[]string{},
		[]string{},
		messages.AuthLoginSubject,
		"message_action",
		data,
	)
	if err != nil {
		return err
	}

	return nil
}
