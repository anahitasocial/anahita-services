package messages

const (
	// Sent when the step-up limiter trips — five failed attempts to confirm a
	// password at /reauth/* inside fifteen minutes.
	//
	// The threat model is narrow and worth stating, because it decides
	// everything else about this message. /reauth/* is only reachable with a
	// live session, so whoever tripped this ALREADY HOLDS ONE. They are
	// guessing the password, which means they do not have it. Nothing has been
	// changed and nothing has been lost — but somebody is signed in as this
	// person and trying to escalate.
	//
	// So the action is not "change your password". It is "end the session they
	// are using". Two reasons: the foothold is the session, and — the part
	// that would otherwise be a trap — the limiter is keyed per person, so
	// while it is tripped the owner cannot pass a step-up either. An email
	// telling them to change their password would point at a door that is
	// bolted for the next fifteen minutes. Ending a session needs no step-up.
	//
	// Two verbs: given name, family name.
	ReauthLockedOutSubject = "Someone is trying to confirm your {{site}} password"
	ReauthLockedOutTitle   = "Too many verification attempts"
	ReauthLockedOutBody    = "Hello %s %s, there have been several failed attempts to confirm the password on your {{site}} account. These attempts came from a device that is already signed in, so if this was not you, someone else has access to your account right now. Open your security settings, review the list of active sessions, and sign out any you do not recognise. Then change your password. If you cannot get in, contact support."

	ReauthLockedOutActionLabel = "Review active sessions"
)
