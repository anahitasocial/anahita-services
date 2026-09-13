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
		user.Name,
	)

	actionURL := fmt.Sprintf("%s/support", h.Config.GetFrontendURL())

	return h.send(
		user.Email,
		messages.TotpEnabledSubject,
		messages.TotpEnabledTitle,
		body,
		messages.TotpEnabledActionLabel,
		actionURL,
	)
}

// EmailTotpDisabled sends an email to the user when TOTP is disabled.
func (h *totpNotifier) EmailTotpDisabled(user *entities.User) error {
	body := fmt.Sprintf(
		messages.TotpDisabledBody,
		user.Name,
	)

	actionURL := fmt.Sprintf("%s/support", h.Config.GetFrontendURL())

	return h.send(
		user.Email,
		messages.TotpDisabledSubject,
		messages.TotpDisabledTitle,
		body,
		messages.TotpDisabledActionLabel,
		actionURL,
	)
}

// EmailTotpDisabledByAdmin notifies somebody that an administrator turned
// their 2FA off, rather than that they did.
//
// A separate method rather than a flag on the one above, because the two
// messages give different advice and the wrong one is worse than none: telling
// somebody "if this wasn't you, reset your password" about an administrative
// action sends them to reset a credential that was never at risk.
func (h *totpNotifier) EmailTotpDisabledByAdmin(user *entities.User) error {
	body := fmt.Sprintf(
		messages.TotpDisabledByAdminBody,
		user.Name,
	)

	return h.send(
		user.Email,
		messages.TotpDisabledByAdminSubject,
		messages.TotpDisabledByAdminTitle,
		body,
		messages.TotpDisabledByAdminActionLabel,
		fmt.Sprintf("%s/support", h.Config.GetFrontendURL()),
	)
}

// EmailRecoveryCodesIssued notifies that a new set of recovery codes exists.
//
// count is how many were issued, not how many remain — this fires at
// generation time, so the two are the same number, but naming the parameter
// for what it is keeps it from being passed a remaining-count later.
func (h *totpNotifier) EmailRecoveryCodesIssued(user *entities.User, count int) error {
	body := fmt.Sprintf(
		messages.TotpRecoveryCodesIssuedBody,
		user.Name,
		count,
	)

	return h.send(
		user.Email,
		messages.TotpRecoveryCodesIssuedSubject,
		messages.TotpRecoveryCodesIssuedTitle,
		body,
		messages.TotpRecoveryCodesIssuedActionLabel,
		h.passwordChangeURL(user),
	)
}

// EmailRecoveryCodeUsed notifies that a recovery code was spent signing in.
//
// remaining picks the wording. Zero is called out rather than reported as a
// number, because "0 remain" buries the one fact that needs acting on — the
// person is one lost authenticator away from being locked out.
func (h *totpNotifier) EmailRecoveryCodeUsed(user *entities.User, remaining int) error {
	var body string

	switch {
	case remaining <= 0:
		body = fmt.Sprintf(
			messages.TotpRecoveryCodeUsedBodyNone,
			user.Name,
		)
	case remaining == 1:
		body = fmt.Sprintf(
			messages.TotpRecoveryCodeUsedBodyOne,
			user.Name,
		)
	default:
		body = fmt.Sprintf(
			messages.TotpRecoveryCodeUsedBody,
			user.Name,
			remaining,
		)
	}

	return h.send(
		user.Email,
		messages.TotpRecoveryCodeUsedSubject,
		messages.TotpRecoveryCodeUsedTitle,
		body,
		messages.TotpRecoveryCodeUsedActionLabel,
		h.passwordChangeURL(user),
	)
}

// EmailLockedOut notifies that repeated wrong passcodes blocked a sign-in.
func (h *totpNotifier) EmailLockedOut(user *entities.User) error {
	body := fmt.Sprintf(
		messages.TotpLockedOutBody,
		user.Name,
	)

	return h.send(
		user.Email,
		messages.TotpLockedOutSubject,
		messages.TotpLockedOutTitle,
		body,
		messages.TotpLockedOutActionLabel,
		h.passwordChangeURL(user),
	)
}

// passwordChangeURL points at the person's own security settings.
//
// A change rather than a reset, and the distinction matters: every message
// using this one is sent to somebody who can still sign in. Reaching a
// recovery-code or lockout event means the password was accepted, so the
// action is to replace a password that is known to somebody else — not to
// recover an account that is already lost.
func (h *totpNotifier) passwordChangeURL(user *entities.User) string {
	return fmt.Sprintf(
		"%s/people/%s/settings/password",
		h.Config.GetFrontendURL(),
		user.Alias,
	)
}
