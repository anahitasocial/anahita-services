package repositories

import (
	"context"
	"database/sql"
	"math"
	"time"

	sq "github.com/Masterminds/squirrel"
)

// dbTimeout is the timeout for the database connection
const dbTimeout = 3 * time.Second

// stmtBuilder is the Squirrel statement builder every repository in this
// package uses.
//
// MySQL takes ? placeholders. A builder configured with sq.Dollar emits
// $1/$2 instead, which MySQL rejects at execution time — there is no
// compile-time or build-time signal, so the failure only shows up when
// that particular query first runs in production. Defined once here so
// no constructor can quietly pick a different dialect.
var stmtBuilder = sq.StatementBuilder.PlaceholderFormat(sq.Question)

// maxRowLimit is MySQL's documented stand-in for "no row limit"
// (2^64-1). Needed because MySQL only accepts OFFSET as part of a LIMIT
// clause — a query that skips rows without capping them has to name
// some upper bound.
const maxRowLimit uint64 = math.MaxUint64

// insertReturningID runs an INSERT built by Squirrel and returns the
// auto-increment id MySQL assigned to the new row.
//
// MySQL has no RETURNING clause — these queries were written against
// Postgres, where `Suffix("RETURNING id")` plus QueryRow().Scan() was
// the idiom. LAST_INSERT_ID is per-connection and the driver reads it
// from the same round trip as the INSERT, so this is safe under
// concurrent writers.
func insertReturningID(ctx context.Context, db *sql.DB, query sq.InsertBuilder) (int64, error) {
	q, args, err := query.ToSql()
	if err != nil {
		return 0, err
	}

	result, err := db.ExecContext(ctx, q, args...)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

// db is the database connection
var db *sql.DB

// Repositories is the struct that contains all of the repositories
type Repositories struct {
	NodeDeletionLogs    NodeDeletionLogsInterface
	AuthLogs            AuthLogsInterface
	Tokens              TokensInterface
	OAuthClients        OAuthClientsInterface
	OAuthAuthorizations OAuthAuthorizationsInterface
	OAuthRefreshTokens  OAuthRefreshTokensInterface
	OAuthSigningKeys    OAuthSigningKeysInterface
	Users               UsersInterface
	WebAuthnCredentials WebAuthnCredentialsInterface
}

// NewRepositories returns a new instance of the repositories
func NewRepositories(dbPool *sql.DB) *Repositories {
	db = dbPool

	return &Repositories{
		NodeDeletionLogs:    NewNodeDeletionLogs(db),
		AuthLogs:            NewAuthLogs(db),
		Tokens:              NewTokens(db),
		OAuthClients:        NewOAuthClients(db),
		OAuthAuthorizations: NewOAuthAuthorizations(db),
		OAuthRefreshTokens:  NewOAuthRefreshTokens(db),
		OAuthSigningKeys:    NewOAuthSigningKeys(db),
		Users:               NewUsers(db),
		WebAuthnCredentials: NewWebAuthnCredentials(db),
	}
}
