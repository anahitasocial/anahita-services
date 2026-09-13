package config

import (
	"os"
	"regexp"
	"strings"
)

// Settings for the two things an installation needs before anybody can use it:
// the terms a person accepts when they create an account, and the first super
// administrator.
//
// Both are read here rather than from a table because both have to be readable
// before the database is usable — the seed runs at boot, and the terms version
// has to be known by a signup that may be the first row in the table.

const (
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
// The TEXT lives in the web app (src/assets/default/pages/tos.md). Nothing
// links the two automatically, so bumping this without editing the page — or
// the reverse — is possible and is the operator's job to keep in step.
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
