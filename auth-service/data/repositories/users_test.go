package repositories

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"auth/data/entities"

	"github.com/DATA-DOG/go-sqlmock"
)

// These pin the SHAPE of statements whose blast radius is larger than it
// looks: which columns they write, and which they narrow on.
//
// Same approach as oauth_refresh_tokens_test.go, which exists because reuse
// detection once revoked by client_id alone and signed out every user of that
// client on every device.
//
// Where the assertion is about the SQL text itself, the statement is captured
// and inspected rather than matched against a regexp — Go's regexp has no
// negative lookahead, and "this clause must NOT mention email" is exactly the
// shape that needs one.

// newCapturingDB returns a mock whose matcher accepts everything and records
// the SQL it was asked to run, so a test can assert on the statement directly.
func newCapturingDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, *[]string) {
	t.Helper()

	var seen []string

	db, mock, err := sqlmock.New(
		sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(
			func(expectedSQL, actualSQL string) error {
				seen = append(seen, actualSQL)
				return nil
			},
		)),
	)
	if err != nil {
		t.Fatalf("open sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	return db, mock, &seen
}

// statementTouching returns the first captured statement containing keyword.
func statementTouching(t *testing.T, seen *[]string, keyword string) string {
	t.Helper()

	for _, q := range *seen {
		if strings.Contains(strings.ToUpper(q), strings.ToUpper(keyword)) {
			return q
		}
	}
	t.Fatalf("no statement containing %q was executed; saw %d statements", keyword, len(*seen))
	return ""
}

// ---------------------------------------------------------------------
// Users.Edit must not carry identity columns
// ---------------------------------------------------------------------

// Edit writes the whole row from the struct it is handed. Email and username
// were taken out of its SET clause because leaving them was a live hazard, not
// a stylistic problem: any caller holding a User read before a concurrent
// change — a TOTP verify, a recovery-code burn — would silently write the old
// address back over the new one.
//
// Email is what password reset delivers to, so whoever can set it can take the
// account. Username is both a sign-in identifier and the profile slug.
func TestUsersEditDoesNotWriteIdentityColumns(t *testing.T) {
	db, mock, seen := newCapturingDB(t)
	repo := &Users{db: db}

	user := entities.NewUser()
	user.ID = 42
	user.Email = "attacker@example.com"
	user.Alias = "impostor"
	user.PersonType = "super-administrator"
	user.Password = "$2a$10$hash"
	user.UpdateTime = time.Now()

	mock.ExpectExec("update").WillReturnResult(sqlmock.NewResult(0, 1))
	// Edit re-reads the row it wrote.
	mock.ExpectQuery("select").WillReturnRows(userRow())

	if _, err := repo.Edit(user); err != nil {
		t.Fatalf("edit: %v", err)
	}

	update := statementTouching(t, seen, "UPDATE")
	setClause := update
	if i := strings.Index(strings.ToUpper(update), "WHERE"); i >= 0 {
		setClause = update[:i]
	}

	// usertype joined this list on 2026-09-07, for the same reason and with
	// the same shape of hazard. Edit runs while enrolling TOTP, verifying a
	// passcode and signing in; a privilege level that can be rewritten as a
	// side effect of any of those is one bad read away from being rewritten
	// wrongly — exactly the trap totp_secret fell into. Until a deliberate
	// promote/demote flow exists, nothing should change a usertype at all.
	for _, forbidden := range []string{"email", "username", "alias", "usertype"} {
		if strings.Contains(strings.ToLower(setClause), forbidden) {
			t.Errorf("Users.Edit writes %q. Identity and privilege columns move "+
				"only through their own single-purpose methods, which the "+
				"step-up-gated flows call:\n%s",
				forbidden, setClause)
		}
	}

	// And the values must not be bound either, which would mean the column
	// arrived by another route.
	for _, forbidden := range []string{"attacker@example.com", "impostor"} {
		if strings.Contains(update, forbidden) {
			t.Errorf("Users.Edit embedded %q in its statement", forbidden)
		}
	}
}

func userRow() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "type", "enabled", "name", "alias",
		"email", "password", "person_type", "service", "filename",
		"last_visit_date", "created_on", "modified_on", "totp_enabled",
		"totp_recovery_codes", "totp_last_verified_at", "totp_secret", "deleted_at",
		"archived_at",
		"tos_accepted", "tos_accepted_version",
		"pp_accepted", "pp_accepted_version",
	}).AddRow(
		42, "node.actor.person-service.person.v1", true, "Ada Lovelace", "ada",
		"ada@example.com", "$2a$10$hash", "registered", "person-service", "",
		time.Now(), time.Now(), time.Now(), false,
		"", time.Now(), "", nil,
		nil,
		true, "1.0", true, "1.0",
	)
}

// ---------------------------------------------------------------------
// Deletion leaves the tombstone
// ---------------------------------------------------------------------

// Deleting an account removes the auth_accounts row and DELIBERATELY leaves
// the nodes row behind. That row is the tombstone: it reserves the handle so
// nobody else can take it, which is what both federation protocols need — a
// recycled handle inherits the previous person's followers on every instance
// that cached them.
func TestUsersDeleteLeavesTheNodeRow(t *testing.T) {
	db, mock, seen := newCapturingDB(t)
	repo := &Users{db: db}

	mock.ExpectExec("delete").WithArgs(int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.Delete(42); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if len(*seen) != 1 {
		t.Fatalf("expected exactly one statement, got %d: %v", len(*seen), *seen)
	}

	stmt := strings.ToLower((*seen)[0])
	if !strings.Contains(stmt, "delete from auth_accounts") {
		t.Errorf("expected a delete against auth_accounts, got:\n%s", stmt)
	}
	// The node row is the tombstone. A DELETE reaching it frees the handle.
	if strings.Contains(stmt, "delete from nodes") {
		t.Errorf("the tombstone row was deleted, freeing the handle:\n%s", stmt)
	}
}

// ---------------------------------------------------------------------
// Identity lookups are case-insensitive
// ---------------------------------------------------------------------

// Uniqueness is checked with LOWER on BOTH sides rather than trusting the
// column collation. A deployment whose collation is case-sensitive would
// otherwise let Ada@example.com and ada@example.com both register — two
// accounts the same password-reset link reaches.
func TestIdentityUniquenessIgnoresCase(t *testing.T) {
	t.Run("email", func(t *testing.T) {
		db, mock, seen := newCapturingDB(t)
		repo := &Users{db: db}

		mock.ExpectQuery("select").WithArgs("Ada@Example.com").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		taken, err := repo.IsEmail("Ada@Example.com")
		if err != nil {
			t.Fatalf("is email: %v", err)
		}
		if !taken {
			t.Error("a differently-cased address was reported available")
		}

		stmt := strings.ToLower(statementTouching(t, seen, "SELECT"))
		if !strings.Contains(stmt, "lower(email) = lower(?)") {
			t.Errorf("the comparison does not fold case on both sides:\n%s", stmt)
		}
	})

	t.Run("username", func(t *testing.T) {
		db, mock, seen := newCapturingDB(t)
		repo := &Users{db: db}

		mock.ExpectQuery("select").WithArgs("AdaLovelace").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		taken, err := repo.IsUsername("AdaLovelace")
		if err != nil {
			t.Fatalf("is username: %v", err)
		}
		if !taken {
			t.Error("a differently-cased handle was reported available")
		}

		stmt := strings.ToLower(statementTouching(t, seen, "SELECT"))
		if !strings.Contains(stmt, "lower(alias) = lower(?)") {
			t.Errorf("the comparison does not fold case on both sides:\n%s", stmt)
		}

		// The handle namespace is every actor, not every person: WebFinger
		// resolves one name to one actor, so a group holding the handle
		// takes it from a person too. Narrowing this to person nodes would
		// hand out a handle the unique index then refuses, turning a 409
		// into a 500.
		if !strings.Contains(stmt, "node.actor.%") {
			t.Errorf("the check is not scoped to the actor namespace:\n%s", stmt)
		}
		if strings.Contains(stmt, "deleted_at") {
			t.Errorf("tombstoned rows are excluded, so a deleted person's "+
				"handle can be re-issued and inherit their followers:\n%s", stmt)
		}
	})
}

func TestIdentityLookupsReportAvailability(t *testing.T) {
	db, mock, _ := newCapturingDB(t)
	repo := &Users{db: db}

	mock.ExpectQuery("select").WithArgs("free@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	taken, err := repo.IsEmail("free@example.com")
	if err != nil {
		t.Fatalf("is email: %v", err)
	}
	if taken {
		t.Error("an unused address was reported taken")
	}
}

// ---------------------------------------------------------------------
// Single-purpose identity writes
// ---------------------------------------------------------------------

// Both narrow on the person's id. Without it in the predicate the statement
// rewrites every row in the table.
func TestEditEmailNarrowsOnTheOwner(t *testing.T) {
	db, mock, seen := newCapturingDB(t)
	repo := &Users{db: db}

	mock.ExpectExec("update").WithArgs("new@example.com", int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.EditEmail("new@example.com", 42); err != nil {
		t.Fatalf("edit email: %v", err)
	}

	stmt := strings.ToLower(statementTouching(t, seen, "UPDATE"))
	if !strings.Contains(stmt, "where") {
		t.Errorf("EditEmail has no predicate; it would rewrite every row:\n%s", stmt)
	}
}

// The EditUsername test is gone with the method. The handle lives only on
// nodes.alias now — auth_accounts has no username column — so there is nothing
// on this side to assert about it. username_change.go moves the alias through
// ActorsGRPC, and the graph's own tests cover the namespace it enforces.

// ---------------------------------------------------------------------
// Restore
// ---------------------------------------------------------------------

// The Restore test is gone with the method. Restoring an account is
// ActorService.Restore now: the graph writes the node_deletion_logs entry and
// publishes the event, neither of which an UPDATE from this side did. What is
// worth asserting about it belongs to graph-grpc-service, which has its own
// lifecycle tests.
