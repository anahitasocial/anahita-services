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
		"id",
		"type",
		"enabled",
		// Nullable in the legacy schema, every one of them.
		"COALESCE(given_name, '') AS given_name",
		"COALESCE(family_name, '') AS family_name",
		"COALESCE(username, '') AS username",
		"COALESCE(email, '') AS email",
		// A NULL password is a real state — an account created by invitation
		// that has never set one. It must read as an empty hash, which
		// CheckPassword then refuses, rather than failing the whole read.
		"COALESCE(password, '') AS password",
		"COALESCE(usertype, '') AS usertype",
		"COALESCE(service, '') AS service",
		"COALESCE(filename, '') AS filename",
		"last_visit_date",
		"created_on",
		"modified_on",
		"totp_enabled",
		"COALESCE(totp_recovery_codes, '') AS totp_recovery_codes",
		"COALESCE(totp_last_verified_at, NOW()) AS totp_last_verified_at",
		"COALESCE(totp_secret, '') AS totp_secret",
		"nodes.deleted_at",
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
	var lastVisit, createdOn, modifiedOn sql.NullTime

	if err := row.Scan(
		&item.ID,
		&item.Type,
		&item.Enabled,
		&item.GivenName,
		&item.FamilyName,
		&item.Username,
		&item.Email,
		&item.Password,
		&item.UserType,
		&item.Service,
		&item.Filename,
		&lastVisit,
		&createdOn,
		&modifiedOn,
		&item.TotpEnabled,
		&item.TotpRecoveryCodes,
		&item.TotpLastVerifiedAt,
		&item.TotpSecret,
		&item.DeletedAt,
	); err != nil {
		return nil, err
	}

	item.LastVisitDate = nullTime(lastVisit)
	item.CreationTime = nullTime(createdOn)
	item.UpdateTime = nullTime(modifiedOn)

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
		"id",
		"type",
		"enabled",
		"COALESCE(given_name, '') AS given_name",
		"COALESCE(family_name, '') AS family_name",
		"COALESCE(username, '') AS username",
		"COALESCE(email, '') AS email",
		"COALESCE(usertype, '') AS usertype",
		"COALESCE(service, '') AS service",
		"COALESCE(filename, '') AS filename",
		"last_visit_date",
		"created_on",
		"modified_on",
	}
}

// scanBrowsedUser reads one row into a User. Column order must match
// userBrowseColumns.
func scanBrowsedUser(row rowScanner) (*entities.User, error) {
	item := entities.NewUser()

	var lastVisit, createdOn, modifiedOn sql.NullTime

	if err := row.Scan(
		&item.ID,
		&item.Type,
		&item.Enabled,
		&item.GivenName,
		&item.FamilyName,
		&item.Username,
		&item.Email,
		&item.UserType,
		&item.Service,
		&item.Filename,
		&lastVisit,
		&createdOn,
		&modifiedOn,
	); err != nil {
		return nil, err
	}

	item.LastVisitDate = nullTime(lastVisit)
	item.CreationTime = nullTime(createdOn)
	item.UpdateTime = nullTime(modifiedOn)

	return item, nil
}
