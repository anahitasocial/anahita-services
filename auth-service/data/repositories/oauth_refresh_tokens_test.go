package repositories

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// newTestDB returns a fresh sqlmock and registers cleanup, so unmet
// expectations are reported against the test that set them.
func newTestDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("open sqlmock: %v", err)
	}
	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet sqlmock expectations: %v", err)
		}
		db.Close()
	})
	return db, mock
}

// -----------------------------------------------------------------------------
// Revocation blast radius
//
// These tests exist because reuse detection used to call RevokeByClientID,
// which filters on client_id alone. One replayed refresh token therefore
// signed out every user of that client on every device. The tests below pin
// which columns each revoke variant narrows on, so that regression cannot
// come back unnoticed.
// -----------------------------------------------------------------------------

func TestOAuthRefreshTokens_RevokeByUserAndClient(t *testing.T) {
	t.Run("narrows on both user_id and client_id", func(t *testing.T) {
		db, mock := newTestDB(t)
		repo := NewOAuthRefreshTokens(db)

		// WithArgs is the assertion that matters: user_id must be bound
		// into the statement. If someone reverts this to a client-only
		// predicate the arg list no longer matches and this fails.
		//
		// squirrel sorts sq.Eq map keys alphabetically, so the bind
		// order is client_id then user_id, after the SET value.
		mock.ExpectExec("UPDATE oauth_refresh_tokens SET revoked").
			WithArgs(true, "anahita-mobile", int64(7), false).
			WillReturnResult(sqlmock.NewResult(0, 3))

		if err := repo.RevokeByUserAndClient(7, "anahita-mobile"); err != nil {
			t.Fatalf("RevokeByUserAndClient: %v", err)
		}
	})

	t.Run("propagates exec error", func(t *testing.T) {
		db, mock := newTestDB(t)
		repo := NewOAuthRefreshTokens(db)

		dbErr := errors.New("connection refused")
		mock.ExpectExec("UPDATE oauth_refresh_tokens SET revoked").
			WillReturnError(dbErr)

		if err := repo.RevokeByUserAndClient(7, "anahita-mobile"); !errors.Is(err, dbErr) {
			t.Errorf("expected the exec error to propagate, got %v", err)
		}
	})
}

func TestOAuthRefreshTokens_RevokeByClientID(t *testing.T) {
	t.Run("does not narrow on user_id", func(t *testing.T) {
		// This is the deliberate large hammer, kept for client
		// deactivation. The test documents that it is client-wide on
		// purpose, so the contrast with RevokeByUserAndClient is
		// explicit rather than accidental.
		db, mock := newTestDB(t)
		repo := NewOAuthRefreshTokens(db)

		mock.ExpectExec("UPDATE oauth_refresh_tokens SET revoked").
			WithArgs(true, "anahita-mobile", false).
			WillReturnResult(sqlmock.NewResult(0, 50))

		if err := repo.RevokeByClientID("anahita-mobile"); err != nil {
			t.Fatalf("RevokeByClientID: %v", err)
		}
	})
}

// -----------------------------------------------------------------------------
// RevokeIfActive — the compare-and-set that serializes rotation
// -----------------------------------------------------------------------------

func TestOAuthRefreshTokens_RevokeIfActive(t *testing.T) {
	t.Run("reports true when it claimed the token", func(t *testing.T) {
		db, mock := newTestDB(t)
		repo := NewOAuthRefreshTokens(db)

		mock.ExpectExec("UPDATE oauth_refresh_tokens SET revoked").
			WithArgs(true, int64(42), false).
			WillReturnResult(sqlmock.NewResult(0, 1))

		claimed, err := repo.RevokeIfActive(42)
		if err != nil {
			t.Fatalf("RevokeIfActive: %v", err)
		}
		if !claimed {
			t.Error("one row updated means this caller won the race")
		}
	})

	t.Run("reports false when another caller claimed it first", func(t *testing.T) {
		// Zero rows affected means the revoked=false guard did not
		// match — a concurrent rotation already flipped it. The caller
		// must not mint a token pair on this path.
		db, mock := newTestDB(t)
		repo := NewOAuthRefreshTokens(db)

		mock.ExpectExec("UPDATE oauth_refresh_tokens SET revoked").
			WithArgs(true, int64(42), false).
			WillReturnResult(sqlmock.NewResult(0, 0))

		claimed, err := repo.RevokeIfActive(42)
		if err != nil {
			t.Fatalf("RevokeIfActive: %v", err)
		}
		if claimed {
			t.Error("zero rows updated means another caller got there first")
		}
	})

	t.Run("guards on revoked = false", func(t *testing.T) {
		// Without this predicate the update always reports one row and
		// the compare-and-set degrades into an unconditional revoke,
		// letting both racers mint a pair.
		db, mock := newTestDB(t)
		repo := NewOAuthRefreshTokens(db)

		mock.ExpectExec("UPDATE oauth_refresh_tokens SET revoked = .+ WHERE .+revoked").
			WithArgs(true, int64(42), false).
			WillReturnResult(sqlmock.NewResult(0, 1))

		if _, err := repo.RevokeIfActive(42); err != nil {
			t.Fatalf("RevokeIfActive: %v", err)
		}
	})

	t.Run("propagates exec error", func(t *testing.T) {
		db, mock := newTestDB(t)
		repo := NewOAuthRefreshTokens(db)

		dbErr := errors.New("connection refused")
		mock.ExpectExec("UPDATE oauth_refresh_tokens SET revoked").
			WillReturnError(dbErr)

		claimed, err := repo.RevokeIfActive(42)
		if !errors.Is(err, dbErr) {
			t.Errorf("expected the exec error to propagate, got %v", err)
		}
		if claimed {
			t.Error("must not report a claim when the update failed")
		}
	})
}

// -----------------------------------------------------------------------------
// revoked_at stamping
//
// Reuse detection tells a rotation race from a replay by how long ago the
// token was revoked, so a revoke path that forgets to stamp revoked_at
// silently disables the grace window for everything that goes through it —
// and the symptom is users being signed out of their other devices, far from
// the missing Set() call. Both paths are pinned here.
//
// The timestamp is SQL NOW() rather than a bind parameter so that every
// replica writes on the database's clock; that is why these assert on the
// statement text instead of on WithArgs.
// -----------------------------------------------------------------------------

func TestOAuthRefreshTokens_StampsRevokedAt(t *testing.T) {
	t.Run("revokeWhere stamps revoked_at", func(t *testing.T) {
		db, mock := newTestDB(t)
		repo := NewOAuthRefreshTokens(db)

		mock.ExpectExec(`SET revoked = \?, revoked_at = NOW\(\)`).
			WithArgs(true, "anahita-mobile", int64(7), false).
			WillReturnResult(sqlmock.NewResult(0, 2))

		if err := repo.RevokeByUserAndClient(7, "anahita-mobile"); err != nil {
			t.Fatalf("RevokeByUserAndClient: %v", err)
		}
	})

	t.Run("RevokeIfActive stamps revoked_at", func(t *testing.T) {
		db, mock := newTestDB(t)
		repo := NewOAuthRefreshTokens(db)

		mock.ExpectExec(`SET revoked = \?, revoked_at = NOW\(\)`).
			WithArgs(true, int64(42), false).
			WillReturnResult(sqlmock.NewResult(0, 1))

		claimed, err := repo.RevokeIfActive(42)
		if err != nil {
			t.Fatalf("RevokeIfActive: %v", err)
		}
		if !claimed {
			t.Error("expected the claim to succeed")
		}
	})

	t.Run("revokeWhere does not restamp an already-revoked row", func(t *testing.T) {
		// The revoked = false guard is what preserves the original
		// revocation time. Without it every replay attempt would push
		// revoked_at forward and the token would look freshly rotated
		// forever, so reuse detection could never fire.
		db, mock := newTestDB(t)
		repo := NewOAuthRefreshTokens(db)

		mock.ExpectExec(`WHERE .*revoked = \?`).
			WithArgs(true, "anahita-mobile", int64(7), false).
			WillReturnResult(sqlmock.NewResult(0, 0))

		if err := repo.RevokeByUserAndClient(7, "anahita-mobile"); err != nil {
			t.Fatalf("RevokeByUserAndClient: %v", err)
		}
	})
}

// -----------------------------------------------------------------------------
// revoked_at read path
// -----------------------------------------------------------------------------

func TestOAuthRefreshTokens_ReadsRevokedAt(t *testing.T) {
	// A NULL revoked_at must arrive as a nil pointer, not as a zero time:
	// RefreshTokenPair reads nil as "revoked before the column existed,
	// treat as long ago" and cascades, whereas a zero time would also
	// cascade but a stale non-nil value would not.
	t.Run("null revoked_at reads as nil", func(t *testing.T) {
		db, mock := newTestDB(t)
		repo := NewOAuthRefreshTokens(db)

		mock.ExpectQuery("SELECT .* FROM oauth_refresh_tokens").
			WithArgs("hash").
			WillReturnRows(sqlmock.NewRows(oauthRefreshTokenFields).
				AddRow(int64(1), "hash", int64(7), "anahita-mobile", "read", false, testTime, testTime, nil))

		token, err := repo.ReadByTokenHash("hash")
		if err != nil {
			t.Fatalf("ReadByTokenHash: %v", err)
		}
		if token.RevokedAt != nil {
			t.Errorf("expected a nil RevokedAt for a NULL column, got %v", *token.RevokedAt)
		}
	})

	t.Run("a set revoked_at is carried through", func(t *testing.T) {
		db, mock := newTestDB(t)
		repo := NewOAuthRefreshTokens(db)

		mock.ExpectQuery("SELECT .* FROM oauth_refresh_tokens").
			WithArgs("hash").
			WillReturnRows(sqlmock.NewRows(oauthRefreshTokenFields).
				AddRow(int64(1), "hash", int64(7), "anahita-mobile", "read", true, testTime, testTime, testTime))

		token, err := repo.ReadByTokenHash("hash")
		if err != nil {
			t.Fatalf("ReadByTokenHash: %v", err)
		}
		if token.RevokedAt == nil {
			t.Fatal("expected RevokedAt to be populated")
		}
		if !token.RevokedAt.Equal(testTime) {
			t.Errorf("RevokedAt = %v, want %v", *token.RevokedAt, testTime)
		}
	})

	// browseWhere declares its NullTime inside the row loop. Shared above
	// it, a row with a NULL revoked_at would silently inherit the previous
	// row's timestamp, because Scan leaves the value untouched when the
	// column is NULL.
	t.Run("a null row does not inherit the previous row's timestamp", func(t *testing.T) {
		db, mock := newTestDB(t)
		repo := NewOAuthRefreshTokens(db)

		mock.ExpectQuery("SELECT .* FROM oauth_refresh_tokens").
			WillReturnRows(sqlmock.NewRows(oauthRefreshTokenFields).
				AddRow(int64(1), "hash-1", int64(7), "anahita-mobile", "read", true, testTime, testTime, testTime).
				AddRow(int64(2), "hash-2", int64(7), "anahita-mobile", "read", false, testTime, testTime, nil))

		tokens, err := repo.BrowseByClientID("anahita-mobile")
		if err != nil {
			t.Fatalf("BrowseByClientID: %v", err)
		}
		if len(tokens) != 2 {
			t.Fatalf("got %d tokens, want 2", len(tokens))
		}
		if tokens[0].RevokedAt == nil {
			t.Error("the revoked row should carry its timestamp")
		}
		if tokens[1].RevokedAt != nil {
			t.Errorf("the active row must not inherit the previous row's timestamp, got %v", *tokens[1].RevokedAt)
		}
	})
}

// testTime is a fixed timestamp for rows returned by sqlmock.
var testTime = time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
