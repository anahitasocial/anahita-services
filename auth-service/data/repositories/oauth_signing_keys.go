package repositories

import (
	"context"
	"database/sql"
	"time"

	"auth/data/entities"

	sq "github.com/Masterminds/squirrel"
)

// NewOAuthSigningKeys returns a new instance of the oauth signing key repository.
func NewOAuthSigningKeys(db *sql.DB) OAuthSigningKeysInterface {
	return &oauthSigningKeys{
		db: db,
		sb: stmtBuilder,
	}
}

type oauthSigningKeys struct {
	db *sql.DB
	sb sq.StatementBuilderType
}

// Browse returns ALL signing keys (including inactive and expired) for
// the admin UI. Excludes private key material.
//
// Distinct from BrowseVerificationKeys, which filters to non-expired
// keys for JWKS publication. Admin operators need to see expired/
// deactivated rows for audit and cleanup decisions.
func (repo *oauthSigningKeys) Browse() ([]*entities.OAuthSigningKey, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := repo.getBaseSelectQuery().OrderBy("created_at DESC")

	q, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := repo.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*entities.OAuthSigningKey
	for rows.Next() {
		key := entities.NewOAuthSigningKey()
		err := rows.Scan(
			&key.ID,
			&key.KeyID,
			&key.Algorithm,
			&key.Backend,
			&key.PublicKey,
			&key.Active,
			&key.ExpiresAt,
			&key.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, key)
	}

	return results, nil
}

// BrowseVerificationKeys returns all non-expired signing keys for the JWKS
// endpoint (/.well-known/jwks.json). Includes inactive-but-unexpired keys
// so that existing tokens signed by a just-rotated-out key still verify.
// Does NOT include private keys.
func (repo *oauthSigningKeys) BrowseVerificationKeys() ([]*entities.OAuthSigningKey, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := repo.getBaseSelectQuery().
		Where(sq.Expr("expires_at > NOW()")).
		OrderBy("created_at DESC")

	q, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := repo.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*entities.OAuthSigningKey
	for rows.Next() {
		key := entities.NewOAuthSigningKey()
		err := rows.Scan(
			&key.ID,
			&key.KeyID,
			&key.Algorithm,
			&key.Backend,
			&key.PublicKey,
			&key.Active,
			&key.ExpiresAt,
			&key.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, key)
	}

	return results, nil
}

// Add inserts a new signing key into the database.
//
// Note: this method does not currently support nullable private_key
// (KMS-backed rows). KMS rows are inserted via SeedKMSSigningKey using
// raw SQL. Adding nullable support here is a follow-up cleanup.
func (repo *oauthSigningKeys) Add(key *entities.OAuthSigningKey) (*entities.OAuthSigningKey, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := repo.sb.Insert("oauth_signing_keys").Columns(
		"key_id",
		"algorithm",
		"backend",
		"public_key",
		"private_key",
		"active",
		"expires_at",
	).Values(
		key.KeyID,
		key.Algorithm,
		key.Backend,
		key.PublicKey,
		key.PrivateKey,
		key.Active,
		key.ExpiresAt,
	)

	id, err := insertReturningID(ctx, repo.db, query)
	if err != nil {
		return nil, err
	}

	key.ID = id

	return repo.ReadByID(key.ID)
}

// SetActive flips a signing key's active flag. Atomic guard against
// deactivating the last active row prevents the signer from ending up
// with no key to mint with.
//
// Returns ErrCannotDeactivateLastKey if the operation would leave zero
// active rows. Callers should translate this to a user-facing error
// (HTTP 409 Conflict is appropriate).
//
// Note: the guard counts ALL active rows, not just rows matching a
// specific algorithm. If you maintain multiple algorithms (RS256
// alongside ES256, say), this guard would prevent deactivating the
// last RS256 key as long as some ES256 key is still active — which is
// wrong for the RS256-only signer. Tighten when you actually run
// multiple algorithms.
func (repo *oauthSigningKeys) SetActive(id int64, active bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if !active {
		var otherActive int
		err = tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM oauth_signing_keys
			WHERE active = TRUE AND id != ?
		`, id).Scan(&otherActive)
		if err != nil {
			return err
		}
		if otherActive == 0 {
			return ErrCannotDeactivateLastKey
		}
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE oauth_signing_keys SET active = ? WHERE id = ?
	`, active, id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// Deactivate is preserved for backward compatibility with existing
// callers (rotation flow). New code should use SetActive(id, false)
// which goes through the same guard. Kept as a thin wrapper so we
// don't break any existing call sites in one PR.
func (repo *oauthSigningKeys) Deactivate(id int64) error {
	return repo.SetActive(id, false)
}

// Delete permanently removes a signing key. Prefer Deactivate + natural
// expiry cleanup over direct deletion — deleting a key that still has
// active tokens in the wild will cause those tokens to fail verification.
func (repo *oauthSigningKeys) Delete(id int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := repo.sb.Delete("oauth_signing_keys").
		Where(sq.Eq{"id": id})

	q, args, err := query.ToSql()
	if err != nil {
		return err
	}

	_, err = repo.db.ExecContext(ctx, q, args...)
	return err
}

// EditPrivateKey replaces the stored private key of one row, and nothing
// else. It exists for one job: sealing a key that was stored in the clear.
func (repo *oauthSigningKeys) EditPrivateKey(id int64, privateKey string) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	q, args, err := repo.sb.Update("oauth_signing_keys").
		Set("private_key", privateKey).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return err
	}

	_, err = repo.db.ExecContext(ctx, q, args...)
	return err
}

// DeleteExpired removes keys that are both inactive and past their
// expires_at. Intended for periodic cleanup. Returns rows deleted.
func (repo *oauthSigningKeys) DeleteExpired() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := repo.sb.Delete("oauth_signing_keys").
		Where(sq.Lt{"expires_at": time.Now()}).
		Where(sq.Eq{"active": false})

	q, args, err := query.ToSql()
	if err != nil {
		return 0, err
	}

	result, err := repo.db.ExecContext(ctx, q, args...)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}
