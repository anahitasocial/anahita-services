package repositories

import (
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// Making everything public registered-only, without a database: what the
// statements say, and in what order. public_access_integration_test.go runs
// the same thing against MySQL and checks what it does to real rows, but only
// where a test database is configured; this runs everywhere.

func TestMakeRegisteredCountsChangesAndRecords(t *testing.T) {
	db, mock, seen := newNodesTestDB(t)
	repo := &publicAccessRepo{db: db}

	mock.ExpectQuery("select").WillReturnRows(
		sqlmock.NewRows([]string{"people", "groups", "posts"}).AddRow(2, 1, 4))
	// One full chunk, so it has to ask again, then a short one.
	mock.ExpectExec("update").WillReturnResult(sqlmock.NewResult(0, publicAccessChunk))
	mock.ExpectExec("update").WillReturnResult(sqlmock.NewResult(0, 12))
	mock.ExpectExec("insert").WillReturnResult(sqlmock.NewResult(1, 1))

	counts, err := repo.MakeRegistered(99)
	if err != nil {
		t.Fatalf("make registered: %v", err)
	}
	if counts.People != 2 || counts.Groups != 1 || counts.Posts != 4 || counts.Total() != 7 {
		t.Errorf("counts: %+v", counts)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("not every statement ran: %v", err)
	}

	statements := *seen
	if len(statements) != 4 {
		t.Fatalf("ran %d statements, want count, two updates and the record", len(statements))
	}

	count, update, record := strings.ToLower(statements[0]), strings.ToLower(statements[1]), strings.ToLower(statements[3])

	// Hashtags and places are nobody's content, and are neither counted nor
	// changed.
	for name, statement := range map[string]string{"count": count, "update": update} {
		if !strings.Contains(statement, "type not like 'node.tag.%'") {
			t.Errorf("the %s does not leave tags out:\n%s", name, statement)
		}
		if !strings.Contains(statement, "access = ?") {
			t.Errorf("the %s is not limited to one access value:\n%s", name, statement)
		}
	}

	// In chunks, so no one statement locks every public row.
	if !strings.Contains(update, "limit ?") {
		t.Errorf("the update is not chunked:\n%s", update)
	}
	// Nobody edited the posts.
	if strings.Contains(update, "updated_at") {
		t.Errorf("the update moves updated_at:\n%s", update)
	}

	if !strings.Contains(record, "insert into site_actions") {
		t.Errorf("the action was not recorded:\n%s", record)
	}
}

// The record is of something that happened. If the change fails there is
// nothing to record.
func TestMakeRegisteredRecordsNothingWhenTheChangeFails(t *testing.T) {
	db, mock, seen := newNodesTestDB(t)
	repo := &publicAccessRepo{db: db}

	mock.ExpectQuery("select").WillReturnRows(
		sqlmock.NewRows([]string{"people", "groups", "posts"}).AddRow(2, 1, 4))
	mock.ExpectExec("update").WillReturnError(sqlmock.ErrCancelled)

	if _, err := repo.MakeRegistered(99); err == nil {
		t.Fatal("a failed change reported success")
	}

	for _, statement := range *seen {
		if strings.Contains(strings.ToLower(statement), "site_actions") {
			t.Error("a change that failed was recorded as done")
		}
	}
}
