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

// Who may issue an invitation.
const (
	// InvitesFromAdministrators — administrators and super administrators
	// only. The default, and what happened before the setting existed.
	InvitesFromAdministrators = "administrators"

	// InvitesFromMembers — any registered person.
	//
	// This changes what an invitation MEANS. An administrator's is a decision;
	// an ordinary member's is a recommendation, because the member is not the
	// person who would otherwise be deciding. Under `approval` that difference
	// is what sends a member's invitee into the queue and an administrator's
	// straight past it.
	InvitesFromMembers = "members"
)

// InvitesFromValues is every valid value, for validation and for saying what
// was expected when an operator mistypes one.
var InvitesFromValues = []string{
	InvitesFromAdministrators,
	InvitesFromMembers,
}

// Default caps on outstanding invitations.
//
// Outstanding, not lifetime. This bounds how much damage one account can do at
// any moment and SELF-HEALS — a slot frees when an invitation is accepted,
// expires, or is revoked — so an honest member is never permanently stuck and
// nobody has to administer a reset.
const (
	// DefaultInvitePendingMax is what an ordinary member may have outstanding.
	DefaultInvitePendingMax = 5

	// DefaultInvitePendingMaxAdmin is the administrator's own cap.
	//
	// High, because onboarding a department is the legitimate case and a limit
	// that makes real work feel broken gets raised until it means nothing. Not
	// unlimited, because this is also what stops a compromised administrator
	// account turning the mail service into a relay.
	DefaultInvitePendingMaxAdmin = 100
)

// RegistrationModes is every valid value, for validation and for saying what
// was expected when an operator mistypes one.
var RegistrationModes = []string{
	RegistrationModeOpen,
	RegistrationModeApproval,
	RegistrationModeInvite,
	RegistrationModeClosed,
}
