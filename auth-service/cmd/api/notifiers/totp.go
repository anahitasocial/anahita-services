package notifiers

import (
	"fmt"

	"auth/messages"

	"auth/data/entities"
)

type totpNotifier struct {
	*Env
}

func newTotpNotifier(env *Env) *totpNotifier {
	return &totpNotifier{
		Env: env,
	}
}

// EmailTotpEnabled sends an email to the user when TOTP is enabled.
func (h *totpNotifier) EmailTotpEnabled(user *entities.User) error {
	body := fmt.Sprintf(
		messages.TotpEnabledBody,
		user.GivenName,
		user.FamilyName,
	)

	actionURL := fmt.Sprintf("%s/support", h.Config.GetCompleteDomainName())

	data := map[string]string{
		"body":        body,
		"actionLabel": messages.TotpEnabledActionLabel,
		"actionURL":   actionURL,
		"title":       messages.TotpEnabledTitle,
	}

	err := h.Srv.MailGRPC.SendFromTemplate(
		h.Config.SystemEmail,
		[]string{user.Email},
		[]string{},
		[]string{},
		messages.TotpEnabledSubject,
		"message_action",
		data,
	)
	if err != nil {
		return err
	}

	return nil
}

// EmailTotpDisabled sends an email to the user when TOTP is disabled.
func (h *totpNotifier) EmailTotpDisabled(user *entities.User) error {
	body := fmt.Sprintf(
		messages.TotpDisabledBody,
		user.GivenName,
		user.FamilyName,
	)

	actionURL := fmt.Sprintf("%s/support", h.Config.GetCompleteDomainName())

	data := map[string]string{
		"body":        body,
		"actionLabel": messages.TotpDisabledActionLabel,
		"actionURL":   actionURL,
		"title":       messages.TotpDisabledTitle,
	}

	err := h.Srv.MailGRPC.SendFromTemplate(
		h.Config.SystemEmail,
		[]string{user.Email},
		[]string{},
		[]string{},
		messages.TotpDisabledSubject,
		"message_action",
		data,
	)
	if err != nil {
		return err
	}

	return nil
}
