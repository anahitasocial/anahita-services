package constants

// NOTE: the token TYPE values here also appear in
// anahita-libs/constants/token.go, and the two must agree — the value is what
// goes in the column, so a mismatch fails at the database rather than at
// compile time.
//
// They are not identical files, whatever a previous note here claimed:
// emailchange, loginotp and the OTP settings are local to this service, and
// libs carries only the types it needs. What has to match is any name that
// appears in both.
const (
	TokenTypePasswordReset = "passwordreset"
	TokenTypeInvite        = "invite"
	TokenTypeEmailChange   = "emailchange"

	// TokenTypeSignupConfirm proves control of the address an account — or an
	// application — was created with. Also in anahita-libs.
	TokenTypeSignupConfirm = "signupconfirm"

	// One-time codes. A type per purpose, so a code issued for one can never
	// be used for another: a code sent to confirm an address cannot reset a
	// password, and one sent to sign in cannot change an email.
	TokenTypeLoginOTP          = "loginotp"
	TokenTypeSignupConfirmCode = "signupconfirmotp"
	TokenTypePasswordResetCode = "passwordresetotp"
	TokenTypeEmailChangeCode   = "emailchangeotp"
	TokenTypeReauthCode        = "reauthotp"

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

	// TokenExpiryTimeCode is fifteen minutes, for the codes that are not a
	// sign-in: confirming an address, resetting a password, changing an
	// email, confirming it is you. Five more than a sign-in code, because
	// these start on one page and are often finished after reading a longer
	// message, and the same as the links they replace gave password reset
	// and email change.
	TokenExpiryTimeCode = 15 * 60

	// OTPMaxAttempts matches the login TOTP step and the credential
	// rate limiter, so a person meets the same number of chances
	// everywhere they can get something wrong.
	//
	// 10^6 combinations against 5 attempts is comfortably out of reach
	// even before the per-address send throttle.
	OTPMaxAttempts = 5

	// How a code reaches the person. Recorded on the token for the audit
	// trail. Email is the only one built; the flow is shaped so that SMS is
	// a second value here and a sender, not a redesign.
	CodeChannelEmail = "email"
	CodeChannelSMS   = "sms"

	TokenStatusPending = "pending"
	TokenStatusUsed    = "used"
	TokenStatusExpired = "expired"
	TokenExpiryTime    = 24 * 60 * 60 // 24 hours
)
