package repositories

import (
	"errors"
	"strings"
	"testing"
	"time"

	"graph/requests"

	"github.com/DATA-DOG/go-sqlmock"
)

// BatchDeleteByFilters builds its WHERE clause out of optional filters, which
// means the interesting cases are the ones where filters are ABSENT. With none
// set it produced `DELETE FROM edges` — every follow, block, group-admin edge,
// subscription and vote in the system — and returned a row count with no error.
//
// The gRPC handler passes the request through unvalidated, so that was
// reachable from any service on the network, and reachable by accident from
// the internal callers any time a node lookup returned 0.

func newEdgesRepo(t *testing.T) (*edgesRepo, sqlmock.Sqlmock, *[]string) {
	t.Helper()

	var seen []string
	db, mock, err := sqlmock.New(
		sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(
			func(expected, actual string) error {
				seen = append(seen, actual)
				return nil
			},
		)),
	)
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	return &edgesRepo{client: db, dbTimeout: 3 * time.Second}, mock, &seen
}

// The catastrophic case: nothing set at all.
func TestBatchDeleteRefusesAnEmptyFilter(t *testing.T) {
	repo, _, seen := newEdgesRepo(t)

	count, err := repo.BatchDeleteByFilters(&requests.EdgeBatchDeleteByFiltersRequest{})

	if !errors.Is(err, ErrEdgeDeleteNotNarrowed) {
		t.Errorf("an empty filter returned %v; it must be refused — the statement "+
			"would be DELETE FROM edges with no WHERE clause", err)
	}
	if count != 0 {
		t.Errorf("reported %d deletions", count)
	}
	if len(*seen) != 0 {
		t.Errorf("a statement reached the database: %v", *seen)
	}
}

// The near-miss: a type but no node. This is what an internal caller produces
// when a node lookup returns 0 and the id clauses silently drop out — it would
// delete every like, or every subscription, fleet-wide.
func TestBatchDeleteRefusesATypeOnlyFilter(t *testing.T) {
	repo, _, seen := newEdgesRepo(t)

	count, err := repo.BatchDeleteByFilters(&requests.EdgeBatchDeleteByFiltersRequest{
		Types: []string{"edge.base.like.v1"},
	})

	if !errors.Is(err, ErrEdgeDeleteNotNarrowed) {
		t.Errorf("a type-only filter returned %v; it would remove every edge of "+
			"that type in the system", err)
	}
	if count != 0 || len(*seen) != 0 {
		t.Errorf("it reached the database: count=%d stmts=%v", count, *seen)
	}
}

// A node id on either side is enough to bound the statement, and that is how
// every real caller uses it.
func TestBatchDeleteAcceptsANodeScopedFilter(t *testing.T) {
	cases := []struct {
		name string
		req  *requests.EdgeBatchDeleteByFiltersRequest
	}{
		{
			"both ends, as subscriptions and likes send it",
			&requests.EdgeBatchDeleteByFiltersRequest{
				Types: []string{"edge.base.subscription.v1"}, NodeAID: 42, NodeBID: 99,
			},
		},
		{"source only", &requests.EdgeBatchDeleteByFiltersRequest{NodeAID: 42}},
		{"target only", &requests.EdgeBatchDeleteByFiltersRequest{NodeBID: 99}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, mock, seen := newEdgesRepo(t)
			mock.ExpectExec("delete").WillReturnResult(sqlmock.NewResult(0, 2))

			if _, err := repo.BatchDeleteByFilters(tc.req); err != nil {
				t.Fatalf("a properly scoped delete was refused: %v", err)
			}

			stmt := strings.ToLower((*seen)[0])
			if !strings.Contains(stmt, "where") {
				t.Errorf("the statement has no predicate:\n%s", stmt)
			}
			if !strings.Contains(stmt, "node_a_id") && !strings.Contains(stmt, "node_b_id") {
				t.Errorf("the statement is not bounded by a node:\n%s", stmt)
			}
		})
	}
}

// The filters that ARE supplied must all reach the statement — a filter
// silently dropped widens the delete rather than narrowing it, which is the
// dangerous direction.
func TestBatchDeleteAppliesEverySuppliedFilter(t *testing.T) {
	repo, mock, seen := newEdgesRepo(t)
	mock.ExpectExec("delete").WillReturnResult(sqlmock.NewResult(0, 1))

	_, err := repo.BatchDeleteByFilters(&requests.EdgeBatchDeleteByFiltersRequest{
		Types:     []string{"edge.base.follow.v1"},
		NodeAID:   42,
		NodeAType: "node.actor.person-service.person.v1",
		NodeBID:   99,
		NodeBType: "node.actor.group-service.group.v1",
	})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	stmt := strings.ToLower((*seen)[0])
	for _, column := range []string{"type", "node_a_id", "node_a_type", "node_b_id", "node_b_type"} {
		if !strings.Contains(stmt, column) {
			t.Errorf("%s was not applied; the delete is wider than requested:\n%s", column, stmt)
		}
	}
}
