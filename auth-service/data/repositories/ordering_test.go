package repositories

import (
	"strings"
	"testing"

	"auth/requests"

	"github.com/DATA-DOG/go-sqlmock"
)

// ORDER BY cannot be bound, so an ordering that reaches a query has been
// concatenated into it. UserBrowseRequest's oneof tag is the first line and
// the live endpoint does reject anything else; safeOrderColumn is the second,
// for a caller that never passed through validation.

func TestSafeOrderColumnRejectsAnythingButAColumn(t *testing.T) {
	payloads := []string{
		"id; DROP TABLE users--",
		"(SELECT SLEEP(5))",
		"1 UNION SELECT password FROM users",
		"id DESC-- ",
		"IF(1=1,SLEEP(5),0)",
		"id, (SELECT 1)",
		"id'",
		"created on",
	}

	for _, payload := range payloads {
		t.Run(payload, func(t *testing.T) {
			if got := safeOrderColumn(payload); got != "" {
				t.Errorf("safeOrderColumn(%q) = %q; anything but \"\" is "+
					"concatenated into a statement", payload, got)
			}
		})
	}
}

func TestSafeOrderColumnAllowsPlainColumns(t *testing.T) {
	for _, column := range []string{"id", "created_on", "usertype", "enabled", "u.created_on"} {
		if got := safeOrderColumn(column); got != column {
			t.Errorf("safeOrderColumn(%q) = %q; the allow-listed orderings "+
				"must pass or every browse loses its sort", column, got)
		}
	}
}

func TestBrowseDropsAnUnsafeOrdering(t *testing.T) {
	db, mock, seen := newCapturingDB(t)
	repo := &Users{db: db}

	mock.ExpectQuery("select").WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, _ = repo.Browse(&requests.UserBrowseRequest{
		Ordering:  "id; DROP TABLE users--",
		Direction: "asc",
		Limit:     10,
	})

	query := statementTouching(t, seen, "SELECT")
	if strings.Contains(query, "DROP TABLE") {
		t.Errorf("the ordering reached the statement:\n%s", query)
	}
}

// Two identical `if req.Ordering != ""` blocks stood here, one bare and one
// with a direction, so an ordered browse emitted
// "ORDER BY created_on, created_on DESC". Valid SQL, and the second term is
// dead — but it says the code was edited twice without either edit noticing
// the other.
func TestBrowseOrdersOnce(t *testing.T) {
	db, mock, seen := newCapturingDB(t)
	repo := &Users{db: db}

	mock.ExpectQuery("select").WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, _ = repo.Browse(&requests.UserBrowseRequest{
		Ordering:  "created_on",
		Direction: "desc",
		Limit:     10,
	})

	query := statementTouching(t, seen, "SELECT")
	orderBy := query[strings.Index(strings.ToUpper(query), "ORDER BY"):]

	if !strings.Contains(strings.ToLower(orderBy), "created_on desc") {
		t.Errorf("the ordering was dropped:\n%s", orderBy)
	}
	if strings.Count(strings.ToLower(orderBy), "created_on") != 1 {
		t.Errorf("created_on appears more than once — the duplicated "+
			"ORDER BY block is back:\n%s", orderBy)
	}
}
