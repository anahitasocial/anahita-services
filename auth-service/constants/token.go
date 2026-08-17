package constants

const (
	TokenTypePasswordReset = "passwordreset"
	TokenTypeInvite        = "invite"

	TokenStatusPending = "pending"
	TokenStatusUsed    = "used"
	TokenStatusExpired = "expired"
	TokenExpiryTime    = 24 * 60 * 60 // 24 hours
)
