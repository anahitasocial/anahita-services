package messages

// The site is named because this is the message most likely to be imitated:
// a fake "new sign-in" alert is a standard phishing opening. One that knows
// which installation it is about is harder to forge convincingly, and one that
// does not is easier for the reader to dismiss.
const (
	AuthLoginSubject        = "New sign-in to your {{site}} account"
	AuthLoginTitle          = "New sign-in detected"
	AuthLoginBody           = "Hello %s, your {{site}} account was just signed in to using %s on a %s running %s from %s, %s. If this was you, no further action is needed. If it wasn't, reset your password immediately and contact support."
	AuthLoginActionLabel    = "Reset password"
	AuthLoginUnknownCity    = "an unknown city"
	AuthLoginUnknownCountry = "unknown country"

	// An administrator ended one of somebody's signed-in sessions.
	//
	// Sent because Perms.AuthLog.CanDelete permits acting on another person's
	// session, and the owner otherwise finds out by being signed out of a
	// device mid-use with nothing to explain it.
	//
	// Deliberately NOT sent for a person ending their own session: they are
	// looking at the screen where they did it. The distinction is the whole
	// reason this message exists separately from a general "signed out" one.
	//
	// Neutral rather than alarmed. The ordinary case is a support action taken
	// at the person's own request, so the message says what happened and
	// leaves the judgement to them.
	//
	// Four verbs: given name, family name, device description, location.
	AuthSessionRevokedSubject     = "One of your sessions was signed out"
	AuthSessionRevokedTitle       = "Session signed out"
	AuthSessionRevokedBody        = "Hello %s, an administrator signed out one of your {{site}} sessions — the one on %s from %s. Your other devices are unaffected and you can sign in again at any time. If you were not expecting this, contact support."
	AuthSessionRevokedActionLabel = "Sign in"
)
