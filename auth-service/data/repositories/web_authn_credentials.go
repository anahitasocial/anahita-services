package repositories

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"auth/data/entities"

	"github.com/Masterminds/squirrel"
)

// ErrWebAuthnCredentialNotFound is the domain sentinel for a missing
// row. sql.ErrNoRows is translated here and never escapes the
// repository — handlers switch on this instead of string-matching the
// driver error the way isNotFoundLikeErr currently has to.
var ErrWebAuthnCredentialNotFound = errors.New("webauthn credential not found")

// webAuthnCredentialColumns is the canonical select list. Kept in one
// place so scanWebAuthnCredential can't drift out of order with it —
// a column added to one and not the other is a runtime scan error
// that no test will necessarily catch.
var webAuthnCredentialColumns = []string{
	"id",
	"user_id",
	"credential_id",
	"public_key",
	"attestation_type",
	"aaguid",
	"sign_count",
	"clone_warning",
	"transports",
	"backup_eligible",
	"backup_state",
	"nickname",
	"last_used_at",
	"created_at",
	"updated_at",
}

type WebAuthnCredentialsRepository struct {
	db *sql.DB
	sb squirrel.StatementBuilderType
}

func NewWebAuthnCredentials(db *sql.DB) *WebAuthnCredentialsRepository {
	return &WebAuthnCredentialsRepository{
		db: db,
		sb: stmtBuilder,
	}
}

// Browse returns every credential enrolled by a person, newest first.
func (r *WebAuthnCredentialsRepository) Browse(userID int64) ([]*entities.WebAuthnCredential, error) {
	var (
		creds = make([]*entities.WebAuthnCredential, 0)
	)

	query, args, err := r.sb.
		Select(webAuthnCredentialColumns...).
		From("webauthn_credentials").
		Where(squirrel.Eq{"user_id": userID}).
		OrderBy("created_at DESC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("webauthn browse build: %w", err)
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("webauthn browse: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		cred, err := scanWebAuthnCredential(rows)
		if err != nil {
			return nil, fmt.Errorf("webauthn browse scan: %w", err)
		}
		creds = append(creds, cred)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("webauthn browse rows: %w", err)
	}

	return creds, nil
}

// Read fetches one credential by its public UUID.
func (r *WebAuthnCredentialsRepository) ReadByID(id int64) (*entities.WebAuthnCredential, error) {
	return r.readWhere(squirrel.Eq{"id": id})
}

// ReadByCredentialID fetches by the raw authenticator-generated
// credential ID. This is the login lookup — the browser hands us this
// value in the assertion and we resolve it to a person.
func (r *WebAuthnCredentialsRepository) ReadByCredentialID(credentialID []byte) (*entities.WebAuthnCredential, error) {
	return r.readWhere(squirrel.Eq{"credential_id": credentialID})
}

func (r *WebAuthnCredentialsRepository) readWhere(pred any) (*entities.WebAuthnCredential, error) {
	query, args, err := r.sb.
		Select(webAuthnCredentialColumns...).
		From("webauthn_credentials").
		Where(pred).
		Limit(1).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("webauthn read build: %w", err)
	}

	cred, err := scanWebAuthnCredential(r.db.QueryRow(query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrWebAuthnCredentialNotFound
		}
		return nil, fmt.Errorf("webauthn read: %w", err)
	}

	return cred, nil
}

// Add inserts a freshly registered credential and returns the
// persisted row with its generated ID, UUID, and timestamps.
func (r *WebAuthnCredentialsRepository) Add(cred *entities.WebAuthnCredential) (*entities.WebAuthnCredential, error) {
	query, args, err := r.sb.
		Insert("webauthn_credentials").
		Columns(
			"user_id",
			"credential_id",
			"public_key",
			"attestation_type",
			"aaguid",
			"sign_count",
			"clone_warning",
			"transports",
			"backup_eligible",
			"backup_state",
			"nickname",
		).
		Values(
			cred.UserID,
			cred.CredentialID,
			cred.PublicKey,
			cred.AttestationType,
			cred.AAGUID,
			cred.SignCount,
			cred.CloneWarning,
			entities.StringSlice(cred.Transports),
			cred.BackupEligible,
			cred.BackupState,
			cred.Nickname,
		).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("webauthn add build: %w", err)
	}

	// MySQL has no RETURNING, so the row is read back by its
	// auto-increment id rather than returned by the INSERT itself.
	result, err := r.db.Exec(query, args...)
	if err != nil {
		return nil, fmt.Errorf("webauthn add: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("webauthn add id: %w", err)
	}

	return r.ReadByID(id)
}

// Edit updates the person-editable field (nickname) plus the
// authenticator state we refresh on each successful assertion.
//
// Deliberately does not touch credential_id or public_key: those are
// fixed at registration and a code path that could rewrite them is a
// code path that could silently swap one person's key for another's.
func (r *WebAuthnCredentialsRepository) Edit(cred *entities.WebAuthnCredential) (*entities.WebAuthnCredential, error) {
	query, args, err := r.sb.
		Update("webauthn_credentials").
		Set("nickname", cred.Nickname).
		Set("sign_count", cred.SignCount).
		Set("clone_warning", cred.CloneWarning).
		Set("backup_eligible", cred.BackupEligible).
		Set("backup_state", cred.BackupState).
		Set("last_used_at", cred.LastUsedAt).
		Set("updated_at", time.Now()).
		Where(squirrel.Eq{"user_id": cred.UserID, "credential_id": cred.CredentialID}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("webauthn edit build: %w", err)
	}

	// MySQL has no RETURNING, so the updated row is read back by the
	// same predicate the UPDATE matched on.
	//
	// A zero-row UPDATE is not distinguishable from a no-op update here
	// — MySQL reports 0 affected rows when the new values equal the old
	// ones — so absence is detected by the read-back rather than by
	// RowsAffected, which would report "not found" for an unchanged row.
	if _, err := r.db.Exec(query, args...); err != nil {
		return nil, fmt.Errorf("webauthn edit: %w", err)
	}

	persisted, err := r.readWhere(squirrel.Eq{
		"user_id":       cred.UserID,
		"credential_id": cred.CredentialID,
	})
	if err != nil {
		return nil, err
	}

	return persisted, nil
}

// TouchAfterLogin records a successful assertion: refreshed
// authenticator state plus last_used_at.
//
// Split out from Edit because the login path must not be able to
// rewrite the nickname — the assertion carries no nickname, so
// reusing Edit there would blank it on every sign-in.
func (r *WebAuthnCredentialsRepository) TouchAfterLogin(cred *entities.WebAuthnCredential) error {
	query, args, err := r.sb.
		Update("webauthn_credentials").
		Set("sign_count", cred.SignCount).
		Set("clone_warning", cred.CloneWarning).
		Set("backup_eligible", cred.BackupEligible).
		Set("backup_state", cred.BackupState).
		Set("last_used_at", time.Now()).
		Set("updated_at", time.Now()).
		Where(squirrel.Eq{"user_id": cred.UserID, "credential_id": cred.CredentialID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("webauthn touch build: %w", err)
	}

	if _, err := r.db.Exec(query, args...); err != nil {
		return fmt.Errorf("webauthn touch: %w", err)
	}

	return nil
}

// Delete removes a single credential.
func (r *WebAuthnCredentialsRepository) Delete(id int64) error {
	query, args, err := r.sb.
		Delete("webauthn_credentials").
		Where(squirrel.Eq{"id": id}).
		ToSql()
	if err != nil {
		return fmt.Errorf("webauthn delete build: %w", err)
	}

	if _, err := r.db.Exec(query, args...); err != nil {
		return fmt.Errorf("webauthn delete: %w", err)
	}

	return nil
}

// DeleteByPersonID removes every credential a person holds and reports
// how many rows went.
//
// Three callers, all of which need the count: the admin revoke-all
// endpoint (to tell the admin what happened and to write a truthful
// audit line), the password-reset path, and person deletion.
//
// There is no FK to people (that table lives behind PersonGRPC, not in
// this database), so nothing cleans these rows up automatically —
// person deletion has to call this explicitly. Left orphaned, a row is
// not a login risk (the credential ID is unguessable and the person
// lookup that follows would fail), but a recycled person_id would
// inherit a stranger's passkey.
//
// Deleting zero rows is not an error. An admin revoking passkeys from
// someone who has none should get "0 revoked", not a failure — the
// requested end state holds either way.
func (r *WebAuthnCredentialsRepository) DeleteByPersonID(userID int64) (int64, error) {
	query, args, err := r.sb.
		Delete("webauthn_credentials").
		Where(squirrel.Eq{"user_id": userID}).
		ToSql()
	if err != nil {
		return 0, fmt.Errorf("webauthn delete by person build: %w", err)
	}

	result, err := r.db.Exec(query, args...)
	if err != nil {
		return 0, fmt.Errorf("webauthn delete by person: %w", err)
	}

	// MySQL reports affected rows reliably, but the driver
	// contract allows RowsAffected to fail. The deletion already
	// succeeded at that point, so report it as such and let the
	// count be unknown rather than failing a completed revocation.
	affected, err := result.RowsAffected()
	if err != nil {
		log.Println("webauthn delete by person: rows affected unavailable:", err)
		return 0, nil
	}

	return affected, nil
}

// Count returns how many credentials a person has enrolled. Used to
// enforce the per-person cap.
//
// SELECT COUNT(*) rather than fetching rows and taking len — the cap
// check runs on every registration and has no use for the rows.
func (r *WebAuthnCredentialsRepository) Count(personID int64) (int, error) {
	var count int

	query, args, err := r.sb.
		Select("COUNT(*)").
		From("webauthn_credentials").
		Where(squirrel.Eq{"user_id": personID}).
		ToSql()
	if err != nil {
		return 0, fmt.Errorf("webauthn count build: %w", err)
	}

	if err := r.db.QueryRow(query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("webauthn count: %w", err)
	}

	return count, nil
}

// webAuthnRowScanner covers both *sql.Row and *sql.Rows so the single-row and
// multi-row paths share one scan implementation.
type webAuthnRowScanner interface {
	Scan(dest ...any) error
}

func scanWebAuthnCredential(s webAuthnRowScanner) (*entities.WebAuthnCredential, error) {
	var (
		cred       = entities.NewWebAuthnCredential()
		transports entities.StringSlice
		aaguid     []byte
		lastUsedAt sql.NullTime
	)

	err := s.Scan(
		&cred.ID,
		&cred.UserID,
		&cred.CredentialID,
		&cred.PublicKey,
		&cred.AttestationType,
		&aaguid,
		&cred.SignCount,
		&cred.CloneWarning,
		&transports,
		&cred.BackupEligible,
		&cred.BackupState,
		&cred.Nickname,
		&lastUsedAt,
		&cred.CreatedAt,
		&cred.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	cred.AAGUID = aaguid
	cred.Transports = []string(transports)
	if lastUsedAt.Valid {
		t := lastUsedAt.Time
		cred.LastUsedAt = &t
	}

	return cred, nil
}
