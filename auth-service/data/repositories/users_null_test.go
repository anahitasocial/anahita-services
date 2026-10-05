package repositories

import (
	"database/sql"
	"strings"
	"testing"
	"time"

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
// Qualified, because the reads join auth_accounts to nodes and several of
// these names exist on both sides.
var columnsThatMayBeNull = []string{
	"nodes.name",
	"nodes.alias",
	"nodes.person_type",
	"nodes.service",
	"nodes.filename",
	"auth_accounts.email",
	"auth_accounts.password",
	"auth_accounts.totp_recovery_codes",
	"auth_accounts.totp_secret",
	"auth_accounts.invited_by",
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
			"auth_accounts.password":            true,
			"auth_accounts.totp_recovery_codes": true,
			"auth_accounts.totp_secret":         true,
			"auth_accounts.invited_by":          true,
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

	for _, column := range []string{"last_visited_at", "created_at", "updated_at", "onboarded_at"} {
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
			"id", "type", "enabled", "name", "alias",
			"email", "password", "person_type", "service", "filename",
			"last_visited_at", "created_at", "updated_at", "totp_enabled",
			"totp_recovery_codes", "totp_last_verified_at", "totp_secret",
			"deleted_at", "archived_at",
			"email_verified_at", "onboarded_at", "has_bio", "invited_by",
			"tos_accepted", "tos_accepted_version",
			"pp_accepted", "pp_accepted_version",
			"language",
		}).AddRow(
			7, "node.actor.person-service.person.v1", true, "", "",
			"", "", "", "", "",
			nil, nil, nil, false,
			"", time.Now(), "",
			nil, nil,
			nil, nil, false, 0,
			false, "", false, "",
			"",
		))

	user, err := repo.ReadByEmail("someone@example.com")
	if err != nil {
		t.Fatalf("a NULL timestamp failed the read, so this account can "+
			"neither sign in nor reset its password: %v", err)
	}
	if user.ID != 7 {
		t.Errorf("ID = %d, want 7", user.ID)
	}
	if user.LastVisitedAt != nil {
		t.Errorf("a NULL last_visited_at became %v; it should stay nil, which "+
			"is what \"never signed in\" means", *user.LastVisitedAt)
	}
	if user.OnboardedAt != nil {
		t.Errorf("a NULL onboarded_at became %v; it should stay nil, which "+
			"is what sends a person with an empty profile to onboarding",
			*user.OnboardedAt)
	}
}

// One row with NULL timestamps must not take out the whole listing.
func TestBrowseSurvivesNullTimestamps(t *testing.T) {
	db, mock, _ := newCapturingDB(t)
	repo := &Users{db: db}

	columns := []string{
		"id", "type", "enabled", "name", "alias",
		"email", "person_type", "service", "filename",
		"last_visited_at", "created_at", "updated_at",
	}
	mock.ExpectQuery("select").WillReturnRows(
		sqlmock.NewRows(columns).
			AddRow(1, "node.actor.person-service.person.v1", true, "Ada Lovelace",
				"ada", "ada@example.com", "registered", "person-service", "a.png",
				nil, nil, nil).
			AddRow(2, "node.actor.person-service.person.v1", true, "", "",
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
			"id", "type", "enabled", "name", "alias",
			"email", "password", "person_type", "service", "filename",
			"last_visited_at", "created_at", "updated_at", "totp_enabled",
			"totp_recovery_codes", "totp_last_verified_at", "totp_secret",
			"deleted_at", "archived_at",
			"email_verified_at", "onboarded_at", "has_bio", "invited_by",
			"tos_accepted", "tos_accepted_version",
			"pp_accepted", "pp_accepted_version",
			"language",
		}).AddRow(
			7, "node.actor.person-service.person.v1", true, "", "invited",
			"invited@example.com", "", "registered", "person-service", "",
			nil, nil, nil, false, "", time.Now(), "", nil, nil,
			nil, nil, false, 0,
			false, "", false, "",
			"",
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

// has_bio is a comparison, and a comparison against NULL is NULL — which a
// bool cannot scan. The body is COALESCEd inside it for the same reason every
// string column above is: most people have no bio at all, and 716 of 822 in
// the live data would otherwise fail to sign in.
func TestHasBioCoalescesTheBody(t *testing.T) {
	joined := strings.Join(userColumns(), " ")

	if !strings.Contains(joined, "COALESCE(nodes.body, '')") {
		t.Errorf("has_bio compares nodes.body without COALESCE; a NULL body "+
			"yields NULL, which does not scan into a bool:\n  %s", joined)
	}
	for _, column := range userColumns() {
		if column == "nodes.body" || strings.HasSuffix(column, "AS body") {
			t.Errorf("the bio text itself is selected (%s); only whether "+
				"there is one is needed", column)
		}
	}
}
