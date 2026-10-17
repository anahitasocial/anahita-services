package repositories

import (
	"database/sql"
	"time"

	"auth/data/entities"
)

// The column list and scan for a user, in one place.
//
// Five read methods — Browse, ReadByID, ReadByEmail, ReadByUsername and
// ReadByEmailOrUsername — each carried their own copy of both, which is how
// the bug below survived: a column was COALESCEd in the copy someone was
// looking at and bare in the other four.
//
// THE BUG. Most of these columns are nullable in the legacy schema, and the
// entity scans them into plain strings and time.Time. A NULL therefore fails
// the scan, the read returns an error, and every caller reads that as "no such
// user" — so a person with a NULL in any one of them is told their password is
// wrong when it is right, and forgot-password quietly does nothing for them.
// They cannot get in and cannot recover. Nothing is logged.
//
// In the development dataset that was 114 of 823 accounts: 95 with no avatar
// filename, 19 who had never signed in.
//
// COALESCE for the strings, because "" is what the entity means by absent. The
// three timestamps take sql.NullTime instead: there is no honest default date
// for "never visited", and the zero time.Time already says it.

// userColumns is the select list every user read shares. Column order must
// match scanUser.
func userColumns() []string {
	return []string{
		"nodes.id",
		"nodes.type",
		"nodes.enabled",
		// The display name and the handle come from the node. They were
		// given_name/family_name and username on people_people, which is gone.
		"COALESCE(nodes.name, '') AS name",
		"COALESCE(nodes.alias, '') AS alias",
		"COALESCE(auth_accounts.email, '') AS email",
		// A NULL password is a real state — an account created by invitation
		// that has never set one. It must read as an empty hash, which
		// CheckPassword then refuses, rather than failing the whole read.
		"COALESCE(auth_accounts.password, '') AS password",
		"COALESCE(nodes.person_type, '') AS person_type",
		"COALESCE(nodes.service, '') AS service",
		"COALESCE(nodes.avatar_filename, '') AS avatar_filename",
		"nodes.last_visited_at",
		"nodes.created_at",
		"nodes.updated_at",
		"auth_accounts.totp_enabled",
		"COALESCE(auth_accounts.totp_recovery_codes, '') AS totp_recovery_codes",
		// NULL is never: read as no time, so a check of how fresh it is fails
		// closed, where reading it as "now" would have passed.
		"auth_accounts.totp_last_verified_at",
		"COALESCE(auth_accounts.totp_secret, '') AS totp_secret",
		"nodes.deleted_at",
		"nodes.archived_at",
		// Consent: the flag and the version, and not the audit trio.
		//
		// These two answer "does this person still need to accept something?",
		// which is asked on every sign-in. The ip/user_agent/os columns answer
		// "who accepted it, and from where", which is asked by a person
		// looking at a row and never by the code — so they are written and not
		// carried through every read.
		// Nullable, and scanned as one: NULL means the address was never
		// confirmed, which is a state the sign-in path acts on.
		"auth_accounts.email_verified_at",
		// Nullable for the same reason: NULL means the person has not been
		// through onboarding, which the web app's gate acts on.
		"auth_accounts.onboarded_at",
		// Whether there is a bio, not the bio. REGEXP rather than TRIM,
		// because TRIM strips spaces only and a body of newlines is as
		// empty as a body of nothing.
		"(COALESCE(nodes.body, '') REGEXP '[^[:space:]]') AS has_bio",
		// 0 for nobody, as signup_requests reads the same column: a self-serve
		// account has no inviter, and that is a value, not a failed read.
		"COALESCE(auth_accounts.invited_by, 0) AS invited_by",
		"auth_accounts.tos_accepted",
		"COALESCE(auth_accounts.tos_accepted_version, '') AS tos_accepted_version",
		"auth_accounts.pp_accepted",
		"COALESCE(auth_accounts.pp_accepted_version, '') AS pp_accepted_version",
		// The language this person usually posts in, which the web app's
		// composer starts on. Empty when they have not said.
		"COALESCE(nodes.language, '') AS language",
	}
}

// rowScanner is what *sql.Row and *sql.Rows have in common, so one scan serves
// both the single reads and Browse.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanUser reads one row into a User. Column order must match userColumns.
func scanUser(row rowScanner) (*entities.User, error) {
	item := entities.NewUser()

	// Nullable timestamps. The entity holds plain time.Time values, so a NULL
	// becomes the zero time — which is what "never signed in" looks like
	// everywhere else in the codebase.
	var lastVisit, createdAt, updatedAt, emailVerified, onboarded sql.NullTime

	if err := row.Scan(
		&item.ID,
		&item.Type,
		&item.Enabled,
		&item.Name,
		&item.Alias,
		&item.Email,
		&item.Password,
		&item.PersonType,
		&item.Service,
		&item.AvatarFilename,
		&lastVisit,
		&createdAt,
		&updatedAt,
		&item.TotpEnabled,
		&item.TotpRecoveryCodes,
		optionalTime{&item.TotpLastVerifiedAt},
		&item.TotpSecret,
		&item.DeletedAt,
		&item.ArchivedAt,
		&emailVerified,
		&onboarded,
		&item.HasBio,
		&item.InvitedBy,
		&item.TermsOfService.Accepted,
		&item.TermsOfService.Version,
		&item.PrivacyPolicy.Accepted,
		&item.PrivacyPolicy.Version,
		&item.Language,
	); err != nil {
		return nil, err
	}

	// LastVisitedAt is a *time.Time on the node: nil means never signed in,
	// which a zero time cannot express.
	if lastVisit.Valid {
		t := lastVisit.Time
		item.LastVisitedAt = &t
	}
	item.CreatedAt = nullTime(createdAt)
	item.UpdatedAt = nullTime(updatedAt)

	if emailVerified.Valid {
		t := emailVerified.Time
		item.EmailVerifiedAt = &t
	}

	if onboarded.Valid {
		t := onboarded.Time
		item.OnboardedAt = &t
	}

	return item, nil
}

func nullTime(v sql.NullTime) time.Time {
	if v.Valid {
		return v.Time
	}
	return time.Time{}
}

// Browse selects fewer columns than the single reads — no password, no TOTP
// state, no deleted_at — so it needs its own list rather than a subset of
// userColumns.
//
// The same NULLs bite harder here: a single row with a NULL fails the scan and
// aborts the whole listing, so one account with no avatar takes out the
// administrator's entire user list rather than just itself.
func userBrowseColumns() []string {
	return []string{
		"nodes.id",
		"nodes.type",
		"nodes.enabled",
		"COALESCE(nodes.name, '') AS name",
		"COALESCE(nodes.alias, '') AS alias",
		"COALESCE(auth_accounts.email, '') AS email",
		"COALESCE(nodes.person_type, '') AS person_type",
		"COALESCE(nodes.service, '') AS service",
		"COALESCE(nodes.avatar_filename, '') AS avatar_filename",
		"nodes.last_visited_at",
		"nodes.created_at",
		"nodes.updated_at",
	}
}

// scanBrowsedUser reads one row into a User. Column order must match
// userBrowseColumns.
func scanBrowsedUser(row rowScanner) (*entities.User, error) {
	item := entities.NewUser()

	var lastVisit, createdAt, updatedAt sql.NullTime

	if err := row.Scan(
		&item.ID,
		&item.Type,
		&item.Enabled,
		&item.Name,
		&item.Alias,
		&item.Email,
		&item.PersonType,
		&item.Service,
		&item.AvatarFilename,
		&lastVisit,
		&createdAt,
		&updatedAt,
	); err != nil {
		return nil, err
	}

	if lastVisit.Valid {
		t := lastVisit.Time
		item.LastVisitedAt = &t
	}
	item.CreatedAt = nullTime(createdAt)
	item.UpdatedAt = nullTime(updatedAt)

	return item, nil
}

// optionalTime scans a date that is NULL when it has not happened into the
// zero time.
type optionalTime struct{ at *time.Time }

func (o optionalTime) Scan(value any) error {
	var read sql.NullTime
	if err := read.Scan(value); err != nil {
		return err
	}
	*o.at = time.Time{}
	if read.Valid {
		*o.at = read.Time
	}
	return nil
}
