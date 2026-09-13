package messages

// The site is named throughout. A password email is the highest-value thing to
// imitate, and "Reset your {{site}} password" is both easier to trust when real
// and easier to doubt when forged.
const (
	PasswordSubject     = "Reset your {{site}} password"
	PasswordTitle       = "Reset your password"
	PasswordBody        = "Hi %s, forgot your {{site}} password? It happens to the best of us. Click the button below to set a new one. If you didn't request a reset, you can safely ignore this email."
	PasswordActionLabel = "Reset password"

	PasswordResetSubject     = "Your {{site}} password was reset"
	PasswordResetTitle       = "Password reset"
	PasswordResetBody        = "Hello %s, your {{site}} password was just reset using a password reset email link. Any passkeys on your account were removed as a precaution — you can add them again from your account settings. If this was you, no further action is needed. If it wasn't, request a new password reset immediately to take back your account, then contact support."
	PasswordResetActionLabel = "Reset password"

	PasswordUpdateSubject     = "Your {{site}} password was changed"
	PasswordUpdateTitle       = "Password changed"
	PasswordUpdateBody        = "Hello %s, your {{site}} password was just changed from your account settings. If this was you, no further action is needed. If it wasn't, reset your password immediately to take back your account, then contact support."
	PasswordUpdateActionLabel = "Reset password"

	// Appended to PasswordUpdateBody when the change ended other
	// sessions. Two forms rather than "1 session(s)": this email is
	// read by someone deciding whether they are being attacked, and
	// the sentence should not read like a debug line.
	//
	// Nothing is appended when the count is zero. "0 other sessions
	// were ended" is noise, and the absence of the sentence says the
	// same thing without asking the reader to parse a number.
	PasswordUpdateSessionsOne  = " One other signed-in session was ended."
	PasswordUpdateSessionsMany = " %d other signed-in sessions were ended."
)
