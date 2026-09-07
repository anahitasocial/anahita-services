package repositories

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"auth/data/entities"
	"auth/requests"

	"github.com/DATA-DOG/go-sqlmock"
)

// Most columns behind a user are nullable in the legacy schema, and the entity
// scans them into plain strings. A NULL therefore failed the scan, the read
// returned an error, and every caller read that as "no such user" — so someone
// with no avatar was told their password was wrong when it was right, and
// forgot-password quietly did nothing for them.
//
// 114 of 823 accounts in the development data were in that state.
//
// These tests are about the SQL: every nullable column must be COALESCEd, or
// scanned as nullable. They fail if a future edit drops one.

// columnsThatMayBeNull are nullable in the schema and scanned into a
// non-nullable Go type, so each needs COALESCE in the select.
var columnsThatMayBeNull = []string{
	"given_name",
	"family_name",
	"username",
	"email",
	"password",
	"usertype",
	"service",
	"filename",
	"totp_recovery_codes",
	"totp_secret",
}

func TestUserColumnsCoalesceEveryNullableColumn(t *testing.T) {
	for _, list := range []struct {
		name    string
		columns []string
		skip    map[string]bool
	}{
		{"userColumns", userColumns(), nil},
		// Browse selects no password and no TOTP state.
		{"userBrowseColumns", userBrowseColumns(), map[string]bool{
			"password": true, "totp_recovery_codes": true, "totp_secret": true,
		}},
	} {
		t.Run(list.name, func(t *testing.T) {
			joined := strings.Join(list.columns, " ")

			for _, column := range columnsThatMayBeNull {
				if list.skip[column] {
					continue
				}
				if !strings.Contains(joined, "COALESCE("+column+",") {
					t.Errorf("%s is nullable and scanned into a non-nullable "+
						"type, but is selected bare — a NULL fails the scan "+
						"and the account can neither sign in nor reset:\n  %s",
						column, joined)
				}
			}
		})
	}
}

// The timestamps take the other route: sql.NullTime in the scan, so they must
// NOT be COALESCEd to a fabricated date.
func TestTimestampsAreNotGivenAFakeDefault(t *testing.T) {
	joined := strings.Join(userColumns(), " ")

	for _, column := range []string{"last_visit_date", "created_on", "modified_on"} {
		if strings.Contains(joined, "COALESCE("+column+",") {
			t.Errorf("%s is COALESCEd to a real date; \"never\" would then "+
				"read as an actual visit. It is scanned as sql.NullTime "+
				"instead", column)
		}
	}
}

// The scan's own half. COALESCE is resolved by MySQL, so a row arriving here
// already carries "" for those columns — sqlmock hands back exactly what it is
// given and runs no SQL, so it cannot test the COALESCE itself; the column-list
// tests above do that, and the live database check is in
// users_null_integration_test.go.
//
// What IS tested here is the part Go owns: the three timestamps are scanned as
// sql.NullTime, and a NULL in any of them must not fail the read.
func TestReadByEmailSurvivesNullTimestamps(t *testing.T) {
	db, mock, _ := newCapturingDB(t)
	repo := &Users{db: db}

	// Every nullable column NULL: an invited account that never signed in,
	// never set a password and has no avatar.
	mock.ExpectQuery("select").WillReturnRows(
		sqlmock.NewRows([]string{
			"id", "type", "enabled", "given_name", "family_name", "username",
			"email", "password", "usertype", "service", "filename",
			"last_visit_date", "created_on", "modified_on", "totp_enabled",
			"totp_recovery_codes", "totp_last_verified_at", "totp_secret",
			"deleted_at",
		}).AddRow(
			7, "node.actor.person-service.person.v1", true, "", "", "",
			"", "", "", "", "",
			nil, nil, nil, false,
			"", time.Now(), "",
			nil,
		))

	user, err := repo.ReadByEmail("someone@example.com")
	if err != nil {
		t.Fatalf("a NULL timestamp failed the read, so this account can "+
			"neither sign in nor reset its password: %v", err)
	}
	if user.ID != 7 {
		t.Errorf("ID = %d, want 7", user.ID)
	}
	if !user.LastVisitDate.IsZero() {
		t.Errorf("a NULL last_visit_date became %v; it should stay the zero "+
			"time, which is what \"never signed in\" means", user.LastVisitDate)
	}
}

// One row with NULL timestamps must not take out the whole listing.
func TestBrowseSurvivesNullTimestamps(t *testing.T) {
	db, mock, _ := newCapturingDB(t)
	repo := &Users{db: db}

	columns := []string{
		"id", "type", "enabled", "given_name", "family_name", "username",
		"email", "usertype", "service", "filename",
		"last_visit_date", "created_on", "modified_on",
	}
	mock.ExpectQuery("select").WillReturnRows(
		sqlmock.NewRows(columns).
			AddRow(1, "node.actor.person-service.person.v1", true, "Ada", "Lovelace",
				"ada", "ada@example.com", "registered", "person-service", "a.png",
				nil, nil, nil).
			AddRow(2, "node.actor.person-service.person.v1", true, "", "", "",
				"", "", "", "", nil, nil, nil))

	users, err := repo.Browse(&requests.UserBrowseRequest{Limit: 10})
	if err != nil {
		t.Fatalf("one row with NULL timestamps aborted the listing: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("got %d users, want 2 — a NULL row was dropped", len(users))
	}
}

// A NULL password COALESCEs to "", and that empty hash must be refused rather
// than satisfied — an account created by invitation that never set one is not
// an account anybody may sign into.
func TestEmptyPasswordHashIsUnusable(t *testing.T) {
	db, mock, _ := newCapturingDB(t)
	repo := &Users{db: db}

	mock.ExpectQuery("select").WillReturnRows(
		sqlmock.NewRows([]string{
			"id", "type", "enabled", "given_name", "family_name", "username",
			"email", "password", "usertype", "service", "filename",
			"last_visit_date", "created_on", "modified_on", "totp_enabled",
			"totp_recovery_codes", "totp_last_verified_at", "totp_secret",
			"deleted_at",
		}).AddRow(
			7, "node.actor.person-service.person.v1", true, "", "", "invited",
			"invited@example.com", "", "registered", "person-service", "",
			nil, nil, nil, false, "", time.Now(), "", nil,
		))

	user, err := repo.ReadByEmail("invited@example.com")
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if user.Password != "" {
		t.Errorf("Password = %q, want empty", user.Password)
	}
	if user.CheckPassword("") {
		t.Error("an empty stored hash was satisfied by an empty password")
	}
	if user.CheckPassword("anything") {
		t.Error("an empty stored hash was satisfied")
	}
}

var _ = sql.ErrNoRows

// Signup and invite acceptance both hand Add a user built by NewUser, which
// leaves the timestamps at the zero time. The INSERT names created_on and
// modified_on, and MySQL runs NO_ZERO_DATE with STRICT_TRANS_TABLES — so every
// registration failed with "Incorrect datetime value: '0000-00-00'" and came
// back as a 500. Nobody could create an account by any route.
func TestAddFillsTheTimestampsTheCallersDoNotSet(t *testing.T) {
	db, mock, _ := newCapturingDB(t)
	repo := &Users{db: db}

	mock.ExpectBegin()
	mock.ExpectExec("insert").WillReturnResult(sqlmock.NewResult(7, 1))
	mock.ExpectExec("insert").WillReturnResult(sqlmock.NewResult(7, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("select").WillReturnRows(userRow())

	// Exactly what signup.go and invite_accept.go pass.
	user := entities.NewUser()
	user.Email = "ada@example.com"
	user.Username = "ada"

	before := time.Now().Add(-time.Second)
	_, _ = repo.Add(user)

	if user.CreationTime.IsZero() {
		t.Error("created_on is the zero time; MySQL rejects it as " +
			"'0000-00-00' and the registration fails with a 500")
	}
	if user.UpdateTime.IsZero() {
		t.Error("modified_on is the zero time")
	}
	if user.CreationTime.Before(before) {
		t.Errorf("created_on is %v, which is not now", user.CreationTime)
	}
}

// A caller with a deliberate value keeps it — a migration or a fixture
// restoring a real join date must not be overwritten with today.
func TestAddKeepsTimestampsACallerSet(t *testing.T) {
	db, mock, _ := newCapturingDB(t)
	repo := &Users{db: db}

	mock.ExpectBegin()
	mock.ExpectExec("insert").WillReturnResult(sqlmock.NewResult(7, 1))
	mock.ExpectExec("insert").WillReturnResult(sqlmock.NewResult(7, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("select").WillReturnRows(userRow())

	joined := time.Date(2011, 4, 1, 10, 0, 0, 0, time.UTC)
	user := entities.NewUser()
	user.Email = "ada@example.com"
	user.Username = "ada"
	user.CreationTime = joined

	_, _ = repo.Add(user)

	if !user.CreationTime.Equal(joined) {
		t.Errorf("created_on was overwritten with %v, losing the real join date",
			user.CreationTime)
	}
}
