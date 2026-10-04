package repositories

import (
	"strings"
	"testing"
	"time"

	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"

	"anahita-libs/constants"
	"graph/requests"

	"github.com/DATA-DOG/go-sqlmock"
)

// ORDER BY is the one part of these queries that is concatenated rather than
// bound, so it is the one part an attacker can write SQL into. The HTTP
// services reject anything but their allow-listed values, and a live probe
// confirmed that — but graph-grpc is reached over gRPC too, and nothing on
// that path re-validates. These tests hold the second line.

// Payloads that must never reach a statement. Each is a real shape rather than
// a random string: a stacked statement, a time-based probe, a UNION, a
// comment-terminated tail, and a subquery.
var orderingInjections = []string{
	"node.id; DROP TABLE nodes--",
	"(SELECT SLEEP(5))",
	"1 UNION SELECT password FROM users",
	"node.id DESC-- ",
	"IF(1=1,SLEEP(5),0)",
	"node.id, (SELECT 1)",
	"node.id`",
	"node.id'",
	"node.id ASC", // a direction is appended by the caller, not supplied
	"created on",  // whitespace at all
	"",
}

func TestSafeOrderColumnRejectsAnythingButAColumn(t *testing.T) {
	for _, payload := range orderingInjections {
		t.Run(payload, func(t *testing.T) {
			if got := safeOrderColumn(payload); got != "" {
				t.Errorf("safeOrderColumn(%q) = %q; anything but \"\" is "+
					"concatenated into a statement", payload, got)
			}
		})
	}
}

func TestSafeOrderColumnAllowsPlainColumns(t *testing.T) {
	for _, column := range []string{"created_at", "node.created_at", "id", "_x", "n1.col2"} {
		if got := safeOrderColumn(column); got != column {
			t.Errorf("safeOrderColumn(%q) = %q; a plain column must pass or "+
				"ordinary browses lose their sort", column, got)
		}
	}
}

// The unit test above can pass while the caller ignores the helper, so drive
// the real query and assert the payload is not in the SQL that would be sent.
func TestBrowseDropsAnUnsafeOrdering(t *testing.T) {
	const payload = "node.id; DROP TABLE nodes--"

	db, mock, seen := newNodesTestDB(t)
	repo := newRepo(db)

	mock.ExpectQuery("select").WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, _ = repo.Browse(&requests.NodeBrowseRequest{
		Ordering:    payload,
		OrderingDir: constants.DirAsc,
	})

	if len(*seen) == 0 {
		t.Fatal("no query was issued")
	}
	query := (*seen)[0]

	if strings.Contains(query, "DROP TABLE") {
		t.Errorf("the ordering reached the statement:\n%s", query)
	}
}

func TestBrowseKeepsASafeOrdering(t *testing.T) {
	db, mock, seen := newNodesTestDB(t)
	repo := newRepo(db)

	mock.ExpectQuery("select").WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, _ = repo.Browse(&requests.NodeBrowseRequest{
		Ordering:    "created_at",
		OrderingDir: constants.DirAsc,
	})

	query := strings.ToLower((*seen)[0])
	if !strings.Contains(query, "order by created_at asc") {
		t.Errorf("an allow-listed ordering was dropped; the guard is too "+
			"tight and every browse is now unsorted:\n%s", query)
	}
}

// FIELD() preserves the caller's order. The alias list used to be built by
// wrapping each value in single quotes and concatenating them.
func TestBatchReadByAliasesBindsAliases(t *testing.T) {
	const payload = "x') OR 1=1 -- "

	db, mock, seen := newNodesTestDB(t)
	repo := newRepo(db)

	mock.ExpectQuery("select").WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, _ = repo.BatchReadByAliases(&requests.NodeBatchReadByAliasesRequest{
		Aliases:  []string{"first", payload},
		NodeType: constants.NodeBaseHashtagEntityType,
	})

	if len(*seen) == 0 {
		t.Fatal("no query was issued")
	}
	query := (*seen)[0]

	if strings.Contains(query, payload) {
		t.Errorf("the alias was interpolated rather than bound:\n%s", query)
	}
	if !strings.Contains(query, "FIELD(node.alias, ?, ?)") {
		t.Errorf("expected a placeholder FIELD list, so ordering survives "+
			"without concatenation:\n%s", query)
	}
}

func TestBatchReadBindsIDs(t *testing.T) {
	db, mock, seen := newNodesTestDB(t)
	repo := newRepo(db)

	mock.ExpectQuery("select").WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, _ = repo.BatchRead(&requests.NodeBatchReadRequest{IDs: []int64{3, 1, 2}})

	query := (*seen)[0]
	if !strings.Contains(query, "FIELD(node.id, ?, ?, ?)") {
		t.Errorf("expected a placeholder FIELD list:\n%s", query)
	}
}

// An empty list must not emit FIELD() with nothing in it — that is a syntax
// error, and the old code would have produced one.
func TestBatchReadByAliasesWithNoAliasesEmitsNoFieldClause(t *testing.T) {
	db, mock, seen := newNodesTestDB(t)
	repo := newRepo(db)

	mock.ExpectQuery("select").WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, _ = repo.BatchReadByAliases(&requests.NodeBatchReadByAliasesRequest{
		Aliases:  []string{},
		NodeType: constants.NodeBaseHashtagEntityType,
	})

	if strings.Contains((*seen)[0], "FIELD(") {
		t.Errorf("empty FIELD() is a syntax error:\n%s", (*seen)[0])
	}
}

// ---------------------------------------------------------------------
// LIKE patterns
//
// The search term is bound, so this is pattern injection rather than SQL
// injection: an unstripped "%" is a second wildcard, and against node.body it
// is also a cheap way to buy a lot of database work with one request.

// Count and Browse build the same predicate. If only one sanitises, the total
// describes a different set than the rows and pagination reports a page count
// the results cannot fill.
func TestSearchCountAndBrowseSanitiseAlike(t *testing.T) {
	const wildcard = "%"

	for _, name := range []string{"count", "browse"} {
		t.Run(name, func(t *testing.T) {
			db, mock, seen := newNodesTestDB(t)
			repo := &SearchRepo{db: db, dbTimeout: 3 * time.Second}

			if name == "count" {
				mock.ExpectQuery("select").WillReturnRows(
					sqlmock.NewRows([]string{"count"}).AddRow(0))
				_, _ = repo.Count(&graphpb.SearchCountRequest{Keywords: wildcard},
					newTestViewer(1))
			} else {
				mock.ExpectQuery("select").WillReturnRows(sqlmock.NewRows([]string{"id"}))
				_, _ = repo.Browse(&graphpb.SearchBrowseRequest{Keywords: wildcard, Limit: 10},
					newTestViewer(1))
			}

			if len(*seen) == 0 {
				t.Fatal("no query was issued")
			}
			if strings.Contains(strings.ToLower((*seen)[0]), "node.name like") {
				t.Errorf("a bare wildcard produced a LIKE clause, so the "+
					"search matches every row:\n%s", (*seen)[0])
			}
		})
	}
}

func newTestViewer(id int64) *entities.Viewer {
	v := &entities.Viewer{}
	v.ID = id
	return v
}
