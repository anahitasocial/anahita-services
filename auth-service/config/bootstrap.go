package config

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"auth/constants"

	libs_constants "anahita-libs/constants"
)

// Settings for the two things an installation needs before anybody can use it:
// the terms a person accepts when they create an account, and the first super
// administrator.
//
// Both are read here rather than from a table because both have to be readable
// before the database is usable — the seed runs at boot, and the terms version
// has to be known by a signup that may be the first row in the table.

const (
	// defaultTosPath and defaultPrivacyPath are where the web app serves the
	// documents from today. Overridable, because where the terms live is a
	// deployment's decision — a path when they are pages of the app, or a
	// whole URL when legal keeps them somewhere else entirely.
	defaultTosPath     = "/legal/tos"
	defaultPrivacyPath = "/legal/privacy"
)

// GetTosURL returns where the Terms of Service can be read.
//
// TOS_URL takes either form:
//
//	/legal/terms                   a path, resolved against the web app
//	https://example.org/terms      a whole URL, for documents hosted elsewhere
//
// Both are wanted. A deployment that renames the route needs the first; one
// whose legal team keeps the canonical text on a corporate site needs the
// second, and forcing that through the app would mean maintaining a copy that
// can silently disagree with the original.
func (config *Config) GetTosURL() string {
	return config.agreementURL(config.TosURL, defaultTosPath)
}

// GetPrivacyURL returns where the Privacy Policy can be read. Same contract as
// GetTosURL.
func (config *Config) GetPrivacyURL() string {
	return config.agreementURL(config.PrivacyURL, defaultPrivacyPath)
}

// agreementURL resolves a configured value, falling back to a default path.
//
// An absolute URL is returned untouched. Anything else is treated as a path on
// the web app — with a leading slash added if it is missing, because
// "legal/terms" and "/legal/terms" are the same intention and only one of them
// produces a working link.
func (config *Config) agreementURL(configured, fallbackPath string) string {
	value := strings.TrimSpace(configured)
	if value == "" {
		value = fallbackPath
	}

	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}

	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}

	return strings.TrimRight(config.GetFrontendURL(), "/") + value
}

// GetRegistrationMode returns how this installation lets people in.
//
// **Defaults to closed**, and that is a decision rather than caution. The
// alternative is an installation that nobody has configured being open to the
// internet, and the failure modes are not comparable: a closed installation is
// noticed within a minute by the operator trying to use it, while an open one
// is noticed when the spam arrives.
//
// It governs SELF-SERVE registration. Invitations are their own door — see the
// constants — except under `closed`, which shuts both.
func (config *Config) GetRegistrationMode() string {
	mode := strings.ToLower(strings.TrimSpace(config.RegistrationMode))
	if mode == "" {
		return constants.RegistrationModeClosed
	}

	return mode
}

// AllowsSelfServeRegistration reports whether a stranger can start an account
// without an invitation. Under `approval` they can — they start a signup
// request, which is a different thing from an account and is item 5's
// problem, not this one's.
func (config *Config) AllowsSelfServeRegistration() bool {
	switch config.GetRegistrationMode() {
	case constants.RegistrationModeOpen, constants.RegistrationModeApproval:
		return true
	default:
		return false
	}
}

// RequiresRegistrationApproval reports whether a self-serve applicant waits
// for an administrator.
func (config *Config) RequiresRegistrationApproval() bool {
	return config.GetRegistrationMode() == constants.RegistrationModeApproval
}

// AllowsInvitations reports whether invitations may be issued and accepted.
//
// True in every mode but `closed`. An invitation is an administrator's
// explicit decision about one person, so it does not need self-serve
// registration to be open — and turning it off is the whole point of `closed`.
func (config *Config) AllowsInvitations() bool {
	return config.GetRegistrationMode() != constants.RegistrationModeClosed
}

// GetInvitesFrom returns the minimum role that may issue an invitation.
//
// Defaults to administrators, which is what happened before this setting
// existed — so no installation changes behaviour by upgrading into it.
//
// No normalisation beyond case and whitespace. There was an alias here,
// `members`, mapping to `registered`; it is gone, because a level that
// is not the plural of a person type is a person type Anahita does not
// have. An unrecognised value now falls through to validateBootstrap,
// which refuses to start and names the three that exist.
func (config *Config) GetInvitesFrom() string {
	value := strings.ToLower(strings.TrimSpace(config.InvitesFrom))
	if value == "" {
		return constants.InvitesFromAdministrators
	}

	return value
}

// GetGroupsFrom returns the minimum role that may create a group.
//
// Defaults to administrators, matching group-service's own default — the two
// resolve the same variable independently, so they have to agree on what
// unset means or the published document would contradict the enforcement.
func (config *Config) GetGroupsFrom() string {
	value := libs_constants.NormalizeRoleLevel(config.GroupsFrom)
	if value == "" {
		return libs_constants.RoleLevelAdministrators
	}

	return value
}

// GetEventsFrom returns the minimum role that may make an event of their
// own.
//
// Defaults to registered, matching actor-service, which enforces it. The two
// resolve the same variable independently and have to agree on what unset
// means.
func (config *Config) GetEventsFrom() string {
	value := libs_constants.NormalizeRoleLevel(config.EventsFrom)
	if value == "" {
		return libs_constants.RoleLevelRegistered
	}

	return value
}

// MayInvite reports whether somebody of this person type meets the
// configured minimum.
//
// The comparison is by RANK, not by equality, and that is the whole
// reason this function exists. The permission it replaced asked its
// questions as a ladder of early returns, the first of which was an
// unconditional `if viewer.IsAdminOrSuperAdmin() { return nil }`. That
// bypassed the setting: an administrator could invite no matter what
// the operator had configured, so `super-administrators` would have
// been a level that did nothing.
func (config *Config) MayInvite(personType string) bool {
	return libs_constants.MeetsRoleLevel(personType, config.GetInvitesFrom())
}

// Above every role, so an unknown level is refused rather than waved
// through.
const unknownInviteLevel = 99

// GetInvitePendingMax returns how many outstanding invitations one person may
// hold, which depends on whether they administer the site.
//
// Two numbers rather than one because the two cases are not comparable: five
// is right for a member and absurd for somebody onboarding a department, and a
// single figure generous enough for the second is no cap at all on the first.
//
// Zero means UNSET and falls back to the default. "Nobody may invite" is
// already expressible — REGISTRATION_MODE=closed suspends invitations
// entirely — so the cap does not need a second way to say it, and reading a
// bare 0 as a lockout would turn a typo into one.
//
// Negative is refused by Validate rather than read as unlimited: an operator
// writing -1 for "no limit" would otherwise get a cap that refuses every
// invitation, since any count exceeds a negative one.
func (config *Config) GetInvitePendingMax(isAdmin bool) int {
	if isAdmin {
		if config.InvitePendingMaxAdmin != 0 {
			return config.InvitePendingMaxAdmin
		}
		return constants.DefaultInvitePendingMaxAdmin
	}

	if config.InvitePendingMax != 0 {
		return config.InvitePendingMax
	}

	return constants.DefaultInvitePendingMax
}

// GetSupportEmail returns the address people are told to write to.
func (config *Config) GetSupportEmail() string {
	return strings.TrimSpace(config.SupportEmail)
}

// GetSupportPhone returns the support phone number, or "" when there is none.
func (config *Config) GetSupportPhone() string {
	return strings.TrimSpace(config.SupportPhone)
}

// GetSupportWebsite returns the support site, or "" when there is none.
func (config *Config) GetSupportWebsite() string {
	return strings.TrimSpace(config.SupportWebsite)
}

// GetSuperAdminEmail returns the address the first super administrator is
// seeded against, or "" when the operator has not set one.
//
// Empty is a legitimate state and not an error: an installation that already
// has a super administrator — every existing one does — needs no seed, and
// requiring the variable would make the service refuse to start.
func (config *Config) GetSuperAdminEmail() string {
	return strings.TrimSpace(config.SuperAdminEmail)
}

// GetSuperAdminUsername returns the handle to give the seeded account, or ""
// to derive one from the email local part.
func (config *Config) GetSuperAdminUsername() string {
	return strings.TrimSpace(config.SuperAdminUsername)
}

// GetSuperAdminName returns the display name for the seeded account, falling
// back to the email local part.
//
// nodes.name is the display name now, and a person with an empty one has no
// usable profile, so this always returns something when an email is set.
func (config *Config) GetSuperAdminName() string {
	if name := strings.TrimSpace(config.SuperAdminName); name != "" {
		return name
	}

	local, _, _ := strings.Cut(config.GetSuperAdminEmail(), "@")

	return local
}

// bootstrapFromEnv fills in the settings above. Called by NewConfig.
func (config *Config) bootstrapFromEnv() {
	config.RegistrationMode = os.Getenv("REGISTRATION_MODE")
	config.SupportPhone = os.Getenv("SUPPORT_PHONE")
	config.SupportWebsite = os.Getenv("SUPPORT_WEBSITE")
	config.InvitesFrom = os.Getenv("INVITES_FROM")
	config.GroupsFrom = os.Getenv("GROUPS_FROM")
	config.EventsFrom = os.Getenv("EVENTS_FROM")
	config.InvitePendingMax = getIntEnv("INVITE_PENDING_MAX", 0)
	config.InvitePendingMaxAdmin = getIntEnv("INVITE_PENDING_MAX_ADMIN", 0)
	config.TosURL = os.Getenv("TOS_URL")
	config.PrivacyURL = os.Getenv("PRIVACY_URL")
	config.SuperAdminEmail = os.Getenv("SUPER_ADMIN_EMAIL")
	config.SuperAdminUsername = os.Getenv("SUPER_ADMIN_USERNAME")
	config.SuperAdminName = os.Getenv("SUPER_ADMIN_NAME")
}

// validateBootstrap appends any problems to missingFields. Called by Validate.
func (config *Config) validateBootstrap(missingFields []string) []string {

	// A mistyped mode must not fall back to a default. "invites" or "Open " is
	// an operator who believes they have configured something, and silently
	// answering `closed` would look like the setting was ignored — while
	// silently answering `open` would be worse. Empty is the only value that
	// means "not set".
	if config.RegistrationMode != "" &&
		!slices.Contains(constants.RegistrationModes, config.GetRegistrationMode()) {
		missingFields = append(missingFields, fmt.Sprintf(
			"REGISTRATION_MODE must be one of %s (got %q)",
			strings.Join(constants.RegistrationModes, ", "), config.RegistrationMode))
	}

	if config.InvitesFrom != "" &&
		!slices.Contains(constants.InvitesFromValues, config.GetInvitesFrom()) {
		missingFields = append(missingFields, fmt.Sprintf(
			"INVITES_FROM must be one of %s (got %q)",
			strings.Join(constants.InvitesFromValues, ", "), config.InvitesFrom))
	}

	// Validated here as well as in group-service, which is what enforces it.
	// A typo would otherwise be caught only by the service that refuses every
	// group creation, while the document this service publishes advertised the
	// mistyped value to every client.
	if config.GroupsFrom != "" &&
		!libs_constants.IsRoleLevel(config.GetGroupsFrom()) {
		missingFields = append(missingFields, fmt.Sprintf(
			"GROUPS_FROM must be one of %s (got %q)",
			strings.Join(libs_constants.RoleLevelValues, ", "), config.GroupsFrom))
	}

	if config.EventsFrom != "" &&
		!libs_constants.IsRoleLevel(config.GetEventsFrom()) {
		missingFields = append(missingFields, fmt.Sprintf(
			"EVENTS_FROM must be one of %s (got %q)",
			strings.Join(libs_constants.RoleLevelValues, ", "), config.EventsFrom))
	}

	// Negative is refused rather than read as unlimited. An operator who
	// wrote -1 meaning "no limit" would otherwise get a cap that refuses
	// every invitation, since any count is greater than a negative one.
	if config.InvitePendingMax < 0 {
		missingFields = append(missingFields,
			"INVITE_PENDING_MAX cannot be negative — to stop invitations "+
				"entirely, set REGISTRATION_MODE=closed")
	}
	if config.InvitePendingMaxAdmin < 0 {
		missingFields = append(missingFields,
			"INVITE_PENDING_MAX_ADMIN cannot be negative — to stop invitations "+
				"entirely, set REGISTRATION_MODE=closed")
	}

	// SUPER_ADMIN_EMAIL is optional, but a malformed one is not: the seed
	// would create an account nobody can receive the set-password link for,
	// and the operator would have no way to tell that from "the seed decided
	// it was not needed".
	if email := config.GetSuperAdminEmail(); email != "" && !strings.Contains(email, "@") {
		missingFields = append(missingFields,
			"SUPER_ADMIN_EMAIL is set but is not an email address")
	}

	return missingFields
}

// defaultAccountDormantDays is two years.
const defaultAccountDormantDays = 730

// GetAccountDormantDays returns how many days without activity make an
// account dormant in the administration list, from ACCOUNT_DORMANT_DAYS.
//
// It only sorts accounts into tiers for a super administrator to look at;
// nothing is removed because of it. Unset, unreadable or not positive falls
// back to two years.
func (config *Config) GetAccountDormantDays() int {
	days := getIntEnv("ACCOUNT_DORMANT_DAYS", defaultAccountDormantDays)
	if days <= 0 {
		return defaultAccountDormantDays
	}
	return days
}
