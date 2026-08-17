package messages

const (
	// TOTP enabled — neutral tone. Enabling 2FA strengthens the
	// account; an alarmist alert here trains users to ignore future
	// security emails.
	TotpEnabledSubject     = "Two-factor authentication enabled"
	TotpEnabledTitle       = "Two-factor authentication enabled"
	TotpEnabledBody        = "Hello %s %s, two-factor authentication was just enabled on your account. Your account is now more secure. If this wasn't you, contact support."
	TotpEnabledActionLabel = "Contact support"

	// TOTP disabled — urgent. This is the dangerous direction; an
	// attacker disabling 2FA is preparing to take over.
	TotpDisabledSubject     = "Two-factor authentication disabled"
	TotpDisabledTitle       = "Two-factor authentication disabled"
	TotpDisabledBody        = "Hello %s %s, two-factor authentication was just disabled on your account. If this was you, no further action is needed. If it wasn't, reset your password immediately and contact support."
	TotpDisabledActionLabel = "Reset password"
)
