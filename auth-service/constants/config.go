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
//
// A MINIMUM ROLE, not a set. Each value admits that role and every role
// above it, so `registered` includes administrators and `administrators`
// includes super administrators. Anything else would be a setting where
// tightening the level could hand the power to somebody who did not
// have it before.
const (
	// InvitesFromRegistered — any registered person, and everyone above.
	//
	// This changes what an invitation MEANS. An administrator's is a
	// decision; an ordinary person's is a recommendation, because they
	// are not who would otherwise be deciding. Under `approval` that
	// difference is what sends their invitee into the queue and an
	// administrator's invitee straight past it.
	InvitesFromRegistered = "registered"

	// InvitesFromAdministrators — administrators and super administrators.
	// The default, and what happened before the setting existed.
	InvitesFromAdministrators = "administrators"

	// InvitesFromSuperAdministrators — super administrators only.
	//
	// For a site where an invitation is effectively an account grant and
	// only the owner should be making them.
	InvitesFromSuperAdministrators = "super-administrators"
)

// There was a fourth value here, `members`, from an earlier pass at this
// setting. It is gone.
//
// Anahita has four person types — guest, registered, administrator,
// super-administrator — and "member" is not one of them. Each value
// above is the plural of a real one, which is what lets a level be
// ranked against a viewer's role at all. A value outside that set is a
// fifth person type in all but name, and it had begun behaving like
// one: it reached the operator-facing validation message as though it
// were a supported option.
//
// Calling people members in prose is fine, and calling them people is
// better. Encoding one as a role in the configuration is neither.
//
// Nothing sets it — not docker-compose, not k8s, not any env file in
// this repository. An installation that did will now fail to start,
// naming the three valid values, which is a louder and more fixable
// failure than quietly meaning something the code no longer has a word
// for.

// InvitesFromValues is every value an operator may set, for validation
// and for saying what was expected when one is mistyped.
//
// Ordered from most permissive to least, matching how they rank.
var InvitesFromValues = []string{
	InvitesFromRegistered,
	InvitesFromAdministrators,
	InvitesFromSuperAdministrators,
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
