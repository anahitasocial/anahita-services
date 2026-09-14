package repositories

import (
	"regexp"
	"strings"
	"testing"

	"auth/requests"

	"github.com/DATA-DOG/go-sqlmock"
)

// Every read here selects FROM auth_accounts LEFT JOIN nodes, so a bare column
// name in a predicate is a coin toss: it resolves, or it is ambiguous, or it
// does not exist — and which one depends on a schema this file cannot see.
//
// That has now failed three separate ways in this refactor:
//
//	last_visit_date  landed on nodes while auth_accounts had it too, and every
//	                 joined read became "Column 'last_visit_date' in where
//	                 clause is ambiguous"
//	username         moved to nodes.alias, and sign-in became "Unknown column
//	                 'username' in 'where clause'" — for everyone
//	created_on       was about to go the same way through ORDER BY
//
// None of it is visible to the other tests in this package: sqlmock returns
// whatever rows it was handed and never parses the statement, so a query that
// MySQL would reject passes cleanly. The compiler sees strings. The only thing
// left that can catch it before a person cannot sign in is the statement text.
//
// So this asserts the rule rather than the schema: in these queries, a column
// reference is qualified. It says nothing about whether the column exists —
// that is what the running stack is for — but an unqualified name cannot be
// correct here even when it happens to work today, because the next column
// added to either table silently turns it ambiguous.

// bareColumnInPredicate finds identifiers in a WHERE or ORDER BY that carry no
// table qualifier. The negative lookbehind Go's regexp lacks is done by
// matching the dot explicitly and discarding those matches.
var (
	identifier = regexp.MustCompile(`[a-zA-Z_][a-zA-Z0-9_.]*`)

	// Words that are syntax rather than columns.
	sqlKeywords = map[string]bool{
		"and": true, "or": true, "not": true, "in": true, "is": true,
		"null": true, "like": true, "asc": true, "desc": true, "by": true,
		"where": true, "order": true, "limit": true, "offset": true,
		"lower": true, "coalesce": true, "group": true, "having": true,
		"between": true, "exists": true, "select": true, "from": true,
		"as": true, "on": true, "join": true, "left": true, "inner": true,
	}
)

// predicateOf returns everything from WHERE onward, which is where an
// unqualified name does its damage. The select list is exempt: it is checked
// by users_null_test.go, and its COALESCE wrappers already carry the table.
func predicateOf(statement string) string {
	upper := strings.ToUpper(statement)

	i := strings.Index(upper, "WHERE")
	if i < 0 {
		i = strings.Index(upper, "ORDER BY")
	}
	if i < 0 {
		return ""
	}

	return statement[i:]
}

func assertQualified(t *testing.T, method, statement string) {
	t.Helper()

	predicate := predicateOf(statement)
	if predicate == "" {
		t.Fatalf("%s produced no predicate to check:\n%s", method, statement)
	}

	for _, word := range identifier.FindAllString(predicate, -1) {
		if sqlKeywords[strings.ToLower(word)] || strings.Contains(word, ".") {
			continue
		}

		t.Errorf("%s narrows on a bare %q. This select joins auth_accounts to "+
			"nodes, so an unqualified column is ambiguous or missing depending "+
			"on a schema this test cannot see — and the failure lands on a "+
			"person trying to sign in, not here. Qualify it:\n%s",
			method, word, predicate)
	}
}

func TestReadPredicatesAreTableQualified(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(repo *Users) error
	}{
		{"ReadByID", func(r *Users) error { _, err := r.ReadByID(1); return err }},
		{"ReadByEmail", func(r *Users) error { _, err := r.ReadByEmail("a@b.c"); return err }},
		{"ReadByUsername", func(r *Users) error { _, err := r.ReadByUsername("ada"); return err }},
		{"ReadByEmailOrUsername", func(r *Users) error {
			_, err := r.ReadByEmailOrUsername("ada")
			return err
		}},
		// Browse with every filter and an ordering set at once, so one pass
		// covers all of them.
		{"Browse", func(r *Users) error {
			_, err := r.Browse(&requests.UserBrowseRequest{
				Limit:       10,
				Keywords:    "ada",
				PersonType:  "registered",
				PersonTypes: []string{"administrator", "super-administrator"},
				Ordering:    "created_on",
				Direction:   "desc",
			})
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, seen := newCapturingDB(t)
			repo := &Users{db: db}

			// The rows do not matter — only the statement does. A scan error
			// is fine and is ignored below.
			mock.ExpectQuery("select").WillReturnRows(sqlmock.NewRows([]string{"id"}))

			_ = tc.call(repo)

			if len(*seen) == 0 {
				t.Fatalf("%s ran no statement", tc.name)
			}
			assertQualified(t, tc.name, (*seen)[0])
		})
	}
}

// Browse's ordering is concatenated rather than bound, so the qualifier has to
// be added by Browse itself; safeOrderColumn only decides whether the value is
// a plain identifier. Pinned separately because the two are easy to confuse and
// a test passing on the WHERE clause alone would miss it.
func TestBrowseOrderingCarriesItsTable(t *testing.T) {
	db, mock, seen := newCapturingDB(t)
	repo := &Users{db: db}

	mock.ExpectQuery("select").WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, _ = repo.Browse(&requests.UserBrowseRequest{
		Limit: 10, Ordering: "person_type", Direction: "asc",
	})

	stmt := strings.ToLower(statementTouching(t, seen, "SELECT"))
	if !strings.Contains(stmt, "order by nodes.person_type asc") {
		t.Errorf("the ordering is not qualified; person_type lives on nodes "+
			"and a bare name fails the whole listing:\n%s", stmt)
	}
}

// The invite-accept notifier asks for administrators and super-administrators
// so it can tell them somebody joined. The field it sets was not read, so the
// filter never happened and the mail went to every account on the site.
func TestBrowseFiltersOnTheRequestedPersonTypes(t *testing.T) {
	db, mock, seen := newCapturingDB(t)
	repo := &Users{db: db}

	mock.ExpectQuery("select").WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, _ = repo.Browse(&requests.UserBrowseRequest{
		Limit:       10,
		PersonTypes: []string{"administrator", "super-administrator"},
	})

	stmt := strings.ToLower(statementTouching(t, seen, "SELECT"))
	if !strings.Contains(stmt, "person_type in (?,?)") {
		t.Errorf("PersonTypes was ignored, so a browse meant for two "+
			"administrators returns every account — and the caller is a "+
			"broadcast email:\n%s", stmt)
	}
}
