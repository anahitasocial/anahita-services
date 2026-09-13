package config

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"auth/constants"
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
	defaultTosPath     = "/pages/tos"
	defaultPrivacyPath = "/pages/privacy"

	// defaultAgreementVersion is what an installation runs on until the
	// operator sets one. It is a real version rather than an empty string so
	// an acceptance is always recorded against something; the alternative is a
	// consent row that says "they agreed to whatever was on the page that
	// day", which is the thing the version column exists to avoid.
	defaultAgreementVersion = "1.0"

	// ErrInvalidTosVersion and its privacy twin are reported by Validate.
	ErrInvalidTosVersion     = "TOS_VERSION must be a dotted version number, e.g. 1.0 or 2.1.3"
	ErrInvalidPrivacyVersion = "PRIVACY_VERSION must be a dotted version number, e.g. 1.0 or 2.1.3"
)

// agreementVersion matches a dotted numeric version and nothing else. It is
// compared for equality and ordered by nobody, so the only requirement is that
// two different texts get two different strings — the shape is there to stop a
// deployment drifting into free text like "current" or "v2 (draft)", which
// cannot answer "who has not accepted the latest?".
var agreementVersion = regexp.MustCompile(`^[0-9]+(\.[0-9]+){0,2}$`)

// AgreementConfig is the part of Config the signup and invite-accept paths
// need. Kept as an interface so a handler test can supply versions without
// building a whole Config.
type AgreementConfig interface {
	GetTosVersion() string
	GetPrivacyVersion() string
}

// GetTosVersion returns the Terms of Service version currently in force,
// falling back to the default when the operator has set none.
//
// This is the authority. A submitted acceptance is checked against it and
// refused if it does not match, so somebody who left the signup page open
// across a terms revision is asked again rather than silently recorded as
// having accepted text they never saw.
//
// The version and the URL are set together, in this one place, so an
// installation has a single answer to "which terms are in force, and where are
// they". Nothing verifies that the document at the URL actually IS that
// version — that is the operator's job, and it is the reason the two settings
// sit next to each other rather than in different files.
func (config *Config) GetTosVersion() string {
	if config.TosVersion == "" {
		return defaultAgreementVersion
	}

	return config.TosVersion
}

// GetPrivacyVersion returns the Privacy Policy version currently in force.
// Same contract as GetTosVersion.
func (config *Config) GetPrivacyVersion() string {
	if config.PrivacyVersion == "" {
		return defaultAgreementVersion
	}

	return config.PrivacyVersion
}

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
// without an invitation. Under `approval` they can — they start an
// application, which is a different thing from an account and is item 5's
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
	config.TosVersion = getEnv("TOS_VERSION", defaultAgreementVersion)
	config.PrivacyVersion = getEnv("PRIVACY_VERSION", defaultAgreementVersion)
	config.RegistrationMode = os.Getenv("REGISTRATION_MODE")
	config.TosURL = os.Getenv("TOS_URL")
	config.PrivacyURL = os.Getenv("PRIVACY_URL")
	config.SuperAdminEmail = os.Getenv("SUPER_ADMIN_EMAIL")
	config.SuperAdminUsername = os.Getenv("SUPER_ADMIN_USERNAME")
	config.SuperAdminName = os.Getenv("SUPER_ADMIN_NAME")
}

// validateBootstrap appends any problems to missingFields. Called by Validate.
func (config *Config) validateBootstrap(missingFields []string) []string {
	// Empty means "not set", which the getters answer with the default. Only
	// a value the operator actually supplied is checked — a Config built in a
	// test, or any other path that does not go through NewConfig, must not be
	// told it is missing a field it never had to set.
	if config.TosVersion != "" && !agreementVersion.MatchString(config.TosVersion) {
		missingFields = append(missingFields, ErrInvalidTosVersion)
	}

	if config.PrivacyVersion != "" && !agreementVersion.MatchString(config.PrivacyVersion) {
		missingFields = append(missingFields, ErrInvalidPrivacyVersion)
	}

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

	// `approval` is refused until the queue behind it exists.
	//
	// The predicates already describe it correctly and signup already treats
	// an applicant as allowed — which is right, and is exactly the hazard
	// while there is nowhere for the application to go: an operator asking
	// for approval would get OPEN registration, on an installation they
	// believe is gated. Nothing would look wrong.
	//
	// Refusing to boot is the loud version of that, and the right one for a
	// setting whose whole purpose is to keep strangers out. Remove this when
	// the queue lands; the tests covering `approval` describe the intended
	// behaviour and already pass.
	if config.GetRegistrationMode() == constants.RegistrationModeApproval {
		missingFields = append(missingFields,
			"REGISTRATION_MODE=approval is not available yet — the approval "+
				"queue is not built, so this would accept registrations "+
				"immediately while appearing to hold them. Use `invite` or "+
				"`closed` until it lands")
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
