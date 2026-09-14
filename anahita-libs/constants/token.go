package constants

const (
	TokenTypePasswordReset = "passwordreset"
	TokenTypeInvite        = "invite"

	// TokenTypeSignupConfirm proves control of the address an account was
	// created with. Issued at signup, spent by following the link.
	TokenTypeSignupConfirm = "signupconfirm"

	TokenStatusPending = "pending"
	TokenStatusUsed    = "used"
	TokenStatusExpired = "expired"
	TokenExpiryTime    = 24 * 60 * 60 // 24 hours
)
