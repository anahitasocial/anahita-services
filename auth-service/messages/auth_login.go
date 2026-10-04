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

	// What a sign-in from a private address is recorded as. In production a
	// private address here means c.RealIP() returned a proxy hop instead of
	// the client, so "development" in a deployed sign-in log says the trusted
	// proxy setup is wrong, not that the environment is.
	AuthLoginLocalCity    = "localhost"
	AuthLoginLocalCountry = "development"

	// A sign-in code.
	//
	// The code is in the subject as well as the body, so it can be read off a
	// lock screen or a notification without opening the message.
	//
	// The three sentences after the code are the defence against the one way
	// a code is weaker than a link: a person can be talked into reading it
	// out. It says how long it lasts, that it only works where they asked for
	// it, and that nobody will ever ask for it.
	//
	// No action link. A link here could only carry the code or lead somewhere
	// to type it, and the page that wants it is already open in front of the
	// person. Mail scanners follow links; they do not type.
	//
	// The code is not in the body text. The mail template shows it by itself,
	// in a box between the title and this text, in the largest type on the
	// page.
	//
	// Verbs: the code (subject); name (body).
	AuthLoginCodeSubject = "%s is your {{site}} sign-in code"
	AuthLoginCodeTitle   = "Your sign-in code"
	AuthLoginCodeBody    = "Hello %s, enter this code to sign in to {{site}}. It expires in 10 minutes and only works in the browser where you asked for it. We will never ask you for this code: do not share it with anyone. If you did not try to sign in, you can ignore this message. Nobody can sign in without the code."

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
