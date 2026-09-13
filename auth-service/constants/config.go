package constants

const (
	ConfigServiceName = "auth-service"
)

// How an installation lets people in.
//
// The setting governs SELF-SERVE registration. Invitations are a separate
// door, because each one is an explicit decision by an administrator — except
// under RegistrationModeClosed, which is the only mode that shuts both.
const (
	// RegistrationModeOpen — anybody, instantly.
	RegistrationModeOpen = "open"

	// RegistrationModeApproval — anybody may apply; an administrator decides.
	// An invited person skips the queue: the invitation was the decision.
	RegistrationModeApproval = "approval"

	// RegistrationModeInvite — no self-serve. Invitations work. The ordinary
	// running state for a private community.
	RegistrationModeInvite = "invite"

	// RegistrationModeClosed — nothing gets in, and outstanding invitations
	// stop working too.
	//
	// This is what makes it different from RegistrationModeInvite and the
	// reason both exist: "stop issuing invitations" does not help when the
	// ones already sent are still live, which is exactly the situation during
	// an abuse incident. The tokens are suspended rather than destroyed, so
	// lifting the lockdown brings them back.
	RegistrationModeClosed = "closed"
)

// RegistrationModes is every valid value, for validation and for saying what
// was expected when an operator mistypes one.
var RegistrationModes = []string{
	RegistrationModeOpen,
	RegistrationModeApproval,
	RegistrationModeInvite,
	RegistrationModeClosed,
}
