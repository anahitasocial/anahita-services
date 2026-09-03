package constants

// NOTE: this block is duplicated byte-for-byte in
// anahita-libs/constants/token.go. Change both together — they must
// agree, because the values are TOKEN_TYPE enum members in Postgres and
// a mismatch fails at the database rather than at compile time.
const (
	TokenTypePasswordReset = "passwordreset"
	TokenTypeInvite        = "invite"
	TokenTypeEmailChange   = "emailchange"

	// One-time codes. Two types so a code issued to verify a handset
	// can never be replayed to sign in.
	TokenTypeLoginOTP = "loginotp"

	// TokenExpiryTimeEmailChange is deliberately short, matching the
	// 15 minutes password reset uses. This token authorises changing a
	// credential, so the window in which a link sitting in an inbox is
	// worth stealing should be small.
	TokenExpiryTimeEmailChange = 15 * 60

	// OTPCodeLength is 6 digits: the industry norm, and the most a
	// person will retype off a lock screen without error. Alphanumeric
	// buys keyspace we do not need — the attempt cap and the ten-minute
	// window already put brute force out of reach — at the cost of
	// 0/O and 1/l confusion for an older professional user base.
	OTPCodeLength = 6

	// TokenExpiryTimeOTP is ten minutes: long enough for a slow mail
	// hop or a phone in another room, short enough that a code sitting
	// in a notification is not worth much later.
	TokenExpiryTimeOTP = 10 * 60

	// OTPMaxAttempts matches the login TOTP step and the credential
	// rate limiter, so a person meets the same number of chances
	// everywhere they can get something wrong.
	//
	// 10^6 combinations against 5 attempts is comfortably out of reach
	// even before the per-address send throttle.
	OTPMaxAttempts = 5

	TokenStatusPending = "pending"
	TokenStatusUsed    = "used"
	TokenStatusExpired = "expired"
	TokenExpiryTime    = 24 * 60 * 60 // 24 hours
)
