package repositories

import (
	"context"
	"database/sql"
	"errors"

	"auth/data/entities"

	sq "github.com/Masterminds/squirrel"
)

// ErrCannotDeactivateLastKey is returned by SetActive when a deactivate
// would leave zero active rows for any algorithm — the signer would have
// nothing to mint with on its next ActiveKeyID call.
var ErrCannotDeactivateLastKey = errors.New("oauth_signing_keys: cannot deactivate the last active key")

// oauthSigningKeyPublicFields: columns safe to expose to any caller that
// needs JWKS-style info (no private key material). Includes backend so
// admin views can distinguish local vs kms-backed rows.
var oauthSigningKeyPublicFields = []string{
	"id",
	"key_id",
	"algorithm",
	"backend",
	"public_key",
	"active",
	"expires_at",
	"created_at",
}

// oauthSigningKeyFullFields: includes private_key. Only used internally
// by the token-signing service.
//
// private_key is nullable in the schema (KMS-backed rows have no local
// private key — the material lives in AWS KMS HSMs). getOneWithPrivate
// scans this column via sql.NullString to handle the NULL case; for
// KMS rows, the entity's PrivateKey field is left as the empty string.
var oauthSigningKeyFullFields = []string{
	"id",
	"key_id",
	"algorithm",
	"backend",
	"public_key",
	"private_key",
	"active",
	"expires_at",
	"created_at",
}

// getBaseSelectQuery returns the base select query for oauth signing keys
// without the private key material. Used for JWKS and the admin Browse
// endpoint.
func (repo *oauthSigningKeys) getBaseSelectQuery() sq.SelectBuilder {
	return repo.sb.Select(oauthSigningKeyPublicFields...).From("oauth_signing_keys")
}

// getBaseSelectQueryWithPrivate returns the base select query for oauth
// signing keys including the private key material.
func (repo *oauthSigningKeys) getBaseSelectQueryWithPrivate() sq.SelectBuilder {
	return repo.sb.Select(oauthSigningKeyFullFields...).From("oauth_signing_keys")
}

// getOneWithPrivate scans a single signing key including the private key.
//
// private_key is scanned via sql.NullString because the column is
// nullable for KMS-backed signing keys. SignerLocal — the only
// component that consumes PrivateKey for actual cryptographic
// operations — should never receive a kms-backend row in practice;
// if it does, parsing the empty PEM fails loudly with a clear error.
func (repo *oauthSigningKeys) getOneWithPrivate(query sq.SelectBuilder) (*entities.OAuthSigningKey, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	q, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	row := repo.db.QueryRowContext(ctx, q, args...)
	key := entities.NewOAuthSigningKey()

	var privateKeyNS sql.NullString

	err = row.Scan(
		&key.ID,
		&key.KeyID,
		&key.Algorithm,
		&key.Backend,
		&key.PublicKey,
		&privateKeyNS,
		&key.Active,
		&key.ExpiresAt,
		&key.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	key.PrivateKey = privateKeyNS.String
	return key, nil
}
